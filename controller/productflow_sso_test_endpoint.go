/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// productFlowTestRequest accepts an optional draft base URL so admins can
// validate a candidate value before saving (PRD R6). When omitted the saved
// configuration is exercised instead.
type productFlowTestRequest struct {
	BaseURL string `json:"base_url"`
}

// productFlowTestResult is persisted to OptionMap and returned to the status
// endpoint so the dashboard can render the last probe outcome.
type productFlowTestResult struct {
	OK            bool   `json:"ok"`
	Category      string `json:"category"`
	LatencyMs     int    `json:"latency_ms"`
	TestedAgainst string `json:"tested_against"`
	TestedAt      int64  `json:"tested_at"`
	Message       string `json:"message"`
}

// productFlowTestHTTPClient enforces the 3s timeout policy (PRD R6) for the
// connection probe; isolated as a package-level var so tests can swap it.
var productFlowTestHTTPClient = &http.Client{Timeout: 3 * time.Second}

// TestProductFlowSSOConnection probes ProductFlow's /api/health/sso endpoint
// and persists the classified outcome to OptionMap. The categorisation
// (connected / network_error / application_error / other) feeds the
// 4-tier UI hint required by PRD R15: `other` is the catch-all for
// boundary failures we cannot cleanly blame on either side (malformed URL,
// unparseable health body) so operators are not misled by a "looks like
// ProductFlow's fault" badge when the cause is local.
func TestProductFlowSSOConnection(c *gin.Context) {
	var req productFlowTestRequest
	_ = common.DecodeJson(c.Request.Body, &req)

	cfg := getProductFlowSSOConfig()
	baseURL := strings.TrimSpace(req.BaseURL)
	source := "draft"
	if baseURL == "" {
		baseURL = cfg.BaseURL
		source = "saved"
	}
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "no base_url to test",
		})
		return
	}

	result := performProductFlowHealthProbe(baseURL, source)
	persistProductFlowLastTestResult(result)
	common.SysLog(fmt.Sprintf(
		"Atelier SSO test connection: base_url=%s result=%s latency=%dms",
		baseURL, result.Category, result.LatencyMs,
	))
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// performProductFlowHealthProbe issues GET <baseURL>/api/health/sso and maps
// the response to one of three categories. Any timeout/network failure is
// reported via classifyProductFlowTransportError so the surfaced message
// never carries raw exception text.
func performProductFlowHealthProbe(baseURL, source string) productFlowTestResult {
	start := time.Now()
	fullURL := strings.TrimRight(baseURL, "/") + "/api/health/sso"
	now := time.Now().Unix()

	httpReq, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		// URL construction failure is not strictly ProductFlow's fault and
		// not a transport problem either; classify as `other` so the UI
		// surfaces "check your input" without misattributing blame.
		common.SysError(fmt.Sprintf(
			"Atelier SSO test build request failed: %v", err,
		))
		return productFlowTestResult{
			OK:            false,
			Category:      "other",
			LatencyMs:     0,
			TestedAgainst: source,
			TestedAt:      now,
			Message:       "Invalid Atelier base URL",
		}
	}

	resp, err := productFlowTestHTTPClient.Do(httpReq)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return productFlowTestResult{
			OK:            false,
			Category:      "network_error",
			LatencyMs:     latency,
			TestedAgainst: source,
			TestedAt:      now,
			Message:       classifyProductFlowTransportError(err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return productFlowTestResult{
			OK:            false,
			Category:      "application_error",
			LatencyMs:     latency,
			TestedAgainst: source,
			TestedAt:      now,
			Message:       fmt.Sprintf("HTTP %d from Atelier", resp.StatusCode),
		}
	}

	var body struct {
		OK          bool   `json:"ok"`
		Version     string `json:"version"`
		SupportsSSO bool   `json:"supports_sso"`
	}
	if err := common.DecodeJson(resp.Body, &body); err != nil {
		// A 200 body that fails to parse is almost always something between
		// us and ProductFlow (CDN HTML page, gzip mismatch, edge proxy) —
		// neither cleanly transport nor cleanly application. Use `other`.
		return productFlowTestResult{
			OK:            false,
			Category:      "other",
			LatencyMs:     latency,
			TestedAgainst: source,
			TestedAt:      now,
			Message:       "Atelier returned invalid health body",
		}
	}
	if !body.OK || !body.SupportsSSO {
		return productFlowTestResult{
			OK:            false,
			Category:      "application_error",
			LatencyMs:     latency,
			TestedAgainst: source,
			TestedAt:      now,
			Message:       "Atelier reports SSO not supported",
		}
	}

	version := strings.TrimSpace(body.Version)
	if version == "" {
		version = "unknown"
	}
	return productFlowTestResult{
		OK:            true,
		Category:      "connected",
		LatencyMs:     latency,
		TestedAgainst: source,
		TestedAt:      now,
		Message:       fmt.Sprintf("Atelier %s", version),
	}
}

// classifyProductFlowTransportError maps low-level transport failures to a
// small set of operator-friendly strings. The raw cause is logged to SysError
// for diagnosis (PRD R4); the return value is safe to surface to the client.
func classifyProductFlowTransportError(err error) string {
	common.SysError(fmt.Sprintf(
		"Atelier SSO test transport error: %v", err,
	))

	if errors.Is(err, context.DeadlineExceeded) {
		return "Connection timed out after 3s"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return "Connection timed out after 3s"
		}
		inner := strings.ToLower(urlErr.Err.Error())
		if strings.Contains(inner, "tls") || strings.Contains(inner, "certificate") {
			return "TLS certificate error"
		}
		if strings.Contains(inner, "no such host") {
			return "DNS lookup failed"
		}
		if strings.Contains(inner, "connection refused") {
			return "Connection refused"
		}
		return "Network error contacting Atelier"
	}
	return "Network error contacting Atelier"
}

// persistProductFlowLastTestResult stores the most recent probe outcome so
// the status endpoint can render the freshness/category badge after page
// reloads. Failures are logged but do not affect the API response.
func persistProductFlowLastTestResult(result productFlowTestResult) {
	encoded, err := common.Marshal(result)
	if err != nil {
		common.SysError(fmt.Sprintf(
			"Atelier SSO test result marshal failed: %v", err,
		))
		return
	}
	if err := model.UpdateOption(productFlowOptionLastTestResult, string(encoded)); err != nil {
		common.SysError(fmt.Sprintf(
			"Atelier SSO test result persist failed: %v", err,
		))
	}
}
