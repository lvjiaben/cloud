// This file verifies billing invariants against an isolated real PostgreSQL schema.

package billing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

const billingIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

// PostgreSQL needs one lock entry for every object created or dropped with an
// isolated schema. Bound the complete schema lifecycle so high-count shuffled
// runs cannot exhaust the server lock table while retaining parallel coverage.
var billingIntegrationSchemaSlots = make(chan struct{}, 2)

func TestBalanceLedgerPostgreSQLPreservesFundsAndIdempotency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create billing balance service: %v", err)
	}

	rechargeInput := ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 10_000,
		SourceReference: "payment-confirmed-001", SourceSnapshotDigest: strings.Repeat("1", 64),
	}
	recharged, err := service.PostConfirmedRecharge(ctx, rechargeInput)
	if err != nil || recharged.BalanceMinor != 10_000 || recharged.AvailableBalanceMinor != 10_000 || recharged.Reused {
		t.Fatalf("post confirmed recharge: result=%+v err=%v", recharged, err)
	}
	replayed, err := service.PostConfirmedRecharge(ctx, rechargeInput)
	if err != nil || !replayed.Reused || replayed.TransactionNo != recharged.TransactionNo {
		t.Fatalf("replay confirmed recharge: result=%+v err=%v", replayed, err)
	}
	changedRecharge := rechargeInput
	changedRecharge.AmountMinor++
	if _, err = service.PostConfirmedRecharge(ctx, changedRecharge); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed recharge replay should conflict, got %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_merchant_profile SET enabled = FALSE WHERE tenant_id = 0`); err != nil {
		t.Fatalf("disable merchant for replay check: %v", err)
	}
	if replayed, err = service.PostConfirmedRecharge(ctx, rechargeInput); err != nil || !replayed.Reused {
		t.Fatalf("merchant disable blocked immutable replay: result=%+v err=%v", replayed, err)
	}
	if _, err = service.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1,
		SourceReference: "disabled-merchant-recharge", SourceSnapshotDigest: strings.Repeat("f", 64),
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("disabled merchant accepted new recharge: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_merchant_profile SET enabled = TRUE WHERE tenant_id = 0`); err != nil {
		t.Fatalf("restore merchant after replay check: %v", err)
	}

	hold, err := service.CreateHold(ctx, HoldCreateInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 6_000,
		SourceType: "checkout", SourceReference: "checkout-hold-001",
		SourceSnapshotDigest: strings.Repeat("2", 64), ExpiresAt: time.Now().Add(10 * time.Minute),
	})
	if err != nil || hold.Status != "active" || hold.RemainingAmountMinor != 6_000 {
		t.Fatalf("create balance hold: result=%+v err=%v", hold, err)
	}
	if _, err = service.CreateHold(ctx, HoldCreateInput{
		CustomerNo: "customer-ledger-other", CurrencyCode: "CNY", AmountMinor: 1,
		SourceType: "checkout", SourceReference: "checkout-hold-001",
		SourceSnapshotDigest: strings.Repeat("2", 64), ExpiresAt: hold.ExpiresAt,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("hold source was rebound to another customer: %v", err)
	}
	if _, err = service.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 5_000,
		SourceReference: "overspend-held-balance", SourceSnapshotDigest: strings.Repeat("3", 64),
	}); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("held balance was spendable: %v", err)
	}
	if _, err = service.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "USD", AmountMinor: 1,
		SourceReference: "cross-currency-spend", SourceSnapshotDigest: strings.Repeat("4", 64),
	}); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("cross-currency balance was spendable: %v", err)
	}

	verifyConcurrentAvailableBalanceConsumption(t, ctx, service)
	heldConsumption := HoldConsumeInput{
		HoldNo: hold.HoldNo, AmountMinor: 2_500, SourceReference: "hold-consumption-001",
		SourceSnapshotDigest: strings.Repeat("5", 64),
	}
	consumedHold, err := service.ConsumeHold(ctx, heldConsumption)
	if err != nil || consumedHold.BalanceMinor != 4_500 || consumedHold.HeldBalanceMinor != 3_500 ||
		consumedHold.AvailableBalanceMinor != 1_000 {
		t.Fatalf("consume held balance: result=%+v err=%v", consumedHold, err)
	}
	consumedHoldReplay, err := service.ConsumeHold(ctx, heldConsumption)
	if err != nil || !consumedHoldReplay.Reused || consumedHoldReplay.TransactionNo != consumedHold.TransactionNo {
		t.Fatalf("replay held consumption: result=%+v err=%v", consumedHoldReplay, err)
	}
	released, err := service.ReleaseHold(ctx, hold.HoldNo)
	if err != nil || released.Status != "released" || released.ReleasedAmountMinor != 3_500 ||
		released.RemainingAmountMinor != 0 {
		t.Fatalf("release remaining hold: result=%+v err=%v", released, err)
	}

	spendInput := BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceReference: "balance-spend-001", SourceSnapshotDigest: strings.Repeat("6", 64),
	}
	spent, err := service.ConsumeAvailableBalance(ctx, spendInput)
	if err != nil || spent.BalanceMinor != 3_500 || spent.AvailableBalanceMinor != 3_500 {
		t.Fatalf("consume released balance: result=%+v err=%v", spent, err)
	}
	reversalInput := ReversalInput{
		OriginalTransactionNo: spent.TransactionNo, SourceReference: "reverse-balance-spend-001",
		SourceSnapshotDigest: strings.Repeat("7", 64),
	}
	reversed, err := service.ReverseTransaction(ctx, reversalInput)
	if err != nil || reversed.BalanceMinor != 4_500 || reversed.AmountMinor != 1_000 {
		t.Fatalf("reverse balance consumption: result=%+v err=%v", reversed, err)
	}
	reversedReplay, err := service.ReverseTransaction(ctx, reversalInput)
	if err != nil || !reversedReplay.Reused || reversedReplay.TransactionNo != reversed.TransactionNo {
		t.Fatalf("replay balance reversal: result=%+v err=%v", reversedReplay, err)
	}
	if _, err = service.ReverseTransaction(ctx, ReversalInput{
		OriginalTransactionNo: spent.TransactionNo, SourceReference: "reverse-balance-spend-002",
		SourceSnapshotDigest: strings.Repeat("8", 64),
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("second full reversal should conflict, got %v", err)
	}
	if _, err = service.ReverseTransaction(ctx, ReversalInput{
		OriginalTransactionNo: recharged.TransactionNo, SourceReference: "reverse-spent-recharge",
		SourceSnapshotDigest: strings.Repeat("9", 64),
	}); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("spent recharge reversal should preserve non-negative balance, got %v", err)
	}

	projection, err := service.RebuildCustomerAccountProjection(
		ctx, "customer-ledger", "CNY", AccountTypeCustomerBalance,
	)
	if err != nil || !projection.Consistent || projection.RebuiltBalanceMinor != 4_500 ||
		projection.SnapshotBalanceMinor != 4_500 || projection.AvailableBalanceMinor != 4_500 ||
		projection.TransactionCount != 5 || projection.EntryCount != 5 {
		t.Fatalf("rebuild customer balance projection: result=%+v err=%v", projection, err)
	}
	assertLedgerDatabaseInvariants(t, ctx, database, projection.AccountNo)
}

