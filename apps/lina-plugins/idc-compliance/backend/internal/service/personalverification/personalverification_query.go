// This file performs authoritative provider queries and commits fenced verification outcomes.

package personalverification

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/model/entity"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

type queryState struct {
	attempt *entity.PersonalVerificationAttempt
	caseRow *entity.VerificationCase
}

// QueryCurrent rechecks only a case owned by the authenticated personal customer.
func (s *serviceImpl) QueryCurrent(ctx context.Context, attemptNo string) (*Verification, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !validBusinessNo(attemptNo, 64) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	subject, err := s.identities.CurrentPersonal(ctx)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeNotAccessible)
	}
	if subject.ManualVerification {
		return s.Current(ctx)
	}
	return s.query(ctx, attemptNo, subject.CustomerNo)
}

func (s *serviceImpl) query(
	ctx context.Context,
	attemptNo string,
	expectedCustomerNo string,
) (*Verification, error) {
	state, err := s.loadQueryState(ctx, attemptNo)
	if err != nil {
		return nil, err
	}
	if state == nil || state.attempt == nil || state.caseRow == nil ||
		(expectedCustomerNo != "" && state.caseRow.CustomerNo != expectedCustomerNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	if terminal := projectTerminal(state); terminal != nil {
		return terminal, nil
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	if result, handled, handleErr := s.handlePreQueryState(ctx, state, now); handled {
		return result, handleErr
	}
	configuration, err := s.configurations.Resolve(ctx)
	if err != nil || configuration.ConfigurationNo != state.attempt.ConfigurationNo ||
		configuration.Version != state.attempt.ConfigurationVersion {
		if markErr := s.markReviewState(ctx, state, reasonProviderConfigurationChanged, attemptStatusCanceled); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return projectReview(state, reasonProviderConfigurationChanged), nil
	}
	client, err := s.clients.Create(configuration)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeProviderUnavailable)
	}
	result, err := client.Query(ctx, aliyunface.QueryRequest{
		SceneID: configuration.SceneID, CertifyID: *state.attempt.CertifyId,
	})
	if err != nil {
		return s.handleQueryError(ctx, state, now, err)
	}
	return s.handleQueryResult(ctx, state, now, result)
}

func (s *serviceImpl) handlePreQueryState(
	ctx context.Context,
	state *queryState,
	now time.Time,
) (*Verification, bool, error) {
	switch attemptStatusCode(state.attempt.StatusCode) {
	case attemptStatusPreparing:
		if state.attempt.InitializationStartedAt == nil ||
			now.Sub(state.attempt.InitializationStartedAt.UTC()) < preparingTimeout {
			return projectVerifying(state), true, nil
		}
		if err := s.markReviewState(ctx, state, reasonProviderInitializationUnconfirmed, attemptStatusUnknown); err != nil {
			return nil, true, bizerr.WrapCode(err, CodeUnavailable)
		}
		return projectReview(state, reasonProviderInitializationUnconfirmed), true, nil
	case attemptStatusUnknown:
		if caseStatusCode(state.caseRow.StatusCode) != caseStatusReviewRequired {
			if err := s.markReviewState(ctx, state, reasonProviderInitializationUnknown, attemptStatusUnknown); err != nil {
				return nil, true, bizerr.WrapCode(err, CodeUnavailable)
			}
		}
		return projectReview(state, reasonProviderInitializationUnknown), true, nil
	case attemptStatusReady, attemptStatusPending:
	default:
		return nil, true, bizerr.NewCode(CodeUnavailable)
	}
	if state.attempt.CertifyExpiresAt == nil || state.attempt.CertifyId == nil ||
		*state.attempt.CertifyId == "" {
		if err := s.markReviewState(ctx, state, reasonProviderAttemptInconsistent, attemptStatusCanceled); err != nil {
			return nil, true, bizerr.WrapCode(err, CodeUnavailable)
		}
		return projectReview(state, reasonProviderAttemptInconsistent), true, nil
	}
	if !now.Before(state.attempt.CertifyExpiresAt.UTC()) {
		result, err := s.completeExpired(ctx, state, now, reasonCertificationExpired)
		return result, true, err
	}
	return nil, false, nil
}

func (s *serviceImpl) handleQueryError(
	ctx context.Context,
	state *queryState,
	now time.Time,
	queryErr error,
) (*Verification, error) {
	var callError *aliyunface.CallError
	if errors.As(queryErr, &callError) && callError.Code == aliyunface.ErrorProviderRejected {
		if markErr := s.markReviewState(ctx, state, reasonProviderQueryRejected, attemptStatusCanceled); markErr != nil {
			return nil, bizerr.WrapCode(markErr, CodeUnavailable)
		}
		return projectReview(state, reasonProviderQueryRejected), nil
	}
	if observeErr := s.observePending(ctx, state, now, reasonProviderQueryUnknown, false); observeErr != nil {
		return nil, bizerr.WrapCode(observeErr, CodeUnavailable)
	}
	return projectVerifying(state), nil
}

