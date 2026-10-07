// This file reads the current customer's latest safe enterprise case state.

package enterpriseverification

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// Current returns at most one customer-owned case ordered by immutable creation
// position. Absence is a valid unverified state.
func (s *serviceImpl) Current(ctx context.Context) (*CustomerCase, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	subject, err := s.identities.CurrentEnterprise(ctx)
	if err != nil || subject == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var row *entity.VerificationCase
	err = dao.VerificationCase.Ctx(ctx).Fields(
		"id", "tenant_id", "case_no", "customer_no", "subject_type_code", "status_code",
		"decision_reason_code", "legal_change_request_no", "submitted_at", "verified_at",
		"rejected_at", "expires_at", "updated_at", "version",
	).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
		SubjectTypeCode: enterpriseSubjectType,
	}).OrderDesc(dao.VerificationCase.Columns().CreatedAt).
		OrderDesc(dao.VerificationCase.Columns().Id).Limit(1).Scan(&row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, nil
	}
	if row.TenantId != idccap.DefaultTenantID || row.CustomerNo != subject.CustomerNo ||
		row.SubjectTypeCode != enterpriseSubjectType || !validCustomerCaseStatus(row.StatusCode) ||
		!validBusinessNo(row.CaseNo, 64) || row.Version <= 0 ||
		(row.LegalChangeRequestNo != "" && !validBusinessNo(row.LegalChangeRequestNo, 40)) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	materials, err := s.currentMaterials(ctx, row.CaseNo)
	if err != nil {
		return nil, err
	}
	return &CustomerCase{
		CaseNo: row.CaseNo, StatusCode: row.StatusCode,
		DecisionReasonCode:   row.DecisionReasonCode,
		LegalChangeRequestNo: row.LegalChangeRequestNo,
		Reverification:       row.LegalChangeRequestNo != "",
		SubmittedAt:          row.SubmittedAt, VerifiedAt: row.VerifiedAt,
		RejectedAt: row.RejectedAt, ExpiresAt: row.ExpiresAt,
		UpdatedAt: row.UpdatedAt, Version: row.Version, Materials: materials,
	}, nil
}

func (s *serviceImpl) currentMaterials(ctx context.Context, caseNo string) ([]Material, error) {
	var rows []*entity.EnterpriseMaterial
	err := dao.EnterpriseMaterial.Ctx(ctx).Fields(
		"material_no", "material_type_code", "sequence_no", "media_type", "byte_size", "status_code",
	).Where(do.EnterpriseMaterial{
		TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
		CaseSubjectTypeCode: enterpriseSubjectType,
	}).OrderAsc(dao.EnterpriseMaterial.Columns().SequenceNo).
		OrderAsc(dao.EnterpriseMaterial.Columns().Id).Limit(maximumSelectedCount + 1).Scan(&rows)
	if err != nil || len(rows) > maximumSelectedCount {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	items := make([]Material, 0, len(rows))
	for _, row := range rows {
		if row == nil || !validBusinessNo(row.MaterialNo, 64) ||
			!validateMaterialType(MaterialType(row.MaterialTypeCode)) || row.SequenceNo <= 0 ||
			row.ByteSize <= 0 || row.ByteSize > maximumMaterialBytes ||
			!validCustomerMaterialMediaType(row.MediaType) || !validCustomerMaterialStatus(row.StatusCode) {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		items = append(items, Material{
			MaterialNo: row.MaterialNo, MaterialType: MaterialType(row.MaterialTypeCode),
			SequenceNo: row.SequenceNo, MediaType: row.MediaType, ByteSize: row.ByteSize,
			StatusCode: row.StatusCode,
		})
	}
	return items, nil
}

func validCustomerMaterialMediaType(mediaType string) bool {
	switch mediaType {
	case "application/pdf", "image/jpeg", "image/png":
		return true
	default:
		return false
	}
}

func validCustomerMaterialStatus(status string) bool {
	switch status {
	case materialStatusAvailable, materialStatusLegalHold, "superseded", "expired":
		return true
	default:
		return false
	}
}

func validCustomerCaseStatus(status string) bool {
	switch status {
	case caseStatusDraft, caseStatusPending, caseStatusVerifying, caseStatusReviewRequired,
		caseStatusVerified, caseStatusRejected, "expired", "canceled":
		return true
	default:
		return false
	}
}