func TestBalanceLedgerPostgreSQLConcurrentReplayPostsOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create billing balance service: %v", err)
	}
	input := ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 2_000,
		SourceReference: "concurrent-payment-confirmation", SourceSnapshotDigest: strings.Repeat("a", 64),
	}
	const workers = 8
	results := make([]*Posting, workers)
	errors := make([]error, workers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errors[index] = service.PostConfirmedRecharge(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	transactionNo := ""
	newCount := 0
	for index := range results {
		if errors[index] != nil || results[index] == nil {
			t.Fatalf("concurrent recharge[%d]: result=%+v err=%v", index, results[index], errors[index])
		}
		if transactionNo == "" {
			transactionNo = results[index].TransactionNo
		} else if results[index].TransactionNo != transactionNo {
			t.Fatalf("concurrent replay produced multiple transactions: %q != %q",
				results[index].TransactionNo, transactionNo)
		}
		if !results[index].Reused {
			newCount++
		}
	}
	if newCount != 1 {
		t.Fatalf("concurrent replay new result count=%d want=1", newCount)
	}
	count, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_billing_ledger_transaction
WHERE source_type_code = 'confirmed_recharge' AND source_reference = ?`, input.SourceReference)
	if err != nil || count.Int() != 1 {
		t.Fatalf("concurrent replay transaction count=%d err=%v", count.Int(), err)
	}
}

func TestBalanceLedgerPostgreSQLExpiresHoldWithoutSpending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	service, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create billing balance service: %v", err)
	}
	if _, err = service.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 500,
		SourceReference: "expiry-recharge", SourceSnapshotDigest: strings.Repeat("b", 64),
	}); err != nil {
		t.Fatalf("prepare expiring hold balance: %v", err)
	}
	hold, err := service.CreateHold(ctx, HoldCreateInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 400,
		SourceType: "checkout", SourceReference: "expiring-hold",
		SourceSnapshotDigest: strings.Repeat("c", 64), ExpiresAt: time.Now().Add(300 * time.Millisecond),
	})
	if err != nil {
		t.Fatalf("create expiring hold: %v", err)
	}
	if _, err = service.ExpireHold(ctx, hold.HoldNo); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("future hold expired early: %v", err)
	}
	time.Sleep(350 * time.Millisecond)
	expired, err := service.ExpireHold(ctx, hold.HoldNo)
	if err != nil || expired.Status != "expired" || expired.ReleasedAmountMinor != 400 {
		t.Fatalf("expire elapsed hold: result=%+v err=%v", expired, err)
	}
	projection, err := service.RebuildCustomerAccountProjection(
		ctx, "customer-ledger", "CNY", AccountTypeCustomerBalance,
	)
	if err != nil || projection.RebuiltBalanceMinor != 500 || projection.HeldBalanceMinor != 0 {
		t.Fatalf("expiry changed ledger balance: result=%+v err=%v", projection, err)
	}
}

func TestInvoiceLifecyclePostgreSQLDerivesPaymentRefundAndReversalStates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceLifecycle(t, ctx, database)
	balanceService, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create invoice lifecycle balance service: %v", err)
	}
	invoiceService, err := NewInvoiceService(database)
	if err != nil {
		t.Fatalf("create invoice lifecycle service: %v", err)
	}
	issued, err := invoiceService.IssueDraft(ctx, "invoice-draft-lifecycle")
	if err != nil || issued.Status != InvoiceStatusPendingPayment || issued.IssuedAt == nil || issued.Reused {
		t.Fatalf("issue draft invoice: result=%+v err=%v", issued, err)
	}
	issuedReplay, err := invoiceService.IssueDraft(ctx, "invoice-draft-lifecycle")
	if err != nil || !issuedReplay.Reused || issuedReplay.Status != InvoiceStatusPendingPayment {
		t.Fatalf("replay draft invoice issue: result=%+v err=%v", issuedReplay, err)
	}
	cancelled, err := invoiceService.CancelUnpaid(ctx, "invoice-draft-lifecycle")
	if err != nil || cancelled.Status != InvoiceStatusCancelled || cancelled.CancelledAt == nil {
		t.Fatalf("cancel unpaid invoice: result=%+v err=%v", cancelled, err)
	}
	cancelledReplay, err := invoiceService.CancelUnpaid(ctx, "invoice-draft-lifecycle")
	if err != nil || !cancelledReplay.Reused || cancelledReplay.Status != InvoiceStatusCancelled {
		t.Fatalf("replay unpaid invoice cancellation: result=%+v err=%v", cancelledReplay, err)
	}
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceReference: "invoice-lifecycle-recharge", SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund invoice lifecycle balance: %v", err)
	}
	firstPayment, err := balanceService.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 400,
		SourceReference: "invoice-lifecycle-payment-1", SourceSnapshotDigest: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("post first invoice payment ledger transaction: %v", err)
	}
	firstAllocation, err := invoiceService.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: "invoice-lifecycle", SourceType: PaymentSourceBalance,
		SourceReference:     "invoice-lifecycle-allocation-1",
		LedgerTransactionNo: firstPayment.TransactionNo, AmountMinor: 400,
	})
	if err != nil || firstAllocation.Status != InvoiceStatusPartiallyPaid ||
		firstAllocation.PaidAmountMinor != 400 || firstAllocation.RefundableAmountMinor != 0 ||
		firstAllocation.BecamePaid {
		t.Fatalf("apply first invoice payment: result=%+v err=%v", firstAllocation, err)
	}
	if _, err = invoiceService.MarkOverdue(ctx, "invoice-lifecycle"); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("future invoice became overdue: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_billing_invoice
SET payment_due_at = CURRENT_TIMESTAMP - INTERVAL '1 minute'
WHERE invoice_no = 'invoice-lifecycle'`); err != nil {
		t.Fatalf("move invoice due date into past: %v", err)
	}
	overdue, err := invoiceService.MarkOverdue(ctx, "invoice-lifecycle")
	if err != nil || overdue.Status != InvoiceStatusOverdue || overdue.PaidAmountMinor != 400 {
		t.Fatalf("mark partially-paid invoice overdue: result=%+v err=%v", overdue, err)
	}
	secondPayment, err := balanceService.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 600,
		SourceReference: "invoice-lifecycle-payment-2", SourceSnapshotDigest: strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatalf("post second invoice payment ledger transaction: %v", err)
	}
	secondInput := InvoicePaymentInput{
		InvoiceNo: "invoice-lifecycle", SourceType: PaymentSourceBalance,
		SourceReference:     "invoice-lifecycle-allocation-2",
		LedgerTransactionNo: secondPayment.TransactionNo, AmountMinor: 600,
	}
	paid, err := invoiceService.ApplyPayment(ctx, secondInput)
	if err != nil || paid.Status != InvoiceStatusPaid || paid.PaidAmountMinor != 1_000 ||
		paid.OutstandingAmountMinor != 0 || paid.RefundableAmountMinor != 800 ||
		paid.SettledAt == nil || !paid.BecamePaid {
		t.Fatalf("settle invoice: result=%+v err=%v", paid, err)
	}
	replayed, err := invoiceService.ApplyPayment(ctx, secondInput)
	if err != nil || !replayed.Reused || replayed.BecamePaid ||
		replayed.AllocationNo != paid.AllocationNo {
		t.Fatalf("replay invoice settlement: result=%+v err=%v", replayed, err)
	}
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1,
		SourceReference: "invoice-overpayment-recharge", SourceSnapshotDigest: strings.Repeat("d", 64),
	}); err != nil {
		t.Fatalf("fund overpayment evidence: %v", err)
	}
	overpayment, err := balanceService.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1,
		SourceReference: "invoice-overpayment-ledger", SourceSnapshotDigest: strings.Repeat("e", 64),
	})
	if err != nil {
		t.Fatalf("post overpayment ledger evidence: %v", err)
	}
	if _, err = invoiceService.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: "invoice-lifecycle", SourceType: PaymentSourceBalance,
		SourceReference:     "invoice-overpayment-allocation",
		LedgerTransactionNo: overpayment.TransactionNo, AmountMinor: 1,
	}); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("settled invoice accepted overpayment: %v", err)
	}
	refundTransactionNo := postTestRefundLedger(
		t, ctx, balanceService.(*serviceImpl), 300, "invoice-lifecycle-refund-ledger",
	)
	refundInput := InvoiceRefundInput{
		InvoiceNo: "invoice-lifecycle", InvoiceItemNo: "invoice-lifecycle-item",
		PaymentAllocationNo: paid.AllocationNo, SourceType: RefundSourceBalanceCredit,
		SourceReference:     "invoice-lifecycle-refund-allocation",
		LedgerTransactionNo: refundTransactionNo, AmountMinor: 300,
	}
	partiallyRefunded, err := invoiceService.ApplyRefund(ctx, refundInput)
	if err != nil || partiallyRefunded.Status != InvoiceStatusPartiallyRefunded ||
		partiallyRefunded.RefundedAmountMinor != 300 ||
		partiallyRefunded.RefundableAmountMinor != 500 {
		t.Fatalf("apply partial invoice refund: result=%+v err=%v", partiallyRefunded, err)
	}
	refundReplay, err := invoiceService.ApplyRefund(ctx, refundInput)
	if err != nil || !refundReplay.Reused || refundReplay.AllocationNo != partiallyRefunded.AllocationNo {
		t.Fatalf("replay partial invoice refund: result=%+v err=%v", refundReplay, err)
	}
	refundReversal, err := balanceService.ReverseTransaction(ctx, ReversalInput{
		OriginalTransactionNo: refundTransactionNo,
		SourceReference:       "invoice-lifecycle-refund-ledger-reversal",
		SourceSnapshotDigest:  strings.Repeat("f", 64),
	})
	if err != nil {
		t.Fatalf("post refund ledger reversal: %v", err)
	}
	refundedBack, err := invoiceService.ReverseRefund(ctx, InvoiceRefundReversalInput{
		OriginalRefundAllocationNo: partiallyRefunded.AllocationNo,
		SourceReference:            "invoice-lifecycle-refund-allocation-reversal",
		LedgerTransactionNo:        refundReversal.TransactionNo, AmountMinor: 300,
	})
	if err != nil || refundedBack.Status != InvoiceStatusPaid ||
		refundedBack.RefundedAmountMinor != 0 || refundedBack.RefundableAmountMinor != 800 {
		t.Fatalf("reverse invoice refund: result=%+v err=%v", refundedBack, err)
	}
	firstFullRefundTransactionNo := postTestRefundLedger(
		t, ctx, balanceService.(*serviceImpl), 400, "invoice-lifecycle-full-refund-ledger-1",
	)
	firstFullRefund, err := invoiceService.ApplyRefund(ctx, InvoiceRefundInput{
		InvoiceNo: "invoice-lifecycle", InvoiceItemNo: "invoice-lifecycle-item",
		PaymentAllocationNo: firstAllocation.AllocationNo, SourceType: RefundSourceBalanceCredit,
		SourceReference:     "invoice-lifecycle-full-refund-allocation-1",
		LedgerTransactionNo: firstFullRefundTransactionNo, AmountMinor: 400,
	})
	if err != nil || firstFullRefund.Status != InvoiceStatusPartiallyRefunded ||
		firstFullRefund.RefundedAmountMinor != 400 || firstFullRefund.RefundableAmountMinor != 400 {
		t.Fatalf("apply first full-refund tranche: result=%+v err=%v", firstFullRefund, err)
	}
	secondFullRefundTransactionNo := postTestRefundLedger(
		t, ctx, balanceService.(*serviceImpl), 400, "invoice-lifecycle-full-refund-ledger-2",
	)
	secondFullRefund, err := invoiceService.ApplyRefund(ctx, InvoiceRefundInput{
		InvoiceNo: "invoice-lifecycle", InvoiceItemNo: "invoice-lifecycle-item",
		PaymentAllocationNo: paid.AllocationNo, SourceType: RefundSourceBalanceCredit,
		SourceReference:     "invoice-lifecycle-full-refund-allocation-2",
		LedgerTransactionNo: secondFullRefundTransactionNo, AmountMinor: 400,
	})
	if err != nil || secondFullRefund.Status != InvoiceStatusRefunded ||
		secondFullRefund.RefundedAmountMinor != 800 || secondFullRefund.RefundableAmountMinor != 0 {
		t.Fatalf("complete full invoice refund: result=%+v err=%v", secondFullRefund, err)
	}
	secondFullRefundReversal := reverseTestTransaction(
		t, ctx, balanceService, secondFullRefundTransactionNo,
		"invoice-lifecycle-full-refund-ledger-reversal-2", "7",
	)
	secondFullRefundBack, err := invoiceService.ReverseRefund(ctx, InvoiceRefundReversalInput{
		OriginalRefundAllocationNo: secondFullRefund.AllocationNo,
		SourceReference:            "invoice-lifecycle-full-refund-allocation-reversal-2",
		LedgerTransactionNo:        secondFullRefundReversal.TransactionNo, AmountMinor: 400,
	})
	if err != nil || secondFullRefundBack.Status != InvoiceStatusPartiallyRefunded ||
		secondFullRefundBack.RefundedAmountMinor != 400 || secondFullRefundBack.RefundableAmountMinor != 400 {
		t.Fatalf("reverse second full-refund tranche: result=%+v err=%v", secondFullRefundBack, err)
	}
	firstFullRefundReversal := reverseTestTransaction(
		t, ctx, balanceService, firstFullRefundTransactionNo,
		"invoice-lifecycle-full-refund-ledger-reversal-1", "8",
	)
	firstFullRefundBack, err := invoiceService.ReverseRefund(ctx, InvoiceRefundReversalInput{
		OriginalRefundAllocationNo: firstFullRefund.AllocationNo,
		SourceReference:            "invoice-lifecycle-full-refund-allocation-reversal-1",
		LedgerTransactionNo:        firstFullRefundReversal.TransactionNo, AmountMinor: 400,
	})
	if err != nil || firstFullRefundBack.Status != InvoiceStatusPaid ||
		firstFullRefundBack.RefundedAmountMinor != 0 || firstFullRefundBack.RefundableAmountMinor != 800 {
		t.Fatalf("restore paid invoice after full-refund reversals: result=%+v err=%v", firstFullRefundBack, err)
	}
	paymentReversal, err := balanceService.ReverseTransaction(ctx, ReversalInput{
		OriginalTransactionNo: secondPayment.TransactionNo,
		SourceReference:       "invoice-lifecycle-payment-ledger-reversal",
		SourceSnapshotDigest:  strings.Repeat("0", 64),
	})
	if err != nil {
		t.Fatalf("post payment ledger reversal: %v", err)
	}
	reopened, err := invoiceService.ReversePayment(ctx, InvoicePaymentReversalInput{
		OriginalAllocationNo: paid.AllocationNo,
		SourceReference:      "invoice-lifecycle-payment-allocation-reversal",
		LedgerTransactionNo:  paymentReversal.TransactionNo, AmountMinor: 600,
	})
	if err != nil || reopened.Status != InvoiceStatusOverdue || reopened.PaidAmountMinor != 400 ||
		reopened.RefundableAmountMinor != 0 || reopened.SettledAt != nil {
		t.Fatalf("reverse settled invoice payment: result=%+v err=%v", reopened, err)
	}
	writeoffTransactionNo := postTestWriteoffLedger(
		t, ctx, balanceService.(*serviceImpl), 600, "invoice-lifecycle-writeoff-ledger",
	)
	badDebtInput := InvoiceBadDebtInput{
		InvoiceNo: "invoice-lifecycle", LedgerTransactionNo: writeoffTransactionNo,
	}
	badDebt, err := invoiceService.RecognizeBadDebt(ctx, badDebtInput)
	if err != nil || badDebt.Status != InvoiceStatusBadDebt ||
		badDebt.PaidAmountMinor != 400 || badDebt.OutstandingAmountMinor != 600 {
		t.Fatalf("recognize invoice bad debt: result=%+v err=%v", badDebt, err)
	}
	badDebtReplay, err := invoiceService.RecognizeBadDebt(ctx, badDebtInput)
	if err != nil || !badDebtReplay.Reused || badDebtReplay.Status != InvoiceStatusBadDebt {
		t.Fatalf("replay invoice bad debt: result=%+v err=%v", badDebtReplay, err)
	}
	if _, err = invoiceService.ApplyPayment(ctx, InvoicePaymentInput{
		InvoiceNo: "invoice-lifecycle", SourceType: PaymentSourceBalance,
		SourceReference:     "invoice-bad-debt-recovery-without-writeoff-reversal",
		LedgerTransactionNo: overpayment.TransactionNo, AmountMinor: 1,
	}); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("bad debt accepted payment without writeoff recovery: %v", err)
	}
	assertInvoiceAllocationInvariants(t, ctx, database)
}

