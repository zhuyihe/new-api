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
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const productFlowOptionLastTestResult = "productflow_sso.last_test_result"

// ProductFlowSSOStatusResponse is the admin-facing status snapshot consumed
// by the system settings UI. It collapses the disabled/configured/redis trio
// into a single shape so the front-end status card can render without
// chaining additional requests.
type ProductFlowSSOStatusResponse struct {
	Enabled            bool                   `json:"enabled"`
	Configured         bool                   `json:"configured"`
	RedisEnabled       bool                   `json:"redis_enabled"`
	CallbackURLPreview string                 `json:"callback_url_preview"`
	LastTestResult     *productFlowTestResult `json:"last_test_result"`
}

// GetProductFlowSSOStatus returns the current SSO bridge status. Configured
// means BaseURL parses as an absolute URL AND SharedSecret is populated;
// Enabled mirrors the admin toggle stored in OptionMap. The URL-syntax check
// matches validateForStart so the dashboard cannot report "connected" while
// a real redirect would refuse the saved value.
func GetProductFlowSSOStatus(c *gin.Context) {
	cfg := getProductFlowSSOConfig()
	baseURLValid := isProductFlowBaseURLValid(cfg.BaseURL)
	configured := baseURLValid && cfg.SharedSecret != ""
	var callbackPreview string
	if baseURLValid {
		if callbackURL, err := buildProductFlowCallbackBaseURL(cfg.BaseURL); err == nil {
			callbackPreview = callbackURL.String()
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": ProductFlowSSOStatusResponse{
			Enabled:            cfg.Enabled,
			Configured:         configured,
			RedisEnabled:       common.RedisEnabled,
			CallbackURLPreview: callbackPreview,
			LastTestResult:     loadProductFlowLastTestResult(),
		},
	})
}

// loadProductFlowLastTestResult reads the persisted last test outcome from
// OptionMap. Returns nil for missing or malformed entries so the front-end
// can render the empty state cleanly.
func loadProductFlowLastTestResult() *productFlowTestResult {
	common.OptionMapRWMutex.RLock()
	raw, ok := common.OptionMap[productFlowOptionLastTestResult]
	common.OptionMapRWMutex.RUnlock()
	if !ok || raw == "" {
		return nil
	}
	var out productFlowTestResult
	if err := common.UnmarshalJsonStr(raw, &out); err != nil {
		return nil
	}
	return &out
}
