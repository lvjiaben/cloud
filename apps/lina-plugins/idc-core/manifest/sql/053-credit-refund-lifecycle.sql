-- Phase 6.13: make partial credit refunds append-only and expose an explicit
-- refunded terminal statement state without rewriting the phase-6 foundation.

ALTER TABLE plugin_idc_core_credit_statement
    DROP CONSTRAINT IF EXISTS ck_idc_credit_statement_status;
ALTER TABLE plugin_idc_core_credit_statement
    ADD CONSTRAINT ck_idc_credit_statement_status CHECK (
        "status_code" IN (
            'open', 'partially_paid', 'paid', 'overdue',
            'written_off', 'cancelled', 'refunded'
        )
    );

ALTER TABLE plugin_idc_core_credit_statement
    DROP CONSTRAINT IF EXISTS ck_idc_credit_statement_lifecycle;
ALTER TABLE plugin_idc_core_credit_statement
    ADD CONSTRAINT ck_idc_credit_statement_lifecycle CHECK (
        ("status_code" = 'open' AND "paid_amount_minor" = 0
            AND "written_off_amount_minor" = 0
            AND "overdue_at" IS NULL AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'partially_paid' AND "paid_amount_minor" > 0
            AND "outstanding_amount_minor" > 0 AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'paid' AND "outstanding_amount_minor" = 0
            AND "written_off_amount_minor" = 0 AND "settled_at" IS NOT NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'overdue' AND "outstanding_amount_minor" > 0
            AND "overdue_at" IS NOT NULL AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'written_off' AND "outstanding_amount_minor" = 0
            AND "written_off_amount_minor" > 0 AND "written_off_at" IS NOT NULL
            AND "cancelled_at" IS NULL)
        OR ("status_code" = 'cancelled' AND "paid_amount_minor" = 0
            AND "written_off_amount_minor" = 0 AND "cancelled_at" IS NOT NULL
            AND "settled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'refunded' AND "total_amount_minor" = 0
            AND "adjustment_minor" = -("principal_minor" + "fee_minor")
            AND "paid_amount_minor" = 0
            AND "written_off_amount_minor" = 0
            AND "outstanding_amount_minor" = 0
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "written_off_at" IS NULL)
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_statement()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit statements cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."statement_no", OLD."credit_account_no",
        OLD."customer_no", OLD."currency_code", OLD."terms_no", OLD."terms_version",
        OLD."period_started_at", OLD."period_ended_at", OLD."principal_minor",
        OLD."content_digest", OLD."issued_at", OLD."payment_due_at", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."statement_no", NEW."credit_account_no",
        NEW."customer_no", NEW."currency_code", NEW."terms_no", NEW."terms_version",
        NEW."period_started_at", NEW."period_ended_at", NEW."principal_minor",
        NEW."content_digest", NEW."issued_at", NEW."payment_due_at", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit statement composition is immutable';
    END IF;
    IF ROW(
        OLD."line_count", OLD."fee_minor", OLD."adjustment_minor",
        OLD."total_amount_minor"
    ) IS DISTINCT FROM ROW(
        NEW."line_count", NEW."fee_minor", NEW."adjustment_minor",
        NEW."total_amount_minor"
    ) AND NOT (
        (
            OLD."status_code" = 'overdue' AND NEW."status_code" = 'overdue'
            AND NEW."line_count" = OLD."line_count" + 1
            AND NEW."fee_minor" > OLD."fee_minor"
            AND NEW."adjustment_minor" = OLD."adjustment_minor"
            AND NEW."total_amount_minor" - OLD."total_amount_minor" =
                NEW."fee_minor" - OLD."fee_minor"
            AND NEW."outstanding_amount_minor" - OLD."outstanding_amount_minor" =
                NEW."fee_minor" - OLD."fee_minor"
        ) OR (
            NEW."line_count" = OLD."line_count" + 1
            AND NEW."fee_minor" = OLD."fee_minor"
            AND NEW."adjustment_minor" < OLD."adjustment_minor"
            AND OLD."total_amount_minor" - NEW."total_amount_minor" =
                OLD."adjustment_minor" - NEW."adjustment_minor"
            AND NEW."paid_amount_minor" <= OLD."paid_amount_minor"
            AND NEW."outstanding_amount_minor" <= OLD."outstanding_amount_minor"
            AND NEW."written_off_amount_minor" = OLD."written_off_amount_minor"
            AND OLD."total_amount_minor" - NEW."total_amount_minor" =
                (OLD."paid_amount_minor" - NEW."paid_amount_minor")
                + (OLD."outstanding_amount_minor" - NEW."outstanding_amount_minor")
        )
    ) THEN
        RAISE EXCEPTION 'credit statement totals can only change through fee accrual or refund';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit statement version must advance exactly once';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_movement_chain()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_account_no VARCHAR(40);
    account_row RECORD;
    latest_row RECORD;
    invalid_count BIGINT;
BEGIN
    target_account_no := COALESCE(NEW."credit_account_no", OLD."credit_account_no");
    SELECT * INTO account_row
    FROM plugin_idc_core_credit_account
    WHERE "credit_account_no" = target_account_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    WITH chain AS (
        SELECT movement.*,
               ROW_NUMBER() OVER (ORDER BY "account_version_after") AS "expected_version",
               LAG("status_after_code") OVER (ORDER BY "account_version_after") AS "previous_status",
               LAG("total_limit_after_minor") OVER (ORDER BY "account_version_after") AS previous_total,
               LAG("available_after_minor") OVER (ORDER BY "account_version_after") AS previous_available,
               LAG("unbilled_after_minor") OVER (ORDER BY "account_version_after") AS previous_unbilled,
               LAG("billed_after_minor") OVER (ORDER BY "account_version_after") AS previous_billed,
               LAG("delinquent_after_minor") OVER (ORDER BY "account_version_after") AS previous_delinquent,
               LAG("written_off_after_minor") OVER (ORDER BY "account_version_after") AS previous_written_off
        FROM plugin_idc_core_credit_account_movement movement
        WHERE "credit_account_no" = target_account_no
    )
    SELECT COUNT(*) INTO invalid_count
    FROM chain
    WHERE "account_version_after" <> "expected_version"
       OR "account_version_before" <> "account_version_after" - 1
       OR ("account_version_before" > 0 AND (
            "status_before_code" IS DISTINCT FROM "previous_status"
            OR "total_limit_before_minor" IS DISTINCT FROM previous_total
            OR "available_before_minor" IS DISTINCT FROM previous_available
            OR "unbilled_before_minor" IS DISTINCT FROM previous_unbilled
            OR "billed_before_minor" IS DISTINCT FROM previous_billed
            OR "delinquent_before_minor" IS DISTINCT FROM previous_delinquent
            OR "written_off_before_minor" IS DISTINCT FROM previous_written_off
       ))
       OR NOT (
           ("movement_type_code" = 'limit_granted'
                AND "account_version_before" = 0
                AND "status_after_code" IN ('pending_activation', 'active')
                AND "total_limit_after_minor" = "amount_minor"
                AND "available_after_minor" = "amount_minor"
                AND "unbilled_after_minor" = 0 AND "billed_after_minor" = 0
                AND "delinquent_after_minor" = 0 AND "written_off_after_minor" = 0)
           OR ("movement_type_code" = 'limit_adjusted'
                AND ABS("total_limit_after_minor" - "total_limit_before_minor") = "amount_minor"
                AND "available_after_minor" - "available_before_minor" =
                    "total_limit_after_minor" - "total_limit_before_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'reserve'
                AND "status_before_code" = 'active'
                AND "status_after_code" = 'active'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "available_before_minor" - "available_after_minor" = "amount_minor"
                AND "unbilled_after_minor" - "unbilled_before_minor" = "amount_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'bill'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "unbilled_before_minor" - "unbilled_after_minor" = "amount_minor"
                AND "billed_after_minor" - "billed_before_minor" = "amount_minor"
                AND "available_after_minor" = "available_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'repayment'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "billed_before_minor" - "billed_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "delinquent_after_minor" BETWEEN
                    GREATEST(0, "delinquent_before_minor" - "amount_minor")
                    AND "delinquent_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'prepayment'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "unbilled_before_minor" - "unbilled_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'release'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND ("unbilled_before_minor" + "billed_before_minor")
                    - ("unbilled_after_minor" + "billed_after_minor") = "amount_minor"
                AND "delinquent_before_minor" - "delinquent_after_minor"
                    BETWEEN 0 AND "amount_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'refund'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "available_after_minor" - "available_before_minor" =
                    ("unbilled_before_minor" + "billed_before_minor")
                    - ("unbilled_after_minor" + "billed_after_minor")
                AND "available_after_minor" - "available_before_minor"
                    BETWEEN 0 AND "amount_minor"
                AND "delinquent_before_minor" - "delinquent_after_minor"
                    BETWEEN 0 AND "available_after_minor" - "available_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'writeoff'
                AND "status_before_code" IN ('frozen', 'bad_debt')
                AND "status_after_code" IN ('frozen', 'bad_debt')
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "billed_before_minor" - "billed_after_minor" = "amount_minor"
                AND "written_off_after_minor" - "written_off_before_minor" = "amount_minor"
                AND "available_after_minor" = "available_before_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "delinquent_before_minor" - "delinquent_after_minor"
                    BETWEEN 0 AND "amount_minor"
                AND ("status_after_code" = "status_before_code"
                    OR ("status_before_code" = 'frozen' AND "status_after_code" = 'bad_debt')))
           OR ("movement_type_code" = 'recovery'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "written_off_before_minor" - "written_off_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'late_fee'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor",
                        "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor",
                        "status_before_code"))
           OR ("movement_type_code" = 'delinquency'
                AND "delinquent_after_minor" - "delinquent_before_minor" = "amount_minor"
                AND "delinquent_after_minor" <= "billed_after_minor"
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "written_off_after_minor", "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "written_off_before_minor", "status_before_code"))
           OR ("movement_type_code" = 'cure'
                AND "delinquent_before_minor" - "delinquent_after_minor" = "amount_minor"
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "written_off_after_minor", "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "written_off_before_minor", "status_before_code"))
           OR ("movement_type_code" = 'freeze'
                AND "status_before_code" IN ('active', 'expired')
                AND "status_after_code" IN ('frozen', 'bad_debt')
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'unfreeze'
                AND "status_before_code" IN ('frozen', 'bad_debt')
                AND "status_after_code" IN ('active', 'expired')
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'expire'
                AND "status_after_code" = 'expired'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'close'
                AND "status_after_code" = 'closed'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR "movement_type_code" = 'migration'
       );
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit account % movement chain is inconsistent', target_account_no;
    END IF;

    SELECT * INTO latest_row
    FROM plugin_idc_core_credit_account_movement
    WHERE "credit_account_no" = target_account_no
    ORDER BY "account_version_after" DESC
    LIMIT 1;
    IF NOT FOUND
       OR ROW(
           latest_row."account_version_after", latest_row."status_after_code",
           latest_row."total_limit_after_minor", latest_row."available_after_minor",
           latest_row."unbilled_after_minor", latest_row."billed_after_minor",
           latest_row."delinquent_after_minor", latest_row."written_off_after_minor"
       ) IS DISTINCT FROM ROW(
           account_row."version", account_row."status_code",
           account_row."total_limit_minor", account_row."available_limit_minor",
           account_row."unbilled_principal_minor", account_row."billed_principal_minor",
           account_row."delinquent_principal_minor", account_row."written_off_principal_minor"
       ) THEN
        RAISE EXCEPTION 'credit account % projection has no matching movement', target_account_no;
    END IF;
    RETURN NULL;
END;
$$;

ALTER TABLE plugin_idc_core_credit_exposure
    DROP CONSTRAINT IF EXISTS ck_idc_credit_exposure_lifecycle;
ALTER TABLE plugin_idc_core_credit_exposure
    ADD CONSTRAINT ck_idc_credit_exposure_lifecycle CHECK (
        ("status_code" = 'unbilled' AND "unbilled_principal_minor" > 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'partially_billed' AND "unbilled_principal_minor" > 0
            AND "billed_principal_minor" > 0 AND "repaid_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'billed' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" > 0 AND "repaid_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'partially_repaid'
            AND ("unbilled_principal_minor" + "billed_principal_minor") > 0
            AND "repaid_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'settled' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'released' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" = 0
            AND "released_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'written_off' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0
            AND "written_off_principal_minor" > 0)
    );

ALTER TABLE plugin_idc_core_credit_repayment_allocation
    DROP CONSTRAINT IF EXISTS uk_idc_credit_allocation_reversal;
CREATE INDEX IF NOT EXISTS idx_idc_credit_allocation_original
    ON plugin_idc_core_credit_repayment_allocation (
        "tenant_id", "original_allocation_no", "created_at", "id"
    ) WHERE "original_allocation_no" IS NOT NULL;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_repayment()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_repayment_no VARCHAR(40);
    repayment_row RECORD;
    transaction_row RECORD;
    allocation_total NUMERIC;
    invalid_count BIGINT;
BEGIN
    target_repayment_no := COALESCE(NEW."repayment_no", OLD."repayment_no");
    SELECT * INTO repayment_row
    FROM plugin_idc_core_credit_repayment
    WHERE "repayment_no" = target_repayment_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    IF repayment_row."ledger_transaction_no" IS NOT NULL THEN
        SELECT * INTO transaction_row
        FROM plugin_idc_core_billing_ledger_transaction
        WHERE "transaction_no" = repayment_row."ledger_transaction_no"
          AND "currency_code" = repayment_row."currency_code";
        IF transaction_row."source_reference" <> repayment_row."funding_source_reference"
           OR transaction_row."debit_total_minor" <> repayment_row."amount_minor"
           OR transaction_row."credit_total_minor" <> repayment_row."amount_minor"
           OR NOT (
               (repayment_row."funding_source_type_code" = 'online_payment'
                    AND transaction_row."transaction_type_code" = 'credit')
               OR (repayment_row."funding_source_type_code" = 'customer_balance'
                    AND transaction_row."transaction_type_code" = 'balance_payment')
               OR (repayment_row."funding_source_type_code" = 'manual_receipt'
                    AND transaction_row."transaction_type_code" = 'manual_receipt')
               OR (repayment_row."funding_source_type_code" = 'migration'
                    AND transaction_row."transaction_type_code" = 'migration')
           ) THEN
            RAISE EXCEPTION 'credit repayment % ledger source is inconsistent', target_repayment_no;
        END IF;
    END IF;

    SELECT COALESCE(SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO allocation_total
    FROM plugin_idc_core_credit_repayment_allocation
    WHERE "repayment_no" = target_repayment_no;
    IF allocation_total < 0
       OR allocation_total <> repayment_row."allocated_amount_minor" THEN
        RAISE EXCEPTION 'credit repayment % allocations do not match', target_repayment_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_credit_repayment_allocation allocation
    LEFT JOIN plugin_idc_core_credit_repayment_allocation "original"
      ON "original"."allocation_no" = allocation."original_allocation_no"
    WHERE allocation."repayment_no" = target_repayment_no
      AND (
          (allocation."direction_code" = 'reverse' AND (
              "original"."allocation_no" IS NULL
              OR "original"."direction_code" <> 'apply'
              OR "original"."repayment_no" <> allocation."repayment_no"
              OR "original"."credit_account_no" <> allocation."credit_account_no"
              OR "original"."allocation_type_code" <> allocation."allocation_type_code"
              OR "original"."statement_no" IS DISTINCT FROM allocation."statement_no"
              OR "original"."exposure_no" IS DISTINCT FROM allocation."exposure_no"
              OR allocation."amount_minor" > "original"."amount_minor"
          ))
          OR (repayment_row."repayment_type_code" = 'statement'
              AND allocation."allocation_type_code" NOT IN ('statement_principal', 'statement_fee'))
          OR (repayment_row."repayment_type_code" = 'prepayment'
              AND allocation."allocation_type_code" <> 'unbilled_principal')
          OR (repayment_row."repayment_type_code" = 'bad_debt_recovery'
              AND allocation."allocation_type_code" <> 'written_off_recovery')
      );
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit repayment % contains an invalid allocation', target_repayment_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_credit_repayment_allocation "original"
    WHERE "original"."repayment_no" = target_repayment_no
      AND "original"."direction_code" = 'apply'
      AND COALESCE((
          SELECT SUM(reversal."amount_minor")
          FROM plugin_idc_core_credit_repayment_allocation reversal
          WHERE reversal."original_allocation_no" = "original"."allocation_no"
            AND reversal."direction_code" = 'reverse'
      ), 0) > "original"."amount_minor";
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit repayment % allocation reversal exceeds its source', target_repayment_no;
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_delinquency()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_case_no VARCHAR(40);
    case_row RECORD;
    statement_row RECORD;
    account_row RECORD;
    terms_row RECORD;
BEGIN
    target_case_no := COALESCE(NEW."case_no", OLD."case_no");
    SELECT * INTO case_row
    FROM plugin_idc_core_credit_delinquency_case
    WHERE "case_no" = target_case_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT * INTO statement_row
    FROM plugin_idc_core_credit_statement
    WHERE "statement_no" = case_row."statement_no";
    SELECT * INTO account_row
    FROM plugin_idc_core_credit_account
    WHERE "credit_account_no" = case_row."credit_account_no";
    SELECT * INTO terms_row
    FROM plugin_idc_core_credit_account_terms
    WHERE "terms_no" = case_row."terms_no"
      AND "credit_account_no" = case_row."credit_account_no";

    IF case_row."terms_no" <> statement_row."terms_no"
       OR case_row."terms_version" <> statement_row."terms_version" THEN
        RAISE EXCEPTION 'credit delinquency % terms do not match its statement', target_case_no;
    END IF;
    IF case_row."opened_at" < statement_row."payment_due_at"
       OR case_row."grace_until" <>
          case_row."opened_at" + make_interval(days => terms_row."grace_period_days")
       OR case_row."late_fee_minor" > terms_row."maximum_late_fee_minor" THEN
        RAISE EXCEPTION 'credit delinquency % timing or fee exceeds its terms', target_case_no;
    END IF;
    IF case_row."status_code" IN ('grace_period', 'frozen', 'service_action_pending')
       AND (statement_row."status_code" <> 'overdue'
         OR case_row."outstanding_amount_minor" <> statement_row."outstanding_amount_minor") THEN
        RAISE EXCEPTION 'credit delinquency % does not match the overdue statement', target_case_no;
    END IF;
    IF case_row."status_code" IN ('frozen', 'service_action_pending', 'written_off')
       AND account_row."status_code" NOT IN ('frozen', 'bad_debt') THEN
        RAISE EXCEPTION 'credit delinquency % requires a frozen credit account', target_case_no;
    END IF;
    IF case_row."status_code" = 'written_off'
       AND statement_row."status_code" <> 'written_off' THEN
        RAISE EXCEPTION 'credit delinquency % lacks a written-off statement', target_case_no;
    END IF;
    IF case_row."status_code" = 'resolved'
       AND statement_row."status_code" NOT IN ('paid', 'written_off', 'cancelled', 'refunded') THEN
        RAISE EXCEPTION 'credit delinquency % cannot resolve an unsettled statement', target_case_no;
    END IF;
    RETURN NULL;
END;
$$;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort",
    "tag_style", "status", "is_builtin", "created_at", "updated_at"
)
SELECT 0, 'idc_credit_statement_status', '已退款', 'refunded', 7,
       'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM sys_dict_data
    WHERE "tenant_id" = 0 AND "dict_type" = 'idc_credit_statement_status'
      AND "value" = 'refunded'
);
