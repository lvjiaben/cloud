-- Channel master data and the local evidence needed to project core-owned
-- customer, order, settlement and lifecycle facts without cross-plugin tables.
-- Purpose: Stores governed channel accounts and their sales availability.
-- 用途：保存受治理的渠道账户及其销售可用状态。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_account (
    "channel_id" VARCHAR(80) PRIMARY KEY,
    "customer_no" VARCHAR(40) NOT NULL,
    "display_name" VARCHAR(160) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_approval',
    new_sales_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    "approved_by" BIGINT NOT NULL DEFAULT 0,
    approved_at TIMESTAMPTZ,
    paused_at TIMESTAMPTZ,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_channel_account_status CHECK (
        "status_code" IN ('pending_approval', 'approved', 'paused', 'closed')
    ),
    CONSTRAINT ck_idc_channel_account_sales CHECK (
        ("status_code" = 'approved' AND new_sales_enabled)
        OR ("status_code" <> 'approved' AND NOT new_sales_enabled)
    ),
    CONSTRAINT ck_idc_channel_account_approval CHECK (
        ("status_code" = 'pending_approval' AND approved_at IS NULL)
        OR ("status_code" <> 'pending_approval' AND approved_at IS NOT NULL)
    ),
    UNIQUE ("customer_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_channel_account_status
    ON plugin_idc_channel_account("status_code", updated_at DESC, "channel_id");

-- Purpose: Stores the channel-to-core mapping and activation state of external customers.
-- 用途：保存渠道外部客户到核心客户的映射及激活状态。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_external_customer (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "external_customer_id" VARCHAR(160) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "invitation_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_activation',
    "identity_status_code" VARCHAR(24) NOT NULL DEFAULT 'unverified',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_channel_external_customer_status CHECK (
        "status_code" IN ('pending_activation', 'active', 'suspended', 'closed')
    ),
    CONSTRAINT ck_idc_channel_external_identity CHECK (
        "identity_status_code" IN ('unverified', 'pending', 'verified', 'rejected', 'expired', 'revoked')
    ),
    CONSTRAINT fk_idc_channel_external_channel FOREIGN KEY ("channel_id")
        REFERENCES plugin_idc_channel_account("channel_id") ON DELETE RESTRICT,
    UNIQUE ("channel_id", "external_customer_id"),
    UNIQUE ("channel_id", "customer_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_channel_external_customer_state
    ON plugin_idc_channel_external_customer("channel_id", "status_code", "identity_status_code", "id");

ALTER TABLE plugin_idc_channel_publication
    ADD COLUMN IF NOT EXISTS "public_slug" VARCHAR(160),
    ADD COLUMN IF NOT EXISTS "configuration_revision" VARCHAR(160),
    ADD COLUMN IF NOT EXISTS billing_term_token VARCHAR(256),
    ADD COLUMN IF NOT EXISTS "purchase_cycles" INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "quantity" INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS purchase_values JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS price_minor BIGINT;

UPDATE plugin_idc_channel_publication
SET price_minor = ROUND("price" * 100)::BIGINT
WHERE price_minor IS NULL;

ALTER TABLE plugin_idc_channel_publication
    ALTER COLUMN price_minor SET NOT NULL;

DO $constraint$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='ck_idc_channel_publication_purchase' AND conrelid='plugin_idc_channel_publication'::regclass) THEN
        ALTER TABLE plugin_idc_channel_publication ADD CONSTRAINT ck_idc_channel_publication_purchase CHECK (
            "purchase_cycles" BETWEEN 1 AND 120 AND "quantity" BETWEEN 1 AND 1000
            AND price_minor >= 0 AND jsonb_typeof(purchase_values)='array'
        );
    END IF;
END
$constraint$;

ALTER TABLE plugin_idc_channel_order
    ADD COLUMN IF NOT EXISTS core_order_no VARCHAR(40),
    ADD COLUMN IF NOT EXISTS core_invoice_no VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "payer_customer_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "service_owner_customer_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "total_amount_minor" BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS identity_ready BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_channel_order_core_order
    ON plugin_idc_channel_order(core_order_no) WHERE core_order_no IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_channel_order_channel_status
    ON plugin_idc_channel_order("channel_id", "status", "id" DESC);

ALTER TABLE plugin_idc_channel_request
    ADD COLUMN IF NOT EXISTS "token_id" BIGINT,
    ADD COLUMN IF NOT EXISTS "result_code" VARCHAR(64) NOT NULL DEFAULT 'accepted',
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_idc_channel_request_audit
    ON plugin_idc_channel_request("channel_id", created_at DESC, "id" DESC);
