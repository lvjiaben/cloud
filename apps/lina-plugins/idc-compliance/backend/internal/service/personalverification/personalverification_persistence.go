// This file persists personal verification initiation and review transitions behind version fences.

package personalverification

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/dao"
	"lina-plugin-idc-compliance/backend/internal/model/do"
	"lina-plugin-idc-compliance/backend/internal/service/providerconfig"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
)

// Persisted personal-verification enum types prevent ad hoc state and reason values.
type caseStatusCode string
type attemptStatusCode string
type verificationReasonCode string

const (
	caseStatusCanceled                            caseStatusCode         = "canceled"
	caseStatusDraft                               caseStatusCode         = "draft"
	caseStatusPending                             caseStatusCode         = "pending"
	caseStatusVerifying                           caseStatusCode         = "verifying"
	caseStatusReviewRequired                      caseStatusCode         = "review_required"
	caseStatusVerified                            caseStatusCode         = "verified"
	caseStatusRejected                            caseStatusCode         = "rejected"
	caseStatusExpired                             caseStatusCode         = "expired"
	attemptStatusPreparing                        attemptStatusCode      = "preparing"
	attemptStatusUnknown                          attemptStatusCode      = "initialization_unknown"
	attemptStatusReady                            attemptStatusCode      = "ready"
	attemptStatusPending                          attemptStatusCode      = "pending"
	attemptStatusPassed                           attemptStatusCode      = "passed"
	attemptStatusExpired                          attemptStatusCode      = "expired"
	attemptStatusCanceled                         attemptStatusCode      = "canceled"
	reasonManualClear                             verificationReasonCode = "manual_identity_clear"
	reasonSnapshotInvalid                         verificationReasonCode = "identity_snapshot_invalid"
	reasonSnapshotUnavailable                     verificationReasonCode = "identity_snapshot_unavailable"
	reasonSnapshotPersistenceFailed               verificationReasonCode = "identity_snapshot_persistence_failed"
	reasonProjectionUnavailable                   verificationReasonCode = "identity_projection_unavailable"
	reasonProviderClientUnavailable               verificationReasonCode = "provider_client_unavailable"
	reasonProviderInitializationUnknown           verificationReasonCode = "provider_initialization_unknown"
	reasonProviderInitializationRejected          verificationReasonCode = "provider_initialization_rejected"
	reasonProviderInitializationPersistenceFailed verificationReasonCode = "provider_initialization_persistence_failed"
	reasonProviderConfigurationChanged            verificationReasonCode = "provider_configuration_changed"
	reasonProviderInitializationUnconfirmed       verificationReasonCode = "provider_initialization_unconfirmed"
	reasonProviderAttemptInconsistent             verificationReasonCode = "provider_attempt_inconsistent"
	reasonCertificationExpired                    verificationReasonCode = "certification_expired"
	reasonProviderQueryRejected                   verificationReasonCode = "provider_query_rejected"
	reasonProviderQueryUnknown                    verificationReasonCode = "provider_query_unknown"
	reasonProviderExpired                         verificationReasonCode = "provider_expired"
	reasonNotPassed                               verificationReasonCode = "not_passed"
	reasonDuplicateIdentity                       verificationReasonCode = "duplicate_identity_requires_review"
	reasonPassed                                  verificationReasonCode = "passed"
	reasonIssued                                  verificationReasonCode = "issued"
)

type initiationStage struct {
	caseNo         string
	attemptNo      string
	outerOrder     string
	configuration  providerconfig.RuntimeConfiguration
	startedAt      time.Time
	maskedName     string
	maskedIdentity string
}

