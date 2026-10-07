// Assessment persistence keeps draft and published privacy assessments versioned.

package privacygovernance

import (
	"context"
	"sort"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const assessmentDirectoryTarget = "privacy-assessment-directory"

type assessmentRow struct {
	ID                       int64      `json:"id"`
	AssessmentNo             string     `json:"assessment_no"`
	AssessmentTypeCode       string     `json:"assessment_type_code"`
	Version                  int64      `json:"version"`
	StatusCode               string     `json:"status_code"`
	Purpose                  string     `json:"purpose"`
	Necessity                string     `json:"necessity"`
	Impact                   string     `json:"impact"`
	RecipientDescription     string     `json:"recipient_description"`
	StorageLocation          string     `json:"storage_location"`
	RetentionDescription     string     `json:"retention_description"`
	RightsEntry              string     `json:"rights_entry"`
	MerchantRole             string     `json:"merchant_role"`
	ThirdPartyRole           string     `json:"third_party_role"`
	ProcessingInstruction    string     `json:"processing_instruction"`
	SubcontractingBoundary   string     `json:"subcontracting_boundary"`
	AuditEvidenceDescription string     `json:"audit_evidence_description"`
	CrossBorderTransfer      bool       `json:"cross_border_transfer"`
	ConfirmedByUserID        int        `json:"confirmed_by_user_id"`
	ConfirmedAt              *time.Time `json:"confirmed_at"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	RowVersion               int64      `json:"row_version"`
}

type aiFieldRow struct {
	AssessmentNo string `json:"assessment_no"`
	FieldCode    string `json:"field_code"`
}

func (s *serviceImpl) ListAssessments(ctx context.Context, assessmentType AssessmentType, limit int) (*AssessmentPage, error) {
	if _, err := s.authorize(ctx, PermissionQuery, assessmentDirectoryTarget); err != nil {
		return nil, err
	}
	if assessmentType != "" && !validAssessmentType(assessmentType) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if limit == 0 {
		limit = 60
	}
	if limit < 1 || limit > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := s.database.Model("plugin_idc_compliance_privacy_assessment").Ctx(ctx).
		Where("tenant_id", idccap.DefaultTenantID)
	if assessmentType != "" {
		model = model.Where("assessment_type_code", assessmentType)
	}
	var rows []assessmentRow
	if err := model.Fields(assessmentFields()).OrderDesc("version").OrderDesc("id").Limit(limit).Scan(&rows); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.projectAssessmentRows(ctx, rows)
}

func (s *serviceImpl) CurrentPublished(ctx context.Context, assessmentType AssessmentType) (*Assessment, error) {
	if !validAssessmentType(assessmentType) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var row *assessmentRow
	if err := s.database.Model("plugin_idc_compliance_privacy_assessment").Ctx(ctx).
		Fields(assessmentFields()).Where("tenant_id", idccap.DefaultTenantID).
		Where("assessment_type_code", assessmentType).Where("status_code", AssessmentPublished).
		Limit(1).Scan(&row); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodePolicyRequired)
	}
	page, err := s.projectAssessmentRows(ctx, []assessmentRow{*row})
	if err != nil || len(page.Items) != 1 {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &page.Items[0], nil
}

func (s *serviceImpl) SaveAssessmentDraft(ctx context.Context, input AssessmentDraftInput) (*Assessment, error) {
	input, err := normalizeDraft(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionConfigure, "privacy-assessment:"+string(input.Type))
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	var assessmentNo string
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var existingRows []assessmentRow
		if loadErr := s.database.GetScan(ctx, &existingRows, `SELECT `+assessmentFields()+` FROM plugin_idc_compliance_privacy_assessment WHERE tenant_id = ? AND assessment_type_code = ? AND status_code = 'draft' FOR UPDATE`, idccap.DefaultTenantID, input.Type); loadErr != nil {
			return loadErr
		}
		var existing *assessmentRow
		if len(existingRows) == 1 {
			existing = &existingRows[0]
		}
		before := map[string]any{"status": "missing", "row_version": 0}
		if existing == nil {
			if input.ExpectedRowVersion != 0 {
				return bizerr.NewCode(CodeConflict)
			}
			assessmentNo, err = newBusinessNo("PIA_")
			if err != nil {
				return err
			}
			nextVersionValue, loadErr := s.database.GetValue(ctx, `SELECT COALESCE(MAX(version), 0) + 1 FROM plugin_idc_compliance_privacy_assessment WHERE tenant_id = ? AND assessment_type_code = ?`, idccap.DefaultTenantID, input.Type)
			if loadErr != nil {
				return loadErr
			}
			nextVersion := nextVersionValue.Int64()
			_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_assessment
    (tenant_id, assessment_no, assessment_type_code, version, status_code,
     purpose, necessity, impact, recipient_description, storage_location,
     retention_description, rights_entry, merchant_role, third_party_role,
     processing_instruction, subcontracting_boundary, audit_evidence_description,
     cross_border_transfer, created_by_user_id, updated_by_user_id, row_version,
     created_at, updated_at)
VALUES (?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, FALSE, ?, ?, 1, ?, ?)`,
				idccap.DefaultTenantID, assessmentNo, input.Type, nextVersion,
				input.Purpose, input.Necessity, input.Impact, input.Recipients,
				input.StorageLocation, input.RetentionDescription, input.RightsEntry,
				input.MerchantRole, input.ThirdPartyRole, input.ProcessingInstruction,
				input.SubcontractingBoundary, input.AuditEvidenceDescription,
				actor, actor, now, now)
			if err != nil {
				return err
			}
		} else {
			assessmentNo = existing.AssessmentNo
			if existing.RowVersion != input.ExpectedRowVersion {
				return bizerr.NewCode(CodeConflict)
			}
			before = map[string]any{"status": existing.StatusCode, "row_version": existing.RowVersion}
			result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_assessment
SET purpose = ?, necessity = ?, impact = ?, recipient_description = ?, storage_location = ?,
    retention_description = ?, rights_entry = ?, merchant_role = ?, third_party_role = ?,
    processing_instruction = ?, subcontracting_boundary = ?, audit_evidence_description = ?,
    updated_by_user_id = ?, row_version = row_version + 1, updated_at = ?
WHERE tenant_id = ? AND assessment_no = ? AND status_code = 'draft' AND row_version = ?`,
				input.Purpose, input.Necessity, input.Impact, input.Recipients,
				input.StorageLocation, input.RetentionDescription, input.RightsEntry,
				input.MerchantRole, input.ThirdPartyRole, input.ProcessingInstruction,
				input.SubcontractingBoundary, input.AuditEvidenceDescription,
				actor, now, idccap.DefaultTenantID, assessmentNo, input.ExpectedRowVersion)
			if updateErr != nil {
				return updateErr
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil || affected != 1 {
				return bizerr.NewCode(CodeConflict)
			}
		}
		if _, deleteErr := s.database.Exec(ctx, `DELETE FROM plugin_idc_compliance_privacy_ai_field WHERE tenant_id = ? AND assessment_no = ?`, idccap.DefaultTenantID, assessmentNo); deleteErr != nil {
			return deleteErr
		}
		for _, field := range input.AIAllowedFields {
			if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_ai_field (tenant_id, assessment_no, field_code) VALUES (?, ?, ?)`, idccap.DefaultTenantID, assessmentNo, field); insertErr != nil {
				return insertErr
			}
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.assessment-draft",
			IdempotencyKey:   sha256Hex("privacy-assessment-draft\x00" + assessmentNo + "\x00" + strconvFormatInt(now.UnixNano())),
			OperationCode:    "compliance.privacy.assessment-draft", TargetType: "privacy_assessment",
			TargetNo: assessmentNo, ResultCode: "succeeded", ReasonCode: "privacy_assessment_draft_saved",
			RiskCode: "high", RequiredPermission: PermissionConfigure, BeforeSnapshot: before,
			AfterSnapshot: map[string]any{"type": input.Type, "status": AssessmentDraft, "ai_field_count": len(input.AIAllowedFields)},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getAssessment(ctx, assessmentNo)
}

