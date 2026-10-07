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
		!oneOf(reason, "contract", "tax", "dispute", "audit", "legal_hold") ||
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
		if release {
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_retention_hold SET status_code = 'released', released_by_owner = ?, released_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND reason_code = ? AND status_code = 'active'`, owner, idccap.DefaultTenantID, targetType, targetNo, reason)
			if updateErr != nil {
				return updateErr
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
			if _, updateErr = s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code = 'pending', hold_reason_code = '', version = version + 1, updated_at = CURRENT_TIMESTAMP WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND status_code = 'blocked' AND hold_reason_code = ?`, idccap.DefaultTenantID, targetType, targetNo, reason); updateErr != nil {
				return updateErr
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
		actionNo := "PRA_" + sha256Hex(action + "\x00" + targetNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, action, targetNo, digest, fingerprint, reason, actor); insertErr != nil {
			return insertErr
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
			AfterSnapshot:      map[string]any{"hold_status": resultCode, "owner": owner},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return err
		}
		return bizerr.WrapCode(err, CodeUnavailable)
	}
	return nil
}

func validDeletionTargetType(value string) bool {
	return oneOf(value, "personal_verification_case", "enterprise_verification_material", "electronic_contract_file", "tax_invoice_file", "sensitive_access_audit", "local_sensitive_export")
}