func (s *serviceImpl) handleQueryResult(
	ctx context.Context,
	state *queryState,
	now time.Time,
	result aliyunface.QueryResult,
) (*Verification, error) {
	switch result.State {
	case aliyunface.QueryStatePassed:
		return s.completePassed(ctx, state, now)
	case aliyunface.QueryStateExpired:
		return s.completeExpired(ctx, state, now, reasonProviderExpired)
	case aliyunface.QueryStateNotPassed:
		if err := s.observePending(ctx, state, now, reasonNotPassed, true); err != nil {
			return nil, bizerr.WrapCode(err, CodeUnavailable)
		}
		return projectVerifying(state), nil
	default:
		return nil, bizerr.NewCode(CodeUnavailable)
	}
}

func (s *serviceImpl) completePassed(
	ctx context.Context,
	state *queryState,
	now time.Time,
) (*Verification, error) {
	if state.caseRow.LegalProfileSnapshotRef == nil || state.caseRow.LegalProfileSha256 == nil {
		return s.completeReview(ctx, state, reasonSnapshotUnavailable, attemptStatusCanceled)
	}
	handle, err := secretcap.NewHandle(*state.caseRow.LegalProfileSnapshotRef)
	if err != nil {
		return s.completeReview(ctx, state, reasonSnapshotUnavailable, attemptStatusCanceled)
	}
	purpose, err := secretcap.NewPurpose("compliance.personal-identity-projection")
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	var projection *identitycap.PersonalProjection
	var transactionErr error
	err = s.snapshotAccessor.Access(ctx, request, func(payload []byte) error {
		challenge, name, identityNumber, decodeErr := decodeSnapshot(payload)
		if decodeErr != nil || subtle.ConstantTimeCompare(
			[]byte(snapshotDigest(challenge)), []byte(*state.caseRow.LegalProfileSha256),
		) != 1 {
			return gerror.New("personal identity snapshot integrity check failed")
		}
		transactionErr = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			projection, decodeErr = s.identities.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
				CustomerNo: state.caseRow.CustomerNo, CaseNo: state.caseRow.CaseNo,
				Status: identitycap.StatusVerified, LegalName: name,
				MaskedName: state.caseRow.MaskedName, IdentityNumber: identityNumber,
				MaskedIdentityNo: state.caseRow.MaskedIdentityNo, VerifiedAt: &now,
			})
			if decodeErr != nil {
				return decodeErr
			}
			if projection == nil || projection.DuplicateRisk {
				return nil
			}
			if projection.Status != identitycap.StatusVerified || projection.IdentityLookupDigest == "" {
				return gerror.New("personal identity verified projection is inconsistent")
			}
			return s.persistPassed(ctx, state, now, projection.IdentityLookupDigest)
		})
		return transactionErr
	})
	if transactionErr != nil {
		if errors.Is(transactionErr, identitycap.ErrDuplicateIdentity) {
			return s.completeReview(ctx, state, reasonDuplicateIdentity, attemptStatusCanceled)
		}
		return nil, bizerr.WrapCode(transactionErr, CodeUnavailable)
	}
	if err != nil {
		return s.completeReview(ctx, state, reasonSnapshotUnavailable, attemptStatusCanceled)
	}
	if projection == nil || projection.DuplicateRisk {
		return s.completeReview(ctx, state, reasonDuplicateIdentity, attemptStatusCanceled)
	}
	if projection.Status != identitycap.StatusVerified || projection.IdentityLookupDigest == "" {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Verification{
		CaseNo: state.caseRow.CaseNo, AttemptNo: state.attempt.AttemptNo,
		Status: StatusVerified,
	}, nil
}

