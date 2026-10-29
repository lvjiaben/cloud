// This file verifies reviewed manual receipts and invoice waivers against an
// isolated real PostgreSQL schema.

package billing

import (
	"context"
	"strings"
	"sync"
	"testing"

	"lina-core/pkg/bizerr"
)

func TestFinancialAdjustmentPostgreSQLSeparatesReceiptAndWaiver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceLifecycle(t, ctx, database)
	service, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create financial adjustment service: %v", err)
	}
	policy, err := service.ConfigureApprovalPolicy(ctx, AdjustmentPolicyInput{
		CurrencyCode: "CNY", ReviewThresholdMinor: 500, ConfiguredBy: 100,
	})
	if err != nil || policy.Version != 1 || policy.ReviewThresholdMinor != 500 || policy.Reused {
		t.Fatalf("configure adjustment policy: result=%+v err=%v", policy, err)
	}
	policyReplay, err := service.ConfigureApprovalPolicy(ctx, AdjustmentPolicyInput{
		CurrencyCode: "CNY", ReviewThresholdMinor: 500, ConfiguredBy: 999,
	})
	if err != nil || !policyReplay.Reused || policyReplay.PolicyNo != policy.PolicyNo {
		t.Fatalf("replay adjustment policy: result=%+v err=%v", policyReplay, err)
	}

	receiptInput := ManualReceiptInput{
		InvoiceNo: "invoice-lifecycle", AmountMinor: 400, ReasonCode: "offline_transfer",
		ProofReference:            "private-proof-receipt-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "invoice_payment", EvidenceDigest: strings.Repeat("a", 64),
		IdempotencyKey: "manual-receipt-invoice-0001", RequestedBy: 101,
	}
	receipt, err := service.CreateManualReceipt(ctx, receiptInput)
	if err != nil || receipt == nil || receipt.Status != AdjustmentStatusPosted ||
		receipt.ApprovalRequired || receipt.Invoice == nil ||
		receipt.Invoice.Status != InvoiceStatusPartiallyPaid ||
		receipt.Invoice.PaidAmountMinor != 400 || receipt.Invoice.WaivedAmountMinor != 0 ||
		receipt.LedgerTransactionNo == "" || receipt.AllocationNo == "" {
		t.Fatalf("post low-value invoice receipt: result=%+v err=%v", receipt, err)
	}
	receiptReplay, err := service.CreateManualReceipt(ctx, receiptInput)
	if err != nil || !receiptReplay.Reused || receiptReplay.AdjustmentNo != receipt.AdjustmentNo ||
		receiptReplay.LedgerTransactionNo != receipt.LedgerTransactionNo {
		t.Fatalf("replay invoice receipt: result=%+v err=%v", receiptReplay, err)
	}
	conflictingReceipt := receiptInput
	conflictingReceipt.AmountMinor++
	if _, err = service.CreateManualReceipt(ctx, conflictingReceipt); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("changed invoice receipt reused idempotency key: %v", err)
	}

	waiverInput := InvoiceWaiverInput{
		InvoiceNo: "invoice-lifecycle", AmountMinor: 600, ReasonCode: "service_credit",
		EvidenceDigest: strings.Repeat("b", 64), IdempotencyKey: "invoice-waiver-review-0001",
		RequestedBy: 102,
	}
	waiver, err := seedLegacyWaiver(t, ctx, service, waiverInput)
	if err != nil || waiver == nil || waiver.Status != AdjustmentStatusPendingReview ||
		!waiver.ApprovalRequired || waiver.LedgerTransactionNo != "" {
		t.Fatalf("create reviewed invoice waiver: result=%+v err=%v", waiver, err)
	}
	reviewInput := AdjustmentReviewInput{
		AdjustmentNo: waiver.AdjustmentNo, Decision: AdjustmentDecisionApprove, ReviewerID: 102,
		CommentDigest: strings.Repeat("c", 64), EvidenceDigest: strings.Repeat("d", 64),
	}
	approved, err := service.ReviewAdjustment(ctx, reviewInput)
	if err != nil || approved == nil || approved.Status != AdjustmentStatusPosted ||
		approved.Invoice == nil || approved.Invoice.Status != InvoiceStatusPaid ||
		approved.Invoice.PaidAmountMinor != 400 || approved.Invoice.WaivedAmountMinor != 600 ||
		approved.Invoice.OutstandingAmountMinor != 0 || approved.AllocationNo == "" ||
		approved.LedgerTransactionNo == "" {
		t.Fatalf("approve invoice waiver: result=%+v err=%v", approved, err)
	}
	approvalReplay, err := service.ReviewAdjustment(ctx, reviewInput)
	if err != nil || !approvalReplay.Reused || approvalReplay.AdjustmentNo != waiver.AdjustmentNo {
		t.Fatalf("replay waiver approval: result=%+v err=%v", approvalReplay, err)
	}
	assertReviewedInvoiceAdjustmentEvidence(t, ctx, database, receipt, approved)
}

