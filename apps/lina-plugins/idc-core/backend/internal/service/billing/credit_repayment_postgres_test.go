package billing

import (
	"context"
	"strings"
	"testing"
)

func TestCreditUsageServiceAppliesConfirmedPrepaymentWithoutCommissionMixing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "prepayment",
	)
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}
	exposure, err := service.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-prepayment-exposure-0001",
		SourceSnapshotDigest: strings.Repeat("1", 64),
		ActorType:            "system", ActorID: 0,
	})
	if err != nil {
		t.Fatalf("reserve prepayment exposure: %v", err)
	}
	fundingReference := "credit-prepayment-funding-0001"
	transactionNo := postCreditRepaymentLedger(t, ctx, database, 400, fundingReference)
	input := CreditPrepaymentInput{
		ExposureNo: exposure.ExposureNo, FundingSourceType: "online_payment",
		FundingSourceReference: fundingReference, LedgerTransactionNo: transactionNo,
		AmountMinor: 400, SourceSnapshotDigest: strings.Repeat("2", 64),
		ConfirmationEvidenceDigest: strings.Repeat("3", 64),
	}
	repayment, err := service.ApplyConfirmedCreditPrepayment(ctx, input)
	if err != nil {
		t.Fatalf("apply confirmed credit prepayment: %v", err)
	}
	if repayment.Reused || repayment.RepaymentType != "prepayment" ||
		repayment.AmountMinor != 400 || repayment.AllocatedAmountMinor != 400 ||
		repayment.UnallocatedAmountMinor != 0 || repayment.StatusCode != "allocated" ||
		repayment.ExposureStatusCode != "partially_repaid" ||
		repayment.AccountAvailableMinor != 7_400 || repayment.AccountUnbilledMinor != 600 ||
		repayment.AccountVersion != 3 {
		t.Fatalf("unexpected credit prepayment: %+v", repayment)
	}
	replayed, err := service.ApplyConfirmedCreditPrepayment(ctx, input)
	if err != nil || !replayed.Reused || replayed.RepaymentNo != repayment.RepaymentNo ||
		replayed.AccountVersion != 3 {
		t.Fatalf("prepayment replay=%+v err=%v", replayed, err)
	}
	changed := input
	changed.ConfirmationEvidenceDigest = strings.Repeat("4", 64)
	if _, err = service.ApplyConfirmedCreditPrepayment(ctx, changed); err == nil {
		t.Fatal("prepayment replay accepted changed confirmation evidence")
	}
	commission := input
	commission.FundingSourceType = "commission"
	commission.FundingSourceReference = "credit-prepayment-commission-0001"
	if _, err = service.ApplyConfirmedCreditPrepayment(ctx, commission); err == nil {
		t.Fatal("credit prepayment accepted commission funding")
	}

	overReference := "credit-prepayment-over-limit-0001"
	overTransaction := postCreditRepaymentLedger(t, ctx, database, 700, overReference)
	if _, err = service.ApplyConfirmedCreditPrepayment(ctx, CreditPrepaymentInput{
		ExposureNo: exposure.ExposureNo, FundingSourceType: "online_payment",
		FundingSourceReference: overReference, LedgerTransactionNo: overTransaction,
		AmountMinor: 700, SourceSnapshotDigest: strings.Repeat("5", 64),
		ConfirmationEvidenceDigest: strings.Repeat("6", 64),
	}); err == nil {
		t.Fatal("credit prepayment exceeded remaining unbilled principal")
	}

	wrongReference := "credit-prepayment-wrong-ledger-0001"
	wrongTransaction := postCreditRepaymentLedger(t, ctx, database, 1, wrongReference)
	if _, err = service.ApplyConfirmedCreditPrepayment(ctx, CreditPrepaymentInput{
		ExposureNo: exposure.ExposureNo, FundingSourceType: "online_payment",
		FundingSourceReference: wrongReference, LedgerTransactionNo: wrongTransaction,
		AmountMinor: 2, SourceSnapshotDigest: strings.Repeat("7", 64),
		ConfirmationEvidenceDigest: strings.Repeat("8", 64),
	}); err == nil {
		t.Fatal("credit prepayment accepted mismatched ledger amount")
	}

	clearingReference := "credit-prepayment-clearing-ledger-0001"
	clearingTransaction := postCreditRepaymentLedgerWithDebitAccount(
		t, ctx, database, 100, clearingReference,
		accountSpec{
			OwnerType: "merchant", OwnerReference: "merchant-ledger",
			AccountType: "payment_clearing", NormalBalance: "debit",
		},
	)
	if _, err = service.ApplyConfirmedCreditPrepayment(ctx, CreditPrepaymentInput{
		ExposureNo: exposure.ExposureNo, FundingSourceType: "online_payment",
		FundingSourceReference: clearingReference, LedgerTransactionNo: clearingTransaction,
		AmountMinor: 100, SourceSnapshotDigest: strings.Repeat("9", 64),
		ConfirmationEvidenceDigest: strings.Repeat("a", 64),
	}); err == nil {
		t.Fatal("credit prepayment accepted payment-clearing debit as confirmed merchant cash")
	}

	var facts struct {
		RepaymentCount      int64 `orm:"repayment_count"`
		AllocationCount     int64 `orm:"allocation_count"`
		PrepaymentMovements int64 `orm:"prepayment_movements"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment
     WHERE credit_account_no = ?) AS repayment_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment_allocation
     WHERE repayment_no = ?) AS allocation_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE repayment_no = ? AND movement_type_code = 'prepayment') AS prepayment_movements`,
		repayment.CreditAccountNo, repayment.RepaymentNo, repayment.RepaymentNo); err != nil {
		t.Fatalf("load prepayment facts: %v", err)
	}
	if facts.RepaymentCount != 1 || facts.AllocationCount != 1 || facts.PrepaymentMovements != 1 {
		t.Fatalf("unexpected prepayment fact counts: %+v", facts)
	}
}