func TestInvoiceBalancePaymentPostgreSQLIsAtomicAndIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceLifecycle(t, ctx, database)
	balanceService, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create atomic invoice balance service: %v", err)
	}
	invoiceService, err := NewInvoiceService(database)
	if err != nil {
		t.Fatalf("create atomic invoice service: %v", err)
	}
	input := InvoiceBalancePaymentInput{
		InvoiceNo: "invoice-lifecycle", SourceReference: "renewal-balance-payment-001",
		SourceSnapshotDigest: strings.Repeat("9", 64),
	}
	if _, err = invoiceService.PayFromBalance(ctx, input); !bizerr.Is(err, CodeInsufficientBalance) {
		t.Fatalf("insufficient atomic invoice payment should fail closed: %v", err)
	}
	assertBillingCount(t, ctx, database,
		`SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
		 WHERE source_type_code = 'invoice_balance_payment'
		   AND source_reference = 'renewal-balance-payment-001'`, 0,
		"ledger transaction after insufficient invoice payment")
	assertBillingCount(t, ctx, database,
		`SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
		 WHERE payment_source_reference = 'renewal-balance-payment-001'`, 0,
		"allocation after insufficient invoice payment")
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceReference:      "renewal-balance-recharge-001",
		SourceSnapshotDigest: strings.Repeat("8", 64),
	}); err != nil {
		t.Fatalf("fund atomic invoice balance: %v", err)
	}
	paid, err := invoiceService.PayFromBalance(ctx, input)
	if err != nil || paid == nil || paid.Reused || paid.Posting == nil || paid.Invoice == nil ||
		paid.Posting.AmountMinor != 1_000 || paid.Posting.BalanceMinor != 0 ||
		paid.Invoice.Status != InvoiceStatusPaid || paid.Invoice.PaidAmountMinor != 1_000 ||
		!paid.Invoice.BecamePaid {
		t.Fatalf("settle invoice atomically from balance: result=%+v err=%v", paid, err)
	}
	replayed, err := invoiceService.PayFromBalance(ctx, input)
	if err != nil || replayed == nil || !replayed.Reused || !replayed.Posting.Reused ||
		!replayed.Invoice.Reused || replayed.Invoice.BecamePaid ||
		replayed.Posting.TransactionNo != paid.Posting.TransactionNo ||
		replayed.Invoice.AllocationNo != paid.Invoice.AllocationNo {
		t.Fatalf("replay atomic invoice balance payment: result=%+v err=%v", replayed, err)
	}
	changed := input
	changed.SourceSnapshotDigest = strings.Repeat("7", 64)
	if _, err = invoiceService.PayFromBalance(ctx, changed); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("changed atomic invoice payment replay should conflict: %v", err)
	}
	assertBillingCount(t, ctx, database,
		`SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction
		 WHERE source_type_code = 'invoice_balance_payment'
		   AND source_reference = 'renewal-balance-payment-001'`, 1,
		"atomic invoice payment ledger transaction")
	assertBillingCount(t, ctx, database,
		`SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation
		 WHERE payment_source_reference = 'renewal-balance-payment-001'`, 1,
		"atomic invoice payment allocation")
}