func (s *serviceImpl) createInitiationStage(
	ctx context.Context,
	subject identitycap.PersonalSubject,
	consentNo string,
	configuration providerconfig.RuntimeConfiguration,
	maskedName string,
	maskedIdentity string,
	retentionUntil time.Time,
) (initiationStage, error) {
	caseEntropy, err := randomHex(s.random, 16)
	if err != nil {
		return initiationStage{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	attemptEntropy, err := randomHex(s.random, 16)
	if err != nil {
		return initiationStage{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	outerOrder, err := randomHex(s.random, 16)
	if err != nil {
		return initiationStage{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	stage := initiationStage{
		caseNo: "case_" + caseEntropy, attemptNo: "pva_" + attemptEntropy,
		outerOrder: outerOrder, configuration: configuration,
		startedAt:  s.now().UTC().Truncate(time.Microsecond),
		maskedName: maskedName, maskedIdentity: maskedIdentity,
	}
	if subject.ManualVerification && subject.VerificationStatus == identitycap.StatusUnverified && subject.VerificationUpdatedAt != nil {
		if err := s.cancelClearedCases(ctx, subject); err != nil {
			return initiationStage{}, err
		}
	}
	active, err := s.database.Model(dao.VerificationCase.Table()).Ctx(ctx).
		Where(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo,
			SubjectTypeCode: personalSubjectType,
		}).WhereIn("status_code", []caseStatusCode{
		caseStatusDraft, caseStatusPending, caseStatusVerifying, caseStatusReviewRequired,
	}).Count()
	if err != nil {
		return initiationStage{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	if active > 0 {
		return initiationStage{}, bizerr.NewCode(CodeConflict)
	}
	err = dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, insertErr := dao.VerificationCase.Ctx(ctx).Data(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CaseNo: stage.caseNo,
			CustomerNo: subject.CustomerNo, PrincipalNo: subject.PrincipalReference,
			SubjectTypeCode: personalSubjectType, ProviderCode: providerCode,
			StatusCode: caseStatusDraft, ConsentNo: consentNo,
			MaskedName: maskedName, MaskedIdentityNo: maskedIdentity,
			Version: 1,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_compliance_verification_case SET retention_until = ? WHERE tenant_id = ? AND case_no = ?`, retentionUntil, idccap.DefaultTenantID, stage.caseNo); updateErr != nil {
			return updateErr
		}
		_, insertErr := dao.PersonalVerificationAttempt.Ctx(ctx).
			Data(do.PersonalVerificationAttempt{
				TenantId: idccap.DefaultTenantID, AttemptNo: stage.attemptNo,
				CaseNo: stage.caseNo, SubjectTypeCode: personalSubjectType,
				ConfigurationNo: configuration.ConfigurationNo, ProviderCode: providerCode,
				ConfigurationVersion: configuration.Version, OuterOrderNo: stage.outerOrder,
				StatusCode: attemptStatusPreparing, InitializationStartedAt: &stage.startedAt,
				Version: 1,
			}).Insert()
		return insertErr
	})
	if err != nil {
		if strings.Contains(err.Error(), "uk_plugin_idc_compliance_verification_case_active") {
			return initiationStage{}, bizerr.NewCode(CodeConflict)
		}
		return initiationStage{}, bizerr.WrapCode(err, CodeUnavailable)
	}
	return stage, nil
}

func (s *serviceImpl) attachSnapshot(
	ctx context.Context,
	stage initiationStage,
	handleRef string,
	digest string,
) error {
	handleReference := handleRef
	digestValue := digest
	result, err := s.database.Model(dao.VerificationCase.Table()).Ctx(ctx).
		Where(do.VerificationCase{
			TenantId: idccap.DefaultTenantID, CaseNo: stage.caseNo,
			StatusCode: caseStatusDraft, Version: 1,
		}).Data(do.VerificationCase{
		LegalProfileSnapshotRef: &handleReference, LegalProfileSha256: &digestValue,
		Version: 2,
	}).Update()
	if err != nil {
		return gerror.Wrap(err, "attach encrypted personal identity snapshot")
	}
	return requireOneRow(result, "attach encrypted personal identity snapshot")
}

func (s *serviceImpl) completeInitiation(
	ctx context.Context,
	stage initiationStage,
	certifyID string,
	issuedAt time.Time,
) error {
	expiresAt := issuedAt.Add(30 * time.Minute)
	certifyIDValue := certifyID
	return dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		attemptResult, err := dao.PersonalVerificationAttempt.Ctx(ctx).
			Where(do.PersonalVerificationAttempt{
				TenantId: idccap.DefaultTenantID, AttemptNo: stage.attemptNo,
				StatusCode: attemptStatusPreparing, Version: 1,
			}).Data(do.PersonalVerificationAttempt{
			CertifyId: &certifyIDValue, StatusCode: attemptStatusPending,
			LastProviderResultCode: reasonIssued, IssuedAt: &issuedAt,
			CertifyExpiresAt: &expiresAt, Version: 2,
		}).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(attemptResult, "complete personal verification attempt"); err != nil {
			return err
		}
		caseResult, err := dao.VerificationCase.Ctx(ctx).
			Where(do.VerificationCase{
				TenantId: idccap.DefaultTenantID, CaseNo: stage.caseNo,
				StatusCode: caseStatusDraft, Version: 2,
			}).Data(do.VerificationCase{
			ProviderCaseRef: &certifyIDValue, StatusCode: caseStatusVerifying,
			SubmittedAt: &issuedAt, ExpiresAt: &expiresAt, Version: 3,
		}).Update()
		if err != nil {
			return err
		}
		return requireOneRow(caseResult, "complete personal verification case")
	})
}

func (s *serviceImpl) requireReview(
	ctx context.Context,
	stage initiationStage,
	reason verificationReasonCode,
	attemptStatus attemptStatusCode,
) error {
	return dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		attemptResult, err := dao.PersonalVerificationAttempt.Ctx(ctx).
			Where(do.PersonalVerificationAttempt{
				TenantId: idccap.DefaultTenantID, AttemptNo: stage.attemptNo,
			}).WhereIn("status_code", []attemptStatusCode{attemptStatusPreparing, attemptStatusReady, attemptStatusPending}).
			Data(do.PersonalVerificationAttempt{
				StatusCode: attemptStatus, LastProviderResultCode: reason,
				Version: gdb.Raw("version + 1"),
			}).Update()
		if err != nil {
			return err
		}
		if err = requireOneRow(attemptResult, "mark personal verification attempt for review"); err != nil {
			return err
		}
		caseResult, err := dao.VerificationCase.Ctx(ctx).
			Where(do.VerificationCase{TenantId: idccap.DefaultTenantID, CaseNo: stage.caseNo}).
			WhereIn("status_code", []caseStatusCode{caseStatusDraft, caseStatusVerifying}).
			Data(do.VerificationCase{
				StatusCode: caseStatusReviewRequired, DecisionReasonCode: reason,
				Version: gdb.Raw("version + 1"),
			}).Update()
		if err != nil {
			return err
		}
		return requireOneRow(caseResult, "mark personal verification case for review")
	})
}

func requireOneRow(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, operation)
	}
	if affected != 1 {
		return gerror.New(operation + " lost its version fence")
	}
	return nil
}

// cancelClearedCases retires only cases older than the staff reset, retaining all materials and attempts.
func (s *serviceImpl) cancelClearedCases(ctx context.Context, subject identitycap.PersonalSubject) error {
	return dao.VerificationCase.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err := s.database.Model(dao.VerificationCase.Table()).Ctx(ctx).Where(do.VerificationCase{TenantId: idccap.DefaultTenantID, CustomerNo: subject.CustomerNo, SubjectTypeCode: personalSubjectType}).WhereIn("status_code", []caseStatusCode{caseStatusDraft, caseStatusPending, caseStatusVerifying, caseStatusReviewRequired}).WhereLT("created_at", subject.VerificationUpdatedAt).Data(do.VerificationCase{StatusCode: caseStatusCanceled, Version: gdb.Raw("version + 1")}).Update()
		if err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return bizerr.WrapCode(err, CodeUnavailable)
		}
		if count == 0 {
			return nil
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "compliance.personal-verification.clear", IdempotencyKey: subject.CustomerNo + subject.VerificationUpdatedAt.UTC().Format(time.RFC3339Nano), OperationCode: "compliance.personal-verification.clear", TargetType: "customer", TargetNo: subject.CustomerNo, CustomerNo: subject.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: string(reasonManualClear), RiskCode: "low", CorrelationNo: subject.CustomerNo, BeforeSnapshot: struct{ Count int64 }{count}, AfterSnapshot: struct{ Status caseStatusCode }{caseStatusCanceled}})
	})
}
