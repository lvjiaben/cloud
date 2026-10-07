// This file implements bounded management list and detail projections.

package enterpriseverification

import (
	"context"
	"lina-plugin-idc-compliance/backend/internal/service/numberedpage"
	"sort"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

// managementCaseStatuses is the closed filter and projection status catalog.
var managementCaseStatuses = map[string]struct{}{
	caseStatusPending: {}, caseStatusVerifying: {}, caseStatusReviewRequired: {},
	caseStatusVerified: {}, caseStatusRejected: {}, caseStatusExpired: {}, caseStatusCanceled: {},
}

// ListCases authorizes the fixed directory and returns one bounded page with its authorized total.
func (s *managementServiceImpl) ListCases(
	ctx context.Context,
	input CaseListInput,
) (*CaseList, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permissionEnterpriseCaseList, TargetNo: enterpriseCaseDirectoryTarget,
	}); err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	input, err := normalizeCaseListInput(input)
	if err != nil {
		return nil, err
	}
	cursor, err := s.decodeCaseCursor(ctx, input)
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	model := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, SubjectTypeCode: enterpriseSubjectType,
	}).Where("submitted_at IS NOT NULL")
	if input.StatusCode != "" {
		model = model.Where("status_code", input.StatusCode)
	}
	if input.CustomerNo != "" {
		model = model.Where("customer_no", input.CustomerNo)
	}
	countModel := model.Clone()
	if cursor != nil {
		lastSubmitted := cursorSubmittedAt(cursor)
		model = model.Where(
			"(submitted_at < ?) OR (submitted_at = ? AND case_no < ?)",
			lastSubmitted, lastSubmitted, cursor.CaseNo,
		)
	}
	var rows []*entity.VerificationCase
	total, err := numberedpage.Scan(ctx, s.database, countModel, model.Fields(
		"case_no", "customer_no", "status_code", "masked_name", "masked_identity_no",
		"applicant_role_code", "legal_change_request_no", "decision_reason_code",
		"submitted_at", "updated_at", "version",
	).OrderDesc("submitted_at").OrderDesc("case_no").Offset(max(input.Page-1, 0)*input.PageSize).Limit(input.PageSize+1), &rows)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	result := &CaseList{Total: total, Items: make([]CaseListItem, 0, min(input.PageSize, len(rows)))}
	hasMore := len(rows) > input.PageSize
	if hasMore {
		rows = rows[:input.PageSize]
	}
	for _, row := range rows {
		item, valid := managementListItem(row)
		if !valid {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		result.Items = append(result.Items, item)
	}
	if hasMore && len(result.Items) > 0 {
		result.NextCursor, err = s.encodeCaseCursor(ctx, input, result.Items[len(result.Items)-1])
		if err != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
	}
	return result, nil
}

