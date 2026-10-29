// This file implements protected customer and management credit command entry points.

package billing

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

// AccountCreditApplicationIntent carries a customer request without an externally supplied owner.
type AccountCreditApplicationIntent struct {
	CurrencyCode          string
	RequestedLimitMinor   int64
	RequestedValidFrom    time.Time
	RequestedValidUntil   time.Time
	RiskEvidenceReference string
	RiskEvidenceDigest    string
	UploadNos             []string
	IdempotencyKey        string
}

// AccountCreditBalanceRepaymentIntent binds a statement, amount, and stable request identity.
type AccountCreditBalanceRepaymentIntent struct {
	StatementNo string
	AmountMinor int64
	RequestNo   string
}

// ManagementCreditPolicyIntent is the complete immutable policy preview.
type ManagementCreditPolicyIntent struct {
	RequestNo                string
	CurrencyCode             string
	MaximumLimitMinor        int64
	DualReviewThresholdMinor int64
	BillingDay               int
	RepaymentPeriodDays      int
	GracePeriodDays          int
	DailyLateFeeBasisPoints  int
	MaximumLateFeeMinor      int64
}

// ManagementCreditReviewIntent is the complete independent application-review preview.
type ManagementCreditReviewIntent struct {
	RequestNo      string
	ApplicationNo  string
	Decision       CreditDecision
	CommentDigest  string
	EvidenceDigest string
}

// AccountCreditCommandService owns customer credit application and protected
// same-currency balance repayment. Customer identity is never accepted from a request.
type AccountCreditCommandService interface {
	ReserveCreditEvidence(context.Context, string, int64) (*CreditEvidenceReservation, error)
	UploadCreditEvidence(context.Context, string, string, io.Reader) (*CreditEvidenceItem, error)
	// SubmitCreditApplication derives ownership from the current customer and atomically audits the submitted request.
	SubmitCreditApplication(context.Context, AccountCreditApplicationIntent) (*CreditApplication, error)
	// IssueCreditBalanceRepaymentProof validates ownership and recent login before returning a short-lived proof.
	IssueCreditBalanceRepaymentProof(context.Context, AccountCreditBalanceRepaymentIntent) (*reauth.ProofContext, error)
	// RepayCreditStatementFromBalance consumes an exact proof and atomically audits the same-currency balance repayment.
	RepayCreditStatementFromBalance(context.Context, AccountCreditBalanceRepaymentIntent, string) (*CreditBalanceRepayment, error)
}

// ManagementCreditCommandService owns the two currently safe staff command
// boundaries. Write-off and restoration remain closed until their independent
// approval evidence owners are composed.
type ManagementCreditCommandService interface {
	// IssueCreditPolicyProof authorizes an immutable policy preview and returns a short-lived proof.
	IssueCreditPolicyProof(context.Context, ManagementCreditPolicyIntent) (*reauth.ProofContext, error)
	// ConfigureCreditPolicy consumes an exact proof and atomically appends and audits one policy revision.
	ConfigureCreditPolicy(context.Context, ManagementCreditPolicyIntent, string) (*CreditPolicy, error)
	// IssueCreditApplicationReviewProof authorizes an independent review preview and returns a short-lived proof.
	IssueCreditApplicationReviewProof(context.Context, ManagementCreditReviewIntent) (*reauth.ProofContext, error)
	// ReviewCreditApplication consumes an exact proof and atomically appends and audits the independent review.
	ReviewCreditApplication(context.Context, ManagementCreditReviewIntent, string) (*CreditApplication, error)
}

type accountCreditCommandImpl struct {
	database   gdb.DB
	principal  principal.Reader
	audit      idccap.BusinessAuditWriter
	recentAuth reauth.Service
	approval   CreditApprovalService
	usage      CreditUsageService
	evidence   CreditEvidenceService
}

type managementCreditCommandImpl struct {
	database   gdb.DB
	audit      idccap.BusinessAuditWriter
	authorizer ManagementCreditAuthorizer
	recentAuth reauth.Service
	approval   CreditApprovalService
}

type creditApplicationCommandStatus string

const creditApplicationCommandStatusPendingReview creditApplicationCommandStatus = "pending_review"

var _ AccountCreditCommandService = (*accountCreditCommandImpl)(nil)
var _ ManagementCreditCommandService = (*managementCreditCommandImpl)(nil)

