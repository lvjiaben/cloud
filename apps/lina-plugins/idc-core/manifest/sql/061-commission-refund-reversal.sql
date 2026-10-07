-- Minimal commission accrual and refund-reversal truth required by the
-- paid required-contract compensation flow. Settlement remains stage 12.

-- Purpose: Immutable-basis commission snapshot; stage 10 uses only pending refund reversal.
-- 用途：保存佣金计提相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commission_accrual (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "commission_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "invoice_item_no" VARCHAR(40) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "rule_version" BIGINT NOT NULL,
    "basis_amount_minor" BIGINT NOT NULL,
    "invoice_item_amount_minor" BIGINT NOT NULL,
    "original_amount_minor" BIGINT NOT NULL,
    "pending_amount_minor" BIGINT NOT NULL,
    "matured_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "reversed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_maturity',
    "matures_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_commission_accrual_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_commission_accrual_identity CHECK (
        "beneficiary_type_code" IN ('agent', 'affiliate', 'sales')
        AND length("beneficiary_reference") BETWEEN 1 AND 128
        AND "currency_code" ~ '^[A-Z]{3}$' AND "rule_version" > 0
    ),
    CONSTRAINT ck_idc_commission_accrual_amounts CHECK (
        "basis_amount_minor" > 0 AND "invoice_item_amount_minor" > 0
        AND "basis_amount_minor" <= "invoice_item_amount_minor"
        AND "original_amount_minor" > 0
        AND "pending_amount_minor" >= 0 AND "matured_amount_minor" >= 0
        AND "reversed_amount_minor" >= 0
        AND "pending_amount_minor" + "matured_amount_minor"
            + "reversed_amount_minor" = "original_amount_minor"
    ),
    CONSTRAINT ck_idc_commission_accrual_status CHECK (
        ("status_code" = 'pending_maturity' AND "pending_amount_minor" > 0)
        OR ("status_code" = 'partially_reversed' AND "pending_amount_minor" > 0
            AND "reversed_amount_minor" > 0)
        OR ("status_code" = 'matured' AND "matured_amount_minor" > 0
            AND "pending_amount_minor" = 0)
        OR ("status_code" = 'reversed' AND "reversed_amount_minor" > 0
            AND "pending_amount_minor" = 0 AND "matured_amount_minor" = 0)
    ),
    CONSTRAINT uk_idc_commission_accrual_no UNIQUE ("commission_no"),
    CONSTRAINT fk_idc_commission_accrual_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_commission_accrual_item FOREIGN KEY (
        "invoice_no", "invoice_item_no"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_commission_accrual_refund
    ON plugin_idc_core_commission_accrual (
        "tenant_id", "invoice_no", "invoice_item_no", "status_code", "id"
    ) WHERE "pending_amount_minor" > 0;

-- Purpose: Append-only proportional reversal of commission that has not matured.
-- 用途：保存佣金退款冲回相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commission_refund_reversal (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reversal_no" VARCHAR(40) NOT NULL,
    "commission_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "source_event_no" VARCHAR(40) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_commission_reversal_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_commission_reversal_amount CHECK ("amount_minor" > 0),
    CONSTRAINT uk_idc_commission_reversal_no UNIQUE ("reversal_no"),
    CONSTRAINT uk_idc_commission_reversal_source UNIQUE (
        "commission_no", "refund_no"
    ),
    CONSTRAINT fk_idc_commission_reversal_accrual FOREIGN KEY ("commission_no")
        REFERENCES plugin_idc_core_commission_accrual ("commission_no")
        ON DELETE RESTRICT
);

-- Purpose: Idempotent consumption receipt for billing refund commission events.
-- 用途：保存佣金退款收款相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commission_refund_receipt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "payload_digest" CHAR(64) NOT NULL,
    "reversal_count" INT NOT NULL,
    "reversed_amount_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_commission_receipt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_commission_receipt_values CHECK (
        "payload_digest" ~ '^[0-9a-f]{64}$' AND "reversal_count" >= 0
        AND "reversed_amount_minor" >= 0
    ),
    CONSTRAINT uk_idc_commission_receipt_event UNIQUE ("event_no"),
    CONSTRAINT uk_idc_commission_receipt_refund UNIQUE ("refund_no")
);

COMMENT ON TABLE plugin_idc_core_commission_accrual IS
    'Immutable-basis commission snapshot; stage 10 uses only pending refund reversal';
COMMENT ON TABLE plugin_idc_core_commission_refund_reversal IS
    'Append-only proportional reversal of commission that has not matured';
COMMENT ON TABLE plugin_idc_core_commission_refund_receipt IS
    'Idempotent consumption receipt for billing refund commission events';
