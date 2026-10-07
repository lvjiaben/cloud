// This file persists retained customer account workflows. Every authoritative
// mutation, consent record, durable task creation, and audit transition shares
// the customer-subject transaction; cross-domain closure data arrives only
// through the validated service projection defined in customer.go.

package customer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/audit"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

const (
	workflowNumberAttempts = 16

	legalChangeNumberType   = "customer_legal_change"
	legalChangeNumberOwner  = "customer_legal_change"
	legalChangeNumberPrefix = "LCR_"
	legalChangeOperation    = "customer.legal_change.request"

	accountTransitionNumberType   = "customer_account_transition"
	accountTransitionNumberOwner  = "customer_account_transition"
	accountTransitionNumberPrefix = "CAT_"
	accountTransitionOperation    = "customer.account.transition"

	closureNumberType   = "customer_closure"
	closureNumberOwner  = "customer_closure"
	closureNumberPrefix = "CLR_"
	closureOperation    = "customer.closure.request"

	dataExportNumberType   = "customer_data_export"
	dataExportNumberOwner  = "customer_data_export"
	dataExportNumberPrefix = "DEX_"
	dataExportOperation    = "customer.data_export.request"

	consentNumberPrefix           = "CNS_"
	consentNumberOwner            = "customer_consent"
	closureConsentType            = "account_closure"
	dataExportConsentType         = "data_export"
	closurePolicyReference        = "idc://customer/account-closure"
	dataExportPolicyReference     = "idc://customer/data-export"
	customerWorkflowPolicyVersion = "1"
)

type legalChangeAuditSnapshot struct {
	Status             LegalChangeStatus  `json:"status"`
	Reason             LegalChangeReason  `json:"reason"`
	Fields             []LegalChangeField `json:"fields"`
	ProfileVersion     int64              `json:"profile_version"`
	VerificationStatus VerificationStatus `json:"verification_status"`
	VerificationCaseNo string             `json:"verification_case_no"`
}

func (p *daoPersistence) requestOwnLegalChange(
	ctx context.Context,
	actorUserID int,
	input normalizedLegalChange,
) (*LegalChangeRequest, error) {
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	fingerprint, err := input.fingerprint()
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint legal change request")
	}
	idempotencyDigest := digest("customer-legal-change-idempotency-v1", input.idempotencyKey)
	var result *LegalChangeRequest
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		profile, loadErr := p.loadProfile(ctx, input.customerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if profile == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		existing, loadErr := loadLegalChangeByIdempotency(ctx, input.customerNo, idempotencyDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint || existing.SubmittedByUserId != actorUserID {
				return bizerr.NewCode(CodeIdempotencyConflict)
			}
			result, loadErr = projectLegalChange(existing)
			return loadErr
		}
		if !statusAllowsOwnMutation(profile.Subject.Status) {
			return bizerr.NewCode(CodeStateDenied)
		}
		verificationStatus, verificationCaseNo, profileVersion, verifyErr :=
			verifiedLegalProjection(profile, input)
		if verifyErr != nil {
			return verifyErr
		}
		active, loadErr := hasActiveLegalChange(ctx, input.customerNo)
		if loadErr != nil {
			return loadErr
		}
		if active {
			return bizerr.NewCode(CodeStateDenied)
		}
		now := databaseTime(p.now())
		requestNo, reserveErr := p.reserveWorkflowNumber(
			ctx, legalChangeNumberType, legalChangeNumberPrefix, input.customerNo,
			idempotencyDigest,
		)
		if reserveErr != nil {
			return reserveErr
		}
		rowID, insertErr := dao.CustomerLegalChangeRequest.Ctx(ctx).Data(do.CustomerLegalChangeRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: requestNo,
			CustomerNo: input.customerNo, CustomerTypeCode: string(profile.Subject.Type),
			StatusCode: string(LegalChangeStatusSubmitted), ReasonCode: string(input.reason),
			ChangeLegalName:               input.changeLegalName,
			ChangeLegalIdentifier:         input.changeLegalIdentifier,
			CurrentVerificationStatusCode: string(verificationStatus),
			CurrentVerificationCaseNo:     verificationCaseNo,
			CurrentProfileVersion:         profileVersion,
			IdempotencyKeyDigest:          idempotencyDigest, RequestFingerprint: fingerprint,
			SubmittedByUserId: actorUserID, SubmittedAt: &now,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer legal change request")
		}
		if assignErr := p.assignWorkflowNumber(
			ctx, requestNo, legalChangeNumberOwner, rowID, now,
		); assignErr != nil {
			return assignErr
		}
		fields := legalChangeFields(input.changeLegalName, input.changeLegalIdentifier)
		_, auditErr := p.audit.AppendTransition(ctx, audit.TransitionInput{
			Fields: audit.Fields{
				IdempotencyScope: legalChangeOperation, IdempotencyKey: input.idempotencyKey,
				OperationCode: legalChangeOperation, TargetType: legalChangeNumberOwner,
				TargetNo: requestNo, CustomerNo: input.customerNo,
				Result: audit.ResultSucceeded, ReasonCode: string(input.reason), Risk: audit.RiskHigh,
				ContextSnapshot: map[string]any{"source": "customer_self_service", "raw_material_stored": false},
			},
			BeforeSnapshot: legalChangeAuditSnapshot{
				ProfileVersion: profileVersion, VerificationStatus: verificationStatus,
				VerificationCaseNo: verificationCaseNo,
			},
			AfterSnapshot: legalChangeAuditSnapshot{
				Status: LegalChangeStatusSubmitted, Reason: input.reason, Fields: fields,
				ProfileVersion: profileVersion, VerificationStatus: verificationStatus,
				VerificationCaseNo: verificationCaseNo,
			},
		})
		if auditErr != nil {
			return gerror.Wrap(auditErr, "append legal change audit transition")
		}
		result = &LegalChangeRequest{
			RequestNo: requestNo, CustomerNo: input.customerNo, CustomerType: profile.Subject.Type,
			Status: LegalChangeStatusSubmitted, Reason: input.reason, Fields: fields,
			VerificationStatus: verificationStatus, VerificationCaseNo: verificationCaseNo,
			CurrentProfileVersion: profileVersion, SubmittedByUserID: actorUserID, SubmittedAt: now,
		}
		return nil
	})
	return result, err
}