// NewAccountCreditCommandService constructs customer credit commands from startup-owned dependencies.
func NewAccountCreditCommandService(
	database gdb.DB,
	principalReader principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
	recentAuth reauth.Service,
	approval CreditApprovalService,
	usage CreditUsageService,
) (AccountCreditCommandService, error) {
	for _, dependency := range []any{database, principalReader, auditWriter, recentAuth, approval, usage} {
		if isNilFinanceDirectoryDependency(dependency) {
			return nil, gerror.New("account credit commands require database, principal, audit, recent authentication, approval, and usage services")
		}
	}
	return &accountCreditCommandImpl{
		database: database, principal: principalReader, audit: auditWriter,
		recentAuth: recentAuth, approval: approval, usage: usage,
	}, nil
}

// NewAccountCreditCommandServiceWithEvidence enables customer-safe evidence uploads.
func NewAccountCreditCommandServiceWithEvidence(
	database gdb.DB,
	principalReader principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
	recentAuth reauth.Service,
	approval CreditApprovalService,
	usage CreditUsageService,
	evidence CreditEvidenceService,
) (AccountCreditCommandService, error) {
	service, err := NewAccountCreditCommandService(database, principalReader, auditWriter, recentAuth, approval, usage)
	if err != nil {
		return nil, err
	}
	if isNilFinanceDirectoryDependency(evidence) {
		return nil, gerror.New("account credit commands require evidence service")
	}
	impl, ok := service.(*accountCreditCommandImpl)
	if !ok {
		return nil, gerror.New("account credit command implementation is unavailable")
	}
	impl.evidence = evidence
	return impl, nil
}

// NewManagementCreditCommandService constructs protected staff credit commands from startup-owned dependencies.
func NewManagementCreditCommandService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementCreditAuthorizer,
	recentAuth reauth.Service,
	approval CreditApprovalService,
) (ManagementCreditCommandService, error) {
	for _, dependency := range []any{database, auditWriter, authorizer, recentAuth, approval} {
		if isNilFinanceDirectoryDependency(dependency) {
			return nil, gerror.New("management credit commands require database, audit, authorization, recent authentication, and approval services")
		}
	}
	return &managementCreditCommandImpl{
		database: database, audit: auditWriter, authorizer: authorizer,
		recentAuth: recentAuth, approval: approval,
	}, nil
}

func (s *accountCreditCommandImpl) ReserveCreditEvidence(ctx context.Context, fileName string, size int64) (*CreditEvidenceReservation, error) {
	if s.evidence == nil {
		return nil, bizerr.NewCode(CodeCreditUnavailable)
	}
	return s.evidence.ReserveAccountEvidence(ctx, fileName, size)
}

func (s *accountCreditCommandImpl) UploadCreditEvidence(ctx context.Context, uploadNo, mediaType string, body io.Reader) (*CreditEvidenceItem, error) {
	if s.evidence == nil {
		return nil, bizerr.NewCode(CodeCreditUnavailable)
	}
	return s.evidence.UploadAccountEvidence(ctx, uploadNo, mediaType, body)
}

