// This file verifies bounded merchant-timezone credit statement scheduling on PostgreSQL.

package billing

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCreditStatementSchedulerPostgreSQLIssuesDueAccountOnce(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load merchant timezone: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_merchant_profile
SET timezone = 'America/Los_Angeles'
WHERE tenant_id = 0 AND merchant_no = 'merchant-ledger'`); err != nil {
		t.Fatalf("configure merchant timezone: %v", err)
	}
	localNow := time.Now().In(location)
	latestCutoff := time.Date(
		localNow.Year(), localNow.Month(), 15, 0, 0, 0, 0, location,
	)
	if latestCutoff.After(localNow) {
		latestCutoff = latestCutoff.AddDate(0, -1, 0)
	}
	validFrom := latestCutoff.AddDate(0, -2, 0).Add(-time.Hour)
	account := activateCreditAccountForUsageTestWithTerms(
		t, ctx, database, "customer-ledger", 8_000, "scheduled-statement",
		validFrom, 10, 3,
	)
	service, err := NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create credit usage service: %v", err)
	}
	exposureCreatedAt := latestCutoff.Add(-time.Hour).UTC()
	mustCreditTransaction(t, ctx, database, "seed due scheduled exposure", func(ctx context.Context) error {
		if _, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure (
    exposure_no, credit_account_no, customer_no, currency_code,
    source_type_code, source_reference, original_principal_minor,
    unbilled_principal_minor, status_code, source_snapshot_digest,
    content_digest, created_at, updated_at
) VALUES ('credit-scheduled-exposure-1', ?, 'customer-ledger', 'CNY',
          'manual', 'scheduled-statement-exposure-0001', 1200, 1200,
          'unbilled', ?, ?, ?, ?)`,
			account.CreditAccountNo, strings.Repeat("1", 64), strings.Repeat("2", 64),
			exposureCreatedAt, exposureCreatedAt); execErr != nil {
			return execErr
		}
		if _, execErr := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 6800, unbilled_principal_minor = 1200,
    version = 2, updated_at = ?
WHERE tenant_id = 0 AND credit_account_no = ?`,
			exposureCreatedAt, account.CreditAccountNo); execErr != nil {
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
) VALUES ('credit-scheduled-reserve-movement-1', ?, 'customer-ledger', 'CNY',
          'credit-scheduled-exposure-1', 'reserve', 1200, 1, 2,
          'active', 'active', 8000, 8000, 8000, 6800, 0, 1200,
          0, 0, 0, 0, 0, 0, 'manual',
          'scheduled-statement-exposure-0001', ?, ?, ?, 'system', 0, ?)`,
			account.CreditAccountNo, strings.Repeat("3", 64), strings.Repeat("4", 64),
			strings.Repeat("5", 64), exposureCreatedAt)
		return execErr
	})

	batch, err := service.RunCreditStatementBatch(ctx, 32)
	if err != nil {
		t.Fatalf("run credit statement scheduler: %v", err)
	}
	if batch.Selected != 1 || batch.Issued != 1 || batch.Reused != 0 || batch.Failures != 0 {
		t.Fatalf("unexpected credit statement batch: %+v", batch)
	}
	var statement creditStatementRow
	if err = database.GetScan(ctx, &statement, `
SELECT statement_no, credit_account_no, customer_no, currency_code,
       terms_no, terms_version, period_started_at, period_ended_at,
       line_count, principal_minor, fee_minor, total_amount_minor,
       paid_amount_minor, written_off_amount_minor, outstanding_amount_minor,
       status_code, issued_at, payment_due_at, version
FROM plugin_idc_core_credit_statement
WHERE tenant_id = 0 AND credit_account_no = ?`, account.CreditAccountNo); err != nil {
		t.Fatalf("load scheduled statement: %v", err)
	}
	if statement.StatementNo == "" || statement.PrincipalMinor != 1_200 ||
		statement.PeriodEndedAt.In(location).Day() != 15 ||
		!statement.PeriodEndedAt.Equal(latestCutoff.UTC()) ||
		statement.PeriodEndedAt.Sub(statement.PeriodStartedAt) > 32*24*time.Hour {
		t.Fatalf("unexpected scheduled statement: %+v", statement)
	}

	replayed, err := service.RunCreditStatementBatch(ctx, 32)
	if err != nil || replayed.Selected != 0 || replayed.Issued != 0 || replayed.Failures != 0 {
		t.Fatalf("scheduler replay batch=%+v err=%v", replayed, err)
	}
	if _, err = service.RunCreditStatementBatch(ctx, maximumCreditStatementBatch+1); err == nil {
		t.Fatal("scheduler accepted an unbounded batch")
	}
}
