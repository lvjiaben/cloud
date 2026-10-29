// Completion resumes only durable human-approved deletion intents and preserves approval evidence across failures.

package privacygovernance

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// completeConfirmedDeletion retries idempotent provider removal under the same target-before-task locks.
func (s *serviceImpl) completeConfirmedDeletion(ctx context.Context, taskNo string, executionActor int) error {
	return dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		locked, err := s.lockDeletionTask(ctx, taskNo)
		if err != nil {
			return err
		}
		row := *locked
		if row.StatusCode == "completed" {
			return nil
		}
		if row.StatusCode != deletionStatusConfirmed || row.ConfirmedAt == nil || row.ConfirmedByUserID <= 0 {
			return bizerr.NewCode(CodeConflict)
		}
		intent, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'confirm_deletion' AND target_no = ?)`, idccap.DefaultTenantID, taskNo)
		if err != nil {
			return err
		}
		if !intent.Bool() {
			return bizerr.NewCode(CodeUnavailable)
		}
		path, handleRef, hold, err := s.loadDeletionTarget(ctx, row)
		if err != nil {
			return err
		}
		if hold != "" {
			return bizerr.NewCode(CodeConflict)
		}
		if err = s.deletePrivateOriginal(ctx, path, handleRef); err != nil {
			return err
		}
		now := s.now().UTC().Truncate(time.Microsecond)
		if err = s.markDeletionTarget(ctx, row, now); err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code = 'completed', completed_at = ?, version = version + 1, updated_at = ? WHERE tenant_id = ? AND task_no = ? AND status_code = 'confirmed' AND version = ?`, now, now, idccap.DefaultTenantID, taskNo, row.Version)
		if err = requireSingleDeletionUpdate(result, err); err != nil {
			return err
		}
		digest := sha256Hex("privacy-deletion-complete\x00" + taskNo)
		actionNo := "PRA_" + digest[:32]
		fingerprint := sha256Hex("privacy-deletion-complete\x00" + taskNo + "\x00" + row.ConfirmedAt.UTC().Format(time.RFC3339Nano))
		if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'complete_deletion', ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, taskNo, digest, fingerprint, row.TargetNo, executionActor); err != nil {
			return err
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.deletion", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.deletion-confirm", TargetType: row.TargetTypeCode, TargetNo: row.TargetNo,
			ResultCode: "succeeded", ReasonCode: "retention_expired_original_deleted", RiskCode: "critical", RequiredPermission: PermissionConfigure,
			BeforeSnapshot: map[string]any{"status": deletionStatusConfirmed, "confirmed_by_user_id": row.ConfirmedByUserID},
			AfterSnapshot:  map[string]any{"status": "completed", "original_unavailable": true, "execution_actor_user_id": executionActor},
		})
	})
}