func assertBillingCount(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	query string,
	want int,
	label string,
) {
	t.Helper()
	value, err := database.GetValue(ctx, query)
	if err != nil {
		t.Fatalf("count %s: %v", label, err)
	}
	if got := value.Int(); got != want {
		t.Fatalf("%s count=%d, want %d", label, got, want)
	}
}

func TestInvoicePaymentPostgreSQLSerializesConcurrentReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	seedInvoiceLifecycle(t, ctx, database)
	balanceService, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create concurrent invoice balance service: %v", err)
	}
	invoiceService, err := NewInvoiceService(database)
	if err != nil {
		t.Fatalf("create concurrent invoice service: %v", err)
	}
	if _, err = invoiceService.IssueDraft(ctx, "invoice-draft-lifecycle"); err != nil {
		t.Fatalf("issue concurrent invoice fixture: %v", err)
	}
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 100,
		SourceReference: "invoice-concurrent-recharge", SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("fund concurrent invoice payment: %v", err)
	}
	payment, err := balanceService.ConsumeAvailableBalance(ctx, BalanceConsumptionInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 100,
		SourceReference: "invoice-concurrent-payment", SourceSnapshotDigest: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("post concurrent invoice payment evidence: %v", err)
	}
	input := InvoicePaymentInput{
		InvoiceNo: "invoice-draft-lifecycle", SourceType: PaymentSourceBalance,
		SourceReference:     "invoice-concurrent-allocation",
		LedgerTransactionNo: payment.TransactionNo, AmountMinor: 100,
	}
	const workerCount = 8
	results := make([]*Invoice, workerCount)
	errors := make([]error, workerCount)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for index := range workerCount {
		go func() {
			defer workers.Done()
			<-start
			results[index], errors[index] = invoiceService.ApplyPayment(ctx, input)
		}()
	}
	close(start)
	workers.Wait()
	basedAllocationNo := ""
	becamePaidCount := 0
	for index := range workerCount {
		if errors[index] != nil {
			t.Fatalf("concurrent payment worker %d: %v", index, errors[index])
		}
		if results[index] == nil {
			t.Fatalf("concurrent payment worker %d returned no projection", index)
		}
		if results[index].Status != InvoiceStatusPaid || results[index].PaidAmountMinor != 100 {
			t.Fatalf("concurrent payment worker %d result=%+v", index, results[index])
		}
		if basedAllocationNo == "" {
			basedAllocationNo = results[index].AllocationNo
		} else if results[index].AllocationNo != basedAllocationNo {
			t.Fatalf("concurrent replay created different allocations: first=%q current=%q", basedAllocationNo, results[index].AllocationNo)
		}
		if results[index].BecamePaid {
			becamePaidCount++
		}
	}
	if becamePaidCount != 1 {
		t.Fatalf("concurrent replay produced %d paid transitions, want 1", becamePaidCount)
	}
	var allocationCount int
	allocationCountValue, err := database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_billing_payment_allocation