func verifiedLegalProjection(
	profile *Profile,
	input normalizedLegalChange,
) (VerificationStatus, string, int64, error) {
	switch profile.Subject.Type {
	case TypeIndividual:
		if input.reason == LegalChangeReasonEnterpriseRegistration || profile.Individual == nil ||
			profile.Individual.IdentityStatus != VerificationVerified ||
			profile.Individual.Version != input.expectedProfileVersion ||
			!validBoundedText(profile.Individual.IdentityCaseNo, 40) {
			return "", "", 0, bizerr.NewCode(CodeStateDenied)
		}
		if (input.reason == LegalChangeReasonNameChange && !input.changeLegalName) ||
			(input.reason == LegalChangeReasonIdentifierCorrection && !input.changeLegalIdentifier) {
			return "", "", 0, bizerr.NewCode(CodeWorkflowInvalid)
		}
		return profile.Individual.IdentityStatus, profile.Individual.IdentityCaseNo,
			profile.Individual.Version, nil
	case TypeEnterprise:
		if profile.Enterprise == nil || profile.Enterprise.VerificationStatus != VerificationVerified ||
			profile.Enterprise.Version != input.expectedProfileVersion ||
			!validBoundedText(profile.Enterprise.VerificationCaseNo, 40) {
			return "", "", 0, bizerr.NewCode(CodeStateDenied)
		}
		if (input.reason == LegalChangeReasonNameChange && !input.changeLegalName) ||
			(input.reason == LegalChangeReasonIdentifierCorrection && !input.changeLegalIdentifier) {
			return "", "", 0, bizerr.NewCode(CodeWorkflowInvalid)
		}
		return profile.Enterprise.VerificationStatus, profile.Enterprise.VerificationCaseNo,
			profile.Enterprise.Version, nil
	default:
		return "", "", 0, gerror.New("customer legal projection has unsupported type")
	}
}

func loadLegalChangeByIdempotency(
	ctx context.Context,
	customerNo string,
	idempotencyDigest string,
) (*entity.CustomerLegalChangeRequest, error) {
	var row *entity.CustomerLegalChangeRequest
	if err := dao.CustomerLegalChangeRequest.Ctx(ctx).Where(do.CustomerLegalChangeRequest{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load legal change request by idempotency")
	}
	return row, nil
}

func (p *daoPersistence) currentOwnLegalChange(
	ctx context.Context,
	customerNo string,
) (*LegalChangeRequest, error) {
	var row *entity.CustomerLegalChangeRequest
	err := dao.CustomerLegalChangeRequest.Ctx(ctx).
		Where(do.CustomerLegalChangeRequest{
			TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		}).
		WhereIn(
			dao.CustomerLegalChangeRequest.Columns().StatusCode,
			[]string{string(LegalChangeStatusSubmitted), string(LegalChangeStatusUnderReview)},
		).
		OrderDesc(dao.CustomerLegalChangeRequest.Columns().SubmittedAt).
		OrderDesc(dao.CustomerLegalChangeRequest.Columns().Id).
		Limit(1).
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load current legal change request")
	}
	if row == nil {
		return nil, nil
	}
	return projectLegalChange(row)
}

func hasActiveLegalChange(ctx context.Context, customerNo string) (bool, error) {
	count, err := dao.CustomerLegalChangeRequest.Ctx(ctx).
		Where(do.CustomerLegalChangeRequest{TenantId: idccap.DefaultTenantID, CustomerNo: customerNo}).
		WhereIn(dao.CustomerLegalChangeRequest.Columns().StatusCode,
			[]string{string(LegalChangeStatusSubmitted), string(LegalChangeStatusUnderReview)}).
		Count()
	if err != nil {
		return false, gerror.Wrap(err, "check active legal change request")
	}
	return count > 0, nil
}