func TestFinancialAdjustmentPostgreSQLReviewsBalanceReceiptBeforePosting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create financial adjustment service: %v", err)
	}
	input := ManualReceiptInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 800,
		ReasonCode: "offline_balance_credit", ProofReference: "private-proof-balance-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "balance_recharge", EvidenceDigest: strings.Repeat("e", 64),
		IdempotencyKey: "manual-receipt-balance-0001", RequestedBy: 301,
	}
	pending, err := seedLegacyReceipt(t, ctx, service, input)
	if err != nil || pending == nil || pending.Status != AdjustmentStatusPendingReview ||
		!pending.ApprovalRequired || pending.Balance != nil || pending.LedgerTransactionNo != "" {
		t.Fatalf("unconfigured policy did not fail closed: result=%+v err=%v", pending, err)
	}
	balanceBefore, err := database.GetValue(ctx, `
SELECT COALESCE(MAX(closing_balance_minor), 0)
FROM plugin_idc_core_billing_balance_snapshot snapshot
JOIN plugin_idc_core_billing_ledger_account account
  ON account.account_no = snapshot.account_no
WHERE account.customer_no = 'customer-ledger' AND account.currency_code = 'CNY'
  AND account.account_type_code = 'customer_balance'`)
	if err != nil || balanceBefore.Int64() != 0 {
		t.Fatalf("pending receipt changed balance=%d err=%v", balanceBefore.Int64(), err)
	}
	approved, err := service.ReviewAdjustment(ctx, AdjustmentReviewInput{
		AdjustmentNo: pending.AdjustmentNo, Decision: AdjustmentDecisionApprove, ReviewerID: 302,
		CommentDigest: strings.Repeat("f", 64), EvidenceDigest: strings.Repeat("1", 64),
	})
	if err != nil || approved.Status != AdjustmentStatusPosted || approved.Balance == nil ||
		approved.Balance.BalanceMinor != 800 || approved.Balance.AvailableBalanceMinor != 800 ||
		approved.LedgerTransactionNo == "" {
		t.Fatalf("approve balance receipt: result=%+v err=%v", approved, err)
	}

	rejectedInput := input
	rejectedInput.AmountMinor = 200
	rejectedInput.ProofReference = "private-proof-balance-002"
	rejectedInput.EvidenceDigest = strings.Repeat("2", 64)
	rejectedInput.IdempotencyKey = "manual-receipt-balance-reject-0001"
	rejectedInput.RequestedBy = 303
	rejected, err := seedLegacyReceipt(t, ctx, service, rejectedInput)
	if err != nil || rejected.Status != AdjustmentStatusPendingReview ||
		rejected.LedgerTransactionNo != "" || rejected.Balance != nil {
		t.Fatalf("create balance receipt for rejection: result=%+v err=%v", rejected, err)
	}
	reviewInput := AdjustmentReviewInput{
		AdjustmentNo: rejected.AdjustmentNo, Decision: AdjustmentDecisionReject, ReviewerID: 304,
		CommentDigest: strings.Repeat("3", 64), EvidenceDigest: strings.Repeat("4", 64),
	}
	rejected, err = service.ReviewAdjustment(ctx, reviewInput)
	if err != nil || rejected.Status != AdjustmentStatusRejected || rejected.Reused ||
		rejected.LedgerTransactionNo != "" || rejected.Balance != nil {
		t.Fatalf("reject balance receipt: result=%+v err=%v", rejected, err)
	}
	rejectedReplay, err := service.ReviewAdjustment(ctx, reviewInput)
	if err != nil || !rejectedReplay.Reused || rejectedReplay.Status != AdjustmentStatusRejected {
		t.Fatalf("replay balance receipt rejection: result=%+v err=%v", rejectedReplay, err)
	}
	var rejectedState struct {
		Transactions int   `orm:"transactions"`
		BalanceMinor int64 `orm:"balance_minor"`
	}
	if err = database.GetScan(ctx, &rejectedState, `
SELECT (SELECT count(*) FROM plugin_idc_core_billing_ledger_transaction
        WHERE source_type_code = 'manual_receipt' AND source_reference = ?) AS transactions,
       COALESCE((SELECT MAX(snapshot.closing_balance_minor)
                 FROM plugin_idc_core_billing_balance_snapshot snapshot
                 JOIN plugin_idc_core_billing_ledger_account account
                   ON account.account_no = snapshot.account_no
                 WHERE account.customer_no = 'customer-ledger'
                   AND account.currency_code = 'CNY'
                   AND account.account_type_code = 'customer_balance'), 0) AS balance_minor`,
		rejected.AdjustmentNo); err != nil {
		t.Fatalf("load rejected balance receipt state: %v", err)
	}
	if rejectedState.Transactions != 0 || rejectedState.BalanceMinor != 800 {
		t.Fatalf("rejected balance receipt changed financial state=%+v", rejectedState)
	}
}

