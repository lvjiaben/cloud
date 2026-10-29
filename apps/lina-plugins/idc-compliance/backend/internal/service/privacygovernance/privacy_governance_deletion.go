// Deletion persistence schedules due records and requires explicit confirmation before removal.

package privacygovernance

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type deletionTaskRow struct {
	TaskNo            string     `json:"task_no"`
	DataTypeCode      string     `json:"data_type_code"`
	TargetTypeCode    string     `json:"target_type_code"`
	TargetNo          string     `json:"target_no"`
	StatusCode        string     `json:"status_code"`
	DueAt             time.Time  `json:"due_at"`
	HoldReasonCode    string     `json:"hold_reason_code"`
	ConfirmedByUserID int        `json:"confirmed_by_user_id"`
	ConfirmedAt       *time.Time `json:"confirmed_at"`
	CompletedAt       *time.Time `json:"completed_at"`
	Version           int64      `json:"version"`
}

const deletionStatusConfirmed = "confirmed"

type dueDeletionCandidate struct {
	DataTypeCode   string    `json:"data_type_code"`
	TargetTypeCode string    `json:"target_type_code"`
	TargetNo       string    `json:"target_no"`
	DueAt          time.Time `json:"due_at"`
}

func (s *serviceImpl) GenerateDueDeletionTasks(ctx context.Context, limit int) (int, error) {
	if _, err := s.authorize(ctx, PermissionConfigure, "privacy-deletion-directory"); err != nil {
		return 0, err
	}
	return s.ScanDueDeletionTasks(ctx, limit)
}

