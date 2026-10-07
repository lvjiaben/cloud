// This file verifies billing input normalization and pure lifecycle derivation.

package billing

import (
	"math"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestBillingValidationRejectsAmbiguousOrUnsafeInputs(t *testing.T) {
	valid := ConfirmedRechargeInput{
		CustomerNo: "customer-001", CurrencyCode: "CNY", AmountMinor: 100,
		SourceReference: "payment-confirmation-001", SourceSnapshotDigest: strings.Repeat("a", 64),
	}
	if _, err := normalizeConfirmedRecharge(valid); err != nil {
		t.Fatalf("normalize valid confirmed recharge: %v", err)
	}
	cases := []ConfirmedRechargeInput{
		{CustomerNo: "customer-001", CurrencyCode: "cny", AmountMinor: 100,
			SourceReference: "payment-confirmation-001", SourceSnapshotDigest: strings.Repeat("a", 64)},
		{CustomerNo: "customer-001", CurrencyCode: "CNY", AmountMinor: 0,
			SourceReference: "payment-confirmation-001", SourceSnapshotDigest: strings.Repeat("a", 64)},
		{CustomerNo: "customer-001", CurrencyCode: "CNY", AmountMinor: 100,
			SourceReference: " payment-confirmation-001", SourceSnapshotDigest: strings.Repeat("a", 64)},
		{CustomerNo: "customer-001", CurrencyCode: "CNY", AmountMinor: 100,
			SourceReference: "payment-confirmation-001", SourceSnapshotDigest: strings.Repeat("A", 64)},
	}
	for index, input := range cases {
		if _, err := normalizeConfirmedRecharge(input); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("unsafe recharge case %d should be invalid, got %v", index, err)
		}
	}
	hold := HoldCreateInput{
		CustomerNo: "customer-001", CurrencyCode: "CNY", AmountMinor: 100,
		SourceType: "checkout", SourceReference: "quote-001",
		SourceSnapshotDigest: strings.Repeat("b", 64), ExpiresAt: time.Now().Add(time.Minute),
	}
	if _, err := normalizeHoldCreate(hold); err != nil {
		t.Fatalf("normalize valid hold: %v", err)
	}
	hold.SourceType = "Checkout"
	if _, err := normalizeHoldCreate(hold); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("uppercase hold source should be invalid, got %v", err)
	}
}

func TestBillingAccountNumbersSeparateCurrencyAndFundKind(t *testing.T) {
	balanceCNY := deterministicAccountNo("customer", "customer-001", AccountTypeCustomerBalance, "CNY")
	balanceUSD := deterministicAccountNo("customer", "customer-001", AccountTypeCustomerBalance, "USD")
	giftCNY := deterministicAccountNo("customer", "customer-001", AccountTypeCustomerGift, "CNY")
	commissionCNY := deterministicAccountNo("customer", "customer-001", AccountTypeCustomerCommission, "CNY")
	if balanceCNY == balanceUSD || balanceCNY == giftCNY || giftCNY == commissionCNY {
		t.Fatalf("account identity collapsed separated funds: %q %q %q %q",
			balanceCNY, balanceUSD, giftCNY, commissionCNY)
	}
	for _, number := range []string{balanceCNY, balanceUSD, giftCNY, commissionCNY} {
		if !strings.HasPrefix(number, "bac_") || len(number) != 36 {
			t.Fatalf("unexpected opaque account number %q", number)
		}
	}
}

func TestCalculateNormalBalanceRejectsOverflow(t *testing.T) {
	if value, ok := calculateNormalBalance(100, 20, 5, "debit"); !ok || value != 115 {
		t.Fatalf("debit-normal balance=%d ok=%v", value, ok)
	}
	if value, ok := calculateNormalBalance(100, 20, 5, "credit"); !ok || value != 85 {
		t.Fatalf("credit-normal balance=%d ok=%v", value, ok)
	}
	if _, ok := calculateNormalBalance(math.MaxInt64, 1, 0, "debit"); ok {
		t.Fatal("accepted positive int64 overflow")
	}
	if _, ok := calculateNormalBalance(math.MinInt64, 0, 1, "debit"); ok {
		t.Fatal("accepted negative int64 overflow")
	}
}

func TestProjectionPagePreservesSnapshotChainAcrossPages(t *testing.T) {
	entries := []projectionEntryRow{{
		TransactionID: 2001, TransactionNo: "txn-page-1001", LineNumber: 1,
		Direction: "credit", Role: "customer_fund", AmountMinor: 100,
		Reason: "confirmed_recharge", EntryDigest: strings.Repeat("a", 64),
		TransactionDigest: strings.Repeat("b", 64),
	}}
	steps, rebuiltBalance, err := rebuildSteps("credit", 500, entries)
	if err != nil {
		t.Fatalf("rebuild page after non-zero opening balance: %v", err)
	}
	if rebuiltBalance != 600 {
		t.Fatalf("rebuilt balance=%d, want 600", rebuiltBalance)
	}
	snapshots := []projectionSnapshotRow{{
		SnapshotNo: "snapshot-1001", TransactionNo: "txn-page-1001", SequenceNo: 1001,
		PreviousSnapshotNo: "snapshot-1000", PreviousSequenceNo: 1000,
		OpeningBalanceMinor: 500, CreditAmountMinor: 100, ClosingBalanceMinor: 600,
		ContentDigest: strings.Repeat("c", 64),
	}}
	consistent, snapshotBalance, previousSnapshotNo := projectionPageMatchesSnapshots(
		steps, snapshots, "credit", 1000, 500, "snapshot-1000",
	)
	if !consistent || snapshotBalance != 600 || previousSnapshotNo != "snapshot-1001" {
		t.Fatalf("cross-page chain consistent=%v balance=%d previous=%q",
			consistent, snapshotBalance, previousSnapshotNo)
	}
}