func (s *accountCreditCommandImpl) SubmitCreditApplication(
	ctx context.Context,
	intent AccountCreditApplicationIntent,
) (*CreditApplication, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	input := CreditApplicationInput{
		CustomerNo: audience.CustomerNo, CurrencyCode: intent.CurrencyCode,
		RequestedLimitMinor: intent.RequestedLimitMinor,
		RequestedValidFrom:  intent.RequestedValidFrom, RequestedValidUntil: intent.RequestedValidUntil,
		RiskEvidenceReference: intent.RiskEvidenceReference,
		RiskEvidenceDigest:    intent.RiskEvidenceDigest, RequestActorType: "customer",
		RequestedBy: audience.UserID, IdempotencyKey: intent.IdempotencyKey,
	}
	if len(intent.UploadNos) == 0 && (input.RiskEvidenceReference == "" || input.RiskEvidenceDigest == "") {
		return nil, bizerr.NewCode(CodeCreditInvalid)
	}
	if len(intent.UploadNos) > 0 && s.evidence == nil {
		return nil, bizerr.NewCode(CodeCreditUnavailable)
	}
	if len(intent.UploadNos) == 0 {
		if _, err = normalizeCreditApplication(input); err != nil {
			return nil, err
		}
	}
	var result *CreditApplication
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if len(intent.UploadNos) > 0 {
			input.RiskEvidenceReference, input.RiskEvidenceDigest, err = s.evidence.PrepareAccountEvidence(ctx, intent.UploadNos)
			if err != nil {
				return err
			}
		}
		if _, err = normalizeCreditApplication(input); err != nil {
			return err
		}
		result, err = s.approval.SubmitCreditApplication(ctx, input)
		if err != nil {
			return err
		}
		if len(intent.UploadNos) > 0 {
			if err = s.evidence.BindAccountEvidence(ctx, intent.UploadNos, result.ApplicationNo); err != nil {
				return err
			}
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"state": "not_created"},
			IdempotencyScope: "billing.credit_application.submit",
			IdempotencyKey:   result.ApplicationNo,
			OperationCode:    "billing.credit_application.submit",
			TargetType:       "credit_application", TargetNo: result.ApplicationNo,
			CustomerNo: result.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "customer_credit_request", RiskCode: "high",
			AfterSnapshot: map[string]any{
				"state": result.StatusCode, "currency_code": result.CurrencyCode,
				"requested_limit_minor":   result.RequestedLimitMinor,
				"required_approval_count": result.RequiredApprovalCount,
			},
		})
	})
	if err != nil {
		return nil, creditCommandResultError(err)
	}
	return result, nil
}

func (s *accountCreditCommandImpl) IssueCreditBalanceRepaymentProof(
	ctx context.Context,
	intent AccountCreditBalanceRepaymentIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareBalanceRepayment(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *accountCreditCommandImpl) RepayCreditStatementFromBalance(
	ctx context.Context,
	intent AccountCreditBalanceRepaymentIntent,
	proofToken string,
) (*CreditBalanceRepayment, error) {
	input, binding, err := s.prepareBalanceRepayment(ctx, intent)
	if err != nil {
		return nil, err
	}
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	if err = consumeCreditProof(ctx, s.recentAuth, audience.UserID, proofToken, binding); err != nil {
		return nil, err
	}
	var result *CreditBalanceRepayment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if ownerErr := requireCreditStatementCustomer(ctx, s.database, input.StatementNo, audience.CustomerNo); ownerErr != nil {
			return ownerErr
		}
		before, loadErr := loadCreditStatementForUpdate(ctx, s.database, input.StatementNo)
		if loadErr != nil {
			return loadErr
		}
		if before == nil {
			return bizerr.NewCode(CodeCreditConflict)
		}
		result, err = s.usage.RepayCreditStatementFromBalance(ctx, input)
		if err != nil {
			return err
		}
		if result.Reused {
			return nil
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"state": before.StatusCode, "currency_code": before.CurrencyCode, "outstanding_minor": before.OutstandingMinor},
			IdempotencyScope: "billing.credit_statement.balance_repayment",
			IdempotencyKey:   result.Repayment.RepaymentNo,
			OperationCode:    "billing.credit_statement.balance_repayment",
			TargetType:       "credit_statement", TargetNo: result.Repayment.StatementNo,
			CustomerNo: result.Repayment.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "customer_balance_repayment", RiskCode: "critical",
			CorrelationNo: result.Repayment.RepaymentNo,
			AfterSnapshot: map[string]any{
				"state":             result.Repayment.StatementStatusCode,
				"currency_code":     result.Repayment.CurrencyCode,
				"amount_minor":      result.Repayment.AmountMinor,
				"outstanding_minor": result.Repayment.StatementOutstandingMinor,
			},
		})
	})
	if err != nil {
		return nil, creditCommandResultError(err)
	}
	return result, nil
}

