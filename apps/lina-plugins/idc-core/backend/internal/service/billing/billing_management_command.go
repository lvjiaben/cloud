package billing

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

// ManagementManualReceiptIntent is the immutable preview used for both proof
// issuance and execution. RequestNo supplies idempotency without exposing raw
// recent-authentication evidence to persistence or audit snapshots.
type ManagementManualReceiptIntent struct {
	RequestNo                 string
	InvoiceNo                 string
	CreditStatementNo         string
	CustomerNo                string
	CurrencyCode              string
	AmountMinor               int64
	ReasonCode                string
	ProofReference            string
	ReceivingAccountReference string
	BusinessPurposeCode       string
	EvidenceDigest            string
}

type ManagementInvoiceWaiverIntent struct {
	RequestNo      string
	InvoiceNo      string
	AmountMinor    int64
	ReasonCode     string
	EvidenceDigest string
}

type ManagementAdjustmentReviewIntent struct {
	RequestNo      string
	AdjustmentNo   string
	Decision       AdjustmentDecision
	CommentDigest  string
	EvidenceDigest string
}

type ManagementAdjustmentPolicyIntent struct {
	RequestNo            string
	CurrencyCode         string
	ReviewThresholdMinor int64
}

// ManagementBalanceAdjustmentIntent binds the exact customer, currency,
// operation, amount and evidence to one proof and one immutable request.
type ManagementBalanceAdjustmentIntent struct {
	RequestNo      string
	CustomerNo     string
	CurrencyCode   string
	AmountMinor    int64
	Deduct         bool
	ReasonCode     string
	EvidenceDigest string
}

// ManagementFinanceCommandService owns staff-only, operation-bound financial
// mutations. Every execution re-authorizes, validates a proof only if supplied, and
// appends its business audit event inside the authoritative transaction.
type ManagementFinanceCommandService interface {
	// IssueBalanceAdjustmentProof authorizes the complete preview for the current staff session.
	IssueBalanceAdjustmentProof(context.Context, ManagementBalanceAdjustmentIntent) (*reauth.ProofContext, error)
	// CreateBalanceAdjustment re-authorizes and posts directly; a supplied proof is validated and audit failure rolls back the adjustment.
	CreateBalanceAdjustment(context.Context, ManagementBalanceAdjustmentIntent, string) (*FinancialAdjustment, error)
	IssueManualReceiptProof(context.Context, ManagementManualReceiptIntent) (*reauth.ProofContext, error)
	CreateManualReceipt(context.Context, ManagementManualReceiptIntent, string) (*FinancialAdjustment, error)
	IssueInvoiceWaiverProof(context.Context, ManagementInvoiceWaiverIntent) (*reauth.ProofContext, error)
	CreateInvoiceWaiver(context.Context, ManagementInvoiceWaiverIntent, string) (*FinancialAdjustment, error)
	IssueAdjustmentReviewProof(context.Context, ManagementAdjustmentReviewIntent) (*reauth.ProofContext, error)
	ReviewAdjustment(context.Context, ManagementAdjustmentReviewIntent, string) (*FinancialAdjustment, error)
	IssueAdjustmentPolicyProof(context.Context, ManagementAdjustmentPolicyIntent) (*reauth.ProofContext, error)
	ConfigureAdjustmentPolicy(context.Context, ManagementAdjustmentPolicyIntent, string) (*AdjustmentPolicy, error)
}

type managementFinanceCommandImpl struct {
	database    gdb.DB
	audit       idccap.BusinessAuditWriter
	authorizer  ManagementFinanceAuthorizer
	recentAuth  reauth.Service
	adjustments FinancialAdjustmentService
	settlements FinancialSettlementService
}

var _ ManagementFinanceCommandService = (*managementFinanceCommandImpl)(nil)

func NewManagementFinanceCommandService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer ManagementFinanceAuthorizer,
	recentAuth reauth.Service,
	adjustments FinancialAdjustmentService,
	settlements FinancialSettlementService,
) (ManagementFinanceCommandService, error) {
	dependencies := []any{
		database, auditWriter, authorizer, recentAuth, adjustments, settlements,
	}
	for _, dependency := range dependencies {
		if isNilFinanceDirectoryDependency(dependency) {
			return nil, gerror.New("management financial commands require database, audit, authorization, recent authentication, and adjustment services")
		}
	}
	return &managementFinanceCommandImpl{
		database: database, audit: auditWriter, authorizer: authorizer,
		recentAuth: recentAuth, adjustments: adjustments, settlements: settlements,
	}, nil
}