func TestInvoiceValidationRejectsCallerDefinedSources(t *testing.T) {
	balancePayment := InvoiceBalancePaymentInput{
		InvoiceNo: "invoice-001", SourceReference: "renewal-operation-001",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}
	if _, err := normalizeInvoiceBalancePayment(balancePayment); err != nil {
		t.Fatalf("normalize valid invoice balance payment: %v", err)
	}
	balancePayment.SourceSnapshotDigest = "caller-defined-digest"
	if _, err := normalizeInvoiceBalancePayment(balancePayment); !bizerr.Is(err, CodeInvoiceInvalid) {
		t.Fatalf("invalid invoice balance evidence should be rejected, got %v", err)
	}
	payment := InvoicePaymentInput{
		InvoiceNo: "invoice-001", SourceType: PaymentSourceChannel,
		SourceReference: "payment-001", LedgerTransactionNo: "transaction-001",
		AmountMinor: 100,
	}
	if _, err := normalizeInvoicePayment(payment); err != nil {
		t.Fatalf("normalize valid invoice payment: %v", err)
	}
	payment.SourceType = PaymentSourceType("operator_override")
	if _, err := normalizeInvoicePayment(payment); !bizerr.Is(err, CodeInvoiceInvalid) {
		t.Fatalf("caller-defined payment source should be invalid, got %v", err)
	}
	refund := InvoiceRefundInput{
		InvoiceNo: "invoice-001", PaymentAllocationNo: "allocation-001",
		SourceType: RefundSourceChannel, SourceReference: "refund-001",
		LedgerTransactionNo: "transaction-002", AmountMinor: 50,
	}
	if _, err := normalizeInvoiceRefund(refund); !bizerr.Is(err, CodeInvoiceInvalid) {
		t.Fatalf("item-less refund should be invalid, got %v", err)
	}
}

func TestDeriveInvoiceStatePreservesAmountsAndDueSemantics(t *testing.T) {
	now := time.Now().UTC()
	dueAt := now.Add(-time.Minute)
	issuedAt := now.Add(-time.Hour)
	invoice := &entity.BillingInvoice{
		InvoiceNo: "invoice-001", StatusCode: string(InvoiceStatusPartiallyPaid),
		TotalAmountMinor: 1_000, PaymentDueAt: &dueAt, IssuedAt: &issuedAt,
	}
	state, err := deriveInvoiceState(invoice, invoiceAggregate{
		PaymentMinor: 400, RefundableCapMinor: 800,
	}, now)
	if err != nil || state.Status != InvoiceStatusOverdue || state.PaidMinor != 400 ||
		state.RefundableMinor != 0 || state.SettledAt != nil {
		t.Fatalf("derive overdue partial payment: state=%+v err=%v", state, err)
	}

	dueAt = now.Add(time.Hour)
	state, err = deriveInvoiceState(invoice, invoiceAggregate{
		PaymentMinor: 1_000, RefundableCapMinor: 800,
	}, now)
	if err != nil || state.Status != InvoiceStatusPaid || state.RefundableMinor != 800 ||
		state.SettledAt == nil {
		t.Fatalf("derive paid invoice: state=%+v err=%v", state, err)
	}
	settledAt := *state.SettledAt
	invoice.StatusCode = string(InvoiceStatusPaid)
	invoice.SettledAt = &settledAt
	state, err = deriveInvoiceState(invoice, invoiceAggregate{
		PaymentMinor: 1_000, RefundMinor: 300, RefundableCapMinor: 800,
	}, now)
	if err != nil || state.Status != InvoiceStatusPartiallyRefunded ||
		state.RefundableMinor != 500 || state.RefundedMinor != 300 ||
		state.SettledAt == nil || !state.SettledAt.Equal(settledAt) {
		t.Fatalf("derive partial refund: state=%+v err=%v", state, err)
	}
	state, err = deriveInvoiceState(invoice, invoiceAggregate{
		PaymentMinor: 1_000, RefundMinor: 800, RefundableCapMinor: 800,
	}, now)
	if err != nil || state.Status != InvoiceStatusRefunded || state.RefundableMinor != 0 {
		t.Fatalf("derive full refundable refund: state=%+v err=%v", state, err)
	}
	if _, err = deriveInvoiceState(invoice, invoiceAggregate{
		PaymentMinor: 900, RefundMinor: 100, RefundableCapMinor: 800,
	}, now); !bizerr.Is(err, CodeInvoiceConflict) {
		t.Fatalf("refund with no longer fully-paid invoice should conflict, got %v", err)
	}
}
