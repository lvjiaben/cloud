package billing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreditUsageServiceIssuesBoundedStatementAndMovesPrincipal(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	account := activateCreditAccountForUsageTest(
		t, ctx, database, "customer-ledger", 8_000, "statement",
	)

	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load test timezone: %v", err)
	}
	localNow := time.Now().In(location)
	periodEnd := time.Date(localNow.Year(), localNow.Month(), 15, 0, 0, 0, 0, location)
	if !periodEnd.Before(localNow) {
		periodEnd = periodEnd.AddDate(0, -1, 0)
	}
	periodStart := periodEnd.AddDate(0, -1, 0)
	createdAt := periodEnd.Add(-24 * time.Hour).UTC()
	mustCreditTransaction(t, ctx, database, "seed historical unbilled exposure", func(ctx context.Context) error {
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure (
    exposure_no, credit_account_no, customer_no, currency_code,
    source_type_code, source_reference, original_principal_minor,
    unbilled_principal_minor, status_code, source_snapshot_digest,
    content_digest, created_at, updated_at
) VALUES ('credit-statement-exposure-1', ?, 'customer-ledger', 'CNY',
          'manual', 'credit-statement-exposure-source-1', 1000, 1000,
	          'unbilled', ?, ?, ?, ?)`,
			account.CreditAccountNo, strings.Repeat("1", 64), strings.Repeat("2", 64),
			createdAt, createdAt); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 7000, unbilled_principal_minor = 1000,
    version = 2, updated_at = ?
WHERE credit_account_no = ?`, createdAt, account.CreditAccountNo); execErr != nil {
			return execErr
		}
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    movement_no, credit_account_no, customer_no, currency_code, exposure_no,
    movement_type_code, amount_minor, account_version_before,
    account_version_after, status_before_code, status_after_code,
    total_limit_before_minor, total_limit_after_minor, available_before_minor,
    available_after_minor, unbilled_before_minor, unbilled_after_minor,
    billed_before_minor, billed_after_minor, delinquent_before_minor,
    delinquent_after_minor, written_off_before_minor, written_off_after_minor,
    source_type_code, source_reference, idempotency_digest, evidence_digest,
    content_digest, actor_type_code, actor_id, occurred_at
) VALUES ('credit-statement-reserve-movement-1', ?, 'customer-ledger', 'CNY',
          'credit-statement-exposure-1', 'reserve', 1000, 1, 2,
          'active', 'active', 8000, 8000, 8000, 7000, 0, 1000,
          0, 0, 0, 0, 0, 0, 'manual',
          'credit-statement-exposure-source-1', ?, ?, ?, 'system', 0, ?)`,
			account.CreditAccountNo, strings.Repeat("3", 64), strings.Repeat("4", 64),
			strings.Repeat("5", 64), createdAt)
		return execErr
	})

	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}
	input := CreditStatementIssueInput{
		CreditAccountNo: account.CreditAccountNo,
		PeriodStartedAt: periodStart.UTC(), PeriodEndedAt: periodEnd.UTC(),
		IdempotencyKey: "credit-statement-issue-0001",
	}
	statement, err := service.IssueCreditStatement(ctx, input)
	if err != nil {
		t.Fatalf("issue credit statement: %v", err)
	}
	if statement.Reused || statement.LineCount != 1 || statement.PrincipalMinor != 1_000 ||
		statement.OutstandingMinor != 1_000 || statement.StatusCode != "open" ||
		statement.AccountAvailableMinor != 7_000 || statement.AccountUnbilledMinor != 0 ||
		statement.AccountBilledMinor != 1_000 || statement.AccountVersion != 3 {
		t.Fatalf("unexpected credit statement: %+v", statement)
	}
	if !statement.PaymentDueAt.Equal(statement.IssuedAt.AddDate(0, 0, 10)) {
		t.Fatalf("unexpected repayment period issued=%v due=%v", statement.IssuedAt, statement.PaymentDueAt)
	}
	replayed, err := service.IssueCreditStatement(ctx, input)
	if err != nil || !replayed.Reused || replayed.StatementNo != statement.StatementNo ||
		replayed.AccountVersion != 3 {
		t.Fatalf("statement replay=%+v err=%v", replayed, err)
	}
	changed := input
	changed.PeriodStartedAt = changed.PeriodStartedAt.Add(time.Hour)
	if _, err = service.IssueCreditStatement(ctx, changed); err == nil {
		t.Fatal("statement idempotency key accepted a changed period")
	}
	overlap := input
	overlap.IdempotencyKey = "credit-statement-overlap-0001"
	overlap.PeriodStartedAt = overlap.PeriodStartedAt.Add(time.Hour)
	if _, err = service.IssueCreditStatement(ctx, overlap); err == nil {
		t.Fatal("credit account accepted an overlapping statement period")
	}

	var facts struct {
		ExposureStatus string `orm:"exposure_status"`
		ExposureBilled int64  `orm:"exposure_billed"`
		LineCount      int64  `orm:"line_count"`
		BillMovements  int64  `orm:"bill_movements"`
	}
	if err = database.GetScan(ctx, &facts, `
SELECT
    (SELECT status_code FROM plugin_idc_core_credit_exposure
     WHERE exposure_no = 'credit-statement-exposure-1') AS exposure_status,
    (SELECT billed_principal_minor FROM plugin_idc_core_credit_exposure
     WHERE exposure_no = 'credit-statement-exposure-1') AS exposure_billed,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_statement_line
     WHERE statement_no = ?) AS line_count,
    (SELECT COUNT(*) FROM plugin_idc_core_credit_account_movement
     WHERE statement_no = ? AND movement_type_code = 'bill') AS bill_movements`,
		statement.StatementNo, statement.StatementNo); err != nil {
		t.Fatalf("load issued statement facts: %v", err)
	}
	if facts.ExposureStatus != "billed" || facts.ExposureBilled != 1_000 ||
		facts.LineCount != 1 || facts.BillMovements != 1 {
		t.Fatalf("unexpected issued statement facts: %+v", facts)
	}
}