func (s *managementFinanceCommandImpl) IssueManualReceiptProof(
	ctx context.Context,
	intent ManagementManualReceiptIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareManualReceipt(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementFinanceCommandImpl) CreateManualReceipt(
	ctx context.Context,
	intent ManagementManualReceiptIntent,
	proofToken string,
) (*FinancialAdjustment, error) {
	input, binding, err := s.prepareManualReceipt(ctx, intent)
	if err != nil {
		return nil, err
	}
	if err = s.consumeProof(ctx, input.RequestedBy, proofToken, binding); err != nil {
		return nil, err
	}
	var result *FinancialAdjustment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.adjustments.CreateManualReceipt(ctx, input)
		if err != nil {
			return err
		}
		return s.appendAdjustmentAudit(ctx, "manual_receipt", intent.RequestNo, input.ReasonCode, result)
	})
	result, err = managementFinanceCommandResult(result, err)
	if err != nil {
		return nil, err
	}
	return s.settlePostedManualReceipt(ctx, result)
}

func (s *managementFinanceCommandImpl) IssueInvoiceWaiverProof(
	ctx context.Context,
	intent ManagementInvoiceWaiverIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareInvoiceWaiver(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementFinanceCommandImpl) CreateInvoiceWaiver(
	ctx context.Context,
	intent ManagementInvoiceWaiverIntent,
	proofToken string,
) (*FinancialAdjustment, error) {
	input, binding, err := s.prepareInvoiceWaiver(ctx, intent)
	if err != nil {
		return nil, err
	}
	if err = s.consumeProof(ctx, input.RequestedBy, proofToken, binding); err != nil {
		return nil, err
	}
	var result *FinancialAdjustment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.adjustments.CreateInvoiceWaiver(ctx, input)
		if err != nil {
			return err
		}
		return s.appendAdjustmentAudit(ctx, "invoice_waiver", intent.RequestNo, input.ReasonCode, result)
	})
	return managementFinanceCommandResult(result, err)
}

// settlePostedManualReceipt runs only after the receipt and audit transaction
// has committed. A later settlement failure must not erase confirmed money;
// replaying the owner command safely retries this boundary.
func (s *managementFinanceCommandImpl) settlePostedManualReceipt(
	ctx context.Context,
	adjustment *FinancialAdjustment,
) (*FinancialAdjustment, error) {
	if adjustment == nil || adjustment.AdjustmentType != "manual_receipt" ||
		adjustment.Status != AdjustmentStatusPosted || adjustment.Invoice == nil ||
		adjustment.Invoice.OrderNo == "" {
		return adjustment, nil
	}
	settled, err := s.settlements.SettleOrder(ctx, adjustment.Invoice.OrderNo)
	if err != nil {
		return nil, err
	}
	if settled == nil || settled.InvoiceNo != adjustment.Invoice.InvoiceNo ||
		settled.Basis != SettlementBasisFundedInvoice {
		return nil, bizerr.NewCode(CodeInvoiceIntegrity)
	}
	return adjustment, nil
}

func (s *managementFinanceCommandImpl) IssueAdjustmentReviewProof(
	ctx context.Context,
	intent ManagementAdjustmentReviewIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareAdjustmentReview(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementFinanceCommandImpl) ReviewAdjustment(
	ctx context.Context,
	intent ManagementAdjustmentReviewIntent,
	proofToken string,
) (*FinancialAdjustment, error) {
	input, binding, err := s.prepareAdjustmentReview(ctx, intent)
	if err != nil {
		return nil, err
	}
	if err = s.consumeProof(ctx, input.ReviewerID, proofToken, binding); err != nil {
		return nil, err
	}
	var result *FinancialAdjustment
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.adjustments.ReviewAdjustment(ctx, input)
		if err != nil {
			return err
		}
		return s.appendAdjustmentReviewAudit(ctx, intent, result)
	})
	result, err = managementFinanceCommandResult(result, err)
	if err != nil {
		return nil, err
	}
	return s.settlePostedManualReceipt(ctx, result)
}