func projectLegalChange(row *entity.CustomerLegalChangeRequest) (*LegalChangeRequest, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || !validCustomerNo(row.CustomerNo) ||
		!validWorkflowNo(row.RequestNo, legalChangeNumberPrefix) || !validType(Type(row.CustomerTypeCode)) ||
		!validLegalChangeStatus(LegalChangeStatus(row.StatusCode)) ||
		!validLegalChangeReason(LegalChangeReason(row.ReasonCode)) ||
		(!row.ChangeLegalName && !row.ChangeLegalIdentifier) ||
		VerificationStatus(row.CurrentVerificationStatusCode) != VerificationVerified ||
		!validBoundedText(row.CurrentVerificationCaseNo, 40) || row.CurrentProfileVersion <= 0 ||
		row.SubmittedByUserId <= 0 || row.SubmittedAt == nil {
		return nil, gerror.New("legal change request projection is inconsistent")
	}
	return &LegalChangeRequest{
		RequestNo: row.RequestNo, CustomerNo: row.CustomerNo, CustomerType: Type(row.CustomerTypeCode),
		Status: LegalChangeStatus(row.StatusCode), Reason: LegalChangeReason(row.ReasonCode),
		Fields:                legalChangeFields(row.ChangeLegalName, row.ChangeLegalIdentifier),
		VerificationStatus:    VerificationStatus(row.CurrentVerificationStatusCode),
		VerificationCaseNo:    row.CurrentVerificationCaseNo,
		CurrentProfileVersion: row.CurrentProfileVersion,
		SubmittedByUserID:     row.SubmittedByUserId, SubmittedAt: databaseTime(*row.SubmittedAt),
	}, nil
}

func validLegalChangeStatus(status LegalChangeStatus) bool {
	switch status {
	case LegalChangeStatusSubmitted, LegalChangeStatusUnderReview, LegalChangeStatusApproved,
		LegalChangeStatusRejected, LegalChangeStatusCancelled:
		return true
	default:
		return false
	}
}

func legalChangeFields(changeName bool, changeIdentifier bool) []LegalChangeField {
	fields := make([]LegalChangeField, 0, 2)
	if changeName {
		fields = append(fields, LegalChangeFieldName)
	}
	if changeIdentifier {
		fields = append(fields, LegalChangeFieldIdentifier)
	}
	return fields
}

type accountTransitionAuditSnapshot struct {
	Status  Status `json:"status"`
	Version int64  `json:"version"`
}

func (p *daoPersistence) transitionAccount(
	ctx context.Context,
	actorUserID int,
	input normalizedAccountTransition,
) (*AccountStateTransition, error) {
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	fingerprint, err := input.fingerprint()
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint account transition")
	}
	idempotencyDigest := digest("customer-account-transition-idempotency-v1", input.idempotencyKey)
	var result *AccountStateTransition
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		profile, loadErr := p.loadProfile(ctx, input.customerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if profile == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		existing, loadErr := loadAccountTransitionByIdempotency(ctx, input.customerNo, idempotencyDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint || existing.ActorUserId != actorUserID {
				return bizerr.NewCode(CodeIdempotencyConflict)
			}
			result, loadErr = projectAccountTransition(existing)
			return loadErr
		}
		if profile.Subject.Version != input.expectedVersion {
			return bizerr.NewCode(CodeVersionConflict)
		}
		if !validAccountTransition(profile.Subject.Status, input) {
			return bizerr.NewCode(CodeStateDenied)
		}
		now := databaseTime(p.now())
		nextVersion := profile.Subject.Version + 1
		transitionNo, reserveErr := p.reserveWorkflowNumber(
			ctx, accountTransitionNumberType, accountTransitionNumberPrefix,
			input.customerNo, idempotencyDigest,
		)
		if reserveErr != nil {
			return reserveErr
		}
		updateResult, updateErr := dao.CustomerSubject.Ctx(ctx).
			Where(do.CustomerSubject{
				TenantId: idccap.DefaultTenantID, CustomerNo: input.customerNo,
				Version: profile.Subject.Version,
			}).
			Data(do.CustomerSubject{
				StatusCode: string(input.toStatus), StatusReasonCode: string(input.reason),
				StatusReason: "", StatusChangedBy: actorUserID, StatusChangedAt: &now,
				Version: nextVersion, UpdatedBy: actorUserID, UpdatedAt: &now,
			}).Update()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "update customer account state")
		}
		affected, updateErr := updateResult.RowsAffected()
		if updateErr != nil {
			return gerror.Wrap(updateErr, "read customer account state update result")
		}
		if affected != 1 {
			return bizerr.NewCode(CodeVersionConflict)
		}
		rowID, insertErr := dao.CustomerAccountStateHistory.Ctx(ctx).Data(do.CustomerAccountStateHistory{
			TenantId: idccap.DefaultTenantID, TransitionNo: transitionNo,
			CustomerNo: input.customerNo, FromStatusCode: string(profile.Subject.Status),
			ToStatusCode: string(input.toStatus), ReasonCode: string(input.reason),
			SourceCode: "staff", ActorUserId: actorUserID, SubjectVersion: nextVersion,
			IdempotencyKeyDigest: idempotencyDigest, RequestFingerprint: fingerprint,
			OccurredAt: &now,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer account state history")
		}
		if assignErr := p.assignWorkflowNumber(
			ctx, transitionNo, accountTransitionNumberOwner, rowID, now,
		); assignErr != nil {
			return assignErr
		}
		_, auditErr := p.audit.AppendTransition(ctx, audit.TransitionInput{
			Fields: audit.Fields{
				IdempotencyScope: accountTransitionOperation, IdempotencyKey: input.idempotencyKey,
				OperationCode: accountTransitionOperation, TargetType: idempotencyResourceCustomer,
				TargetNo: input.customerNo, CustomerNo: input.customerNo,
				Result: audit.ResultSucceeded, ReasonCode: string(input.reason), Risk: audit.RiskHigh,
				CorrelationNo: transitionNo, ContextSnapshot: map[string]any{"source": "staff"},
			},
			BeforeSnapshot: accountTransitionAuditSnapshot{Status: profile.Subject.Status, Version: profile.Subject.Version},
			AfterSnapshot:  accountTransitionAuditSnapshot{Status: input.toStatus, Version: nextVersion},
		})
		if auditErr != nil {
			return gerror.Wrap(auditErr, "append account transition audit")
		}
		result = &AccountStateTransition{
			TransitionNo: transitionNo, CustomerNo: input.customerNo,
			FromStatus: profile.Subject.Status, ToStatus: input.toStatus, Reason: input.reason,
			ActorUserID: actorUserID, SubjectVersion: nextVersion, OccurredAt: now,
		}
		return nil
	})
	return result, err
}