func (s *serviceImpl) PublishAssessment(ctx context.Context, input PublishAssessmentInput) (*Assessment, error) {
	input, digest, fingerprint, err := normalizePublish(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionConfigure, input.AssessmentNo)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	err = dao.PrivacyAction.TransactionWithDatabase(ctx, s.database, func(ctx context.Context) error {
		var rowValues []assessmentRow
		if loadErr := s.database.GetScan(ctx, &rowValues, `SELECT `+assessmentFields()+` FROM plugin_idc_compliance_privacy_assessment WHERE tenant_id = ? AND assessment_no = ? FOR UPDATE`, idccap.DefaultTenantID, input.AssessmentNo); loadErr != nil {
			return loadErr
		}
		if len(rowValues) != 1 {
			return bizerr.NewCode(CodeNotAccessible)
		}
		row := &rowValues[0]
		var replays []struct {
			RequestFingerprint string `json:"request_fingerprint"`
			ResultReference    string `json:"result_reference"`
		}
		if loadErr := s.database.GetScan(ctx, &replays, `SELECT request_fingerprint, result_reference FROM plugin_idc_compliance_privacy_action WHERE tenant_id = ? AND action_code = 'publish_assessment' AND target_no = ? AND idempotency_digest = ?`, idccap.DefaultTenantID, input.AssessmentNo, digest); loadErr != nil {
			return loadErr
		}
		if len(replays) == 1 {
			replay := replays[0]
			if replay.RequestFingerprint != fingerprint || replay.ResultReference != input.AssessmentNo {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.StatusCode != string(AssessmentDraft) || row.RowVersion != input.ExpectedRowVersion {
			return bizerr.NewCode(CodeConflict)
		}
		if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_assessment SET status_code = 'superseded', superseded_at = ?, updated_at = ? WHERE tenant_id = ? AND assessment_type_code = ? AND status_code = 'published'`, now, now, idccap.DefaultTenantID, row.AssessmentTypeCode); updateErr != nil {
			return updateErr
		}
		result, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_privacy_assessment SET status_code = 'published', confirmed_by_user_id = ?, confirmed_at = ?, row_version = row_version + 1, updated_by_user_id = ?, updated_at = ? WHERE tenant_id = ? AND assessment_no = ? AND status_code = 'draft' AND row_version = ?`, actor, now, actor, now, idccap.DefaultTenantID, input.AssessmentNo, input.ExpectedRowVersion)
		if updateErr != nil {
			return updateErr
		}
		affected, affectedErr := result.RowsAffected()
		if affectedErr != nil || affected != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		actionNo := "PRA_" + sha256Hex("publish\x00" + input.AssessmentNo + "\x00" + digest)[:32]
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_compliance_privacy_action (tenant_id, action_no, action_code, target_no, idempotency_digest, request_fingerprint, result_reference, actor_user_id) VALUES (?, ?, 'publish_assessment', ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, actionNo, input.AssessmentNo, digest, fingerprint, input.AssessmentNo, actor); insertErr != nil {
			return insertErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.privacy.assessment-publish", IdempotencyKey: digest,
			OperationCode: "compliance.privacy.assessment-publish", TargetType: "privacy_assessment",
			TargetNo: input.AssessmentNo, ResultCode: "succeeded", ReasonCode: "privacy_assessment_published",
			RiskCode: "critical", RequiredPermission: PermissionConfigure,
			BeforeSnapshot: map[string]any{"status": AssessmentDraft, "row_version": row.RowVersion},
			AfterSnapshot:  map[string]any{"status": AssessmentPublished, "version": row.Version, "confirmed_by": actor},
		})
	})
	if err != nil {
		if bizerr.Is(err, CodeConflict) || bizerr.Is(err, CodeNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return s.getAssessment(ctx, input.AssessmentNo)
}

func (s *serviceImpl) getAssessment(ctx context.Context, assessmentNo string) (*Assessment, error) {
	var row *assessmentRow
	if err := s.database.Model("plugin_idc_compliance_privacy_assessment").Ctx(ctx).
		Fields(assessmentFields()).Where("tenant_id", idccap.DefaultTenantID).
		Where("assessment_no", assessmentNo).Limit(1).Scan(&row); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	page, err := s.projectAssessmentRows(ctx, []assessmentRow{*row})
	if err != nil || len(page.Items) != 1 {
		if err != nil {
			return nil, err
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &page.Items[0], nil
}

func (s *serviceImpl) projectAssessmentRows(ctx context.Context, rows []assessmentRow) (*AssessmentPage, error) {
	page := &AssessmentPage{Items: make([]Assessment, 0, len(rows))}
	if len(rows) == 0 {
		return page, nil
	}
	numbers := make([]string, 0, len(rows))
	for _, row := range rows {
		numbers = append(numbers, row.AssessmentNo)
	}
	var fields []aiFieldRow
	if err := s.database.Model("plugin_idc_compliance_privacy_ai_field").Ctx(ctx).
		Fields("assessment_no, field_code").Where("tenant_id", idccap.DefaultTenantID).
		WhereIn("assessment_no", numbers).OrderAsc("field_code").Scan(&fields); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	byAssessment := make(map[string][]string, len(rows))
	for _, field := range fields {
		byAssessment[field.AssessmentNo] = append(byAssessment[field.AssessmentNo], field.FieldCode)
	}
	for _, row := range rows {
		item := Assessment{
			AssessmentNo: row.AssessmentNo, Type: AssessmentType(row.AssessmentTypeCode),
			Version: row.Version, Status: AssessmentStatus(row.StatusCode), Purpose: row.Purpose,
			Necessity: row.Necessity, Impact: row.Impact, Recipients: row.RecipientDescription,
			StorageLocation: row.StorageLocation, RetentionDescription: row.RetentionDescription,
			RightsEntry: row.RightsEntry, MerchantRole: row.MerchantRole,
			ThirdPartyRole: row.ThirdPartyRole, ProcessingInstruction: row.ProcessingInstruction,
			SubcontractingBoundary:   row.SubcontractingBoundary,
			AuditEvidenceDescription: row.AuditEvidenceDescription,
			AIAllowedFields:          append([]string(nil), byAssessment[row.AssessmentNo]...),
			CrossBorderTransfer:      row.CrossBorderTransfer, ConfirmedByUserID: row.ConfirmedByUserID,
			ConfirmedAt: row.ConfirmedAt, CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
			RowVersion: row.RowVersion,
		}
		sort.Strings(item.AIAllowedFields)
		page.Items = append(page.Items, item)
	}
	return page, nil
}

func assessmentFields() string {
	return `id, assessment_no, assessment_type_code, version, status_code, purpose,
necessity, impact, recipient_description, storage_location, retention_description,
rights_entry, merchant_role, third_party_role, processing_instruction,
subcontracting_boundary, audit_evidence_description, cross_border_transfer,
confirmed_by_user_id, confirmed_at, created_at, updated_at, row_version`
}