func (s *serviceImpl) ScanDueDeletionTasks(ctx context.Context, limit int) (int, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return 0, bizerr.NewCode(CodeInvalid)
	}
	var rows []dueDeletionCandidate
	query := `SELECT candidates.data_type_code, candidates.target_type_code, candidates.target_no, candidates.due_at FROM (SELECT 'personal_verification_material' AS data_type_code, 'personal_verification_case' AS target_type_code, case_no AS target_no, retention_until AS due_at FROM plugin_idc_compliance_verification_case WHERE tenant_id = ? AND subject_type_code = 'personal' AND status_code IN ('verified','expired','rejected','canceled') AND retention_until <= CURRENT_TIMESTAMP AND NOT legal_hold
UNION ALL SELECT 'enterprise_verification_material', 'enterprise_verification_case', case_no, retention_until FROM plugin_idc_compliance_verification_case WHERE tenant_id = ? AND subject_type_code = 'enterprise' AND status_code IN ('verified','expired','rejected','canceled') AND retention_until <= CURRENT_TIMESTAMP AND NOT legal_hold AND legal_profile_snapshot_ref IS NOT NULL
UNION ALL SELECT 'enterprise_verification_material', 'enterprise_verification_material', material_no, retention_until FROM plugin_idc_compliance_enterprise_material WHERE tenant_id = ? AND retention_until <= CURRENT_TIMESTAMP AND NOT legal_hold AND status_code NOT IN ('deleted','deletion_pending')
UNION ALL SELECT 'tax_invoice_evidence', 'tax_invoice_file', file_no, retention_until FROM plugin_idc_compliance_tax_invoice_file WHERE tenant_id = ? AND retention_until <= CURRENT_TIMESTAMP AND NOT legal_hold AND status_code NOT IN ('deleted','deletion_pending')
) candidates WHERE NOT EXISTS (SELECT 1 FROM plugin_idc_compliance_privacy_deletion_task task WHERE task.tenant_id = ? AND task.target_type_code = candidates.target_type_code AND task.target_no = candidates.target_no)
ORDER BY due_at, target_no LIMIT ?`
	if err := s.database.GetScan(ctx, &rows, query, idccap.DefaultTenantID, idccap.DefaultTenantID, idccap.DefaultTenantID, idccap.DefaultTenantID, idccap.DefaultTenantID, limit); err != nil {
		return 0, bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	values := make([]string, 0, len(rows))
	arguments := make([]any, 0, len(rows)*6)
	for _, row := range rows {
		taskNo, err := newBusinessNo("PDT_")
		if err != nil {
			return 0, bizerr.WrapCode(err, CodeUnavailable)
		}
		values = append(values, "(?, ?, ?, ?, ?, 'pending', ?, 1)")
		arguments = append(arguments, idccap.DefaultTenantID, taskNo, row.DataTypeCode, row.TargetTypeCode, row.TargetNo, row.DueAt)
	}
	result, err := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_deletion_task (tenant_id, task_no, data_type_code, target_type_code, target_no, status_code, due_at, version) VALUES `+strings.Join(values, ", ")+` ON CONFLICT (tenant_id, target_type_code, target_no) DO NOTHING`, arguments...)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeUnavailable)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeUnavailable)
	}
	return int(affected), nil
}

// ListDeletionTasks applies one retained-target scope to both the count and bounded page.
func (s *serviceImpl) ListDeletionTasks(ctx context.Context, page, pageSize int) (*DeletionPage, error) {
	if _, err := s.authorize(ctx, PermissionQuery, "privacy-deletion-directory"); err != nil {
		return nil, err
	}
	if page < 1 || page > 100000 || !validDeletionPageSize(pageSize) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model("plugin_idc_compliance_privacy_deletion_task").Ctx(ctx).Where("tenant_id", idccap.DefaultTenantID).WhereIn("target_type_code", []string{"personal_verification_case", "enterprise_verification_case", "enterprise_verification_material", "tax_invoice_file"})
	total, err := model.Clone().Count()
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var rows []deletionTaskRow
	if err = model.Clone().Fields(deletionTaskFields()).OrderAsc("status_code").OrderAsc("due_at").OrderAsc("task_no").Page(page, pageSize).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &DeletionPage{Items: projectDeletionTasks(rows), Total: total}, nil
}

// validDeletionPageSize bounds memory and query work for the management grid.
func validDeletionPageSize(size int) bool {
	return size == 10 || size == 20 || size == 50 || size == 100
}

func (s *serviceImpl) ConfirmDeletion(ctx context.Context, input DeletionConfirmInput) (*DeletionTask, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.Confirmed || !businessNoPattern.MatchString(input.TaskNo) || input.ExpectedVersion <= 0 || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, PermissionConfigure, input.TaskNo)
	if err != nil {
		return nil, err
	}
	// An enclosing transaction could roll back the approval after an external
	// removal. Require this boundary to own both independent commits.
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if nilDependency(s.storage) || nilDependency(s.personalSnapshots) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	digest := sha256Hex("privacy-deletion-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex(fmt.Sprintf("privacy-deletion-v1\x00%s\x00%d\x00%t", input.TaskNo, input.ExpectedVersion, input.Confirmed))
	now := s.now().UTC().Truncate(time.Microsecond)
	blocked := false
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		locked, loadErr := s.lockDeletionTask(ctx, input.TaskNo)
		if loadErr != nil {
			return loadErr
		}
		row := *locked
		var replay []struct {
			RequestFingerprint string `json:"request_fingerprint"`
		}
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_fingerprint FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'confirm_deletion' AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, input.TaskNo, digest); loadErr != nil {
			return loadErr
		}
		if len(replay) == 1 {
			if replay[0].RequestFingerprint != fingerprint {
				return bizerr.NewCode(CodeConflict)
			}
		}
		if row.StatusCode == "completed" {
			return nil
		}
		if row.StatusCode == deletionStatusConfirmed {
			if row.Version != input.ExpectedVersion && len(replay) == 0 {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.StatusCode != "pending" || row.Version != input.ExpectedVersion {
			return bizerr.NewCode(CodeConflict)
		}
		_, _, hold, loadErr := s.loadDeletionTarget(ctx, row)
		if loadErr != nil {
			return loadErr
		}
		if hold != "" {
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code = 'blocked', hold_reason_code = ?, version = version + 1, updated_at = ? WHERE tenant_id = ? AND task_no = ? AND status_code = 'pending' AND version = ?`, hold, now, idccap.DefaultTenantID, row.TaskNo, row.Version)
			if checkErr := requireSingleDeletionUpdate(result, updateErr); checkErr != nil {
				return checkErr
			}
			blocked = true
			return nil
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_deletion_task SET status_code = 'confirmed', hold_reason_code = '', confirmed_by_user_id = ?, confirmed_at = ?, version = version + 1, updated_at = ? WHERE tenant_id = ? AND task_no = ? AND status_code = 'pending' AND version = ?`, actor, now, now, idccap.DefaultTenantID, row.TaskNo, row.Version)
		if checkErr := requireSingleDeletionUpdate(result, updateErr); checkErr != nil {
			return checkErr
		}
		actionNo := "PRA_" + sha256Hex("confirm-deletion\x00" + input.TaskNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'confirm_deletion', ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, input.TaskNo, digest, fingerprint, row.TargetNo, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.privacy.deletion-authorize", IdempotencyKey: sha256Hex("privacy-deletion-authorize\x00" + row.TaskNo), OperationCode: "compliance.privacy.deletion-authorize", TargetType: row.TargetTypeCode, TargetNo: row.TargetNo, ResultCode: "succeeded", ReasonCode: "retention_expired_deletion_confirmed", RiskCode: "critical", RequiredPermission: PermissionConfigure, BeforeSnapshot: map[string]any{"status": "pending"}, AfterSnapshot: map[string]any{"status": deletionStatusConfirmed, "confirmed_by_user_id": actor}})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if blocked {
		return nil, bizerr.NewCode(CodeConflict)
	}
	if err = s.completeConfirmedDeletion(ctx, input.TaskNo, actor); err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getDeletionTask(ctx, input.TaskNo)
}

// deletePrivateOriginal removes an object or revokes an encrypted snapshot handle.
func (s *serviceImpl) deletePrivateOriginal(ctx context.Context, path, handleRef string) error {
	if path != "" {
		if err := s.storage.Delete(ctx, storagecap.DeleteInput{Path: path}); err != nil {
			return err
		}
	}
	if handleRef == "" {
		return nil
	}
	handle, err := secretcap.NewHandle(handleRef)
	if err != nil {
		return err
	}
	metadata, err := s.personalSnapshots.Metadata(ctx, handle)
	if err != nil || !metadata.Configured {
		return err
	}
	result, err := s.personalSnapshots.Revoke(ctx, handle, metadata.UpdatedAt)
	if err != nil {
		return err
	}
	if result.Configured {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

// markDeletionTarget preserves only the safe receipt after the original is gone.
func (s *serviceImpl) markDeletionTarget(ctx context.Context, row deletionTaskRow, now time.Time) error {
	if row.TargetTypeCode == "personal_verification_case" || row.TargetTypeCode == "enterprise_verification_case" {
		return nil
	}
	targets := map[string][2]string{
		"enterprise_verification_material": {"plugin_idc_compliance_enterprise_material", "material_no"},
		"tax_invoice_file":                 {"plugin_idc_compliance_tax_invoice_file", "file_no"},
	}
	target, ok := targets[row.TargetTypeCode]
	if !ok {
		return bizerr.NewCode(CodeInvalid)
	}
	result, err := s.database.Exec(ctx, `UPDATE `+target[0]+` SET status_code = 'deleted', updated_at = ? WHERE tenant_id = ? AND `+target[1]+` = ? AND NOT legal_hold`, now, idccap.DefaultTenantID, row.TargetNo)
	return requireSingleDeletionUpdate(result, err)
}

// requireSingleDeletionUpdate rejects missing or concurrently changed targets.
func requireSingleDeletionUpdate(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return bizerr.NewCode(CodeConflict)
	}
	return nil
}

func (s *serviceImpl) loadDeletionTarget(ctx context.Context, task deletionTaskRow) (path, handleRef, hold string, err error) {
	var rows []struct {
		PrivateObjectRef string `json:"private_object_ref"`
		SnapshotRef      string `json:"snapshot_ref"`
		LegalHold        bool   `json:"legal_hold"`
		Eligible         bool   `json:"eligible"`
	}
	switch task.TargetTypeCode {
	case "enterprise_verification_case":
		err = s.database.GetScan(ctx, &rows, `SELECT COALESCE(legal_profile_snapshot_ref, '') AS private_object_ref, '' AS snapshot_ref, legal_hold, (subject_type_code = 'enterprise' AND status_code IN ('verified','expired','rejected','canceled') AND retention_until <= CURRENT_TIMESTAMP AND legal_profile_snapshot_ref IS NOT NULL) AS eligible FROM plugin_idc_compliance_verification_case WHERE tenant_id = ? AND case_no = ? FOR UPDATE`, idccap.DefaultTenantID, task.TargetNo)
	case "personal_verification_case":
		err = s.database.GetScan(ctx, &rows, `SELECT '' AS private_object_ref, COALESCE(legal_profile_snapshot_ref, '') AS snapshot_ref, legal_hold, (subject_type_code = 'personal' AND status_code IN ('verified','expired','rejected','canceled') AND retention_until <= CURRENT_TIMESTAMP) AS eligible FROM plugin_idc_compliance_verification_case WHERE tenant_id = ? AND case_no = ? FOR UPDATE`, idccap.DefaultTenantID, task.TargetNo)
	case "enterprise_verification_material":
		err = s.database.GetScan(ctx, &rows, `SELECT private_object_ref, '' AS snapshot_ref, legal_hold, (retention_until <= CURRENT_TIMESTAMP AND status_code NOT IN ('deleted','deletion_pending')) AS eligible FROM plugin_idc_compliance_enterprise_material WHERE tenant_id = ? AND material_no = ? FOR UPDATE`, idccap.DefaultTenantID, task.TargetNo)
	case "tax_invoice_file":
		err = s.database.GetScan(ctx, &rows, `SELECT private_object_ref, '' AS snapshot_ref, legal_hold, (retention_until <= CURRENT_TIMESTAMP AND status_code NOT IN ('deleted','deletion_pending')) AS eligible FROM plugin_idc_compliance_tax_invoice_file WHERE tenant_id = ? AND file_no = ? FOR UPDATE`, idccap.DefaultTenantID, task.TargetNo)
	default:
		return "", "", "", bizerr.NewCode(CodeInvalid)
	}
	if err != nil || len(rows) != 1 {
		return "", "", "", bizerr.NewCode(CodeUnavailable)
	}
	if rows[0].LegalHold {
		return "", "", "legal_hold", nil
	}
	if !rows[0].Eligible {
		return "", "", "", bizerr.NewCode(CodeConflict)
	}
	var holds []struct {
		ReasonCode string `json:"reason_code"`
	}
	if err = s.database.GetScan(ctx, &holds, `SELECT reason_code FROM plugin_idc_compliance_privacy_retention_hold WHERE tenant_id = ? AND target_type_code = ? AND target_no = ? AND status_code = 'active' ORDER BY CASE reason_code WHEN 'legal_hold' THEN 1 WHEN 'dispute' THEN 2 WHEN 'audit' THEN 3 WHEN 'tax' THEN 4 ELSE 5 END, id LIMIT 1`, idccap.DefaultTenantID, task.TargetTypeCode, task.TargetNo); err != nil {
		return "", "", "", bizerr.WrapCode(err, CodeUnavailable)
	}
	if len(holds) == 1 {
		return "", "", holds[0].ReasonCode, nil
	}
	return rows[0].PrivateObjectRef, rows[0].SnapshotRef, "", nil
}

func (s *serviceImpl) getDeletionTask(ctx context.Context, taskNo string) (*DeletionTask, error) {
	var rows []deletionTaskRow
	if err := s.database.GetScan(ctx, &rows, `SELECT `+deletionTaskFields()+` FROM plugin_idc_compliance_privacy_deletion_task WHERE tenant_id = ? AND task_no = ? LIMIT 1`, idccap.DefaultTenantID, taskNo); err != nil || len(rows) != 1 {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	items := projectDeletionTasks(rows)
	return &items[0], nil
}

func projectDeletionTasks(rows []deletionTaskRow) []DeletionTask {
	items := make([]DeletionTask, 0, len(rows))
	for _, row := range rows {
		items = append(items, DeletionTask{TaskNo: row.TaskNo, DataTypeCode: row.DataTypeCode, TargetTypeCode: row.TargetTypeCode, TargetNo: row.TargetNo, StatusCode: row.StatusCode, DueAt: row.DueAt.UTC(), HoldReasonCode: row.HoldReasonCode, ConfirmedByUserID: row.ConfirmedByUserID, ConfirmedAt: row.ConfirmedAt, CompletedAt: row.CompletedAt, Version: row.Version})
	}
	return items
}
func deletionTaskFields() string {
	return `task_no, data_type_code, target_type_code, target_no, status_code, due_at, hold_reason_code, confirmed_by_user_id, confirmed_at, completed_at, version`
}