func loadAccountTransitionByIdempotency(
	ctx context.Context,
	customerNo string,
	idempotencyDigest string,
) (*entity.CustomerAccountStateHistory, error) {
	var row *entity.CustomerAccountStateHistory
	if err := dao.CustomerAccountStateHistory.Ctx(ctx).Where(do.CustomerAccountStateHistory{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load account transition by idempotency")
	}
	return row, nil
}

func projectAccountTransition(row *entity.CustomerAccountStateHistory) (*AccountStateTransition, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || !validCustomerNo(row.CustomerNo) ||
		!validWorkflowNo(row.TransitionNo, accountTransitionNumberPrefix) ||
		!validStatus(Status(row.FromStatusCode)) || !validStatus(Status(row.ToStatusCode)) ||
		Status(row.ToStatusCode) == StatusAnonymized || row.SourceCode != "staff" ||
		!validAccountTransitionReason(AccountTransitionReason(row.ReasonCode)) ||
		row.ActorUserId <= 0 || row.SubjectVersion <= 1 || row.OccurredAt == nil {
		return nil, gerror.New("account transition projection is inconsistent")
	}
	normalized := normalizedAccountTransition{
		toStatus: Status(row.ToStatusCode), reason: AccountTransitionReason(row.ReasonCode),
	}
	if !validAccountTransition(Status(row.FromStatusCode), normalized) {
		return nil, gerror.New("account transition projection violates state matrix")
	}
	return &AccountStateTransition{
		TransitionNo: row.TransitionNo, CustomerNo: row.CustomerNo,
		FromStatus: Status(row.FromStatusCode), ToStatus: Status(row.ToStatusCode),
		Reason: AccountTransitionReason(row.ReasonCode), ActorUserID: row.ActorUserId,
		SubjectVersion: row.SubjectVersion, OccurredAt: databaseTime(*row.OccurredAt),
	}, nil
}

type closureAuditSnapshot struct {
	Status                  ClosureStatus    `json:"status"`
	SubjectVersion          int64            `json:"subject_version"`
	Findings                []ClosureFinding `json:"findings"`
	PrecheckContractVersion int              `json:"precheck_contract_version"`
	RetentionPolicyCode     string           `json:"retention_policy_code"`
	AnonymizationPolicyCode string           `json:"anonymization_policy_code"`
}

func (p *daoPersistence) requestOwnClosure(
	ctx context.Context,
	actorUserID int,
	input normalizedClosureRequest,
	precheck ClosurePrecheckResult,
) (*ClosureRequest, error) {
	if actorUserID <= 0 || closureHasHardBlocker(precheck.Findings) {
		return nil, bizerr.NewCode(CodeWorkflowInvalid)
	}
	fingerprint, err := input.fingerprint()
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint closure request")
	}
	idempotencyDigest := digest("customer-closure-idempotency-v1", input.idempotencyKey)
	var result *ClosureRequest
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		profile, loadErr := p.loadProfile(ctx, input.customerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if profile == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		existing, loadErr := loadClosureByIdempotency(ctx, input.customerNo, idempotencyDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint || existing.RequestedByUserId != actorUserID {
				return bizerr.NewCode(CodeIdempotencyConflict)
			}
			result, loadErr = projectClosure(existing)
			return loadErr
		}
		if profile.Subject.Status == StatusAnonymized {
			return bizerr.NewCode(CodeStateDenied)
		}
		if profile.Subject.Version != input.expectedVersion {
			return bizerr.NewCode(CodeVersionConflict)
		}
		active, loadErr := hasActiveClosure(ctx, input.customerNo)
		if loadErr != nil {
			return loadErr
		}
		if active {
			return bizerr.NewCode(CodeStateDenied)
		}
		now := databaseTime(p.now())
		if precheck.CheckedAt.After(now) {
			return bizerr.NewCode(CodeWorkflowUnavailable)
		}
		consentNo, consentErr := p.createWorkflowConsent(
			ctx, actorUserID, input.customerNo, profile.Subject.PreferredLocale,
			closureConsentType, closurePolicyReference, input.idempotencyKey,
			fingerprint, now,
		)
		if consentErr != nil {
			return consentErr
		}
		requestNo, reserveErr := p.reserveWorkflowNumber(
			ctx, closureNumberType, closureNumberPrefix, input.customerNo, idempotencyDigest,
		)
		if reserveErr != nil {
			return reserveErr
		}
		counts := closureFindingCounts(precheck.Findings)
		rowID, insertErr := dao.CustomerClosureRequest.Ctx(ctx).Data(do.CustomerClosureRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: requestNo,
			CustomerNo: input.customerNo, ConsentNo: consentNo,
			StatusCode: string(ClosureStatusPendingReview), SubjectVersion: profile.Subject.Version,
			UnsettledFundsCount:      counts[ClosureFindingUnsettledFunds],
			ActiveServiceCount:       counts[ClosureFindingActiveServices],
			UnsettledInvoiceCount:    counts[ClosureFindingUnsettledInvoices],
			ActiveContractCount:      counts[ClosureFindingActiveContracts],
			InProgressOperationCount: counts[ClosureFindingInProgressOperations],
			LegalRetentionCount:      counts[ClosureFindingLegalRetention],
			PrecheckContractVersion:  precheck.ContractVersion, PrecheckedAt: &precheck.CheckedAt,
			RetentionPolicyCode:     closureRetentionPolicy,
			AnonymizationPolicyCode: closureAnonymizationPolicy,
			IdempotencyKeyDigest:    idempotencyDigest, RequestFingerprint: fingerprint,
			RequestedByUserId: actorUserID, ConfirmedAt: &now, RequestedAt: &now,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer closure request")
		}
		if assignErr := p.assignWorkflowNumber(
			ctx, requestNo, closureNumberOwner, rowID, now,
		); assignErr != nil {
			return assignErr
		}
		_, auditErr := p.audit.AppendTransition(ctx, audit.TransitionInput{
			Fields: audit.Fields{
				IdempotencyScope: closureOperation, IdempotencyKey: input.idempotencyKey,
				OperationCode: closureOperation, TargetType: closureNumberOwner,
				TargetNo: requestNo, CustomerNo: input.customerNo,
				Result: audit.ResultSucceeded, Risk: audit.RiskCritical,
				ContextSnapshot: map[string]any{
					"source": "customer_self_service", "consent_no": consentNo,
					"completion_requires_fresh_precheck": true,
				},
			},
			BeforeSnapshot: closureAuditSnapshot{SubjectVersion: profile.Subject.Version},
			AfterSnapshot: closureAuditSnapshot{
				Status: ClosureStatusPendingReview, SubjectVersion: profile.Subject.Version,
				Findings:                append([]ClosureFinding(nil), precheck.Findings...),
				PrecheckContractVersion: precheck.ContractVersion,
				RetentionPolicyCode:     closureRetentionPolicy,
				AnonymizationPolicyCode: closureAnonymizationPolicy,
			},
		})
		if auditErr != nil {
			return gerror.Wrap(auditErr, "append closure request audit")
		}
		result = &ClosureRequest{
			RequestNo: requestNo, CustomerNo: input.customerNo, ConsentNo: consentNo,
			Status: ClosureStatusPendingReview, SubjectVersion: profile.Subject.Version,
			Findings:                append([]ClosureFinding(nil), precheck.Findings...),
			PrecheckContractVersion: precheck.ContractVersion, PrecheckedAt: precheck.CheckedAt,
			RetentionPolicyCode:     closureRetentionPolicy,
			AnonymizationPolicyCode: closureAnonymizationPolicy,
			RequestedByUserID:       actorUserID, ConfirmedAt: now, RequestedAt: now,
		}
		return nil
	})
	return result, err
}