func (s *managementFinanceCommandImpl) IssueAdjustmentPolicyProof(
	ctx context.Context,
	intent ManagementAdjustmentPolicyIntent,
) (*reauth.ProofContext, error) {
	_, binding, err := s.prepareAdjustmentPolicy(ctx, intent)
	if err != nil {
		return nil, err
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementFinanceCommandImpl) ConfigureAdjustmentPolicy(
	ctx context.Context,
	intent ManagementAdjustmentPolicyIntent,
	proofToken string,
) (*AdjustmentPolicy, error) {
	input, binding, err := s.prepareAdjustmentPolicy(ctx, intent)
	if err != nil {
		return nil, err
	}
	if err = s.consumeProof(ctx, input.ConfiguredBy, proofToken, binding); err != nil {
		return nil, err
	}
	var result *AdjustmentPolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		result, err = s.adjustments.ConfigureApprovalPolicy(ctx, input)
		if err != nil {
			return err
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"policy_no": ""},
			IdempotencyScope: "billing.adjustment_policy.configure", IdempotencyKey: intent.RequestNo,
			OperationCode: "billing.adjustment_policy.configure", TargetType: "billing_adjustment_policy",
			TargetNo: result.PolicyNo, ResultCode: "succeeded", ReasonCode: "adjustment_policy_update",
			RiskCode: "critical", RequiredPermission: permissionBillingAdjustmentPolicyWrite,
			AfterSnapshot: map[string]any{
				"state": "active", "currency_code": result.CurrencyCode,
				"review_threshold_minor": result.ReviewThresholdMinor, "version": result.Version,
			},
		})
	})
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeInvoiceUnavailable)
}

func (s *managementFinanceCommandImpl) prepareManualReceipt(
	ctx context.Context,
	intent ManagementManualReceiptIntent,
) (ManualReceiptInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizeManualReceiptCreate(ctx)
	input := ManualReceiptInput{
		InvoiceNo: intent.InvoiceNo, CreditStatementNo: intent.CreditStatementNo,
		CustomerNo: intent.CustomerNo, CurrencyCode: intent.CurrencyCode,
		AmountMinor: intent.AmountMinor, ReasonCode: intent.ReasonCode,
		ProofReference: intent.ProofReference, ReceivingAccountReference: intent.ReceivingAccountReference,
		BusinessPurposeCode: intent.BusinessPurposeCode, EvidenceDigest: intent.EvidenceDigest,
		IdempotencyKey: intent.RequestNo, RequestedBy: actor,
	}
	if err != nil {
		return ManualReceiptInput{}, reauth.Binding{}, err
	}
	command, err := normalizeManualReceipt(input)
	if err != nil {
		return ManualReceiptInput{}, reauth.Binding{}, err
	}
	input.ReasonCode, input.EvidenceDigest = command.ReasonCode, command.EvidenceDigest
	digest, err := adjustmentRequestDigest(command)
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: digest}, err
}

func (s *managementFinanceCommandImpl) prepareInvoiceWaiver(
	ctx context.Context,
	intent ManagementInvoiceWaiverIntent,
) (InvoiceWaiverInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizeInvoiceWaiverCreate(ctx)
	input := InvoiceWaiverInput{
		InvoiceNo: intent.InvoiceNo, AmountMinor: intent.AmountMinor,
		ReasonCode: intent.ReasonCode, EvidenceDigest: intent.EvidenceDigest,
		IdempotencyKey: intent.RequestNo, RequestedBy: actor,
	}
	if err != nil {
		return InvoiceWaiverInput{}, reauth.Binding{}, err
	}
	command, err := normalizeInvoiceWaiver(input)
	if err != nil {
		return InvoiceWaiverInput{}, reauth.Binding{}, err
	}
	input.ReasonCode, input.EvidenceDigest = command.ReasonCode, command.EvidenceDigest
	digest, err := adjustmentRequestDigest(command)
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: digest}, err
}

func (s *managementFinanceCommandImpl) prepareAdjustmentReview(
	ctx context.Context,
	intent ManagementAdjustmentReviewIntent,
) (AdjustmentReviewInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizeAdjustmentReview(ctx, intent.AdjustmentNo)
	input := AdjustmentReviewInput{
		AdjustmentNo: intent.AdjustmentNo, Decision: intent.Decision, ReviewerID: actor,
		CommentDigest: intent.CommentDigest, EvidenceDigest: intent.EvidenceDigest,
	}
	if err != nil {
		return AdjustmentReviewInput{}, reauth.Binding{}, err
	}
	if !exactlyTrimmed(intent.RequestNo) || !validInvoiceTransformKey(intent.RequestNo) {
		return AdjustmentReviewInput{}, reauth.Binding{}, invoiceInvalidError()
	}
	if input, err = normalizeAdjustmentReview(input); err != nil {
		return AdjustmentReviewInput{}, reauth.Binding{}, err
	}
	digest, err := contentDigest(struct {
		RequestNo, AdjustmentNo, Decision, CommentDigest, EvidenceDigest string
		ReviewerID                                                       int
	}{intent.RequestNo, input.AdjustmentNo, string(input.Decision), input.CommentDigest, input.EvidenceDigest, input.ReviewerID})
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: digest}, err
}