func TestFinancialAdjustmentPostgreSQLReviewsCreditStatementReceiptBeforePosting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(t, ctx, database, -10, "manrec")
	service, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create financial adjustment service: %v", err)
	}
	input := ManualReceiptInput{
		CreditStatementNo: fixture.statementNo, AmountMinor: 400,
		ReasonCode: "offline_credit_repayment", ProofReference: "private-proof-credit-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "credit_repayment", EvidenceDigest: strings.Repeat("5", 64),
		IdempotencyKey: "manual-receipt-credit-0001", RequestedBy: 701,
	}
	pending, err := seedLegacyReceipt(t, ctx, service, input)
	if err != nil || pending == nil || pending.Status != AdjustmentStatusPendingReview ||
		!pending.ApprovalRequired || pending.CreditStatementNo != fixture.statementNo ||
		pending.CreditRepayment != nil || pending.LedgerTransactionNo != "" {
		t.Fatalf("create reviewed credit receipt: result=%+v err=%v", pending, err)
	}
	var before struct {
		Transactions int64 `orm:"transactions"`
		Repayments   int64 `orm:"repayments"`
	}
	if err = database.GetScan(ctx, &before, `
SELECT
    (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
     WHERE source_type_code = 'manual_receipt' AND source_reference = ?) AS transactions,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment
     WHERE funding_source_type_code = 'manual_receipt'
       AND funding_source_reference = ?) AS repayments`,
		pending.AdjustmentNo, pending.AdjustmentNo); err != nil {
		t.Fatalf("load pending credit receipt facts: %v", err)
	}
	if before.Transactions != 0 || before.Repayments != 0 {
		t.Fatalf("pending credit receipt changed financial state: %+v", before)
	}
	review := AdjustmentReviewInput{
		AdjustmentNo: pending.AdjustmentNo, Decision: AdjustmentDecisionApprove,
		ReviewerID: 702, CommentDigest: strings.Repeat("6", 64),
		EvidenceDigest: strings.Repeat("7", 64),
	}
	posted, err := service.ReviewAdjustment(ctx, review)
	if err != nil || posted == nil || posted.Status != AdjustmentStatusPosted ||
		posted.CreditStatementNo != fixture.statementNo || posted.CreditRepayment == nil ||
		posted.CreditRepayment.FundingSourceType != "manual_receipt" ||
		posted.CreditRepayment.StatementOutstandingMinor != 600 ||
		posted.CreditRepayment.AccountAvailableMinor != 7_400 ||
		posted.LedgerTransactionNo == "" {
		t.Fatalf("approve credit receipt: result=%+v err=%v", posted, err)
	}
	replayed, err := service.ReviewAdjustment(ctx, review)
	if err != nil || replayed == nil || !replayed.Reused ||
		replayed.AdjustmentNo != posted.AdjustmentNo ||
		replayed.LedgerTransactionNo != posted.LedgerTransactionNo ||
		replayed.CreditRepayment == nil ||
		replayed.CreditRepayment.RepaymentNo != posted.CreditRepayment.RepaymentNo {
		t.Fatalf("replay credit receipt approval: result=%+v err=%v", replayed, err)
	}
	changed := input
	changed.AmountMinor++
	if _, err = service.CreateManualReceipt(ctx, changed); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("changed credit receipt reused idempotency key: %v", err)
	}

	var facts struct {
		TransactionType  string `orm:"transaction_type"`
		SourceType       string `orm:"source_type"`
		CashDebitMinor   int64  `orm:"cash_debit_minor"`
		ReceivableCredit int64  `orm:"receivable_credit_minor"`
		RepaymentCount   int64  `orm:"repayment_count"`
		AdjustmentCount  int64  `orm:"adjustment_count"`
		ApprovalCount    int64  `orm:"approval_count"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    transaction.transaction_type_code AS transaction_type,
    transaction.source_type_code AS source_type,
    (SELECT COALESCE(SUM(entry.amount_minor), 0)
     FROM plugin_idc_core_billing_ledger_entry entry
     JOIN plugin_idc_core_billing_ledger_account account
       ON account.account_no = entry.account_no
     WHERE entry.transaction_no = transaction.transaction_no
       AND entry.direction_code = 'debit'
       AND account.account_type_code = 'merchant_cash') AS cash_debit_minor,
    (SELECT COALESCE(SUM(entry.amount_minor), 0)
     FROM plugin_idc_core_billing_ledger_entry entry
     JOIN plugin_idc_core_billing_ledger_account account
       ON account.account_no = entry.account_no
     WHERE entry.transaction_no = transaction.transaction_no
       AND entry.direction_code = 'credit'
       AND account.account_type_code = 'accounts_receivable') AS receivable_credit_minor,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment
     WHERE funding_source_type_code = 'manual_receipt'
       AND funding_source_reference = ?) AS repayment_count,
    (SELECT COUNT(*) FROM plugin_idc_core_billing_adjustment_request
     WHERE adjustment_no = ?) AS adjustment_count,
    (SELECT COUNT(*) FROM plugin_idc_core_billing_adjustment_approval
     WHERE adjustment_no = ?) AS approval_count
FROM plugin_idc_core_billing_ledger_transaction transaction
WHERE transaction.transaction_no = ?`, posted.AdjustmentNo, posted.AdjustmentNo,
		posted.AdjustmentNo, posted.LedgerTransactionNo); err != nil {
		t.Fatalf("load posted credit receipt facts: %v", err)
	}
	if facts.TransactionType != "manual_receipt" || facts.SourceType != "credit_repayment" ||
		facts.CashDebitMinor != 400 || facts.ReceivableCredit != 400 ||
		facts.RepaymentCount != 1 || facts.AdjustmentCount != 1 || facts.ApprovalCount != 1 {
		t.Fatalf("unexpected posted credit receipt facts: %+v", facts)
	}
}

