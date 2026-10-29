package billing

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestCreditFoundationPostgreSQLPreservesApprovalAndMoneyInvariants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	database := configureBillingServiceIntegrationDB(t, ctx)
	seedBillingService(t, ctx, database)

	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_policy_revision
    (policy_no, policy_version, currency_code, maximum_limit_minor,
     dual_review_threshold_minor, billing_day, repayment_period_days,
     grace_period_days, daily_late_fee_basis_points, maximum_late_fee_minor,
     configured_by, content_digest)
VALUES ('credit-policy-cny', 1, 'CNY', 50000, 5000, 15, 10, 3, 10, 5000,
        100, repeat('a', 64));
INSERT INTO plugin_idc_core_billing_ledger_account
    (account_no, owner_type_code, owner_reference, customer_no,
     account_type_code, normal_balance_code, currency_code)
VALUES ('credit-ledger-cny', 'customer', 'customer-ledger', 'customer-ledger',
        'customer_credit', 'debit', 'CNY')`); err != nil {
		t.Fatalf("seed credit policy and ledger account: %v", err)
	}

	expectCreditTransactionRejected(t, ctx, database, "approved application without a decision", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application
    (application_no, customer_no, currency_code, policy_no, policy_version,
     requested_limit_minor, requested_valid_from, requested_valid_until,
     required_approval_count, risk_evidence_reference, risk_evidence_digest,
     request_digest, status_code, request_actor_type_code, requested_by, decided_at)
VALUES ('credit-application-invalid', 'customer-ledger', 'CNY',
        'credit-policy-cny', 1, 6000, CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP + INTERVAL '1 year', 2, 'private:risk/invalid',
        repeat('b', 64), repeat('c', 64), 'approved', 'staff', 150,
        CURRENT_TIMESTAMP);
`)
		return err
	})

	mustCreditTransaction(t, ctx, database, "grant approved credit", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application
    (application_no, customer_no, currency_code, policy_no, policy_version,
     requested_limit_minor, requested_valid_from, requested_valid_until,
     required_approval_count, risk_evidence_reference, risk_evidence_digest,
     request_digest, status_code, request_actor_type_code, requested_by, decided_at)
VALUES ('credit-application-cny', 'customer-ledger', 'CNY', 'credit-policy-cny', 1,
        10000, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year', 2,
        'private:risk/approved', repeat('1', 64), repeat('2', 64),
        'approved', 'staff', 150, CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_application_review
    (review_no, application_no, customer_no, currency_code, review_sequence,
     decision_code, reviewer_id, comment_digest, evidence_digest, content_digest)
VALUES
    ('credit-review-cny-1', 'credit-application-cny', 'customer-ledger', 'CNY',
     1, 'approved', 201, repeat('3', 64), repeat('4', 64), repeat('5', 64)),
    ('credit-review-cny-2', 'credit-application-cny', 'customer-ledger', 'CNY',
     2, 'approved', 202, repeat('6', 64), repeat('7', 64), repeat('8', 64));
INSERT INTO plugin_idc_core_credit_account
    (credit_account_no, customer_no, currency_code, ledger_account_no,
     origin_application_no, current_terms_no, current_terms_version, status_code,
     total_limit_minor, available_limit_minor, valid_from, valid_until)
VALUES ('credit-account-cny', 'customer-ledger', 'CNY', 'credit-ledger-cny',
        'credit-application-cny', 'credit-terms-cny-1', 1, 'active', 10000, 10000,
        CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year');
INSERT INTO plugin_idc_core_credit_account_terms
    (terms_no, credit_account_no, customer_no, currency_code, terms_version,
     application_no, policy_no, policy_version, total_limit_minor, billing_day,
     repayment_period_days, grace_period_days, daily_late_fee_basis_points,
     maximum_late_fee_minor, effective_from, effective_until, approved_by,
     approval_digest, content_digest)
SELECT 'credit-terms-cny-1', 'credit-account-cny', customer_no, currency_code, 1,
       application_no, policy_no, policy_version, requested_limit_minor, 15, 10,
       3, 10, 5000, requested_valid_from, requested_valid_until, 202,
       repeat('9', 64), repeat('a', 64)
FROM plugin_idc_core_credit_application
WHERE application_no = 'credit-application-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code,
     movement_type_code, amount_minor, account_version_before,
     account_version_after, status_before_code, status_after_code,
     total_limit_before_minor, total_limit_after_minor,
     available_before_minor, available_after_minor,
     unbilled_before_minor, unbilled_after_minor,
     billed_before_minor, billed_after_minor,
     delinquent_before_minor, delinquent_after_minor,
     written_off_before_minor, written_off_after_minor,
     source_type_code, source_reference, idempotency_digest, evidence_digest,
     content_digest, actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-1', 'credit-account-cny', 'customer-ledger', 'CNY',
        'limit_granted', 10000, 0, 1, NULL, 'active', 0, 10000, 0, 10000,
        0, 0, 0, 0, 0, 0, 0, 0, 'credit_application',
        'credit-application-cny', repeat('b', 64), repeat('c', 64),
        repeat('d', 64), 'staff', 202, CURRENT_TIMESTAMP)`)
		return err
	})

	mustCreditTransaction(t, ctx, database, "reserve and bill credit", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure
    (exposure_no, credit_account_no, customer_no, currency_code,
     source_type_code, source_reference, original_principal_minor,
     unbilled_principal_minor, status_code, source_snapshot_digest, content_digest)
VALUES ('credit-exposure-1', 'credit-account-cny', 'customer-ledger', 'CNY',
        'manual', 'order-credit-1', 3000, 3000, 'unbilled',
        repeat('e', 64), repeat('f', 64));
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 7000, unbilled_principal_minor = 3000,
    version = 2, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     movement_type_code, amount_minor, account_version_before, account_version_after,
     status_before_code, status_after_code, total_limit_before_minor,
     total_limit_after_minor, available_before_minor, available_after_minor,
     unbilled_before_minor, unbilled_after_minor, billed_before_minor,
     billed_after_minor, delinquent_before_minor, delinquent_after_minor,
     written_off_before_minor, written_off_after_minor, source_type_code,
     source_reference, idempotency_digest, evidence_digest, content_digest,
     actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-2', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-1', 'reserve', 3000, 1, 2, 'active', 'active',
        10000, 10000, 10000, 7000, 0, 3000, 0, 0, 0, 0, 0, 0,
        'order', 'order-credit-1', repeat('1', 64), repeat('2', 64),
        repeat('3', 64), 'system', 0, CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = 0, billed_principal_minor = 3000,
    status_code = 'billed', version = 2, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = 'credit-exposure-1';
UPDATE plugin_idc_core_credit_account
SET unbilled_principal_minor = 0, billed_principal_minor = 3000,
    version = 3, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_statement
    (statement_no, credit_account_no, customer_no, currency_code, terms_no,
     terms_version, period_started_at, period_ended_at, line_count,
     principal_minor, total_amount_minor, outstanding_amount_minor,
     content_digest, issued_at, payment_due_at)
VALUES ('credit-statement-1', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-terms-cny-1', 1, CURRENT_TIMESTAMP - INTERVAL '40 days',
        CURRENT_TIMESTAMP - INTERVAL '21 days', 1, 3000, 3000, 3000,
        repeat('4', 64), CURRENT_TIMESTAMP - INTERVAL '20 days',
        CURRENT_TIMESTAMP - INTERVAL '10 days');
INSERT INTO plugin_idc_core_credit_statement_line
    (statement_line_no, statement_no, credit_account_no, customer_no,
     currency_code, line_number, line_type_code, exposure_no, source_type_code,
     source_reference, amount_minor, content_digest)
VALUES ('credit-statement-line-1', 'credit-statement-1', 'credit-account-cny',
        'customer-ledger', 'CNY', 1, 'principal', 'credit-exposure-1',
        'exposure', 'credit-exposure-1', 3000, repeat('5', 64));
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     statement_no, movement_type_code, amount_minor, account_version_before,
     account_version_after, status_before_code, status_after_code,
     total_limit_before_minor, total_limit_after_minor, available_before_minor,
     available_after_minor, unbilled_before_minor, unbilled_after_minor,
     billed_before_minor, billed_after_minor, delinquent_before_minor,
     delinquent_after_minor, written_off_before_minor, written_off_after_minor,
     source_type_code, source_reference, idempotency_digest, evidence_digest,
     content_digest, actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-3', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-1', 'credit-statement-1', 'bill', 3000, 2, 3,
        'active', 'active', 10000, 10000, 7000, 7000, 3000, 0, 0, 3000,
        0, 0, 0, 0, 'statement', 'credit-statement-1', repeat('6', 64),
        repeat('7', 64), repeat('8', 64), 'system', 0, CURRENT_TIMESTAMP)`)
		return err
	})

	postCreditRepaymentLedger(t, ctx, database, 2000, "credit-payment-1")
	mustCreditTransaction(t, ctx, database, "repay billed credit", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment
    (repayment_no, credit_account_no, customer_no, currency_code,
     repayment_type_code, funding_source_type_code, funding_source_reference,
     ledger_transaction_no, amount_minor, allocated_amount_minor,
     unallocated_amount_minor, status_code, source_snapshot_digest,
     confirmation_evidence_digest, confirmed_at)
VALUES ('credit-repayment-1', 'credit-account-cny', 'customer-ledger', 'CNY',
        'statement', 'online_payment', 'credit-payment-1',
        (SELECT transaction_no
         FROM plugin_idc_core_billing_ledger_transaction
         WHERE source_type_code = 'credit_repayment'
           AND source_reference = 'credit-payment-1'),
        2000, 2000, 0, 'allocated', repeat('9', 64), repeat('a', 64),
        CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_repayment_allocation
    (allocation_no, repayment_no, credit_account_no, customer_no, currency_code,
     statement_no, allocation_type_code, amount_minor, source_reference,
     content_digest)
VALUES ('credit-allocation-1', 'credit-repayment-1', 'credit-account-cny',
        'customer-ledger', 'CNY', 'credit-statement-1', 'statement_principal',
        2000, 'credit-allocation-source-1', repeat('b', 64));
UPDATE plugin_idc_core_credit_statement
SET paid_amount_minor = 2000, outstanding_amount_minor = 1000,
    status_code = 'partially_paid', version = 2, updated_at = CURRENT_TIMESTAMP
WHERE statement_no = 'credit-statement-1';
UPDATE plugin_idc_core_credit_exposure
SET billed_principal_minor = 1000, repaid_principal_minor = 2000,
    status_code = 'partially_repaid', version = 3, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = 'credit-exposure-1';
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 9000, billed_principal_minor = 1000,
    version = 4, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     statement_no, repayment_no, movement_type_code, amount_minor,
     account_version_before, account_version_after, status_before_code,
     status_after_code, total_limit_before_minor, total_limit_after_minor,
     available_before_minor, available_after_minor, unbilled_before_minor,
     unbilled_after_minor, billed_before_minor, billed_after_minor,
     delinquent_before_minor, delinquent_after_minor, written_off_before_minor,
     written_off_after_minor, source_type_code, source_reference,
     idempotency_digest, evidence_digest, content_digest, actor_type_code,
     actor_id, occurred_at)
VALUES ('credit-movement-4', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-1', 'credit-statement-1', 'credit-repayment-1',
        'repayment', 2000, 3, 4, 'active', 'active', 10000, 10000, 7000,
        9000, 0, 0, 3000, 1000, 0, 0, 0, 0, 'repayment',
        'credit-repayment-1', repeat('c', 64), repeat('d', 64),
        repeat('e', 64), 'system', 0, CURRENT_TIMESTAMP)`)
		return err
	})

	postCreditRepaymentLedger(t, ctx, database, 500, "credit-prepayment-1")
	mustCreditTransaction(t, ctx, database, "reserve then prepay unbilled credit", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure
    (exposure_no, credit_account_no, customer_no, currency_code,
     source_type_code, source_reference, original_principal_minor,
     unbilled_principal_minor, status_code, source_snapshot_digest, content_digest)
VALUES ('credit-exposure-2', 'credit-account-cny', 'customer-ledger', 'CNY',
        'renewal', 'renewal-credit-1', 500, 500, 'unbilled',
        repeat('f', 64), repeat('1', 64));
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 8500, unbilled_principal_minor = 500,
    version = 5, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     movement_type_code, amount_minor, account_version_before, account_version_after,
     status_before_code, status_after_code, total_limit_before_minor,
     total_limit_after_minor, available_before_minor, available_after_minor,
     unbilled_before_minor, unbilled_after_minor, billed_before_minor,
     billed_after_minor, delinquent_before_minor, delinquent_after_minor,
     written_off_before_minor, written_off_after_minor, source_type_code,
     source_reference, idempotency_digest, evidence_digest, content_digest,
     actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-5', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-2', 'reserve', 500, 4, 5, 'active', 'active',
        10000, 10000, 9000, 8500, 0, 500, 1000, 1000, 0, 0, 0, 0,
        'renewal', 'renewal-credit-1', repeat('2', 64), repeat('3', 64),
        repeat('4', 64), 'system', 0, CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_repayment
    (repayment_no, credit_account_no, customer_no, currency_code,
     repayment_type_code, funding_source_type_code, funding_source_reference,
     ledger_transaction_no, amount_minor, allocated_amount_minor,
     unallocated_amount_minor, status_code, source_snapshot_digest,
     confirmation_evidence_digest, confirmed_at)
VALUES ('credit-prepayment-1', 'credit-account-cny', 'customer-ledger', 'CNY',
        'prepayment', 'online_payment', 'credit-prepayment-1',
        (SELECT transaction_no
         FROM plugin_idc_core_billing_ledger_transaction
         WHERE source_type_code = 'credit_repayment'
           AND source_reference = 'credit-prepayment-1'),
        500, 500, 0, 'allocated', repeat('5', 64), repeat('6', 64),
        CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_repayment_allocation
    (allocation_no, repayment_no, credit_account_no, customer_no, currency_code,
     exposure_no, allocation_type_code, amount_minor, source_reference,
     content_digest)
VALUES ('credit-allocation-2', 'credit-prepayment-1', 'credit-account-cny',
        'customer-ledger', 'CNY', 'credit-exposure-2', 'unbilled_principal',
        500, 'credit-allocation-source-2', repeat('7', 64));
UPDATE plugin_idc_core_credit_exposure
SET unbilled_principal_minor = 0, repaid_principal_minor = 500,
    status_code = 'settled', version = 2, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = 'credit-exposure-2';
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 9000, unbilled_principal_minor = 0,
    version = 6, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     repayment_no, movement_type_code, amount_minor, account_version_before,
     account_version_after, status_before_code, status_after_code,
     total_limit_before_minor, total_limit_after_minor, available_before_minor,
     available_after_minor, unbilled_before_minor, unbilled_after_minor,
     billed_before_minor, billed_after_minor, delinquent_before_minor,
     delinquent_after_minor, written_off_before_minor, written_off_after_minor,
     source_type_code, source_reference, idempotency_digest, evidence_digest,
     content_digest, actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-6', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-2', 'credit-prepayment-1', 'prepayment', 500, 5, 6,
        'active', 'active', 10000, 10000, 8500, 9000, 500, 0, 1000, 1000,
        0, 0, 0, 0, 'repayment', 'credit-prepayment-1', repeat('8', 64),
        repeat('9', 64), repeat('a', 64), 'system', 0, CURRENT_TIMESTAMP)`)
		return err
	})

	mustCreditTransaction(t, ctx, database, "mark credit statement delinquent", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET status_code = 'overdue', overdue_at = CURRENT_TIMESTAMP,
    version = 3, updated_at = CURRENT_TIMESTAMP
WHERE statement_no = 'credit-statement-1';
UPDATE plugin_idc_core_credit_account
SET delinquent_principal_minor = 1000, version = 7,
    updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, statement_no,
     movement_type_code, amount_minor, account_version_before, account_version_after,
     status_before_code, status_after_code, total_limit_before_minor,
     total_limit_after_minor, available_before_minor, available_after_minor,
     unbilled_before_minor, unbilled_after_minor, billed_before_minor,
     billed_after_minor, delinquent_before_minor, delinquent_after_minor,
     written_off_before_minor, written_off_after_minor, source_type_code,
     source_reference, idempotency_digest, evidence_digest, content_digest,
     actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-7', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-statement-1', 'delinquency', 1000, 6, 7, 'active', 'active',
        10000, 10000, 9000, 9000, 0, 0, 1000, 1000, 0, 1000, 0, 0,
        'statement', 'credit-statement-1-overdue', repeat('3', 64),
        repeat('c', 64), repeat('d', 64), 'system', 0, CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_delinquency_case
    (case_no, statement_no, credit_account_no, customer_no, currency_code,
     terms_no, terms_version, status_code, principal_minor, outstanding_amount_minor,
     opened_at, grace_until, reason_code, source_snapshot_digest)
VALUES ('credit-delinquency-1', 'credit-statement-1', 'credit-account-cny',
        'customer-ledger', 'CNY', 'credit-terms-cny-1', 1, 'grace_period',
        1000, 1000, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '3 days',
        'credit.overdue', repeat('e', 64))`)
		return err
	})

	mustCreditTransaction(t, ctx, database, "freeze overdue credit account", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET status_code = 'frozen', freeze_reason_code = 'credit.overdue',
    frozen_at = CURRENT_TIMESTAMP, version = 8, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, statement_no,
     movement_type_code, amount_minor, account_version_before, account_version_after,
     status_before_code, status_after_code, total_limit_before_minor,
     total_limit_after_minor, available_before_minor, available_after_minor,
     unbilled_before_minor, unbilled_after_minor, billed_before_minor,
     billed_after_minor, delinquent_before_minor, delinquent_after_minor,
     written_off_before_minor, written_off_after_minor, source_type_code,
     source_reference, idempotency_digest, evidence_digest, content_digest,
     actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-8', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-statement-1', 'freeze', 0, 7, 8, 'active', 'frozen',
        10000, 10000, 9000, 9000, 0, 0, 1000, 1000, 1000, 1000, 0, 0,
        'delinquency', 'credit-delinquency-1-freeze', repeat('f', 64),
        repeat('1', 64), repeat('2', 64), 'system', 0, CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_credit_delinquency_case
SET status_code = 'frozen', frozen_at = CURRENT_TIMESTAMP,
    version = 2, updated_at = CURRENT_TIMESTAMP
WHERE case_no = 'credit-delinquency-1'`)
		return err
	})

	postCreditWriteoffLedger(t, ctx, database, 1000, "credit-writeoff-1")
	mustCreditTransaction(t, ctx, database, "write off approved bad debt", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_statement
SET written_off_amount_minor = 1000, outstanding_amount_minor = 0,
    status_code = 'written_off', written_off_at = CURRENT_TIMESTAMP,
    version = 4, updated_at = CURRENT_TIMESTAMP
WHERE statement_no = 'credit-statement-1';
UPDATE plugin_idc_core_credit_exposure
SET billed_principal_minor = 0, written_off_principal_minor = 1000,
    status_code = 'written_off', version = 4, updated_at = CURRENT_TIMESTAMP
WHERE exposure_no = 'credit-exposure-1';
UPDATE plugin_idc_core_credit_account
SET billed_principal_minor = 0, delinquent_principal_minor = 0,
    written_off_principal_minor = 1000, status_code = 'bad_debt',
    freeze_reason_code = 'credit.bad_debt', version = 9,
    updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny';
INSERT INTO plugin_idc_core_credit_writeoff
    (writeoff_no, statement_no, credit_account_no, customer_no, currency_code,
     writeoff_type_code, amount_minor, ledger_transaction_no, reason_code,
     requested_by, reviewed_by, approval_evidence_digest, content_digest, posted_at)
VALUES ('credit-writeoff-1', 'credit-statement-1', 'credit-account-cny',
        'customer-ledger', 'CNY', 'writeoff', 1000,
        (SELECT transaction_no
         FROM plugin_idc_core_billing_ledger_transaction
         WHERE source_type_code = 'credit_writeoff'
           AND source_reference = 'credit-writeoff-1'),
        'credit.bad_debt', 301, 302, repeat('3', 64), repeat('4', 64),
        CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_account_movement
    (movement_no, credit_account_no, customer_no, currency_code, exposure_no,
     statement_no, movement_type_code, amount_minor, account_version_before,
     account_version_after, status_before_code, status_after_code,
     total_limit_before_minor, total_limit_after_minor, available_before_minor,
     available_after_minor, unbilled_before_minor, unbilled_after_minor,
     billed_before_minor, billed_after_minor, delinquent_before_minor,
     delinquent_after_minor, written_off_before_minor, written_off_after_minor,
     source_type_code, source_reference, idempotency_digest, evidence_digest,
     content_digest, actor_type_code, actor_id, occurred_at)
VALUES ('credit-movement-9', 'credit-account-cny', 'customer-ledger', 'CNY',
        'credit-exposure-1', 'credit-statement-1', 'writeoff', 1000, 8, 9,
        'frozen', 'bad_debt', 10000, 10000, 9000, 9000, 0, 0, 1000, 0,
        1000, 0, 0, 1000, 'writeoff', 'credit-writeoff-1', repeat('5', 64),
        repeat('6', 64), repeat('7', 64), 'staff', 302, CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_credit_delinquency_case
SET status_code = 'written_off', written_off_at = CURRENT_TIMESTAMP,
    version = 3, updated_at = CURRENT_TIMESTAMP
WHERE case_no = 'credit-delinquency-1'`)
		return err
	})

	expectCreditTransactionRejected(t, ctx, database, "duplicate exposure billing", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_statement
    (statement_no, credit_account_no, customer_no, currency_code, terms_no,
     terms_version, period_started_at, period_ended_at, line_count,
     principal_minor, total_amount_minor, outstanding_amount_minor,
     content_digest, issued_at, payment_due_at)
VALUES ('credit-statement-duplicate', 'credit-account-cny', 'customer-ledger',
        'CNY', 'credit-terms-cny-1', 1, CURRENT_TIMESTAMP - INTERVAL '9 days',
        CURRENT_TIMESTAMP - INTERVAL '8 days', 1, 1, 1, 1, repeat('8', 64),
        CURRENT_TIMESTAMP - INTERVAL '7 days', CURRENT_TIMESTAMP + INTERVAL '3 days');
INSERT INTO plugin_idc_core_credit_statement_line
    (statement_line_no, statement_no, credit_account_no, customer_no,
     currency_code, line_number, line_type_code, exposure_no, source_type_code,
     source_reference, amount_minor, content_digest)
VALUES ('credit-statement-line-duplicate', 'credit-statement-duplicate',
        'credit-account-cny', 'customer-ledger', 'CNY', 1, 'principal',
        'credit-exposure-1', 'exposure', 'duplicate-exposure-billing', 1,
        repeat('9', 64))`)
		return err
	})

	postCreditRepaymentLedger(t, ctx, database, 1, "credit-prepayment-overallocated")
	expectCreditTransactionRejected(t, ctx, database, "prepayment above exposure principal", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_repayment
    (repayment_no, credit_account_no, customer_no, currency_code,
     repayment_type_code, funding_source_type_code, funding_source_reference,
     ledger_transaction_no, amount_minor, allocated_amount_minor,
     unallocated_amount_minor, status_code, source_snapshot_digest,
     confirmation_evidence_digest, confirmed_at)
VALUES ('credit-prepayment-overallocated', 'credit-account-cny',
        'customer-ledger', 'CNY', 'prepayment', 'online_payment',
        'credit-prepayment-overallocated',
        (SELECT transaction_no
         FROM plugin_idc_core_billing_ledger_transaction
         WHERE source_type_code = 'credit_repayment'
           AND source_reference = 'credit-prepayment-overallocated'),
        1, 1, 0, 'allocated', repeat('a', 64), repeat('b', 64),
        CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_credit_repayment_allocation
    (allocation_no, repayment_no, credit_account_no, customer_no, currency_code,
     exposure_no, allocation_type_code, amount_minor, source_reference,
     content_digest)
VALUES ('credit-allocation-overallocated', 'credit-prepayment-overallocated',
        'credit-account-cny', 'customer-ledger', 'CNY', 'credit-exposure-2',
        'unbilled_principal', 1, 'credit-overallocated-source', repeat('c', 64))`)
		return err
	})
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application
    (application_no, customer_no, currency_code, policy_no, policy_version,
     requested_limit_minor, requested_valid_from, requested_valid_until,
     required_approval_count, risk_evidence_reference, risk_evidence_digest,
     request_digest, request_actor_type_code, requested_by)
VALUES ('credit-application-raw-risk', 'customer-ledger', 'CNY',
        'credit-policy-cny', 1, 1, CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP + INTERVAL '1 day', 1, 'https://risk.example/raw',
        repeat('d', 64), repeat('e', 64), 'staff', 150)`); err == nil {
		t.Fatal("credit application accepted a non-private risk reference")
	}

	expectCreditTransactionRejected(t, ctx, database, "projection update without movement", func(ctx context.Context) error {
		_, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account
SET available_limit_minor = 8999, unbilled_principal_minor = 1,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE credit_account_no = 'credit-account-cny'`)
		return err
	})
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_credit_account_movement
SET amount_minor = amount_minor + 1
WHERE movement_no = 'credit-movement-6'`); err == nil {
		t.Fatal("credit movement history accepted an update")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_exposure
    (exposure_no, credit_account_no, customer_no, currency_code,
     source_type_code, source_reference, original_principal_minor,
     unbilled_principal_minor, status_code, source_snapshot_digest, content_digest)
VALUES ('credit-exposure-cross-currency', 'credit-account-cny', 'customer-ledger',
        'USD', 'manual', 'cross-currency', 1, 1, 'unbilled',
        repeat('b', 64), repeat('c', 64))`); err == nil {
		t.Fatal("credit exposure crossed its account currency boundary")
	}
}

func postCreditWriteoffLedger(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	amountMinor int64,
	sourceReference string,
) string {
	t.Helper()
	service, err := newBalanceService(database, rand.Reader)
	if err != nil {
		t.Fatalf("create credit write-off ledger service: %v", err)
	}
	transactionNo := ""
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := service.lockSource(ctx, "credit_writeoff", sourceReference); lockErr != nil {
			return lockErr
		}
		receivableSpec := merchantReceivableSpec("merchant-ledger")
		expenseSpec := accountSpec{
			OwnerType: "merchant", OwnerReference: "merchant-ledger",
			AccountType: "bad_debt_expense", NormalBalance: "debit",
		}
		accounts, ensureErr := service.ensureAndLockAccounts(ctx, "CNY", true, receivableSpec, expenseSpec)
		if ensureErr != nil {
			return ensureErr
		}
		transaction, _, postErr := service.postLocked(
			ctx, "writeoff", "CNY", "credit_writeoff", sourceReference,
			strings.Repeat("e", 64), "",
			[]entrySemantic{
				{AccountNo: deterministicAccountNo(expenseSpec.OwnerType, expenseSpec.OwnerReference, expenseSpec.AccountType, "CNY"), Direction: "debit", Role: "expense", AmountMinor: amountMinor, Reason: "credit_bad_debt"},
				{AccountNo: deterministicAccountNo(receivableSpec.OwnerType, receivableSpec.OwnerReference, receivableSpec.AccountType, "CNY"), Direction: "credit", Role: "receivable", AmountMinor: amountMinor, Reason: "credit_bad_debt"},
			},
			accounts,
		)
		if postErr == nil {
			transactionNo = transaction.TransactionNo
		}
		return postErr
	})
	if err != nil || transactionNo == "" {
		t.Fatalf("post credit write-off ledger source=%q transaction=%q err=%v", sourceReference, transactionNo, err)
	}
	return transactionNo
}

func postCreditRecoveryLedger(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	amountMinor int64,
	sourceReference string,
) string {
	t.Helper()
	service, err := newBalanceService(database, rand.Reader)
	if err != nil {
		t.Fatalf("create credit recovery ledger service: %v", err)
	}
	transactionNo := ""
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := service.lockSource(ctx, "credit_repayment", sourceReference); lockErr != nil {
			return lockErr
		}
		cashSpec := merchantCashSpec("merchant-ledger")
		expenseSpec := accountSpec{
			OwnerType: "merchant", OwnerReference: "merchant-ledger",
			AccountType: "bad_debt_expense", NormalBalance: "debit",
		}
		accounts, ensureErr := service.ensureAndLockAccounts(ctx, "CNY", true, cashSpec, expenseSpec)
		if ensureErr != nil {
			return ensureErr
		}
		transaction, _, postErr := service.postLocked(
			ctx, "credit", "CNY", "credit_repayment", sourceReference,
			strings.Repeat("f", 64), "",
			[]entrySemantic{
				{AccountNo: deterministicAccountNo(cashSpec.OwnerType, cashSpec.OwnerReference, cashSpec.AccountType, "CNY"), Direction: "debit", Role: "cash", AmountMinor: amountMinor, Reason: "bad_debt_recovery"},
				{AccountNo: deterministicAccountNo(expenseSpec.OwnerType, expenseSpec.OwnerReference, expenseSpec.AccountType, "CNY"), Direction: "credit", Role: "expense", AmountMinor: amountMinor, Reason: "bad_debt_recovery"},
			},
			accounts,
		)
		if postErr == nil {
			transactionNo = transaction.TransactionNo
		}
		return postErr
	})
	if err != nil || transactionNo == "" {
		t.Fatalf("post credit recovery ledger source=%q transaction=%q err=%v", sourceReference, transactionNo, err)
	}
	return transactionNo
}

func postCreditRepaymentLedger(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	amountMinor int64,
	sourceReference string,
) string {
	return postCreditRepaymentLedgerWithDebitAccount(
		t, ctx, database, amountMinor, sourceReference, merchantCashSpec("merchant-ledger"),
	)
}

func postCreditRepaymentLedgerWithDebitAccount(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	amountMinor int64,
	sourceReference string,
	debitSpec accountSpec,
) string {
	t.Helper()
	service, err := newBalanceService(database, rand.Reader)
	if err != nil {
		t.Fatalf("create credit repayment ledger service: %v", err)
	}
	transactionNo := ""
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := service.lockSource(ctx, "credit_repayment", sourceReference); lockErr != nil {
			return lockErr
		}
		receivableSpec := merchantReceivableSpec("merchant-ledger")
		accounts, ensureErr := service.ensureAndLockAccounts(ctx, "CNY", true, debitSpec, receivableSpec)
		if ensureErr != nil {
			return ensureErr
		}
		transaction, _, postErr := service.postLocked(
			ctx, "credit", "CNY", "credit_repayment", sourceReference,
			strings.Repeat("d", 64), "",
			[]entrySemantic{
				{AccountNo: deterministicAccountNo(debitSpec.OwnerType, debitSpec.OwnerReference, debitSpec.AccountType, "CNY"), Direction: "debit", Role: "cash", AmountMinor: amountMinor, Reason: "credit_repayment"},
				{AccountNo: deterministicAccountNo(receivableSpec.OwnerType, receivableSpec.OwnerReference, receivableSpec.AccountType, "CNY"), Direction: "credit", Role: "receivable", AmountMinor: amountMinor, Reason: "credit_repayment"},
			},
			accounts,
		)
		if postErr == nil {
			transactionNo = transaction.TransactionNo
		}
		return postErr
	})
	if err != nil || transactionNo == "" {
		t.Fatalf("post credit repayment ledger source=%q transaction=%q err=%v", sourceReference, transactionNo, err)
	}
	return transactionNo
}

func mustCreditTransaction(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	description string,
	operation func(context.Context) error,
) {
	t.Helper()
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return operation(ctx)
	}); err != nil {
		t.Fatalf("%s: %v", description, err)
	}
}

func expectCreditTransactionRejected(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	description string,
	operation func(context.Context) error,
) {
	t.Helper()
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return operation(ctx)
	}); err == nil {
		t.Fatalf("credit database accepted %s", description)
	}
}