func (s *accountCreditCommandImpl) prepareBalanceRepayment(
	ctx context.Context,
	intent AccountCreditBalanceRepaymentIntent,
) (CreditBalanceRepaymentInput, reauth.Binding, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, err
	}
	intent.StatementNo = strings.TrimSpace(intent.StatementNo)
	intent.RequestNo = strings.TrimSpace(intent.RequestNo)
	if !validBusinessReference(intent.StatementNo, 40) ||
		!validBusinessReference(intent.RequestNo, 128) || !validAmount(intent.AmountMinor) {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, bizerr.NewCode(CodeCreditInvalid)
	}
	if err = requireCreditStatementCustomer(ctx, s.database, intent.StatementNo, audience.CustomerNo); err != nil {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, err
	}
	snapshotDigest, err := contentDigest(struct {
		Kind, StatementNo, CustomerNo, RequestNo string
		AmountMinor                              int64
	}{"customer_balance_credit_repayment", intent.StatementNo, audience.CustomerNo, intent.RequestNo, intent.AmountMinor})
	if err != nil {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, err
	}
	evidenceDigest, err := contentDigest(struct {
		Kind, SnapshotDigest string
	}{"recent_auth_customer_balance_authorization", snapshotDigest})
	if err != nil {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, err
	}
	input, err := normalizeCreditBalanceRepayment(CreditBalanceRepaymentInput{
		StatementNo: intent.StatementNo, AmountMinor: intent.AmountMinor,
		SourceReference: intent.RequestNo, SourceSnapshotDigest: snapshotDigest,
		ConfirmationEvidenceDigest: evidenceDigest,
	})
	if err != nil {
		return CreditBalanceRepaymentInput{}, reauth.Binding{}, err
	}
	targetDigest, err := contentDigest(struct {
		CustomerNo string
		Input      CreditBalanceRepaymentInput
	}{audience.CustomerNo, input})
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: targetDigest}, err
}

func (s *managementCreditCommandImpl) IssueCreditPolicyProof(
	ctx context.Context,
	intent ManagementCreditPolicyIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.preparePolicy(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementCreditCommandImpl) ConfigureCreditPolicy(
	ctx context.Context,
	intent ManagementCreditPolicyIntent,
	proofToken string,
) (*CreditPolicy, error) {
	input, binding, err := s.preparePolicy(ctx, intent)
	if err != nil {
		return nil, err
	}
	if proofToken != "" {
		if err = consumeCreditProof(ctx, s.recentAuth, input.ConfiguredBy, proofToken, binding); err != nil {
			return nil, err
		}
	}
	var result *CreditPolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.approval.ConfigureCreditPolicy(ctx, input)
		if err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"policy_no": ""},
			IdempotencyScope: "billing.credit_policy.configure", IdempotencyKey: result.PolicyNo,
			OperationCode: "billing.credit_policy.configure", TargetType: "credit_policy",
			TargetNo: result.PolicyNo, ResultCode: "succeeded",
			ReasonCode: "credit_policy_update", RiskCode: "critical",
			RequiredPermission: permissionCreditPolicyWrite,
			AfterSnapshot: map[string]any{
				"state": "active", "currency_code": result.CurrencyCode,
				"maximum_limit_minor":         result.MaximumLimitMinor,
				"dual_review_threshold_minor": result.DualReviewThresholdMinor,
				"billing_day":                 result.BillingDay, "version": result.Version,
			},
		})
	})
	if err != nil {
		return nil, creditCommandResultError(err)
	}
	return result, nil
}

func (s *managementCreditCommandImpl) IssueCreditApplicationReviewProof(
	ctx context.Context,
	intent ManagementCreditReviewIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareReview(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementCreditCommandImpl) ReviewCreditApplication(
	ctx context.Context,
	intent ManagementCreditReviewIntent,
	proofToken string,
) (*CreditApplication, error) {
	input, binding, err := s.prepareReview(ctx, intent)
	if err != nil {
		return nil, err
	}
	if proofToken != "" {
		if err = consumeCreditProof(ctx, s.recentAuth, input.ReviewerID, proofToken, binding); err != nil {
			return nil, err
		}
	}
	var result *CreditApplication
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.approval.ReviewCreditApplication(ctx, input)
		if err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "billing.credit_application.review",
			IdempotencyKey:   intent.RequestNo,
			OperationCode:    "billing.credit_application.review",
			TargetType:       "credit_application", TargetNo: result.ApplicationNo,
			CustomerNo: result.CustomerNo, ResultCode: "succeeded",
			ReasonCode: "credit_application_review", RiskCode: "critical",
			RequiredPermission: permissionCreditApplicationReview,
			BeforeSnapshot:     map[string]any{"state": creditApplicationCommandStatusPendingReview},
			AfterSnapshot: map[string]any{
				"state": result.StatusCode, "decision": string(intent.Decision),
				"currency_code":           result.CurrencyCode,
				"review_count":            result.ReviewCount,
				"required_approval_count": result.RequiredApprovalCount,
			},
		})
	})
	if err != nil {
		return nil, creditCommandResultError(err)
	}
	return result, nil
}