WHERE invoice_no = 'invoice-draft-lifecycle'`)
	if err != nil {
		t.Fatalf("count concurrent invoice payment allocations: %v", err)
	}
	allocationCount = allocationCountValue.Int()
	if allocationCount != 1 {
		t.Fatalf("concurrent replay persisted %d payment allocations, want 1", allocationCount)
	}
}

func seedInvoiceLifecycle(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, source_code, currency_code, status_code, item_count,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, paid_amount_minor, refundable_amount_minor,
     refunded_amount_minor, source_snapshot_digest, content_digest,
     payment_due_at, issued_at, version, created_at, updated_at)
VALUES
    ('invoice-lifecycle', 'customer-ledger', 'manual', 'CNY', 'pending_payment', 1,
     1000, 0, 1000, 0, 1000, 0, 0, 0, repeat('1', 64), repeat('2', 64),
     CURRENT_TIMESTAMP + INTERVAL '1 hour', CURRENT_TIMESTAMP, 1,
     CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, source_reference, description_zh_cn, description_en_us,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, refundable_cap_minor, content_digest)
VALUES
    ('invoice-lifecycle-item', 'invoice-lifecycle', 'customer-ledger', 'CNY', 1,
     'manual', 'invoice-lifecycle-source', '测试账单', 'Test invoice',
     1000, 0, 1000, 0, 1000, 800, repeat('3', 64));
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, source_code, currency_code, status_code, item_count,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, paid_amount_minor, refundable_amount_minor,
     refunded_amount_minor, source_snapshot_digest, content_digest,
     payment_due_at, version, created_at, updated_at)
VALUES
    ('invoice-draft-lifecycle', 'customer-ledger', 'manual', 'CNY', 'draft', 1,
     100, 0, 100, 0, 100, 0, 0, 0, repeat('4', 64), repeat('5', 64),
     CURRENT_TIMESTAMP + INTERVAL '1 hour', 1,
     CURRENT_TIMESTAMP - INTERVAL '1 minute', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, source_reference, description_zh_cn, description_en_us,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, refundable_cap_minor, content_digest)
VALUES
    ('invoice-draft-lifecycle-item', 'invoice-draft-lifecycle', 'customer-ledger', 'CNY', 1,
     'manual', 'invoice-draft-source', '草稿账单', 'Draft invoice',
     100, 0, 100, 0, 100, 100, repeat('6', 64))`); err != nil {
		t.Fatalf("seed invoice lifecycle: %v", err)
	}
}