func (s *managementFinanceCommandImpl) prepareAdjustmentPolicy(
	ctx context.Context,
	intent ManagementAdjustmentPolicyIntent,
) (AdjustmentPolicyInput, reauth.Binding, error) {
	actor, err := s.authorizer.AuthorizeAdjustmentPolicyWrite(ctx)
	input := AdjustmentPolicyInput{
		CurrencyCode: intent.CurrencyCode, ReviewThresholdMinor: intent.ReviewThresholdMinor,
		ConfiguredBy: actor,
	}
	if err != nil {
		return AdjustmentPolicyInput{}, reauth.Binding{}, err
	}
	if !exactlyTrimmed(intent.RequestNo) || !validInvoiceTransformKey(intent.RequestNo) {
		return AdjustmentPolicyInput{}, reauth.Binding{}, invoiceInvalidError()
	}
	if _, err = normalizeAdjustmentPolicy(input); err != nil {
		return AdjustmentPolicyInput{}, reauth.Binding{}, err
	}
	digest, err := contentDigest(struct {
		RequestNo, CurrencyCode string
		ReviewThresholdMinor    int64
		ConfiguredBy            int
	}{intent.RequestNo, input.CurrencyCode, input.ReviewThresholdMinor, input.ConfiguredBy})
	return input, reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: digest}, err
}

func (s *managementFinanceCommandImpl) consumeProof(
	ctx context.Context,
	actor int,
	proofToken string,
	binding reauth.Binding,
) error {
	proofToken = strings.TrimSpace(proofToken)
	if proofToken == "" {
		return nil
	}
	proof, err := s.recentAuth.Consume(ctx, proofToken, binding)
	if err != nil {
		return err
	}
	if proof == nil || proof.UserID != actor || proof.TenantID != idccap.DefaultTenantID ||
		proof.Purpose != binding.Purpose || proof.TargetDigest != binding.TargetDigest {
		return bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return nil
}

func (s *managementFinanceCommandImpl) appendAdjustmentAudit(
	ctx context.Context,
	operation string,
	requestNo string,
	reasonCode string,
	result *FinancialAdjustment,
) error {
	permission := permissionBillingManualReceiptCreate
	if operation == "customer_balance" {
		permission = permissionBillingBalanceAdjustmentCreate
	}
	if operation == "invoice_waiver" {
		permission = permissionBillingInvoiceWaiverCreate
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"state": "not_created"},
		IdempotencyScope: "billing." + operation + ".create", IdempotencyKey: requestNo,
		OperationCode: "billing." + operation + ".create", TargetType: "billing_adjustment_request",
		TargetNo: result.AdjustmentNo, CustomerNo: result.CustomerNo, ResultCode: "succeeded",
		ReasonCode: reasonCode, RiskCode: "critical", RequiredPermission: permission,
		CorrelationNo: managementAdjustmentCorrelationNo(result),
		AfterSnapshot: map[string]any{
			"state": string(result.Status), "adjustment_type": result.AdjustmentType,
			"currency_code": result.CurrencyCode, "amount_minor": result.AmountMinor,
			"approval_required":   result.ApprovalRequired,
			"credit_statement_no": result.CreditStatementNo,
		},
	})
}

func managementAdjustmentCorrelationNo(result *FinancialAdjustment) string {
	if result == nil {
		return ""
	}
	if result.InvoiceNo != "" {
		return result.InvoiceNo
	}
	return result.CreditStatementNo
}

func (s *managementFinanceCommandImpl) appendAdjustmentReviewAudit(
	ctx context.Context,
	intent ManagementAdjustmentReviewIntent,
	result *FinancialAdjustment,
) error {
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "billing.adjustment.review", IdempotencyKey: intent.RequestNo,
		OperationCode: "billing.adjustment.review", TargetType: "billing_adjustment_request",
		TargetNo: result.AdjustmentNo, CustomerNo: result.CustomerNo, ResultCode: "succeeded",
		ReasonCode: "financial_adjustment_review", RiskCode: "critical",
		RequiredPermission: permissionBillingAdjustmentReview, CorrelationNo: result.InvoiceNo,
		BeforeSnapshot: map[string]any{"state": string(AdjustmentStatusPendingReview)},
		AfterSnapshot: map[string]any{
			"state": string(result.Status), "decision": string(intent.Decision),
			"currency_code": result.CurrencyCode, "amount_minor": result.AmountMinor,
		},
	})
}

func managementFinanceCommandResult(
	result *FinancialAdjustment,
	err error,
) (*FinancialAdjustment, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeInvoiceUnavailable)
}
