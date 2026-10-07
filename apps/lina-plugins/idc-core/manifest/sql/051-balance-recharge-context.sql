-- 051: durable customer-owned balance recharge preparation for the payment owner.

-- Purpose: Stores Balance Recharge Context records.
-- 用途：保存余额充值上下文相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_balance_recharge_context (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "context_no" VARCHAR(40) NOT NULL,
    "payment_attempt_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "principal_ref_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'prepared',
    "content_digest" CHAR(64) NOT NULL,
    "ledger_transaction_no" VARCHAR(40) NOT NULL DEFAULT '',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "confirmed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_balance_recharge_context_no UNIQUE ("context_no"),
    CONSTRAINT uk_idc_balance_recharge_attempt UNIQUE ("payment_attempt_no"),
    CONSTRAINT ck_idc_balance_recharge_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_balance_recharge_money CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_balance_recharge_digest CHECK ("principal_ref_digest" ~ '^[0-9a-f]{64}$' AND "content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_balance_recharge_status CHECK (("status_code" = 'prepared' AND "ledger_transaction_no" = '' AND "confirmed_at" IS NULL) OR ("status_code" = 'confirmed' AND "ledger_transaction_no" <> '' AND "confirmed_at" IS NOT NULL)),
    CONSTRAINT ck_idc_balance_recharge_expiry CHECK ("expires_at" > "created_at")
);

CREATE INDEX IF NOT EXISTS idx_idc_balance_recharge_customer
    ON plugin_idc_core_balance_recharge_context ("tenant_id", "customer_no", "created_at" DESC, "id" DESC);
