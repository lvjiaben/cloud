// Retention holds preserve evidence until the owning business reason is released.

package privacygovernance

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var holdOwnerPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{1,63}$`)

func (s *serviceImpl) PlaceRetentionHold(ctx context.Context, input RetentionHoldInput) error {
	return s.changeRetentionHold(ctx, input.TargetTypeCode, input.TargetNo, input.ReasonCode, input.OwnerCode, input.IdempotencyKey, false)
}

func (s *serviceImpl) ReleaseRetentionHold(ctx context.Context, input RetentionHoldReleaseInput) error {
	return s.changeRetentionHold(ctx, input.TargetTypeCode, input.TargetNo, input.ReasonCode, input.OwnerCode, input.IdempotencyKey, true)
}

func (s *serviceImpl) changeRetentionHold(ctx context.Context, targetType, targetNo, reason, owner, idempotencyKey string, release bool) error {
	targetType = strings.TrimSpace(targetType)
	targetNo = strings.TrimSpace(targetNo)
	reason = strings.TrimSpace(reason)
	owner = strings.TrimSpace(owner)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !validDeletionTargetType(targetType) || !businessNoPattern.MatchString(targetNo) ||
		!oneOf(reason, "tax", "dispute", "audit", "legal_hold") ||
		!holdOwnerPattern.MatchString(owner) || !idempotencyPattern.MatchString(idempotencyKey) {
		return bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionConfigure, targetNo)
	if err != nil {
		return err
	}
	action := "place_retention_hold"
	if release {
		action = "release_retention_hold"
	}
	digest := sha256Hex("privacy-hold-key\x00" + idempotencyKey)
	fingerprint := sha256Hex(fmt.Sprintf("privacy-hold-v1\x00%s\x00%s\x00%s\x00%s\x00%t", targetType, targetNo, reason, owner, release))
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		// The target exists even before the first scan creates a task. Lock it
		// first, then read current task state, so an empty old read cannot bypass approval.
		legalHold, lockErr := s.lockRetentionTarget(ctx, targetType, targetNo)
		if lockErr != nil {
			return lockErr
		}
		var tasks []deletionTaskRow
		if loadErr := s.database.GetScan(ctx, &tasks, `SELECT `+deletionTaskFields()+` FROM plugin_idc_compliance_privacy_deletion_task WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? FOR UPDATE`, idccap.DefaultTenantID, targetType, targetNo); loadErr != nil {
			return loadErr
		}
		var replay []struct {
			RequestFingerprint string `json:"request_fingerprint"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = ? AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, action, targetNo, digest); loadErr != nil {
			return loadErr
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if len(tasks) > 0 && (tasks[0].StatusCode == "completed" || tasks[0].StatusCode == deletionStatusConfirmed) {
			return bizerr.NewCode(CodeConflict)
		}
		if release {
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_retention_hold SET status_code = 'released', released_by_owner = ?, released_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND reason_code = ? AND status_code = 'active' AND created_by_owner = ?`, owner, idccap.DefaultTenantID, targetType, targetNo, reason, owner)
			if updateErr != nil {
				return updateErr
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		} else {
			holdNo, numberErr := newBusinessNo("PRH_")
			if numberErr != nil {
				return numberErr
			}
			if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_retention_hold (tenant_id, hold_no, target_type_code, target_no, reason_code, status_code, created_by_owner) VALUES (?, ?, ?, ?, ?, 'active', ?)`, idccap.DefaultTenantID, holdNo, targetType, targetNo, reason, owner); insertErr != nil {
				return insertErr
			}
		}
		if refreshErr := s.refreshDeletionHold(ctx, targetType, targetNo, legalHold); refreshErr != nil {
			return refreshErr
		}
		actionNo := "PRA_" + sha256Hex(action + "\x00" + targetNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, action, targetNo, digest, fingerprint, reason, actor); insertErr != nil {
			return insertErr
		}
		beforeStatus := "not_created"
		if release {
			beforeStatus = "active"
		}
		resultCode := "active"
		if release {
			resultCode = "released"
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope:   "compliance.privacy." + action,
			IdempotencyKey:     digest,
			OperationCode:      "compliance.privacy." + action,
			TargetType:         targetType,
			TargetNo:           targetNo,
			ResultCode:         "succeeded",
			ReasonCode:         reason,
			RiskCode:           "high",
			RequiredPermission: PermissionConfigure,
			BeforeSnapshot:     map[string]any{"hold_status": beforeStatus},
			AfterSnapshot:      map[string]any{"hold_status": resultCode, "owner": owner},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return err
		}
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}

// lockRetentionTarget rejects nonexistent or already disposed material before a hold mutation.
func (s *serviceImpl) lockRetentionTarget(ctx context.Context, targetType, targetNo string) (bool, error) {
	target, ok := retentionTargetColumns(targetType)
	if !ok {
		return false, bizerr.NewCode(CodeInvalid)
	}
	var rows []struct {
		StatusCode      string `json:"status_code"`
		SubjectTypeCode string `json:"subject_type_code"`
		LegalHold       bool   `json:"legal_hold"`
	}
	fields := "status_code, legal_hold, '' AS subject_type_code"
	if target[0] == "plugin_idc_compliance_verification_case" {
		fields = "status_code, legal_hold, subject_type_code"
	}
	if err := s.database.GetScan(ctx, &rows, `SELECT `+fields+` FROM `+target[0]+` WHERE tenant_id = ? AND `+target[1]+` = ? FOR UPDATE`, idccap.DefaultTenantID, targetNo); err != nil {
		return false, err
	}
	if len(rows) != 1 {
		return false, bizerr.NewCode(CodeNotAccessible)
	}
	if targetType == "personal_verification_case" && rows[0].SubjectTypeCode != "personal" || targetType == "enterprise_verification_case" && rows[0].SubjectTypeCode != "enterprise" {
		return false, bizerr.NewCode(CodeNotAccessible)
	}
	if rows[0].StatusCode == "deleted" || rows[0].StatusCode == "deletion_pending" {
		return false, bizerr.NewCode(CodeConflict)
	}
	return rows[0].LegalHold, nil
}

// refreshDeletionHold projects the highest still-effective protection instead of reopening a partially released task.
func (s *serviceImpl) refreshDeletionHold(ctx context.Context, targetType, targetNo string, legalHold bool) error {
	var reason string
	if legalHold {
		reason = "legal_hold"
	} else {
		value, err := s.database.GetValue(ctx, `SELECT reason_code FROM plugin_idc_compliance_privacy_retention_hold WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND status_code = 'active' ORDER BY CASE reason_code WHEN 'legal_hold' THEN 1 WHEN 'dispute' THEN 2 WHEN 'audit' THEN 3 WHEN 'tax' THEN 4 ELSE 5 END, id LIMIT 1`, idccap.DefaultTenantID, targetType, targetNo)
		if err != nil {
			return err
		}
		reason = value.String()
	}
	status := "pending"
	if reason != "" {
		status = "blocked"
	}
	_, err := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code = ?, hold_reason_code = ?, version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND status_code IN ('pending','blocked') AND (status_code <> ? OR hold_reason_code <> ?)`, status, reason, idccap.DefaultTenantID, targetType, targetNo, status, reason)
	return err
}

func validDeletionTargetType(value string) bool {
	return oneOf(value, "personal_verification_case", "enterprise_verification_case", "enterprise_verification_material", "tax_invoice_file")
}
