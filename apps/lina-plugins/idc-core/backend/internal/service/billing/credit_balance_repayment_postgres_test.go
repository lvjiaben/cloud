// This file verifies atomic same-currency customer-balance credit repayments on PostgreSQL.

package billing

import (
	"context"
	"strings"
	"testing"
)

func TestCreditBalanceRepaymentPostgreSQLPostsAndAllocatesAtomically(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	fixture := prepareCreditDelinquencyFixture(
		t, ctx, database, -10, "balrepay",
	)
	balanceService, err := NewBalanceService(database)
	if err != nil {
		t.Fatalf("create balance service: %v", err)
	}
	if _, err = balanceService.PostConfirmedRecharge(ctx, ConfirmedRechargeInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 2_000,
		SourceReference:      "credit-balance-recharge-0001",
		SourceSnapshotDigest: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("seed customer balance: %v", err)
	}
	usageService, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}
	input := CreditBalanceRepaymentInput{
		StatementNo: fixture.statementNo, AmountMinor: 400,
		SourceReference:            "credit-balance-repayment-0001",
		SourceSnapshotDigest:       strings.Repeat("b", 64),
		ConfirmationEvidenceDigest: strings.Repeat("c", 64),
	}
	repayment, err := usageService.RepayCreditStatementFromBalance(ctx, input)
	if err != nil {
		t.Fatalf("repay credit statement from balance: %v", err)
	}
	if repayment.Reused || repayment.Repayment == nil ||
		repayment.Repayment.FundingSourceType != "customer_balance" ||
		repayment.Repayment.StatementOutstandingMinor != 600 ||
		repayment.Repayment.AccountAvailableMinor != 7_400 ||
		repayment.BalanceMinor != 1_600 || repayment.AvailableBalanceMinor != 1_600 {
		t.Fatalf("unexpected balance repayment: %+v", repayment)
	}
	replayed, err := usageService.RepayCreditStatementFromBalance(ctx, input)
	if err != nil || !replayed.Reused || replayed.Repayment == nil ||
		replayed.Repayment.RepaymentNo != repayment.Repayment.RepaymentNo ||
		replayed.BalanceMinor != 1_600 {
		t.Fatalf("balance repayment replay=%+v err=%v", replayed, err)
	}
	changed := input
	changed.AmountMinor++
	if _, err = usageService.RepayCreditStatementFromBalance(ctx, changed); err == nil {
		t.Fatal("balance repayment replay accepted changed amount")
	}

	var facts struct {
		TransactionType   string `orm:"transaction_type"`
		SourceType        string `orm:"source_type"`
		BalanceDebits     int64  `orm:"balance_debits"`
		ReceivableCredits int64  `orm:"receivable_credits"`
		RepaymentCount    int64  `orm:"repayment_count"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    transaction.transaction_type_code AS transaction_type,
    transaction.source_type_code AS source_type,
    (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_entry entry
     JOIN plugin_idc_core_billing_ledger_account account
       ON account.account_no = entry.account_no
     WHERE entry.transaction_no = transaction.transaction_no
       AND entry.direction_code = 'debit'
       AND account.account_type_code = 'customer_balance') AS balance_debits,
    (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_entry entry
     JOIN plugin_idc_core_billing_ledger_account account
       ON account.account_no = entry.account_no
     WHERE entry.transaction_no = transaction.transaction_no
       AND entry.direction_code = 'credit'
       AND account.account_type_code = 'accounts_receivable') AS receivable_credits,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment
     WHERE funding_source_type_code = 'customer_balance'
       AND funding_source_reference = ?) AS repayment_count
FROM plugin_idc_core_billing_ledger_transaction transaction
WHERE transaction.tenant_id = 0 AND transaction.source_type_code = 'credit_repayment'
  AND transaction.source_reference = ?`, input.SourceReference, input.SourceReference); err != nil {
		t.Fatalf("load balance repayment facts: %v", err)
	}
	if facts.TransactionType != "balance_payment" || facts.SourceType != "credit_repayment" ||
		facts.BalanceDebits != 1 || facts.ReceivableCredits != 1 || facts.RepaymentCount != 1 {
		t.Fatalf("unexpected balance repayment facts: %+v", facts)
	}
}