// GetCase authorizes the exact case and returns only masked, bounded projections.
func (s *managementServiceImpl) GetCase(
	ctx context.Context,
	caseNo string,
) (*CaseDetail, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if _, err := s.authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: permissionEnterpriseCaseQuery, TargetNo: caseNo,
	}); err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	row, err := loadManagementCase(ctx, caseNo)
	if err != nil || row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	item, valid := managementListItem(row)
	if !valid {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	materials, rows, err := loadManagementMaterials(ctx, caseNo, false)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	profile, err := s.loadLegalSnapshot(ctx, row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer profile.clear()
	if !profile.materialSetMatches(rows) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &CaseDetail{
		CaseListItem: item, LegalProfile: profile.masked(), Materials: materials,
		VerifiedAt: cloneTime(row.VerifiedAt), RejectedAt: cloneTime(row.RejectedAt),
		ExpiresAt: cloneTime(row.ExpiresAt),
	}, nil
}

// normalizeCaseListInput applies the default page size and closed filters.
func normalizeCaseListInput(input CaseListInput) (CaseListInput, error) {
	if input.PageSize == 0 {
		input.PageSize = defaultManagementPageSize
	}
	if !numberedpage.Valid(input.Page, input.PageSize, input.Cursor) || input.PageSize < 1 || input.PageSize > maximumManagementPageSize ||
		(input.StatusCode != "" && !validManagementCaseStatus(input.StatusCode)) ||
		(input.CustomerNo != "" && !validBusinessNo(input.CustomerNo, 64)) {
		return CaseListInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

// validManagementCaseStatus accepts only one cataloged management status.
func validManagementCaseStatus(value string) bool {
	_, ok := managementCaseStatuses[value]
	return ok
}

// loadManagementCase reads one submitted enterprise case in the fixed merchant scope.
func loadManagementCase(ctx context.Context, caseNo string) (*entity.VerificationCase, error) {
	if !validBusinessNo(caseNo, 64) {
		return nil, nil
	}
	var row *entity.VerificationCase
	err := dao.VerificationCase.Ctx(ctx).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		SubjectTypeCode: enterpriseSubjectType,
	}).Where("submitted_at IS NOT NULL").Limit(1).Scan(&row)
	return row, err
}

// loadManagementMaterials reads and optionally locks one fixed-size material batch without per-row queries.
func loadManagementMaterials(
	ctx context.Context,
	caseNo string,
	lock bool,
) ([]MaterialItem, []*entity.EnterpriseMaterial, error) {
	var rows []*entity.EnterpriseMaterial
	model := dao.EnterpriseMaterial.Ctx(ctx).Fields(
		"material_no", "case_no", "material_type_code", "sequence_no", "media_type",
		"byte_size", "status_code", "retention_until", "legal_hold", "encryption_key_no",
		"private_object_ref", "content_sha256", "stored_byte_size", "encryption_state_code",
	).Where(do.EnterpriseMaterial{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		CaseSubjectTypeCode: enterpriseSubjectType,
	}).OrderAsc("material_type_code").OrderAsc("sequence_no").Limit(maximumSelectedCount + 1)
	if lock {
		model = model.LockUpdate()
	}
	err := model.Scan(&rows)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) > maximumSelectedCount {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	items := make([]MaterialItem, 0, len(rows))
	for _, row := range rows {
		if row == nil || !validateMaterialType(MaterialType(row.MaterialTypeCode)) ||
			row.SequenceNo <= 0 || row.ByteSize <= 0 {
			return nil, nil, bizerr.NewCode(CodeUnavailable)
		}
		items = append(items, MaterialItem{
			MaterialNo: row.MaterialNo, MaterialType: MaterialType(row.MaterialTypeCode),
			SequenceNo: row.SequenceNo, MediaType: row.MediaType, ByteSize: row.ByteSize,
			StatusCode: row.StatusCode, RetentionUntil: cloneTime(row.RetentionUntil),
			LegalHold: row.LegalHold,
		})
	}
	return items, rows, nil
}

// managementListItem validates and projects one secret-safe directory row.
func managementListItem(row *entity.VerificationCase) (CaseListItem, bool) {
	if row == nil || row.SubmittedAt == nil || row.UpdatedAt == nil ||
		!validBusinessNo(row.CaseNo, 64) || !validBusinessNo(row.CustomerNo, 64) ||
		!validManagementCaseStatus(row.StatusCode) || row.Version <= 0 {
		return CaseListItem{}, false
	}
	return CaseListItem{
		CaseNo: row.CaseNo, CustomerNo: row.CustomerNo, StatusCode: row.StatusCode,
		MaskedLegalName: row.MaskedName, MaskedRegistrationID: row.MaskedIdentityNo,
		ApplicantRoleCode:  row.ApplicantRoleCode,
		Reverification:     row.LegalChangeRequestNo != "",
		DecisionReasonCode: row.DecisionReasonCode,
		SubmittedAt:        row.SubmittedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), Version: row.Version,
	}, true
}

// cloneTime detaches and normalizes an optional database timestamp.
func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

// sortedMaterialTypes returns a deterministic audit-only type projection.
func sortedMaterialTypes(rows []*entity.EnterpriseMaterial) []string {
	values := make([]string, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			values = append(values, row.MaterialTypeCode)
		}
	}
	sort.Strings(values)
	return values
}
