// This file projects current-customer verification state without provider I/O.

package personalverification

import (
	"context"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// Current returns the latest local case and attempt owned by the authenticated customer.
func (s *serviceImpl) Current(ctx context.Context) (*Verification, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentPersonal(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	var caseRow *entity.VerificationCase
	caseColumns := dao.VerificationCase.Columns()
	if err = dao.VerificationCase.Ctx(ctx).
		Fields(
			caseColumns.CaseNo, caseColumns.StatusCode, caseColumns.MaskedName,
			caseColumns.MaskedIdentityNo, caseColumns.DecisionReasonCode,
			caseColumns.SubmittedAt, caseColumns.VerifiedAt, caseColumns.ExpiresAt,
		).
		Where(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
			SubjectTypeCode: personalSubjectType,
		}).OrderDesc(caseColumns.Id).Limit(1).Scan(&caseRow); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if caseRow == nil {
		return &Verification{Status: StatusUnverified}, nil
	}
	var attempt *entity.PersonalVerificationAttempt
	attemptColumns := dao.PersonalVerificationAttempt.Columns()
	if err = dao.PersonalVerificationAttempt.Ctx(ctx).
		Fields(attemptColumns.AttemptNo).
		Where(do.PersonalVerificationAttempt{
			TenantId: idccap.DefaultTenantID, CaseNo: caseRow.CaseNo,
			SubjectTypeCode: personalSubjectType,
		}).OrderDesc(attemptColumns.Id).Limit(1).Scan(&attempt); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if attempt == nil || !validBusinessNo(attempt.AttemptNo, 64) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	status, ok := localCustomerStatus(caseStatusCode(caseRow.StatusCode))
	if !ok {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Verification{
		CaseNo: caseRow.CaseNo, AttemptNo: attempt.AttemptNo, Status: status,
		MaskedName: caseRow.MaskedName, MaskedIdentityNo: caseRow.MaskedIdentityNo,
		SubmittedAt: cloneTime(caseRow.SubmittedAt), VerifiedAt: cloneTime(caseRow.VerifiedAt),
		ExpiresAt: cloneTime(caseRow.ExpiresAt), ReasonCode: caseRow.DecisionReasonCode,
	}, nil
}

func localCustomerStatus(status caseStatusCode) (Status, bool) {
	switch status {
	case caseStatusDraft, caseStatusPending, caseStatusVerifying:
		return StatusVerifying, true
	case caseStatusReviewRequired:
		return StatusReviewRequired, true
	case caseStatusVerified:
		return StatusVerified, true
	case caseStatusRejected:
		return StatusRejected, true
	case caseStatusExpired:
		return StatusExpired, true
	default:
		return "", false
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}