func postTestRefundLedger(
	t *testing.T,
	ctx context.Context,
	service *serviceImpl,
	amountMinor int64,
	sourceReference string,
) string {
	t.Helper()
	transactionNo := ""
	err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := service.lockSource(ctx, "test_refund", sourceReference); err != nil {
			return err
		}
		balanceSpec := customerBalanceSpec("customer-ledger")
		revenueSpec := merchantRevenueSpec("merchant-ledger")
		accounts, err := service.ensureAndLockAccounts(ctx, "CNY", true, balanceSpec, revenueSpec)
		if err != nil {
			return err
		}
		balanceAccountNo := deterministicAccountNo(
			balanceSpec.OwnerType, balanceSpec.OwnerReference, balanceSpec.AccountType, "CNY",
		)
		revenueAccountNo := deterministicAccountNo(
			revenueSpec.OwnerType, revenueSpec.OwnerReference, revenueSpec.AccountType, "CNY",
		)
		transaction, _, err := service.postLocked(
			ctx, "refund", "CNY", "test_refund", sourceReference, strings.Repeat("9", 64), "",
			[]entrySemantic{
				{AccountNo: revenueAccountNo, Direction: "debit", Role: "refund",
					AmountMinor: amountMinor, Reason: "confirmed_refund"},
				{AccountNo: balanceAccountNo, Direction: "credit", Role: "balance",
					AmountMinor: amountMinor, Reason: "confirmed_refund"},
			}, accounts,
		)
		if err == nil {
			transactionNo = transaction.TransactionNo
		}
		return err
	})
	if err != nil || transactionNo == "" {
		t.Fatalf("post test refund ledger transaction=%q err=%v", transactionNo, err)
	}
	return transactionNo
}

func reverseTestTransaction(
	t *testing.T,
	ctx context.Context,
	service BalanceService,
	originalTransactionNo string,
	sourceReference string,
	digestCharacter string,
) *Posting {
	t.Helper()
	reversal, err := service.ReverseTransaction(ctx, ReversalInput{
		OriginalTransactionNo: originalTransactionNo,
		SourceReference:       sourceReference,
		SourceSnapshotDigest:  strings.Repeat(digestCharacter, 64),
	})
	if err != nil {
		t.Fatalf("reverse test ledger transaction %q: %v", originalTransactionNo, err)
	}
	return reversal
}

