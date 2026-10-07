-- Admin-created customer activation invitations. Raw invitation tokens are
-- never stored in this table; only their SHA-256 digest is persisted.

-- Purpose: Single-use 24-hour invitation for one admin-created pending customer.
-- 用途：保存客户激活邀请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_activation_invitation (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "invitation_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "token_digest" CHAR(64) NOT NULL,
    "target_user_id" BIGINT,
    "reason" VARCHAR(512) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_by_user_id" BIGINT,
    "consumed_at" TIMESTAMPTZ,
    "superseded_at" TIMESTAMPTZ,
    "created_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_customer_activation_invitation_no UNIQUE ("invitation_no"),
    CONSTRAINT uk_idc_customer_activation_token UNIQUE ("token_digest"),
    CONSTRAINT uk_idc_customer_activation_request UNIQUE ("created_by", "idempotency_key_digest"),
    CONSTRAINT ck_idc_customer_activation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_customer_activation_target CHECK ("target_user_id" IS NULL OR "target_user_id" > 0),
    CONSTRAINT ck_idc_customer_activation_reason CHECK (length(btrim("reason")) BETWEEN 10 AND 512),
    CONSTRAINT ck_idc_customer_activation_digest CHECK (
        "token_digest" ~ '^[0-9a-f]{64}$' AND
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_customer_activation_status CHECK (
        "status_code" IN ('active', 'consumed', 'superseded', 'expired')
    ),
    CONSTRAINT ck_idc_customer_activation_result CHECK (
        ("status_code" = 'active' AND "consumed_by_user_id" IS NULL AND "consumed_at" IS NULL AND "superseded_at" IS NULL) OR
        ("status_code" = 'consumed' AND "consumed_by_user_id" > 0 AND "consumed_at" IS NOT NULL AND "superseded_at" IS NULL) OR
        ("status_code" = 'superseded' AND "consumed_by_user_id" IS NULL AND "consumed_at" IS NULL AND "superseded_at" IS NOT NULL) OR
        ("status_code" = 'expired' AND "consumed_by_user_id" IS NULL AND "consumed_at" IS NULL AND "superseded_at" IS NULL)
    ),
    CONSTRAINT fk_idc_customer_activation_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_customer_activation_contact FOREIGN KEY ("customer_no", "contact_no")
        REFERENCES plugin_idc_core_customer_contact ("customer_no", "contact_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_customer_activation_one_active_customer
    ON plugin_idc_core_customer_activation_invitation ("customer_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_idc_customer_activation_expiry
    ON plugin_idc_core_customer_activation_invitation ("tenant_id", "status_code", "expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_idc_customer_activation_target
    ON plugin_idc_core_customer_activation_invitation ("tenant_id", "target_user_id", "status_code", "id")
    WHERE "target_user_id" IS NOT NULL;

COMMENT ON TABLE plugin_idc_core_customer_activation_invitation IS
    'Single-use 24-hour invitation for one admin-created pending customer';
COMMENT ON COLUMN plugin_idc_core_customer_activation_invitation."token_digest" IS
    'SHA-256 digest of the invitation token; the raw token is only carried by the delivery event and customer request';