func (s *serviceImpl) completeExpired(
	ctx context.Context,
	state *queryState,
	now time.Time,
	reason verificationReasonCode,
) (*Verification, error) {
	projection, err := s.identities.ProjectPersonal(ctx, identitycap.PersonalProjectionInput{
		CustomerNo: state.caseRow.CustomerNo, CaseNo: state.caseRow.CaseNo,
		Status: identitycap.StatusExpired, ExpiresAt: &now,
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if projection == nil || projection.Status != identitycap.StatusExpired {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if err = s.persistExpired(ctx, state, now, reason); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return &Verification{
		CaseNo: state.caseRow.CaseNo, AttemptNo: state.attempt.AttemptNo,
		Status: StatusExpired, ExpiresAt: &now, ReasonCode: string(reason),
	}, nil
}

func (s *serviceImpl) loadQueryState(ctx context.Context, attemptNo string) (*queryState, error) {
	var attempt *entity.PersonalVerificationAttempt
	if err := s.database.Model(dao.PersonalVerificationAttempt.Table()).Ctx(ctx).
		Where(do.PersonalVerificationAttempt{
			TenantId: idccap.DefaultTenantID, AttemptNo: attemptNo,
		}).Limit(1).Scan(&attempt); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if attempt == nil || attempt.SubjectTypeCode != personalSubjectType ||
		attempt.ProviderCode != providerCode || attempt.Version <= 0 {
		return nil, nil
	}
	var caseRow *entity.VerificationCase
	if err := s.database.Model(dao.VerificationCase.Table()).Ctx(ctx).
		Where(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CaseNo: attempt.CaseNo,
			SubjectTypeCode: personalSubjectType,
		}).Limit(1).Scan(&caseRow); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	if caseRow == nil || caseRow.ProviderCode != providerCode || caseRow.Version <= 0 {
		return nil, nil
	}
	return &queryState{attempt: attempt, caseRow: caseRow}, nil
}

func (s *serviceImpl) observePending(
	ctx context.Context,
	state *queryState,
	now time.Time,
	resultCode verificationReasonCode,
	increment bool,
) error {
	data := do.PersonalVerificationAttempt{
		LastProviderResultCode: resultCode, LastQueriedAt: &now,
		Version: state.attempt.Version + 1,
	}
	if increment {
		data.NotPassedObservationCount = state.attempt.NotPassedObservationCount + 1
	}
	result, err := s.database.Model(dao.PersonalVerificationAttempt.Table()).Ctx(ctx).
		Where(do.PersonalVerificationAttempt{
			Id: state.attempt.Id, TenantId: idccap.DefaultTenantID,
			StatusCode: state.attempt.StatusCode, Version: state.attempt.Version,
		}).Data(data).Update()
	if err != nil {
		return err
	}
	return requireOneRow(result, "observe personal verification query")
}

func (s *serviceImpl) markReviewState(
	ctx context.Context,
	state *queryState,
	reason verificationReasonCode,
	attemptStatus attemptStatusCode,
) error {
	return dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		attemptResult, err := dao.PersonalVerificationAttempt.Ctx(ctx).
			Where(do.PersonalVerificationAttempt{
				Id: state.attempt.Id, TenantId: idccap.DefaultTenantID,
				Version: state.attempt.Version,
			}).Data(do.PersonalVerificationAttempt{
			StatusCode: attemptStatus, LastProviderResultCode: reason,
			Version: state.attempt.Version + 1,
		}).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(attemptResult, "mark personal verification attempt for review"); err != nil {
			return err
		}
		caseResult, err := dao.VerificationCase.Ctx(ctx).
			Where(do.VerificationCase{
				Id: state.caseRow.Id, TenantId: idccap.DefaultTenantID,
				Version: state.caseRow.Version,
			}).Data(do.VerificationCase{
			StatusCode: caseStatusReviewRequired, DecisionReasonCode: reason,
			Version: state.caseRow.Version + 1,
		}).Update()
		if err != nil {
			return err
		}
		return requireOneRow(caseResult, "mark personal verification case for review")
	})
}

func (s *serviceImpl) persistPassed(
	ctx context.Context,
	state *queryState,
	now time.Time,
	lookupDigest string,
) error {
	return s.persistTerminal(ctx, state, now, attemptStatusPassed, caseStatusVerified,
		reasonPassed, lookupDigest)
}

func (s *serviceImpl) persistExpired(
	ctx context.Context,
	state *queryState,
	now time.Time,
	reason verificationReasonCode,
) error {
	return s.persistTerminal(ctx, state, now, attemptStatusExpired, caseStatusExpired, reason, "")
}