func loadClosureByIdempotency(
	ctx context.Context,
	customerNo string,
	idempotencyDigest string,
) (*entity.CustomerClosureRequest, error) {
	var row *entity.CustomerClosureRequest
	if err := dao.CustomerClosureRequest.Ctx(ctx).Where(do.CustomerClosureRequest{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load closure request by idempotency")
	}
	return row, nil
}

func hasActiveClosure(ctx context.Context, customerNo string) (bool, error) {
	count, err := dao.CustomerClosureRequest.Ctx(ctx).
		Where(do.CustomerClosureRequest{TenantId: idccap.DefaultTenantID, CustomerNo: customerNo}).
		WhereIn(dao.CustomerClosureRequest.Columns().StatusCode,
			[]string{string(ClosureStatusPendingReview), string(ClosureStatusApproved), string(ClosureStatusAnonymizing)}).
		Count()
	if err != nil {
		return false, gerror.Wrap(err, "check active closure request")
	}
	return count > 0, nil
}

func projectClosure(row *entity.CustomerClosureRequest) (*ClosureRequest, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || !validCustomerNo(row.CustomerNo) ||
		!validWorkflowNo(row.RequestNo, closureNumberPrefix) || !validConsentNo(row.ConsentNo) ||
		!validPersistedClosureStatus(ClosureStatus(row.StatusCode)) || row.SubjectVersion <= 0 ||
		row.PrecheckContractVersion != closurePrecheckContractVersion || row.PrecheckedAt == nil ||
		row.ConfirmedAt == nil || row.RequestedAt == nil || row.RequestedByUserId <= 0 ||
		row.RetentionPolicyCode != closureRetentionPolicy ||
		row.AnonymizationPolicyCode != closureAnonymizationPolicy {
		return nil, gerror.New("closure request projection is inconsistent")
	}
	findings, err := closureFindingsFromRow(row)
	if err != nil || closureHasHardBlocker(findings) {
		return nil, gerror.New("persisted closure request contains a hard blocker")
	}
	return &ClosureRequest{
		RequestNo: row.RequestNo, CustomerNo: row.CustomerNo, ConsentNo: row.ConsentNo,
		Status: ClosureStatus(row.StatusCode), SubjectVersion: row.SubjectVersion, Findings: findings,
		PrecheckContractVersion: row.PrecheckContractVersion,
		PrecheckedAt:            databaseTime(*row.PrecheckedAt),
		RetentionPolicyCode:     row.RetentionPolicyCode,
		AnonymizationPolicyCode: row.AnonymizationPolicyCode,
		RequestedByUserID:       row.RequestedByUserId,
		ConfirmedAt:             databaseTime(*row.ConfirmedAt), RequestedAt: databaseTime(*row.RequestedAt),
	}, nil
}

func validPersistedClosureStatus(status ClosureStatus) bool {
	switch status {
	case ClosureStatusPendingReview, ClosureStatusApproved, ClosureStatusRejected,
		ClosureStatusAnonymizing, ClosureStatusCompleted, ClosureStatusCancelled:
		return true
	default:
		return false
	}
}

func closureFindingCounts(findings []ClosureFinding) map[ClosureFindingCode]int {
	counts := make(map[ClosureFindingCode]int, len(findings))
	for _, finding := range findings {
		counts[finding.Code] = finding.Count
	}
	return counts
}

func closureFindingsFromRow(row *entity.CustomerClosureRequest) ([]ClosureFinding, error) {
	values := []ClosureFinding{
		{Code: ClosureFindingUnsettledFunds, Count: row.UnsettledFundsCount},
		{Code: ClosureFindingActiveServices, Count: row.ActiveServiceCount},
		{Code: ClosureFindingUnsettledInvoices, Count: row.UnsettledInvoiceCount},
		{Code: ClosureFindingActiveContracts, Count: row.ActiveContractCount},
		{Code: ClosureFindingInProgressOperations, Count: row.InProgressOperationCount},
		{Code: ClosureFindingLegalRetention, Count: row.LegalRetentionCount},
	}
	result := make([]ClosureFinding, 0, len(values))
	for _, finding := range values {
		if finding.Count < 0 || finding.Count > maximumClosureFindingCount {
			return nil, gerror.New("closure finding count is outside bounds")
		}
		if finding.Count > 0 {
			result = append(result, finding)
		}
	}
	return result, nil
}

type dataExportTaskPayload struct {
	SchemaVersion          int              `json:"schema_version"`
	Scope                  DataExportScope  `json:"scope"`
	Format                 DataExportFormat `json:"format"`
	CustomerSubjectVersion int64            `json:"customer_subject_version"`
}

type dataExportAuditSnapshot struct {
	Scope          DataExportScope  `json:"scope"`
	Format         DataExportFormat `json:"format"`
	SubjectVersion int64            `json:"subject_version"`
	TaskNo         string           `json:"task_no"`
	ConsentNo      string           `json:"consent_no"`
}

func (p *daoPersistence) findOwnDataExportReplay(
	ctx context.Context,
	actorUserID int,
	input normalizedDataExport,
) (*DataExportRequest, error) {
	if actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	fingerprint, err := input.fingerprint()
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint data export replay")
	}
	idempotencyDigest := digest("customer-data-export-idempotency-v1", input.idempotencyKey)
	row, err := loadDataExportByIdempotency(ctx, input.customerNo, idempotencyDigest)
	if err != nil || row == nil {
		return nil, err
	}
	if row.RequestFingerprint != fingerprint || row.RequestedByUserId != actorUserID {
		return nil, bizerr.NewCode(CodeIdempotencyConflict)
	}
	return projectDataExport(row)
}

