package billing

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/billingcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestCreditUsageServiceAppliesPartialAndFullStatementRepayment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "statement-repayment",
	)
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}
	exposure, err := service.ReserveCredit(ctx, CreditReserveInput{
		CustomerNo: "customer-ledger", CurrencyCode: "CNY", AmountMinor: 1_000,
		SourceType: "manual", SourceReference: "credit-statement-repayment-exposure-0001",
		SourceSnapshotDigest: strings.Repeat("1", 64),
		ActorType:            "system", ActorID: 0,
	})
	if err != nil {
		t.Fatalf("reserve statement repayment exposure: %v", err)
	}
	statementNo := "credit-repayment-statement-1"
	periodEnd := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Microsecond)
	periodStart := periodEnd.AddDate(0, -1, 0)
	mustCreditTransaction(t, ctx, database, "seed issued credit statement", func(ctx context.Context) error {
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement (
    statement_no, credit_account_no, customer_no, currency_code, terms_no,
    terms_version, period_started_at, period_ended_at, line_count,
    principal_minor, total_amount_minor, outstanding_amount_minor,
    status_code, content_digest, issued_at, payment_due_at
) SELECT ?, account.credit_account_no, account.customer_no, account.currency_code,
         account.current_terms_no, account.current_terms_version, ?, ?, 1,
         1000, 1000, 1000, 'open', ?, CURRENT_TIMESTAMP,
         CURRENT_TIMESTAMP + make_interval(days => terms.repayment_period_days)
  FROM plugin_idc_core_credit_account account
  JOIN plugin_idc_core_credit_account_terms terms
    ON terms.terms_no = account.current_terms_no
   AND terms.credit_account_no = account.credit_account_no
 WHERE account.credit_account_no = ?`, statementNo, periodStart, periodEnd,
			strings.Repeat("2", 64), account.CreditAccountNo); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement_line (
    statement_line_no, statement_no, credit_account_no, customer_no,
    currency_code, line_number, line_type_code, exposure_no,
    source_type_code, source_reference, amount_minor, content_digest
) VALUES ('credit-repayment-statement-line-1', ?, ?, 'customer-ledger',
          'CNY', 1, 'principal', ?, 'exposure', ?, 1000, ?)`,
			statementNo, account.CreditAccountNo, exposure.ExposureNo,
			exposure.ExposureNo, strings.Repeat("3", 64)); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = 0, billed_principal_minor = 1000,
    status_code = 'billed', version = 2, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = ?`, exposure.ExposureNo); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET unbilled_principal_minor = 0, billed_principal_minor = 1000,
    version = 3, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = ?`, account.CreditAccountNo); execErr != nil {
			return execErr
		}
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    movement_no, credit_account_no, customer_no, currency_code, exposure_no,
    statement_no, movement_type_code, amount_minor, account_version_before,
    account_version_after, status_before_code, status_after_code,
    total_limit_before_minor, total_limit_after_minor, available_before_minor,
    available_after_minor, unbilled_before_minor, unbilled_after_minor,
    billed_before_minor, billed_after_minor, delinquent_before_minor,
    delinquent_after_minor, written_off_before_minor, written_off_after_minor,
    source_type_code, source_reference, idempotency_digest, evidence_digest,
    content_digest, actor_type_code, actor_id, occurred_at
) VALUES ('credit-repayment-bill-movement-1', ?, 'customer-ledger', 'CNY', ?, ?,
          'bill', 1000, 2, 3, 'active', 'active', 8000, 8000, 7000, 7000,
          1000, 0, 0, 1000, 0, 0, 0, 0, 'statement', ?, ?, ?, ?,
          'system', 0, CURRENT_TIMESTAMP)`, account.CreditAccountNo,
			exposure.ExposureNo, statementNo, statementNo, strings.Repeat("4", 64),
			strings.Repeat("5", 64), strings.Repeat("6", 64))
		return execErr
	})

	firstReference := "credit-statement-repayment-funding-0001"
	owner, err := newChannelPaymentOwnerTestService(t, database, financialSettlementServiceStub{}, financialPrincipalStub{audience: &principal.Audience{UserID: 77, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer-ledger"}})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := owner.PrepareCreditRepayment(ctx, billingcap.PrepareCreditRepayment{PaymentAttemptNo: firstReference, StatementNo: statementNo, AmountMinor: 400})
	if err != nil || prepared.CurrencyCode != "CNY" || prepared.AmountMinor != 400 || prepared.CustomerRefDigest == "customer-ledger" {
		t.Fatalf("prepared credit repayment=%+v err=%v", prepared, err)
	}
	confirmed, err := owner.ConfirmCreditRepayment(ctx, billingcap.ConfirmedCreditRepayment{PaymentAttemptNo: firstReference, StatementNo: statementNo, CurrencyCode: "CNY", AmountMinor: 400, EvidenceDigest: strings.Repeat("7", 64)})
	if err != nil || confirmed.StatementStatusCode != "partially_paid" || confirmed.StatementOutstandingMinor != 600 {
		t.Fatalf("confirmed credit repayment=%+v err=%v", confirmed, err)
	}
	firstInput := CreditStatementRepaymentInput{
		StatementNo: statementNo, FundingSourceType: "online_payment",
		FundingSourceReference: firstReference, LedgerTransactionNo: confirmed.TransactionNo,
		AmountMinor: 400, SourceSnapshotDigest: strings.Repeat("7", 64),
		ConfirmationEvidenceDigest: strings.Repeat("7", 64),
	}
	partial, err := service.ApplyConfirmedCreditStatementRepayment(ctx, firstInput)
	if err != nil {
		t.Fatalf("apply partial credit statement repayment: %v", err)
	}
	if !partial.Reused || partial.StatementStatusCode != "partially_paid" ||
		partial.StatementOutstandingMinor != 600 || partial.AccountAvailableMinor != 7_400 ||
		partial.AccountBilledMinor != 600 || partial.AccountVersion != 4 {
		t.Fatalf("unexpected partial statement repayment: %+v", partial)
	}
	changed := firstInput
	changed.ConfirmationEvidenceDigest = strings.Repeat("9", 64)
	if _, err = service.ApplyConfirmedCreditStatementRepayment(ctx, changed); err == nil {
		t.Fatal("statement repayment replay accepted changed evidence")
	}
	commission := firstInput
	commission.FundingSourceType = "commission"
	commission.FundingSourceReference = "credit-statement-commission-0001"
	if _, err = service.ApplyConfirmedCreditStatementRepayment(ctx, commission); err == nil {
		t.Fatal("statement repayment accepted commission funding")
	}

	secondReference := "credit-statement-repayment-funding-0002"
	secondTransaction := postCreditRepaymentLedger(t, ctx, database, 600, secondReference)
	full, err := service.ApplyConfirmedCreditStatementRepayment(ctx, CreditStatementRepaymentInput{
		StatementNo: statementNo, FundingSourceType: "online_payment",
		FundingSourceReference: secondReference, LedgerTransactionNo: secondTransaction,
		AmountMinor: 600, SourceSnapshotDigest: strings.Repeat("a", 64),
		ConfirmationEvidenceDigest: strings.Repeat("b", 64),
	})
	if err != nil {
		t.Fatalf("apply final credit statement repayment: %v", err)
	}
	if full.StatementStatusCode != "paid" || full.StatementOutstandingMinor != 0 ||
		full.AccountAvailableMinor != 8_000 || full.AccountBilledMinor != 0 ||
		full.AccountVersion != 5 {
		t.Fatalf("unexpected full statement repayment: %+v", full)
	}
	replayed, err := service.ApplyConfirmedCreditStatementRepayment(ctx, firstInput)
	if err != nil || !replayed.Reused || replayed.StatementStatusCode != "paid" ||
		replayed.AccountVersion != 5 {
		t.Fatalf("statement repayment replay=%+v err=%v", replayed, err)
	}

	overReference := "credit-statement-repayment-overpay-0001"
	overTransaction := postCreditRepaymentLedger(t, ctx, database, 1, overReference)
	if _, err = service.ApplyConfirmedCreditStatementRepayment(ctx, CreditStatementRepaymentInput{
		StatementNo: statementNo, FundingSourceType: "online_payment",
		FundingSourceReference: overReference, LedgerTransactionNo: overTransaction,
		AmountMinor: 1, SourceSnapshotDigest: strings.Repeat("c", 64),
		ConfirmationEvidenceDigest: strings.Repeat("d", 64),
	}); err == nil {
		t.Fatal("paid credit statement accepted an overpayment")
	}

	var facts struct {
		ExposureStatus     string `orm:"exposure_status"`
		ExposureRepaid     int64  `orm:"exposure_repaid"`
		AllocationCount    int64  `orm:"allocation_count"`
		RepaymentMovements int64  `orm:"repayment_movements"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT status_code FROM plugin_idc_core_credit_exposure
     WHERE exposure_no = ?) AS exposure_status,
    (SELECT repaid_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE exposure_no = ?) AS exposure_repaid,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_repayment_allocation
     WHERE statement_no = ?) AS allocation_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE statement_no = ? AND movement_type_code = 'repayment') AS repayment_movements`,
		exposure.ExposureNo, exposure.ExposureNo, statementNo, statementNo); err != nil {
		t.Fatalf("load statement repayment facts: %v", err)
	}
	if facts.ExposureStatus != "settled" || facts.ExposureRepaid != 1_000 ||
		facts.AllocationCount != 2 || facts.RepaymentMovements != 2 {
		t.Fatalf("unexpected statement repayment facts: %+v", facts)
	}
}
