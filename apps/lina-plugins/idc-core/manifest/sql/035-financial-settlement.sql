-- 035: Immutable order financial-settlement facts and their authoritative
-- source evidence. Fulfillment consumes the transactional outbox event later;
-- this migration never advances inventory or service runtime state.

ALTER TABLE plugin_idc_core_billing_invoice
    ADD COLUMN IF NOT EXISTS "credit_amount_minor" BIGINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_status;
ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_status CHECK (
        "status_code" IN (
            'draft', 'pending_payment', 'partially_paid', 'paid', 'overdue',
            'cancelled', 'partially_refunded', 'refunded', 'bad_debt',
            'credit_settled'
        )
    );

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
        AND "credit_amount_minor" >= 0
        AND "paid_amount_minor" + "waived_amount_minor"
            + "credit_amount_minor" <= "total_amount_minor"
        AND "refundable_amount_minor" >= 0
        AND "refunded_amount_minor" >= 0
        AND "refundable_amount_minor" + "refunded_amount_minor"
            <= "paid_amount_minor"
    );

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_lifecycle;
ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_lifecycle CHECK (
        ("status_code" = 'draft' AND "issued_at" IS NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0 AND "waived_amount_minor" = 0
            AND "credit_amount_minor" = 0)
        OR ("status_code" = 'pending_payment' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0 AND "waived_amount_minor" = 0
            AND "credit_amount_minor" = 0)
        OR ("status_code" = 'partially_paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" > 0
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "credit_amount_minor" = 0
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'overdue' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" >= 0
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "credit_amount_minor" = 0
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "credit_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'partially_refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "credit_amount_minor" = 0
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" > 0)
        OR ("status_code" = 'refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" = "total_amount_minor"
            AND "credit_amount_minor" = 0
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" = 0)
        OR ("status_code" = 'cancelled' AND "settled_at" IS NULL
            AND "cancelled_at" IS NOT NULL AND "paid_amount_minor" = 0
            AND "waived_amount_minor" = 0 AND "credit_amount_minor" = 0)
        OR ("status_code" = 'bad_debt' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" + "waived_amount_minor" < "total_amount_minor"
            AND "credit_amount_minor" = 0
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'credit_settled' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "total_amount_minor" > 0 AND "paid_amount_minor" = 0
            AND "waived_amount_minor" = 0
            AND "credit_amount_minor" = "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_credit_exposure_settlement_owner
    ON plugin_idc_core_credit_exposure (
        "exposure_no", "order_no", "invoice_no", "customer_no", "currency_code"
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_tx_original
    ON plugin_idc_core_billing_ledger_transaction (
        "tenant_id", "original_transaction_no", "id"
    ) WHERE "original_transaction_no" IS NOT NULL;

-- Purpose: Immutable authority that one order is financially eligible for fulfillment.
-- 用途：保存财务结算相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_settlement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "settlement_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "basis_code" VARCHAR(24) NOT NULL,
    "total_amount_minor" BIGINT NOT NULL,
    "paid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "credit_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "source_count" INT NOT NULL,
    "identity_evidence_digest" CHAR(64) NOT NULL,
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "settled_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_fin_settle_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_fin_settle_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_fin_settle_basis CHECK (
        "basis_code" IN (
            'funded_invoice', 'credit', 'zero_amount', 'full_discount', 'trial'
        )
    ),
    CONSTRAINT ck_idc_fin_settle_amounts CHECK (
        "total_amount_minor" >= 0
        AND "paid_amount_minor" >= 0
        AND "credit_amount_minor" >= 0
        AND (
            ("basis_code" = 'funded_invoice'
                AND "total_amount_minor" > 0
                AND "paid_amount_minor" = "total_amount_minor"
                AND "credit_amount_minor" = 0)
            OR ("basis_code" = 'credit'
                AND "total_amount_minor" > 0
                AND "paid_amount_minor" = 0
                AND "credit_amount_minor" = "total_amount_minor")
            OR ("basis_code" IN ('zero_amount', 'full_discount', 'trial')
                AND "total_amount_minor" = 0
                AND "paid_amount_minor" = 0
                AND "credit_amount_minor" = 0)
        )
    ),
    CONSTRAINT ck_idc_fin_settle_sources CHECK (
        "source_count" BETWEEN 1 AND 1000
        AND (("basis_code" IN ('credit', 'zero_amount', 'full_discount', 'trial')
                AND "source_count" = 1)
            OR "basis_code" = 'funded_invoice')
    ),
    CONSTRAINT ck_idc_fin_settle_digests CHECK (
        "identity_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_fin_settle_no UNIQUE ("settlement_no"),
    CONSTRAINT uk_idc_fin_settle_order UNIQUE ("order_no"),
    CONSTRAINT uk_idc_fin_settle_invoice UNIQUE ("invoice_no"),
    CONSTRAINT uk_idc_fin_settle_owner UNIQUE (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_fin_settle_order FOREIGN KEY (
        "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_order (
        "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_fin_settle_invoice FOREIGN KEY (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_fin_settle_customer
    ON plugin_idc_core_financial_settlement (
        "tenant_id", "customer_no", "settled_at" DESC, "id" DESC
    );

-- Purpose: Bounded immutable source evidence that conserves one settlement header.
-- 用途：保存财务结算来源相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_settlement_source (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "settlement_source_no" VARCHAR(40) NOT NULL,
    "settlement_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "payment_allocation_no" VARCHAR(40),
    "credit_exposure_no" VARCHAR(40),
    "amount_minor" BIGINT NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_fin_source_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_fin_source_type CHECK (
        "source_type_code" IN (
            'channel_payment', 'balance', 'manual_receipt', 'credit',
            'zero_amount', 'full_discount', 'trial'
        )
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_fin_source_shape CHECK (
        (
            "source_type_code" IN ('channel_payment', 'balance', 'manual_receipt')
            AND "payment_allocation_no" IS NOT NULL
            AND "credit_exposure_no" IS NULL
            AND "amount_minor" > 0
        ) OR (
            "source_type_code" = 'credit'
            AND "payment_allocation_no" IS NULL
            AND "credit_exposure_no" IS NOT NULL
            AND "amount_minor" > 0
        ) OR (
            "source_type_code" IN ('zero_amount', 'full_discount', 'trial')
            AND "payment_allocation_no" IS NULL
            AND "credit_exposure_no" IS NULL
            AND "amount_minor" = 0
        )
    ),
    CONSTRAINT ck_idc_fin_source_digests CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_fin_source_no UNIQUE ("settlement_source_no"),
    CONSTRAINT fk_idc_fin_source_settlement FOREIGN KEY (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_financial_settlement (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_fin_source_payment FOREIGN KEY (
        "payment_allocation_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_payment_allocation (
        "allocation_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_fin_source_credit FOREIGN KEY (
        "credit_exposure_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_exposure (
        "exposure_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_fin_source_identity
    ON plugin_idc_core_financial_settlement_source (
        "settlement_no", "source_type_code", "source_reference",
        COALESCE("payment_allocation_no", ''),
        COALESCE("credit_exposure_no", '')
    );

CREATE INDEX IF NOT EXISTS idx_idc_fin_source_settlement
    ON plugin_idc_core_financial_settlement_source (
        "tenant_id", "settlement_no", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_financial_settlement(
    checked_settlement_no VARCHAR
) RETURNS VOID AS $$
DECLARE
    settlement_row plugin_idc_core_financial_settlement%ROWTYPE;
    actual_count BIGINT;
    actual_amount BIGINT;
    invalid_count BIGINT;
BEGIN
    SELECT * INTO settlement_row
    FROM plugin_idc_core_financial_settlement
    WHERE "tenant_id" = 0 AND "settlement_no" = checked_settlement_no;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'financial settlement header is missing';
    END IF;

    SELECT COUNT(*), COALESCE(SUM("amount_minor"), 0),
           COUNT(*) FILTER (WHERE
               (settlement_row."basis_code" = 'funded_invoice'
                   AND "source_type_code" NOT IN (
                       'channel_payment', 'balance', 'manual_receipt'
                   ))
               OR (settlement_row."basis_code" <> 'funded_invoice'
                   AND "source_type_code" <> settlement_row."basis_code")
           )
    INTO actual_count, actual_amount, invalid_count
    FROM plugin_idc_core_financial_settlement_source
    WHERE "tenant_id" = 0 AND "settlement_no" = checked_settlement_no;

    IF actual_count <> settlement_row."source_count"
       OR actual_amount <> settlement_row."paid_amount_minor"
            + settlement_row."credit_amount_minor"
       OR invalid_count <> 0 THEN
        RAISE EXCEPTION 'financial settlement sources do not conserve the header';
    END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_financial_settlement_header()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM plugin_idc_core_validate_financial_settlement(NEW."settlement_no");
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_financial_settlement_sources()
RETURNS TRIGGER AS $$
DECLARE
    checked_settlement_no VARCHAR(40);
BEGIN
    FOR checked_settlement_no IN
        SELECT DISTINCT "settlement_no" FROM inserted_financial_sources
    LOOP
        PERFORM plugin_idc_core_validate_financial_settlement(
            checked_settlement_no
        );
    END LOOP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_financial_settlement'::regclass
           AND conname = 'trg_idc_fin_settle_validate'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_financial_settlement'::regclass
               AND constraint_row.conname = 'trg_idc_fin_settle_validate'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_fin_settle_validate'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_financial_settlement_header()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_fin_settle_validate catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_fin_settle_validate
        AFTER INSERT ON plugin_idc_core_financial_settlement
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION
        plugin_idc_core_validate_financial_settlement_header();
    END IF;
END;
$constraint_trigger$;

DROP TRIGGER IF EXISTS trg_idc_fin_source_validate
ON plugin_idc_core_financial_settlement_source;
CREATE TRIGGER trg_idc_fin_source_validate
AFTER INSERT ON plugin_idc_core_financial_settlement_source
REFERENCING NEW TABLE AS inserted_financial_sources
FOR EACH STATEMENT EXECUTE FUNCTION
plugin_idc_core_validate_financial_settlement_sources();

DROP TRIGGER IF EXISTS trg_idc_fin_settle_immutable
ON plugin_idc_core_financial_settlement;
CREATE TRIGGER trg_idc_fin_settle_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_financial_settlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_fin_source_immutable
    ON plugin_idc_core_financial_settlement_source;
CREATE TRIGGER trg_idc_fin_source_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_financial_settlement_source
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

COMMENT ON TABLE plugin_idc_core_financial_settlement IS
    'Immutable authority that one order is financially eligible for fulfillment';
COMMENT ON COLUMN plugin_idc_core_financial_settlement."basis_code" IS
    'Mutually exclusive funded, credit, zero-amount, full-discount, or trial basis';
COMMENT ON COLUMN plugin_idc_core_financial_settlement."identity_evidence_digest" IS
    'Digest of the re-read customer identity authority without personal material';
COMMENT ON TABLE plugin_idc_core_financial_settlement_source IS
    'Bounded immutable source evidence that conserves one settlement header';