func postTestWriteoffLedger(
	t *testing.T,
	ctx context.Context,
	service *serviceImpl,
	amountMinor int64,
	sourceReference string,
) string {
	t.Helper()
	transactionNo := ""
	err := service.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := service.lockSource(ctx, "test_writeoff", sourceReference); err != nil {
			return err
		}
		receivableSpec := accountSpec{
			OwnerType: "merchant", OwnerReference: "merchant-ledger",
			AccountType: "accounts_receivable", NormalBalance: "debit",
		}
		expenseSpec := accountSpec{
			OwnerType: "merchant", OwnerReference: "merchant-ledger",
			AccountType: "bad_debt_expense", NormalBalance: "debit",
		}
		accounts, err := service.ensureAndLockAccounts(
			ctx, "CNY", true, receivableSpec, expenseSpec,
		)
		if err != nil {
			return err
		}
		receivableAccountNo := deterministicAccountNo(
			receivableSpec.OwnerType, receivableSpec.OwnerReference,
			receivableSpec.AccountType, "CNY",
		)
		expenseAccountNo := deterministicAccountNo(
			expenseSpec.OwnerType, expenseSpec.OwnerReference, expenseSpec.AccountType, "CNY",
		)
		transaction, _, err := service.postLocked(
			ctx, "writeoff", "CNY", "test_writeoff", sourceReference,
			strings.Repeat("8", 64), "",
			[]entrySemantic{
				{AccountNo: expenseAccountNo, Direction: "debit", Role: "expense",
					AmountMinor: amountMinor, InvoiceNo: "invoice-lifecycle", Reason: "bad_debt"},
				{AccountNo: receivableAccountNo, Direction: "credit", Role: "receivable",
					AmountMinor: amountMinor, InvoiceNo: "invoice-lifecycle", Reason: "bad_debt"},
			}, accounts,
		)
		if err == nil {
			transactionNo = transaction.TransactionNo
		}
		return err
	})
	if err != nil || transactionNo == "" {
		t.Fatalf("post test writeoff ledger transaction=%q err=%v", transactionNo, err)
	}
	return transactionNo
}

func assertInvoiceAllocationInvariants(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	var state struct {
		PaidMinor     int64 `orm:"paid_minor"`
		PaymentMinor  int64 `orm:"payment_minor"`
		RefundedMinor int64 `orm:"refunded_minor"`
		RefundMinor   int64 `orm:"refund_minor"`
		PaymentRows   int64 `orm:"payment_rows"`
		RefundRows    int64 `orm:"refund_rows"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT invoice.paid_amount_minor AS paid_minor,
       COALESCE(SUM(CASE payment.direction_code WHEN 'apply' THEN payment.amount_minor ELSE -payment.amount_minor END), 0) AS payment_minor,
       invoice.refunded_amount_minor AS refunded_minor,
       (SELECT COALESCE(SUM(CASE refund.direction_code WHEN 'apply' THEN refund.amount_minor ELSE -refund.amount_minor END), 0)
        FROM plugin_idc_core_billing_refund_allocation refund
        WHERE refund.invoice_no = invoice.invoice_no) AS refund_minor,
       COUNT(payment.id) AS payment_rows,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_refund_allocation refund
        WHERE refund.invoice_no = invoice.invoice_no) AS refund_rows
FROM plugin_idc_core_billing_invoice invoice
LEFT JOIN plugin_idc_core_billing_payment_allocation payment
  ON payment.invoice_no = invoice.invoice_no
WHERE invoice.invoice_no = 'invoice-lifecycle'
GROUP BY invoice.invoice_no, invoice.paid_amount_minor, invoice.refunded_amount_minor`); err != nil {
		t.Fatalf("load invoice allocation invariants: %v", err)
	}
	if state.PaidMinor != state.PaymentMinor || state.RefundedMinor != state.RefundMinor ||
		state.PaymentRows != 3 || state.RefundRows != 6 {
		t.Fatalf("invoice allocation invariants=%+v", state)
	}
}

func verifyConcurrentAvailableBalanceConsumption(
	t *testing.T,
	ctx context.Context,
	service BalanceService,
) {
	t.Helper()
	inputs := []BalanceConsumptionInput{
		{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 3_000,
			SourceReference: "concurrent-spend-a", SourceSnapshotDigest: strings.Repeat("d", 64)},
		{CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 3_000,
			SourceReference: "concurrent-spend-b", SourceSnapshotDigest: strings.Repeat("e", 64)},
	}
	errors := make([]error, len(inputs))
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, errors[index] = service.ConsumeAvailableBalance(ctx, inputs[index])
		}(index)
	}
	close(start)
	wait.Wait()
	successes := 0
	insufficient := 0
	for _, operationErr := range errors {
		if operationErr == nil {
			successes++
		} else if bizerr.Is(operationErr, CodeInsufficientBalance) {
			insufficient++
		} else {
			t.Fatalf("unexpected concurrent balance error: %v", operationErr)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent balance outcome successes=%d insufficient=%d errors=%v",
			successes, insufficient, errors)
	}
}