func (s *serviceImpl) persistTerminal(
	ctx context.Context,
	state *queryState,
	now time.Time,
	attemptStatus attemptStatusCode,
	caseStatus caseStatusCode,
	reason verificationReasonCode,
	lookupDigest string,
) error {
	return dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		attemptData := do.PersonalVerificationAttempt{
			StatusCode: attemptStatus, LastProviderResultCode: reason,
			LastQueriedAt: &now, Version: state.attempt.Version + 1,
		}
		if attemptStatus == attemptStatusPassed {
			attemptData.PassedAt = &now
		} else {
			attemptData.ExpiredAt = &now
		}
		attemptResult, err := dao.PersonalVerificationAttempt.Ctx(ctx).
			Where(do.PersonalVerificationAttempt{
				Id: state.attempt.Id, TenantId: idccap.DefaultTenantID,
				Version: state.attempt.Version,
			}).Data(attemptData).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(attemptResult, "complete personal verification attempt"); err != nil {
			return err
		}
		caseData := do.VerificationCase{
			StatusCode: caseStatus, DecisionReasonCode: reason,
			Version: state.caseRow.Version + 1,
		}
		if caseStatus == caseStatusVerified {
			lookupDigestValue := lookupDigest
			caseData.VerifiedAt = &now
			caseData.IdentityLookupDigest = &lookupDigestValue
		}
		caseResult, err := dao.VerificationCase.Ctx(ctx).
			Where(do.VerificationCase{
				Id: state.caseRow.Id, TenantId: idccap.DefaultTenantID,
				Version: state.caseRow.Version,
			}).Data(caseData).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(caseResult, "complete personal verification case"); err != nil {
			return err
		}
		if caseStatus != caseStatusVerified {
			return nil
		}
		if err = s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.personal-verification.verified",
			IdempotencyKey:   state.caseRow.CaseNo,
			OperationCode:    "compliance.personal-verification.verify",
			TargetType:       "personal_verification_case", TargetNo: state.caseRow.CaseNo,
			CustomerNo: state.caseRow.CustomerNo, ResultCode: "succeeded",
			ReasonCode: string(reasonPassed), RiskCode: "high",
			CorrelationNo: state.attempt.AttemptNo,
			BeforeSnapshot: map[string]any{
				"case_status":    state.caseRow.StatusCode,
				"attempt_status": state.attempt.StatusCode,
			},
			AfterSnapshot: map[string]any{
				"case_status":    caseStatusVerified,
				"attempt_status": attemptStatusPassed,
				"provider_code":  providerCode,
			},
		}); err != nil {
			return gerror.Wrap(err, "append personal verification audit")
		}
		return nil
	})
}

func (s *serviceImpl) completeReview(
	ctx context.Context,
	state *queryState,
	reason verificationReasonCode,
	attemptStatus attemptStatusCode,
) (*Verification, error) {
	if err := s.markReviewState(ctx, state, reason, attemptStatus); err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return projectReview(state, reason), nil
}

func projectTerminal(state *queryState) *Verification {
	if state == nil || state.attempt == nil || state.caseRow == nil {
		return nil
	}
	result := &Verification{
		CaseNo: state.caseRow.CaseNo, AttemptNo: state.attempt.AttemptNo,
		MaskedName: state.caseRow.MaskedName, MaskedIdentityNo: state.caseRow.MaskedIdentityNo,
		SubmittedAt: copyTime(state.caseRow.SubmittedAt),
		VerifiedAt:  copyTime(state.caseRow.VerifiedAt), ExpiresAt: copyTime(state.caseRow.ExpiresAt),
	}
	switch caseStatusCode(state.caseRow.StatusCode) {
	case caseStatusVerified:
		result.Status = StatusVerified
	case caseStatusExpired:
		result.Status = StatusExpired
		if result.ExpiresAt == nil {
			result.ExpiresAt = copyTime(state.attempt.ExpiredAt)
		}
	case caseStatusReviewRequired:
		result.Status = StatusReviewRequired
		result.ReasonCode = state.caseRow.DecisionReasonCode
	case caseStatusRejected:
		result.Status = StatusRejected
		result.ReasonCode = state.caseRow.DecisionReasonCode
	default:
		return nil
	}
	return result
}

func projectVerifying(state *queryState) *Verification {
	return &Verification{
		CaseNo: state.caseRow.CaseNo, AttemptNo: state.attempt.AttemptNo,
		Status: StatusVerifying, MaskedName: state.caseRow.MaskedName,
		MaskedIdentityNo: state.caseRow.MaskedIdentityNo,
		SubmittedAt:      copyTime(state.caseRow.SubmittedAt),
		ExpiresAt:        copyTime(state.attempt.CertifyExpiresAt),
	}
}

func projectReview(state *queryState, reason verificationReasonCode) *Verification {
	return &Verification{
		CaseNo: state.caseRow.CaseNo, AttemptNo: state.attempt.AttemptNo,
		Status: StatusReviewRequired, ReasonCode: string(reason),
		MaskedName: state.caseRow.MaskedName, MaskedIdentityNo: state.caseRow.MaskedIdentityNo,
		SubmittedAt: copyTime(state.caseRow.SubmittedAt), ExpiresAt: copyTime(state.caseRow.ExpiresAt),
	}
}

func copyTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}

func validBusinessNo(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range []byte(value) {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}
