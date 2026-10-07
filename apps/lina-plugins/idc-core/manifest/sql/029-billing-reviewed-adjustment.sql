-- 029: Record verified manual receipts and invoice waivers as distinct,
-- reviewed financial facts with currency-local approval thresholds.

ALTER TABLE plugin_idc_core_billing_invoice
    ADD COLUMN IF NOT EXISTS "waived_amount_minor" BIGINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_amounts;
ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_amounts CHECK (
        "item_count" BETWEEN 1 AND 10000
        AND "subtotal_minor" >= 0
        AND "discount_minor" >= 0
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "paid_amount_minor" >= 0
        AND "waived_amount_minor" >= 0
        AND "paid_amount_minor" + "waived_amount_minor" <= "total_amount_minor"
        AND "refundable_amount_minor" >= 0
        AND "refunded_amount_minor" >= 0
        AND "refundable_amount_minor" + "refunded_amount_minor" <= "paid_amount_minor"
    );

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_lifecycle;
ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_lifecycle CHECK (
        ("status_code" = 'draft' AND "issued_at" IS NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0 AND "waived_amount_minor" = 0)
        OR ("status_code" = 'pending_payment' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0 AND "waived_amount_minor" = 0)
        OR ("status_code" = 'partially_paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" > 0
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'overdue' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" >= 0
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'partially_refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" > 0)
        OR ("status_code" = 'refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" = 0)
        OR ("status_code" = 'cancelled' AND "settled_at" IS NULL
            AND "cancelled_at" IS NOT NULL AND "paid_amount_minor" = 0
            AND "waived_amount_minor" = 0)
        OR ("status_code" = 'bad_debt' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD COLUMN IF NOT EXISTS "proof_reference" VARCHAR(128),
    ADD COLUMN IF NOT EXISTS "receiving_account_reference" VARCHAR(128),
    ADD COLUMN IF NOT EXISTS "business_purpose_code" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "policy_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "policy_version" BIGINT;

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_target;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (
            "adjustment_type_code" IN ('balance_credit', 'balance_debit')
            AND "invoice_no" IS NULL AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_balance'
        ) OR (
            "adjustment_type_code" IN ('invoice_waiver', 'writeoff')
            AND "invoice_no" IS NOT NULL AND "account_no" IS NULL
            AND "account_type_code" IS NULL
        ) OR (
            "adjustment_type_code" = 'manual_receipt'
            AND (
                ("invoice_no" IS NOT NULL AND "account_no" IS NULL
                    AND "account_type_code" IS NULL)
                OR ("invoice_no" IS NULL AND "account_no" IS NOT NULL
                    AND "account_type_code" = 'customer_balance')
            )
        )
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_receipt_evidence;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_receipt_evidence CHECK (
        (
            "adjustment_type_code" = 'manual_receipt'
            AND "proof_reference" IS NOT NULL
            AND "receiving_account_reference" IS NOT NULL
            AND "business_purpose_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
        ) OR (
            "adjustment_type_code" <> 'manual_receipt'
            AND "proof_reference" IS NULL
            AND "receiving_account_reference" IS NULL
            AND "business_purpose_code" IS NULL
        )
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_bill_adjust_waiver_owner'
          AND conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_billing_adjustment_request
            ADD CONSTRAINT uk_idc_bill_adjust_waiver_owner UNIQUE (
                "adjustment_no", "invoice_no", "customer_no", "currency_code",
                "adjustment_type_code"
            );
    END IF;
END;
$constraint$;

-- Purpose: Append-only currency-local high-value adjustment review thresholds.
-- 用途：保存账务调整策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_adjustment_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "review_threshold_minor" BIGINT NOT NULL,
    "version" BIGINT NOT NULL,
    "configured_by" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_adjust_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_adjust_policy_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_bill_adjust_policy_values CHECK (
        "review_threshold_minor" > 0 AND "version" > 0 AND "configured_by" > 0
    ),
    CONSTRAINT ck_idc_bill_adjust_policy_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_adjust_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_bill_adjust_policy_no_version UNIQUE ("policy_no", "version"),
    CONSTRAINT uk_idc_bill_adjust_policy_version UNIQUE (
        "currency_code", "version"
    )
);

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_policy_snapshot;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_policy_snapshot CHECK (
        ("policy_no" IS NULL AND "policy_version" IS NULL AND "approval_required")
        OR ("policy_no" IS NOT NULL AND "policy_version" IS NOT NULL)
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_bill_adjust_policy_snapshot'
          AND conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_billing_adjustment_request
            ADD CONSTRAINT fk_idc_bill_adjust_policy_snapshot FOREIGN KEY (
                "policy_no", "policy_version"
            ) REFERENCES plugin_idc_core_billing_adjustment_policy (
                "policy_no", "version"
            ) ON DELETE RESTRICT;
    END IF;
END;
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjust_policy_current
    ON plugin_idc_core_billing_adjustment_policy (
        "tenant_id", "currency_code", "version" DESC, "id" DESC
    );

-- Purpose: Append-only invoice waiver fact distinct from customer payment.
-- 用途：保存账务账单减免分配相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_waiver_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "waiver_allocation_no" VARCHAR(40) NOT NULL,
    "adjustment_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "ledger_transaction_no" VARCHAR(40) NOT NULL,
    "ledger_transaction_type_code" VARCHAR(24) NOT NULL DEFAULT 'adjustment',
    "adjustment_type_code" VARCHAR(24) NOT NULL DEFAULT 'invoice_waiver',
    "amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_waiver_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_waiver_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_bill_waiver_types CHECK (
        "adjustment_type_code" = 'invoice_waiver'
        AND "ledger_transaction_type_code" = 'adjustment'
    ),
    CONSTRAINT ck_idc_bill_waiver_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_waiver_no UNIQUE ("waiver_allocation_no"),
    CONSTRAINT uk_idc_bill_waiver_adjustment UNIQUE ("adjustment_no"),
    CONSTRAINT uk_idc_bill_waiver_tx UNIQUE ("ledger_transaction_no"),
    CONSTRAINT fk_idc_bill_waiver_adjustment FOREIGN KEY (
        "adjustment_no", "invoice_no", "customer_no", "currency_code",
        "adjustment_type_code"
    ) REFERENCES plugin_idc_core_billing_adjustment_request (
        "adjustment_no", "invoice_no", "customer_no", "currency_code",
        "adjustment_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_waiver_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_waiver_tx FOREIGN KEY (
        "ledger_transaction_no", "currency_code", "ledger_transaction_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code", "transaction_type_code"
    ) ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_billing_ledger_account
    DROP CONSTRAINT IF EXISTS ck_idc_bill_account_owner;
ALTER TABLE plugin_idc_core_billing_ledger_account
    ADD CONSTRAINT ck_idc_bill_account_owner CHECK (
        ("owner_type_code" = 'customer' AND "customer_no" IS NOT NULL
            AND "owner_reference" = "customer_no"
            AND "account_type_code" IN (
                'customer_balance', 'customer_credit', 'customer_gift',
                'customer_commission'
            ))
        OR ("owner_type_code" IN ('merchant', 'system') AND "customer_no" IS NULL
            AND "account_type_code" IN (
                'merchant_cash', 'accounts_receivable', 'payment_clearing',
                'refund_clearing', 'revenue', 'tax_payable',
                'adjustment_clearing', 'bad_debt_expense', 'waiver_expense'
            ))
    );

ALTER TABLE plugin_idc_core_billing_ledger_account
    DROP CONSTRAINT IF EXISTS ck_idc_bill_account_nature;
ALTER TABLE plugin_idc_core_billing_ledger_account
    ADD CONSTRAINT ck_idc_bill_account_nature CHECK (
        ("account_type_code" IN (
            'customer_balance', 'customer_gift', 'customer_commission',
            'refund_clearing', 'revenue', 'tax_payable', 'adjustment_clearing'
        ) AND "normal_balance_code" = 'credit')
        OR ("account_type_code" IN (
            'customer_credit', 'merchant_cash', 'accounts_receivable',
            'payment_clearing', 'bad_debt_expense', 'waiver_expense'
        ) AND "normal_balance_code" = 'debit')
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_billing_adjustment_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'billing adjustment requests cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."adjustment_no", OLD."customer_no", OLD."invoice_no",
        OLD."account_no", OLD."account_type_code", OLD."currency_code",
        OLD."adjustment_type_code", OLD."amount_minor", OLD."reason_code",
        OLD."evidence_digest", OLD."idempotency_digest", OLD."request_digest",
        OLD."approval_required", OLD."requested_by", OLD."proof_reference",
        OLD."receiving_account_reference", OLD."business_purpose_code", OLD."created_at",
        OLD."policy_no", OLD."policy_version"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."adjustment_no", NEW."customer_no", NEW."invoice_no",
        NEW."account_no", NEW."account_type_code", NEW."currency_code",
        NEW."adjustment_type_code", NEW."amount_minor", NEW."reason_code",
        NEW."evidence_digest", NEW."idempotency_digest", NEW."request_digest",
        NEW."approval_required", NEW."requested_by", NEW."proof_reference",
        NEW."receiving_account_reference", NEW."business_purpose_code", NEW."created_at",
        NEW."policy_no", NEW."policy_version"
    ) THEN
        RAISE EXCEPTION 'billing adjustment intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_adjustment_policy_snapshot()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    "threshold_minor" BIGINT;
BEGIN
    IF NEW."policy_no" IS NULL THEN
        IF NOT NEW."approval_required" THEN
            RAISE EXCEPTION 'billing adjustment without policy must require review';
        END IF;
        RETURN NULL;
    END IF;
    SELECT "review_threshold_minor" INTO "threshold_minor"
    FROM plugin_idc_core_billing_adjustment_policy
    WHERE "policy_no" = NEW."policy_no" AND "version" = NEW."policy_version";
    IF NOT FOUND OR NEW."approval_required" IS DISTINCT FROM
        (NEW."amount_minor" >= "threshold_minor") THEN
        RAISE EXCEPTION 'billing adjustment approval threshold snapshot is invalid';
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_invoice_waivers()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    "target_invoice_no" VARCHAR(40);
    invoice_waived BIGINT;
    allocation_waived NUMERIC;
    invalid_count BIGINT;
BEGIN
    "target_invoice_no" := COALESCE(NEW."invoice_no", OLD."invoice_no");
    SELECT "waived_amount_minor" INTO invoice_waived
    FROM plugin_idc_core_billing_invoice
    WHERE "invoice_no" = "target_invoice_no";
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT COALESCE(SUM("amount_minor"), 0) INTO allocation_waived
    FROM plugin_idc_core_billing_invoice_waiver_allocation
    WHERE "invoice_no" = "target_invoice_no";
    IF allocation_waived <> invoice_waived THEN
        RAISE EXCEPTION 'billing waiver allocations do not match invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_billing_invoice_waiver_allocation waiver
    JOIN plugin_idc_core_billing_adjustment_request adjustment
      ON adjustment."adjustment_no" = waiver."adjustment_no"
    JOIN plugin_idc_core_billing_ledger_transaction transaction
      ON transaction."transaction_no" = waiver."ledger_transaction_no"
    WHERE waiver."invoice_no" = "target_invoice_no"
      AND (
        adjustment."status_code" <> 'posted'
        OR adjustment."adjustment_type_code" <> 'invoice_waiver'
        OR adjustment."amount_minor" <> waiver."amount_minor"
        OR adjustment."ledger_transaction_no" <> waiver."ledger_transaction_no"
        OR transaction."transaction_type_code" <> 'adjustment'
        OR transaction."debit_total_minor" <> waiver."amount_minor"
        OR (SELECT COUNT(*)
            FROM plugin_idc_core_billing_ledger_entry entry
            JOIN plugin_idc_core_billing_ledger_account account
              ON account."account_no" = entry."account_no"
            WHERE entry."transaction_no" = waiver."ledger_transaction_no"
              AND entry."direction_code" = 'debit'
              AND entry."entry_role_code" = 'expense'
              AND account."account_type_code" = 'waiver_expense'
              AND entry."amount_minor" = waiver."amount_minor") <> 1
        OR (SELECT COUNT(*)
            FROM plugin_idc_core_billing_ledger_entry entry
            JOIN plugin_idc_core_billing_ledger_account account
              ON account."account_no" = entry."account_no"
            WHERE entry."transaction_no" = waiver."ledger_transaction_no"
              AND entry."invoice_no" = waiver."invoice_no"
              AND entry."direction_code" = 'credit'
              AND entry."entry_role_code" = 'receivable'
              AND account."account_type_code" = 'accounts_receivable'
              AND entry."amount_minor" = waiver."amount_minor") <> 1
      );
    IF invalid_count <> 0 THEN
        RAISE EXCEPTION 'billing waiver evidence is invalid for invoice %', "target_invoice_no";
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_bill_adjust_policy_immutable
    ON plugin_idc_core_billing_adjustment_policy;
CREATE TRIGGER trg_idc_bill_adjust_policy_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_adjustment_policy
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_waiver_immutable
    ON plugin_idc_core_billing_invoice_waiver_allocation;
CREATE TRIGGER trg_idc_bill_waiver_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_waiver_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_invoice'::regclass
           AND conname = 'trg_idc_bill_invoice_waiver_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_invoice'::regclass
               AND constraint_row.conname = 'trg_idc_bill_invoice_waiver_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_invoice_waiver_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_waivers()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_invoice_waiver_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_invoice_waiver_composition
        AFTER INSERT OR UPDATE ON plugin_idc_core_billing_invoice
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_waivers();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_invoice_waiver_allocation'::regclass
           AND conname = 'trg_idc_bill_waiver_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_invoice_waiver_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_bill_waiver_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_waiver_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_waivers()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_waiver_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_waiver_composition
        AFTER INSERT ON plugin_idc_core_billing_invoice_waiver_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_waivers();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
           AND conname = 'trg_idc_bill_adjust_policy_snapshot'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
               AND constraint_row.conname = 'trg_idc_bill_adjust_policy_snapshot'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_adjust_policy_snapshot'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_adjustment_policy_snapshot()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_adjust_policy_snapshot catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_adjust_policy_snapshot
        AFTER INSERT OR UPDATE ON plugin_idc_core_billing_adjustment_request
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_adjustment_policy_snapshot();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_billing_adjustment_policy
    IS 'Append-only currency-local high-value adjustment review thresholds';
COMMENT ON TABLE plugin_idc_core_billing_invoice_waiver_allocation
    IS 'Append-only invoice waiver fact distinct from customer payment';
COMMENT ON COLUMN plugin_idc_core_billing_invoice."waived_amount_minor"
    IS 'Effective reviewed waiver total; never counted as customer payment';

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_billing_account_type', '减免费用', 'waiver_expense', 13, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