func (s *managementCreditCommandImpl) preparePolicy(
	ctx context.Context,
	intent ManagementCreditPolicyIntent,
) (CreditPolicyInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizePolicyWrite(ctx)
	if err != nil {
		return CreditPolicyInput{}, reauth.Binding{}, err
	}
	input := CreditPolicyInput{
		CurrencyCode: intent.CurrencyCode, MaximumLimitMinor: intent.MaximumLimitMinor,
		DualReviewThresholdMinor: intent.DualReviewThresholdMinor, BillingDay: intent.BillingDay,
		RepaymentPeriodDays: intent.RepaymentPeriodDays, GracePeriodDays: intent.GracePeriodDays,
		DailyLateFeeBasisPoints: intent.DailyLateFeeBasisPoints,
		MaximumLateFeeMinor:     intent.MaximumLateFeeMinor,
		ConfiguredBy:            actor, IdempotencyKey: intent.RequestNo,
	}
	command, err := normalizeCreditPolicy(input)
	if err != nil {
		return CreditPolicyInput{}, reauth.Binding{}, err
	}
	targetDigest, err := contentDigest(command)
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: targetDigest}, err
}

func (s *managementCreditCommandImpl) prepareReview(
	ctx context.Context,
	intent ManagementCreditReviewIntent,
) (CreditReviewInput, reauth.Binding, error) {
	intent.RequestNo = strings.TrimSpace(intent.RequestNo)
	intent.ApplicationNo = strings.TrimSpace(intent.ApplicationNo)
	actor, err := s.authorizer.AuthorizeApplicationReview(ctx, intent.ApplicationNo)
	if err != nil {
		return CreditReviewInput{}, reauth.Binding{}, err
	}
	if !validBusinessReference(intent.RequestNo, 128) {
		return CreditReviewInput{}, reauth.Binding{}, bizerr.NewCode(CodeCreditInvalid)
	}
	_, intent.CommentDigest = adjustmentMetadata("", intent.CommentDigest, "", intent.RequestNo)
	_, intent.EvidenceDigest = adjustmentMetadata("", intent.EvidenceDigest, "", intent.RequestNo)
	input, err := normalizeCreditReview(CreditReviewInput{
		ApplicationNo: intent.ApplicationNo, Decision: intent.Decision,
		ReviewerID: actor, CommentDigest: intent.CommentDigest,
		EvidenceDigest: intent.EvidenceDigest,
	})
	if err != nil {
		return CreditReviewInput{}, reauth.Binding{}, err
	}
	targetDigest, err := contentDigest(struct {
		RequestNo string
		Review    CreditReviewInput
	}{intent.RequestNo, input})
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: targetDigest}, err
}

func consumeCreditProof(
	ctx context.Context,
	recentAuth reauth.Service,
	actor int,
	proofToken string,
	binding reauth.Binding,
) error {
	proofToken = strings.TrimSpace(proofToken)
	if proofToken == "" {
		return bizerr.NewCode(CodeCreditInvalid)
	}
	proof, err := recentAuth.Consume(ctx, proofToken, binding)
	if err != nil {
		return err
	}
	if proof == nil || proof.UserID != actor || proof.TenantID != idccap.DefaultTenantID ||
		proof.Purpose != binding.Purpose || proof.TargetDigest != binding.TargetDigest {
		return bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return nil
}

func requireCreditStatementCustomer(
	ctx context.Context,
	database gdb.DB,
	statementNo string,
	customerNo string,
) error {
	var owner *struct {
		CustomerNo string `orm:"customer_no"`
	}
	if err := database.GetScan(ctx, &owner, `
SELECT customer_no
FROM plugin_idc_core_credit_statement
WHERE tenant_id = 0 AND statement_no = ? AND customer_no = ?`, statementNo, customerNo); err != nil {
		return bizerr.WrapCode(err, CodeCreditUnavailable)
	}
	if owner == nil || owner.CustomerNo != customerNo {
		return bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return nil
}

func creditCommandResultError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodeCreditUnavailable)
}