func (p *daoPersistence) requestOwnDataExport(
	ctx context.Context,
	actorUserID int,
	input normalizedDataExport,
	proof *reauth.Proof,
) (*DataExportRequest, error) {
	if actorUserID <= 0 || proof == nil || p.tasks == nil {
		return nil, bizerr.NewCode(CodeWorkflowUnavailable)
	}
	fingerprint, err := input.fingerprint()
	if err != nil {
		return nil, gerror.Wrap(err, "fingerprint data export request")
	}
	idempotencyDigest := digest("customer-data-export-idempotency-v1", input.idempotencyKey)
	var result *DataExportRequest
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		profile, loadErr := p.loadProfile(ctx, input.customerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if profile == nil {
			return bizerr.NewCode(CodeNotAccessible)
		}
		existing, loadErr := loadDataExportByIdempotency(ctx, input.customerNo, idempotencyDigest)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestFingerprint != fingerprint || existing.RequestedByUserId != actorUserID {
				return bizerr.NewCode(CodeIdempotencyConflict)
			}
			result, loadErr = projectDataExport(existing)
			return loadErr
		}
		if profile.Subject.Status == StatusAnonymized {
			return bizerr.NewCode(CodeStateDenied)
		}
		if profile.Subject.Version != input.expectedVersion {
			return bizerr.NewCode(CodeVersionConflict)
		}
		now := databaseTime(p.now())
		consentNo, consentErr := p.createWorkflowConsent(
			ctx, actorUserID, input.customerNo, profile.Subject.PreferredLocale,
			dataExportConsentType, dataExportPolicyReference, input.idempotencyKey,
			fingerprint, now,
		)
		if consentErr != nil {
			return consentErr
		}
		payload, marshalErr := json.Marshal(dataExportTaskPayload{
			SchemaVersion: 1, Scope: input.scope, Format: input.format,
			CustomerSubjectVersion: profile.Subject.Version,
		})
		if marshalErr != nil {
			return gerror.Wrap(marshalErr, "encode redacted data export task payload")
		}
		taskResult, taskErr := p.tasks.Create(ctx, asynctask.CreateInput{
			TaskType: dataExportTaskType, SubjectType: dataExportTaskSubjectType,
			SubjectNo: input.customerNo, CustomerNo: input.customerNo,
			IdempotencyKey: input.idempotencyKey, Payload: payload,
		})
		if taskErr != nil {
			return gerror.Wrap(taskErr, "create customer data export task")
		}
		if taskResult == nil || !validWorkflowNo(taskResult.Task.TaskNo, "TSK_") {
			return gerror.New("data export task creation returned an invalid task number")
		}
		requestNo, reserveErr := p.reserveWorkflowNumber(
			ctx, dataExportNumberType, dataExportNumberPrefix, input.customerNo, idempotencyDigest,
		)
		if reserveErr != nil {
			return reserveErr
		}
		issuedAt := databaseTime(proof.IssuedAt)
		rowID, insertErr := dao.CustomerDataExportRequest.Ctx(ctx).Data(do.CustomerDataExportRequest{
			TenantId: idccap.DefaultTenantID, RequestNo: requestNo,
			CustomerNo: input.customerNo, ConsentNo: consentNo, TaskNo: taskResult.Task.TaskNo,
			ScopeCode: string(input.scope), FormatCode: string(input.format),
			SubjectVersion:       profile.Subject.Version,
			IdempotencyKeyDigest: idempotencyDigest, RequestFingerprint: fingerprint,
			ReauthIssuedAt: &issuedAt, RequestedByUserId: actorUserID, RequestedAt: &now,
		}).InsertAndGetId()
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert customer data export request")
		}
		if assignErr := p.assignWorkflowNumber(
			ctx, requestNo, dataExportNumberOwner, rowID, now,
		); assignErr != nil {
			return assignErr
		}
		_, auditErr := p.audit.AppendTransition(ctx, audit.TransitionInput{
			Fields: audit.Fields{
				IdempotencyScope: dataExportOperation, IdempotencyKey: input.idempotencyKey,
				OperationCode: dataExportOperation, TargetType: dataExportNumberOwner,
				TargetNo: requestNo, CustomerNo: input.customerNo,
				Result: audit.ResultSucceeded, Risk: audit.RiskHigh,
				CorrelationNo: taskResult.Task.TaskNo,
				ContextSnapshot: map[string]any{
					"source": "customer_self_service", "recent_authentication": true,
					"artifact_private": true, "artifact_time_limited": true,
				},
			},
			BeforeSnapshot: dataExportAuditSnapshot{SubjectVersion: profile.Subject.Version},
			AfterSnapshot: dataExportAuditSnapshot{
				Scope: input.scope, Format: input.format, SubjectVersion: profile.Subject.Version,
				TaskNo: taskResult.Task.TaskNo, ConsentNo: consentNo,
			},
		})
		if auditErr != nil {
			return gerror.Wrap(auditErr, "append data export request audit")
		}
		result = &DataExportRequest{
			RequestNo: requestNo, CustomerNo: input.customerNo, ConsentNo: consentNo,
			TaskNo: taskResult.Task.TaskNo, Scope: input.scope, Format: input.format,
			SubjectVersion:    profile.Subject.Version,
			RequestedByUserID: actorUserID, RequestedAt: now,
		}
		return nil
	})
	return result, err
}