func TestFinancialAdjustmentPostgreSQLSerializesConcurrentReceiptReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatalf("create financial adjustment service: %v", err)
	}
	input := ManualReceiptInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 300,
		ReasonCode: "offline_balance_credit", ProofReference: "private-proof-concurrent-001",
		ReceivingAccountReference: "merchant-collection-cny-001",
		BusinessPurposeCode:       "balance_recharge", EvidenceDigest: strings.Repeat("2", 64),
		IdempotencyKey: "manual-receipt-concurrent-01", RequestedBy: 401,
	}
	const workers = 8
	results := make([]*FinancialAdjustment, workers)
	errors := make([]error, workers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range results {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.CreateManualReceipt(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	created := 0
	adjustmentNo := ""
	for index, result := range results {
		if errors[index] != nil || result == nil {
			t.Fatalf("concurrent receipt[%d]: result=%+v err=%v", index, result, errors[index])
		}
		if adjustmentNo == "" {
			adjustmentNo = result.AdjustmentNo
		} else if result.AdjustmentNo != adjustmentNo {
			t.Fatalf("concurrent receipt produced %q and %q", adjustmentNo, result.AdjustmentNo)
		}
		if !result.Reused {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("concurrent receipt created count=%d want=1", created)
	}
	count, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_adjustment_request
WHERE adjustment_no = ?`, adjustmentNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("concurrent receipt request count=%d err=%v", count.Int(), err)
	}
}

func assertReviewedInvoiceAdjustmentEvidence(
	t *testing.T,
	ctx context.Context,
	database interface {
		GetScan(context.Context, any, string, ...any) error
	},
	receipt *FinancialAdjustment,
	waiver *FinancialAdjustment,
) {
	t.Helper()
	var state struct {
		PaidMinor       int64 `orm:"paid_minor"`
		WaivedMinor     int64 `orm:"waived_minor"`
		ReceiptPayments int   `orm:"receipt_payments"`
		WaiverRows      int   `orm:"waiver_rows"`
		Approvals       int   `orm:"approvals"`
		ReceiptCash     int64 `orm:"receipt_cash"`
		WaiverExpense   int64 `orm:"waiver_expense"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT invoice.paid_amount_minor AS paid_minor,
       invoice.waived_amount_minor AS waived_minor,
       (SELECT count(*) FROM plugin_idc_core_billing_payment_allocation
        WHERE invoice_no = invoice.invoice_no AND payment_source_type_code = 'manual_receipt') AS receipt_payments,
       (SELECT count(*) FROM plugin_idc_core_billing_invoice_waiver_allocation
        WHERE invoice_no = invoice.invoice_no) AS waiver_rows,
       (SELECT count(*) FROM plugin_idc_core_billing_adjustment_approval
        WHERE adjustment_no = ?) AS approvals,
       (SELECT COALESCE(sum(entry.amount_minor), 0)
        FROM plugin_idc_core_billing_ledger_entry entry
        JOIN plugin_idc_core_billing_ledger_account account ON account.account_no = entry.account_no
        WHERE entry.transaction_no = ? AND account.account_type_code = 'merchant_cash'
          AND entry.direction_code = 'debit') AS receipt_cash,
       (SELECT COALESCE(sum(entry.amount_minor), 0)
        FROM plugin_idc_core_billing_ledger_entry entry
        JOIN plugin_idc_core_billing_ledger_account account ON account.account_no = entry.account_no
        WHERE entry.transaction_no = ? AND account.account_type_code = 'waiver_expense'
          AND entry.direction_code = 'debit') AS waiver_expense
FROM plugin_idc_core_billing_invoice invoice
WHERE invoice.invoice_no = 'invoice-lifecycle'`,
		waiver.AdjustmentNo, receipt.LedgerTransactionNo, waiver.LedgerTransactionNo); err != nil {
		t.Fatalf("load reviewed adjustment evidence: %v", err)
	}
	if state.PaidMinor != 400 || state.WaivedMinor != 600 || state.ReceiptPayments != 1 ||
		state.WaiverRows != 1 || state.Approvals != 1 || state.ReceiptCash != 400 ||
		state.WaiverExpense != 600 {
		t.Fatalf("reviewed adjustment evidence=%+v", state)
	}
}

// Permission-authorized waivers post immediately without a threshold policy,
// documents or a human reason, while remaining separate from real cash paid.
func TestFinancialAdjustmentPostgreSQLWaiverPostsDirectlyWithoutDocuments(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceLifecycle(t, ctx, database)
	service, err := NewFinancialAdjustmentService(database)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CreateInvoiceWaiver(ctx, InvoiceWaiverInput{InvoiceNo: "invoice-lifecycle", AmountMinor: 1000, IdempotencyKey: "direct-waiver-without-documents-001", RequestedBy: 101})
	if err != nil || result == nil || result.Status != AdjustmentStatusPosted || result.ApprovalRequired || result.LedgerTransactionNo == "" || result.Invoice == nil || result.Invoice.PaidAmountMinor != 0 || result.Invoice.WaivedAmountMinor != 1000 {
		t.Fatalf("direct waiver: %+v %v", result, err)
	}
}
