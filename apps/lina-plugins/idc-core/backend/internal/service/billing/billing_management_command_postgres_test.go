package billing

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

type managementCommandAuthorizerStub struct {
	createActor int
	reviewActor int
}

func (s managementCommandAuthorizerStub) actor() int {
	if s.createActor > 0 {
		return s.createActor
	}
	return 9001
}
func (s managementCommandAuthorizerStub) AuthorizeFinancialOverview(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeInvoiceList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeInvoiceDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeTransactionList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeTransactionDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeAdjustmentList(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeAdjustmentDetail(context.Context, string) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeManualReceiptCreate(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeInvoiceWaiverCreate(context.Context) (int, error) {
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeAdjustmentReview(context.Context, string) (int, error) {
	if s.reviewActor > 0 {
		return s.reviewActor, nil
	}
	return s.actor(), nil
}
func (s managementCommandAuthorizerStub) AuthorizeAdjustmentPolicyWrite(context.Context) (int, error) {
	return s.actor(), nil
}

type managementCommandRecentAuthStub struct {
	issued       reauth.Binding
	consumed     reauth.Binding
	consumeActor int
	issueCalls   int
	consumeCalls int
}

func (s *managementCommandRecentAuthStub) Issue(
	_ context.Context,
	binding reauth.Binding,
) (*reauth.ProofContext, error) {
	s.issueCalls++
	s.issued = binding
	return &reauth.ProofContext{Token: "single-use-proof", ExpiresAt: time.Now().Add(time.Minute)}, nil
}

func (s *managementCommandRecentAuthStub) Consume(
	_ context.Context,
	token string,
	binding reauth.Binding,
) (*reauth.Proof, error) {
	s.consumeCalls++
	s.consumed = binding
	if token != "single-use-proof" || binding != s.issued {
		return nil, errors.New("proof binding mismatch")
	}
	return &reauth.Proof{
		Purpose: binding.Purpose, TargetDigest: binding.TargetDigest,
		UserID: s.consumeActor, TenantID: idccap.DefaultTenantID,
	}, nil
}

type managementCommandAuditStub struct {
	transitions []idccap.BusinessAuditTransition
	err         error
}

func (s *managementCommandAuditStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if input.BeforeSnapshot == nil || input.AfterSnapshot == nil {
		return errors.New("missing audit snapshots")
	}
	switch input.ResultCode {
	case "succeeded", "denied", "failed", "result_unknown":
	default:
		return errors.New("invalid audit result")
	}

	s.transitions = append(s.transitions, input)
	return s.err
}

func (*managementCommandAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestManagementFinanceCommandPostgreSQLBindsProofAndAuditsDualReview(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := managementCommandAuthorizerStub{createActor: 9001, reviewActor: 9002}
	recentAuth := &managementCommandRecentAuthStub{consumeActor: authorizer.createActor}
	auditWriter := &managementCommandAuditStub{}
	service, err := NewManagementFinanceCommandService(
		database, auditWriter, authorizer, recentAuth, adjustments,
		financialSettlementServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	receipt := ManagementManualReceiptIntent{
		RequestNo: "management-receipt-000001", CustomerNo: "customer-ledger",
		CurrencyCode: "CNY", AmountMinor: 800, ReasonCode: "offline_balance_credit",
		ProofReference:            "private-proof-management-command-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "balance_recharge", EvidenceDigest: strings.Repeat("a", 64),
	}
	issued, err := service.IssueManualReceiptProof(ctx, receipt)
	if err != nil || issued.Token == "" || recentAuth.issued.Purpose != reauth.PurposeHighValueFunds {
		t.Fatalf("issue manual receipt proof: proof=%+v binding=%+v err=%v", issued, recentAuth.issued, err)
	}
	pending, err := service.CreateManualReceipt(ctx, receipt, issued.Token)
	if err != nil || pending.Status != AdjustmentStatusPendingReview || !pending.ApprovalRequired ||
		recentAuth.consumed != recentAuth.issued || len(auditWriter.transitions) != 1 ||
		auditWriter.transitions[0].RequiredPermission != permissionBillingManualReceiptCreate {
		t.Fatalf("create reviewed receipt: result=%+v auth=%+v audit=%+v err=%v", pending, recentAuth, auditWriter.transitions, err)
	}

	review := ManagementAdjustmentReviewIntent{
		RequestNo: "management-review-0000001", AdjustmentNo: pending.AdjustmentNo,
		Decision: AdjustmentDecisionApprove, CommentDigest: strings.Repeat("b", 64),
		EvidenceDigest: strings.Repeat("c", 64),
	}
	recentAuth.consumeActor = authorizer.reviewActor
	issued, err = service.IssueAdjustmentReviewProof(ctx, review)
	if err != nil {
		t.Fatalf("issue review proof: %v", err)
	}
	posted, err := service.ReviewAdjustment(ctx, review, issued.Token)
	if err != nil || posted.Status != AdjustmentStatusPosted || posted.LedgerTransactionNo == "" ||
		len(auditWriter.transitions) != 2 ||
		auditWriter.transitions[1].RequiredPermission != permissionBillingAdjustmentReview {
		t.Fatalf("review receipt: result=%+v audit=%+v err=%v", posted, auditWriter.transitions, err)
	}
}

func TestManagementFinanceCommandRejectsChangedPreviewAndProofActor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := managementCommandAuthorizerStub{createActor: 9101}
	recentAuth := &managementCommandRecentAuthStub{consumeActor: authorizer.createActor}
	service, err := NewManagementFinanceCommandService(
		database, &managementCommandAuditStub{}, authorizer, recentAuth, adjustments,
		financialSettlementServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent := ManagementAdjustmentPolicyIntent{
		RequestNo: "management-policy-0000001", CurrencyCode: "CNY", ReviewThresholdMinor: 1000,
	}
	issued, err := service.IssueAdjustmentPolicyProof(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	changed := intent
	changed.ReviewThresholdMinor++
	if _, err = service.ConfigureAdjustmentPolicy(ctx, changed, issued.Token); err == nil {
		t.Fatal("changed preview consumed a proof issued for another threshold")
	}
	if recentAuth.consumeCalls != 1 {
		t.Fatalf("changed preview consume calls=%d", recentAuth.consumeCalls)
	}
	issued, err = service.IssueAdjustmentPolicyProof(ctx, intent)
	if err != nil {
		t.Fatal(err)
	}
	recentAuth.consumeActor = authorizer.createActor + 1
	if _, err = service.ConfigureAdjustmentPolicy(ctx, intent, issued.Token); err == nil {
		t.Fatal("proof issued to another actor reached the financial transaction")
	}
}

func TestManagementFinanceCommandRollsBackWhenAuditAppendFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	adjustments, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	authorizer := managementCommandAuthorizerStub{createActor: 9201}
	recentAuth := &managementCommandRecentAuthStub{consumeActor: authorizer.createActor}
	auditWriter := &managementCommandAuditStub{err: errors.New("audit append failed")}
	service, err := NewManagementFinanceCommandService(
		database, auditWriter, authorizer, recentAuth, adjustments,
		financialSettlementServiceStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	intent := ManagementManualReceiptIntent{
		RequestNo: "management-receipt-rollback-0001", CustomerNo: "customer-ledger",
		CurrencyCode: "CNY", AmountMinor: 800, ReasonCode: "offline_balance_credit",
		ProofReference:            "private-proof-management-rollback-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "balance_recharge", EvidenceDigest: strings.Repeat("d", 64),
	}
	issued, err := service.IssueManualReceiptProof(ctx, intent)
	if err != nil {
		t.Fatalf("issue manual receipt proof: %v", err)
	}
	before, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_adjustment_request`)
	if err != nil {
		t.Fatalf("count adjustments before failed audit: %v", err)
	}
	if _, err = service.CreateManualReceipt(ctx, intent, issued.Token); err == nil {
		t.Fatal("manual receipt committed even though its audit append failed")
	}
	after, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_adjustment_request`)
	if err != nil {
		t.Fatalf("count adjustments after failed audit: %v", err)
	}
	if after.Int64() != before.Int64() || len(auditWriter.transitions) != 1 {
		t.Fatalf("failed audit rollback: before=%d after=%d attempts=%d",
			before.Int64(), after.Int64(), len(auditWriter.transitions))
	}
}