func loadDataExportByIdempotency(
	ctx context.Context,
	customerNo string,
	idempotencyDigest string,
) (*entity.CustomerDataExportRequest, error) {
	var row *entity.CustomerDataExportRequest
	if err := dao.CustomerDataExportRequest.Ctx(ctx).Where(do.CustomerDataExportRequest{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
		IdempotencyKeyDigest: idempotencyDigest,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load data export request by idempotency")
	}
	return row, nil
}

func projectDataExport(row *entity.CustomerDataExportRequest) (*DataExportRequest, error) {
	if row == nil || row.TenantId != idccap.DefaultTenantID || !validCustomerNo(row.CustomerNo) ||
		!validWorkflowNo(row.RequestNo, dataExportNumberPrefix) || !validConsentNo(row.ConsentNo) ||
		!validWorkflowNo(row.TaskNo, "TSK_") || DataExportScope(row.ScopeCode) != DataExportScopeOwnDeliverable ||
		DataExportFormat(row.FormatCode) != DataExportFormatJSONZip || row.SubjectVersion <= 0 ||
		row.ReauthIssuedAt == nil || row.RequestedByUserId <= 0 || row.RequestedAt == nil {
		return nil, gerror.New("data export request projection is inconsistent")
	}
	return &DataExportRequest{
		RequestNo: row.RequestNo, CustomerNo: row.CustomerNo, ConsentNo: row.ConsentNo,
		TaskNo: row.TaskNo, Scope: DataExportScope(row.ScopeCode), Format: DataExportFormat(row.FormatCode),
		SubjectVersion: row.SubjectVersion, RequestedByUserID: row.RequestedByUserId,
		RequestedAt: databaseTime(*row.RequestedAt),
	}, nil
}

func (p *daoPersistence) createWorkflowConsent(
	ctx context.Context,
	actorUserID int,
	customerNo string,
	locale string,
	consentType string,
	policyReference string,
	idempotencyKey string,
	workflowFingerprint string,
	now time.Time,
) (string, error) {
	if !validLocale(locale) {
		return "", gerror.New("customer consent locale is inconsistent")
	}
	numberType := "customer_consent_" + consentType
	idempotencyDigest := digest(numberType+"-idempotency-v1", idempotencyKey)
	consentNo, err := p.reserveWorkflowNumber(
		ctx, numberType, consentNumberPrefix, customerNo, idempotencyDigest,
	)
	if err != nil {
		return "", err
	}
	var existing *entity.CustomerConsent
	if err = dao.CustomerConsent.Ctx(ctx).Where(do.CustomerConsent{
		TenantId: idccap.DefaultTenantID, ConsentNo: consentNo, CustomerNo: customerNo,
	}).Scan(&existing); err != nil {
		return "", gerror.Wrap(err, "load workflow consent")
	}
	if existing != nil {
		if existing.ConsentTypeCode != consentType || existing.PolicyReference != policyReference ||
			existing.PolicyVersion != customerWorkflowPolicyVersion || existing.ActionCode != "granted" ||
			existing.SourceCode != "portal" || existing.Locale != locale || existing.RecordedBy != actorUserID {
			return "", bizerr.NewCode(CodeIdempotencyConflict)
		}
		return existing.ConsentNo, nil
	}
	evidenceDigest := digest(
		"customer-workflow-consent-evidence-v1",
		customerNo+"\x00"+consentType+"\x00"+policyReference+"\x00"+
			customerWorkflowPolicyVersion+"\x00"+locale+"\x00"+workflowFingerprint,
	)
	rowID, err := dao.CustomerConsent.Ctx(ctx).Data(do.CustomerConsent{
		TenantId: idccap.DefaultTenantID, ConsentNo: consentNo, CustomerNo: customerNo,
		ConsentTypeCode: consentType, PolicyReference: policyReference,
		PolicyVersion: customerWorkflowPolicyVersion, ActionCode: "granted",
		SourceCode: "portal", Locale: locale, EvidenceDigest: evidenceDigest,
		OccurredAt: &now, RecordedBy: actorUserID,
	}).InsertAndGetId()
	if err != nil {
		return "", gerror.Wrap(err, "insert workflow consent")
	}
	if err = p.assignWorkflowNumber(ctx, consentNo, consentNumberOwner, rowID, now); err != nil {
		return "", err
	}
	return consentNo, nil
}

func (p *daoPersistence) reserveWorkflowNumber(
	ctx context.Context,
	numberType string,
	prefix string,
	customerNo string,
	idempotencyDigest string,
) (string, error) {
	reservationScope := digest("customer-workflow-number-scope-v1", numberType+"\x00"+customerNo)
	request := do.BusinessNumber{
		TenantId: idccap.DefaultTenantID, NumberType: numberType,
		ReservationScopeDigest: reservationScope, IdempotencyKeyDigest: idempotencyDigest,
	}
	if existing, err := loadReservedWorkflowNumber(ctx, request); err != nil || existing != "" {
		return existing, err
	}
	for range workflowNumberAttempts {
		candidate, err := newWorkflowNumber(p.random, prefix)
		if err != nil {
			return "", err
		}
		insertResult, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId: idccap.DefaultTenantID, NumberType: numberType, BusinessNo: candidate,
			ReservationScopeDigest: reservationScope, IdempotencyKeyDigest: idempotencyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve customer workflow number")
		}
		affected, err := insertResult.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read customer workflow number reservation")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, err := loadReservedWorkflowNumber(ctx, request); err != nil || existing != "" {
			return existing, err
		}
	}
	return "", gerror.New("customer workflow number collision retry limit reached")
}

