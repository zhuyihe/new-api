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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// BatchOptionUpdateRequest accepts an atomic group of option mutations. All
// entries either commit together or none of them does (see
// model.UpdateOptionsBatchAtomic). The audit log row produced by the
// transaction masks sensitive values according to maskSensitiveOptionValue.
type BatchOptionUpdateRequest struct {
	Updates []OptionUpdateRequest `json:"updates"`
}

type BatchOptionUpdateResponse struct {
	Success    bool     `json:"success"`
	Message    string   `json:"message,omitempty"`
	FailedKeys []string `json:"failed_keys,omitempty"`
}

// UpdateOptionsBatch validates and applies a list of option updates inside
// a single DB transaction. On success the audit log captures a per-key diff
// with sensitive values masked to ***<sha8>; on validation failure no writes
// occur and the offending key is reported.
func UpdateOptionsBatch(c *gin.Context) {
	var req BatchOptionUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, BatchOptionUpdateResponse{
			Success: false,
			Message: "invalid request body",
		})
		return
	}
	if len(req.Updates) == 0 {
		c.JSON(http.StatusOK, BatchOptionUpdateResponse{Success: true})
		return
	}

	normalized, err, failingKey := normalizeBatchOptionUpdates(req.Updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, BatchOptionUpdateResponse{
			Success:    false,
			Message:    err.Error(),
			FailedKeys: []string{failingKey},
		})
		return
	}

	adminIDValue, _ := c.Get("id")
	adminID, _ := adminIDValue.(int)
	adminUsername, _ := model.GetUsernameById(adminID, false)

	applied, err := model.UpdateOptionsBatchAtomic(
		normalized,
		adminID,
		adminUsername,
		"Atelier SSO config updated",
		buildOptionBatchAuditChanges,
	)
	if err != nil {
		// Log the raw cause server-side for diagnosis; return a generic
		// message client-side (PRD R4 / Q2: never surface raw exceptions).
		common.SysError(fmt.Sprintf(
			"option batch save transaction failed: %v", err,
		))
		c.JSON(http.StatusInternalServerError, BatchOptionUpdateResponse{
			Success: false,
			Message: "Failed to save settings (see server logs)",
		})
		return
	}

	_ = applied
	c.JSON(http.StatusOK, BatchOptionUpdateResponse{Success: true})
}

// normalizeBatchOptionUpdates coerces each request entry to a string value,
// then routes it through validateProductFlowOptionValue so the batch and the
// single-key UpdateOption endpoint share the same validation policy.
// Returns the failing key alongside the error when validation rejects an entry.
func normalizeBatchOptionUpdates(
	updates []OptionUpdateRequest,
) ([]model.OptionBatchChange, error, string) {
	normalized := make([]model.OptionBatchChange, 0, len(updates))
	for _, upd := range updates {
		valStr := coerceOptionValueToString(upd.Value)
		validated, err := validateProductFlowOptionValue(upd.Key, valStr)
		if err != nil {
			return nil, err, upd.Key
		}
		normalized = append(normalized, model.OptionBatchChange{
			Key:   upd.Key,
			Value: validated,
		})
	}
	return normalized, nil, ""
}

// coerceOptionValueToString mirrors the type-switch from UpdateOption so the
// batch endpoint accepts the same payload shapes (bool / number / string).
func coerceOptionValueToString(value any) string {
	switch v := value.(type) {
	case bool:
		return common.Interface2String(v)
	case float64:
		return common.Interface2String(v)
	case int:
		return common.Interface2String(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildOptionBatchAuditChanges turns applied changes into the audit JSON
// shape consumed by detail-dialog manage logs. Sensitive values are masked.
func buildOptionBatchAuditChanges(
	applied []model.OptionBatchChange,
) []map[string]string {
	out := make([]map[string]string, 0, len(applied))
	for _, change := range applied {
		out = append(out, map[string]string{
			"key":    change.Key,
			"before": maskSensitiveOptionValue(change.Key, change.PreviousValue),
			"after":  maskSensitiveOptionValue(change.Key, change.Value),
		})
	}
	return out
}

// maskSensitiveOptionValue replaces secret-like values with ***<sha8> so
// the audit log never persists plaintext keys/tokens/secrets. Empty values
// are represented explicitly so operators can tell an unset key apart from a
// rotated one.
func maskSensitiveOptionValue(key, value string) string {
	lower := strings.ToLower(key)
	sensitive := strings.HasSuffix(lower, "secret") ||
		strings.HasSuffix(lower, "key") ||
		strings.HasSuffix(lower, "token") ||
		strings.HasSuffix(lower, "api_key")
	if !sensitive {
		return value
	}
	if value == "" {
		return "(empty)"
	}
	sum := sha256.Sum256([]byte(value))
	return "***" + hex.EncodeToString(sum[:])[:8]
}
