// This file creates and safely replays customer-owned enterprise review drafts.

package enterpriseverification

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	// caseStatusDraft permits customer evidence changes.
	caseStatusDraft = "draft"
	// caseStatusPending is immutable customer input awaiting merchant review.
	caseStatusPending = "pending"
)

// CreateDraft creates or safely replays a customer-owned enterprise case.
func (s *serviceImpl) CreateDraft(
	ctx context.Context,
	input CreateDraftInput,
) (*Draft, error) {
	if ctx == nil || ctx.Err() != nil || !validBusinessNo(input.ConsentNo, 64) ||
		(input.LegalChangeRequestNo != "" && !validBusinessNo(input.LegalChangeRequestNo, 40)) ||
		!validIdempotencyKey(input.IdempotencyKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	retentionUntil, err := s.privacyGate.RequireSensitiveCollection(
		ctx, "enterprise_verification", "enterprise_verification_material",
	)
	if err != nil || retentionUntil.IsZero() {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	preparation, err := s.identities.PrepareEnterprise(
		ctx, input.ConsentNo, input.LegalChangeRequestNo,
	)
	if err != nil || preparation == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	digest := sha256Hex([]byte(input.IdempotencyKey))
	fingerprint := sha256Hex([]byte(
		input.ConsentNo + "\x00" + input.LegalChangeRequestNo + "\x00enterprise-draft-v1",
	))
	if replay, found, loadErr := loadDraftReplay(ctx, preparation.Subject.CustomerNo, digest); loadErr != nil || found {
		if loadErr != nil {
			return nil, bizerr.NewCode(CodeUnavailable)
		}
		return validateDraftReplay(replay, preparation.ReviewCaseNo, fingerprint)
	}
	keyNo, err := s.ensureActiveKey(ctx)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	caseEntropy, err := randomHex(s.random, 16)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	caseNo := "ent_" + caseEntropy
	created := &Draft{
		CaseNo: caseNo, StatusCode: caseStatusDraft,
		LegalChangeRequestNo: input.LegalChangeRequestNo,
		Reverification:       preparation.Reverification,
	}
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, insertErr := dao.VerificationCase.Ctx(ctx).Data(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CaseNo: caseNo,
			CustomerNo:      preparation.Subject.CustomerNo,
			PrincipalNo:     preparation.Subject.PrincipalReference,
			SubjectTypeCode: enterpriseSubjectType, ProviderCode: manualProviderCode,
			StatusCode: caseStatusDraft, ConsentNo: input.ConsentNo,
			LegalChangeRequestNo:        input.LegalChangeRequestNo,
			RequestIdempotencyKeyDigest: digest, RequestFingerprint: fingerprint,
			EncryptionKeyNo: &keyNo, Version: 1,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET retention_until = ? WHERE tenant_id = ? AND case_no = ?`, retentionUntil, idccap.DefaultTenantID, caseNo); updateErr != nil {
			return updateErr
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.enterprise-verification.draft",
			IdempotencyKey:   caseNo, OperationCode: "compliance.enterprise-verification.create-draft",
			TargetType: "enterprise_verification_case", TargetNo: caseNo,
			CustomerNo: preparation.Subject.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "customer_submitted",
			RiskCode: "high", CorrelationNo: input.LegalChangeRequestNo,
			BeforeSnapshot: map[string]any{"case_status": "unverified"},
			AfterSnapshot: map[string]any{
				"case_status": caseStatusDraft, "reverification": preparation.Reverification,
			},
		})
	})
	if err != nil {
		if strings.Contains(err.Error(), "verification_case_active") ||
			strings.Contains(err.Error(), "verification_ent_request") ||
			strings.Contains(err.Error(), "verification_ent_legal_change") {
			if replay, found, loadErr := loadDraftReplay(ctx, preparation.Subject.CustomerNo, digest); loadErr == nil && found {
				return validateDraftReplay(replay, preparation.ReviewCaseNo, fingerprint)
			}
			return nil, bizerr.NewCode(CodeConflict)
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return created, nil
}

func loadDraftReplay(
	ctx context.Context,
	customerNo string,
	digest string,
) (*entity.VerificationCase, bool, error) {
	var row *entity.VerificationCase
	err := dao.VerificationCase.Ctx(ctx).Fields(
		"case_no", "customer_no", "subject_type_code", "status_code", "legal_change_request_no",
		"request_idempotency_key_digest", "request_fingerprint", "encryption_key_no",
	).Where(do.VerificationCase{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		SubjectTypeCode: enterpriseSubjectType, RequestIdempotencyKeyDigest: digest,
	}).Limit(1).Scan(&row)
	return row, row != nil, err
}

func validateDraftReplay(
	row *entity.VerificationCase,
	reviewCaseNo string,
	fingerprint string,
) (*Draft, error) {
	if row == nil || row.RequestFingerprint != fingerprint || row.EncryptionKeyNo == nil ||
		(reviewCaseNo != "" && reviewCaseNo != row.CaseNo) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	return &Draft{
		CaseNo: row.CaseNo, StatusCode: row.StatusCode,
		LegalChangeRequestNo: row.LegalChangeRequestNo,
		Reverification:       row.LegalChangeRequestNo != "", Reused: true,
	}, nil
}