func assertLedgerDatabaseInvariants(t *testing.T, ctx context.Context, database gdb.DB, accountNo string) {
	t.Helper()
	var state struct {
		Transactions int   `orm:"transaction_count"`
		Entries      int   `orm:"entry_count"`
		Snapshots    int   `orm:"snapshot_count"`
		DebitTotal   int64 `orm:"debit_total"`
		CreditTotal  int64 `orm:"credit_total"`
		FundKinds    int   `orm:"fund_kind_count"`
	}
	if err := database.GetScan(ctx, &state, `
SELECT
  (SELECT count(*) FROM plugin_idc_core_billing_ledger_transaction) AS transaction_count,
  (SELECT count(*) FROM plugin_idc_core_billing_ledger_entry) AS entry_count,
  (SELECT count(*) FROM plugin_idc_core_billing_balance_snapshot) AS snapshot_count,
  (SELECT COALESCE(SUM(debit_total_minor), 0) FROM plugin_idc_core_billing_ledger_transaction) AS debit_total,
  (SELECT COALESCE(SUM(credit_total_minor), 0) FROM plugin_idc_core_billing_ledger_transaction) AS credit_total,
  (SELECT count(DISTINCT account.account_type_code)
   FROM plugin_idc_core_billing_ledger_entry entry
   JOIN plugin_idc_core_billing_ledger_account account ON account.account_no = entry.account_no
   WHERE account.customer_no = 'customer-ledger') AS fund_kind_count`); err != nil {
		t.Fatalf("load ledger invariant state: %v", err)
	}
	if state.Transactions != 5 || state.Entries != 10 || state.Snapshots != 10 ||
		state.DebitTotal != state.CreditTotal || state.FundKinds != 1 {
		t.Fatalf("unexpected ledger invariant state: %+v", state)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_billing_balance_snapshot
SET closing_balance_minor = closing_balance_minor + 1
WHERE account_no = ?`, accountNo); err == nil {
		t.Fatal("immutable billing snapshot accepted mutation")
	}
}

func configureBillingServiceIntegrationDB(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(billingIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL billing service tests", billingIntegrationDSNEnv)
	}
	billingIntegrationSchemaSlots <- struct{}{}
	t.Cleanup(func() { <-billingIntegrationSchemaSlots })
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create billing service integration admin database: %v", err)
	}
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate billing service integration schema: %v", err)
	}
	schema := "linapro_billing_service_test_" + hex.EncodeToString(suffix[:])
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteBillingIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated billing service schema: %v", err)
	}
	database, err := gdb.New(gdb.ConfigNode{
		Link: appendBillingDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped billing service database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped billing service database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_billing_service_test_") {
			t.Errorf("refusing to drop unexpected billing service schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteBillingIdentifier(schema)+" CASCADE"); dropErr != nil {
			t.Errorf("drop isolated billing service schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close billing service admin database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, billingServiceDictionaryFixtureSQL); err != nil {
		t.Fatalf("create billing service dictionary fixtures: %v", err)
	}
	for _, path := range billingServiceMigrationPaths(t) {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read billing service migration %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply billing service migration %s: %v", filepath.Base(path), err)
		}
	}
	return database
}

func seedBillingService(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_merchant_profile
    (tenant_id, merchant_no, display_name, legal_name, default_currency, enabled)
VALUES (0, 'merchant-ledger', 'Ledger Merchant', 'Ledger Merchant Ltd.', 'CNY', TRUE);
INSERT INTO plugin_idc_core_customer_subject
    (tenant_id, customer_no, customer_code, customer_type_code, status_code, display_name)
VALUES
    (0, 'customer-ledger', 10001, 'individual', 'active', 'Ledger Customer'),
    (0, 'customer-ledger-other', 10002, 'individual', 'active', 'Other Ledger Customer');
INSERT INTO plugin_idc_core_customer_contact
    (tenant_id, contact_no, customer_no, display_name, phone_country_code, phone, phone_lookup_digest)
VALUES
    (0, 'contact-ledger', 'customer-ledger', 'Ledger Customer', '+86', '13800138001', repeat('1',64)),
    (0, 'contact-ledger-other', 'customer-ledger-other', 'Other Ledger Customer', '+86', '13800138002', repeat('2',64));
INSERT INTO plugin_idc_core_customer_contact_role
    (tenant_id, customer_no, contact_no, role_code, status_code)
VALUES
    (0, 'customer-ledger', 'contact-ledger', 'primary', 'active'),
    (0, 'customer-ledger-other', 'contact-ledger-other', 'primary', 'active')`); err != nil {
		t.Fatalf("seed billing service identities: %v", err)
	}
}

func billingServiceMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve billing service migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	return []string{
		filepath.Join(root, "001-core-foundation.sql"),
		filepath.Join(root, "003-customer-account.sql"),
		filepath.Join(root, "010-catalog-foundation.sql"),
		filepath.Join(root, "013-catalog-billing-policy.sql"),
		filepath.Join(root, "014-catalog-inventory-routing.sql"),
		filepath.Join(root, "019-catalog-tax-policy.sql"),
		filepath.Join(root, "022-commerce-foundation.sql"),
		filepath.Join(root, "023-billing-ledger-foundation.sql"),
		filepath.Join(root, "027-billing-invoice-lifecycle.sql"),
		filepath.Join(root, "028-billing-invoice-transform.sql"),
		filepath.Join(root, "029-billing-reviewed-adjustment.sql"),
		filepath.Join(root, "030-service-renewal-foundation.sql"),
		filepath.Join(root, "031-service-renewal-execution.sql"),
		filepath.Join(root, "032-service-expiry-scheduling.sql"),
		filepath.Join(root, "033-billing-directory-indexes.sql"),
		filepath.Join(root, "034-credit-account-foundation.sql"),
		filepath.Join(root, "035-financial-settlement.sql"),
		filepath.Join(root, "036-trial-qualification.sql"),
		filepath.Join(root, "037-financial-settlement-fulfillment.sql"),
		filepath.Join(root, "051-balance-recharge-context.sql"),
		filepath.Join(root, "052-online-refund.sql"),
		filepath.Join(root, "053-credit-refund-lifecycle.sql"),
		filepath.Join(root, "054-credit-online-refund-integration.sql"),
		filepath.Join(root, "055-credit-evidence.sql"),
		filepath.Join(root, "070-customer-short-code.sql"),
		filepath.Join(root, "108-credit-directory-pagination-indexes.sql"),
	}
}

func quoteBillingIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendBillingDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

const billingServiceDictionaryFixtureSQL = `
CREATE TABLE sys_dict_type (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0, dict_type VARCHAR(128) NOT NULL DEFAULT '',
    label VARCHAR(128) NOT NULL DEFAULT '', value VARCHAR(128) NOT NULL DEFAULT '',
    sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, dict_type, value)
);`