func loadReservedWorkflowNumber(ctx context.Context, request do.BusinessNumber) (string, error) {
	var row *entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).Where(request).Scan(&row); err != nil {
		return "", gerror.Wrap(err, "load customer workflow number reservation")
	}
	if row == nil {
		return "", nil
	}
	return row.BusinessNo, nil
}

func (p *daoPersistence) assignWorkflowNumber(
	ctx context.Context,
	businessNo string,
	ownerType string,
	ownerInternalID int64,
	now time.Time,
) error {
	result, err := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{TenantId: idccap.DefaultTenantID, BusinessNo: businessNo}).
		Data(do.BusinessNumber{
			OwnerType: ownerType, OwnerInternalId: ownerInternalID, AssignedAt: &now,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign customer workflow number")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read customer workflow number assignment")
	}
	if affected != 1 {
		return gerror.New("customer workflow number reservation disappeared before assignment")
	}
	return nil
}

func newWorkflowNumber(random io.Reader, prefix string) (string, error) {
	if random == nil {
		return "", gerror.New("customer workflow number generator is unavailable")
	}
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate customer workflow number")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

func validWorkflowNo(value string, prefix string) bool {
	if len(value) != len(prefix)+32 || len(prefix) == 0 || value[:len(prefix)] != prefix {
		return false
	}
	for _, character := range value[len(prefix):] {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func validConsentNo(value string) bool {
	return validWorkflowNo(value, consentNumberPrefix)
}
