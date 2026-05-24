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
package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// OptionBatchChange describes a single key/value update considered by
// UpdateOptionsBatchAtomic. After the call returns successfully, the
// PreviousValue field is populated for every applied change so the caller can
// build audit metadata (masking sensitive values, etc.).
type OptionBatchChange struct {
	Key           string
	Value         string
	PreviousValue string
}

// OptionBatchAuditMaskFn maps applied changes to per-change audit metadata.
// Implementations are expected to mask sensitive values (secrets, tokens)
// before the result hits the persistent audit log.
type OptionBatchAuditMaskFn func(applied []OptionBatchChange) []map[string]string

// UpdateOptionsBatchAtomic persists multiple option updates inside one
// transaction and writes a single LogTypeManage audit row that captures the
// diff. The returned slice contains only those entries whose value actually
// changed; identical values are skipped silently. OptionMap (in-process cache)
// and the per-key side-effect handlers are refreshed after commit.
//
// The function is generic across configuration domains; callers pass the log
// `content` summary and a mask callback so the audit row stays human readable
// while sensitive values never reach storage in plaintext.
func UpdateOptionsBatchAtomic(
	updates []OptionBatchChange,
	adminID int,
	adminUsername string,
	logContent string,
	maskFn OptionBatchAuditMaskFn,
) ([]OptionBatchChange, error) {
	if len(updates) == 0 {
		return nil, nil
	}

	var applied []OptionBatchChange
	err := DB.Transaction(func(tx *gorm.DB) error {
		applied = applied[:0]
		for _, upd := range updates {
			var existing Option
			err := tx.Where(&Option{Key: upd.Key}).First(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			previous := existing.Value
			if previous == upd.Value {
				continue
			}
			existing.Key = upd.Key
			existing.Value = upd.Value
			// Save acts as upsert on the primary key.
			if err := tx.Save(&existing).Error; err != nil {
				return err
			}
			applied = append(applied, OptionBatchChange{
				Key:           upd.Key,
				Value:         upd.Value,
				PreviousValue: previous,
			})
		}
		if len(applied) == 0 {
			return nil
		}
		changes := maskFn(applied)
		other := map[string]interface{}{
			"admin_info": map[string]interface{}{
				"admin_id":       adminID,
				"admin_username": adminUsername,
				"changes":        changes,
			},
		}
		return tx.Create(&Log{
			UserId:    adminID,
			Username:  adminUsername,
			CreatedAt: common.GetTimestamp(),
			Type:      LogTypeManage,
			Content:   logContent,
			Other:     common.MapToJsonStr(other),
		}).Error
	})
	if err != nil {
		return nil, err
	}

	// Mirror committed values into the in-process OptionMap so subsequent
	// reads see the new state without waiting for the SyncOptions tick.
	for _, change := range applied {
		if err := updateOptionMap(change.Key, change.Value); err != nil {
			common.SysLog("failed to update option map for batch key " +
				change.Key + ": " + err.Error())
		}
	}
	return applied, nil
}
