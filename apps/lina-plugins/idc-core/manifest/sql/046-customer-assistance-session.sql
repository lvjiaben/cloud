-- Short-lived customer impersonation sessions and immutable access history.

-- Purpose: Fifteen-minute single-use customer impersonation sessions; raw access tokens are never persisted.
-- 用途：保存客户协助会话相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_assistance_session (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "session_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "staff_user_id" BIGINT NOT NULL,
    "reason" VARCHAR(512) NOT NULL,
    "ticket_no" VARCHAR(64) NOT NULL DEFAULT '',
    "token_digest" CHAR(64) NOT NULL,
    "host_token_id" VARCHAR(64) NOT NULL DEFAULT '',
    "host_token_ciphertext" TEXT NOT NULL DEFAULT '',
    "exchanged_at" TIMESTAMPTZ,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "started_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "ended_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_assistance_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_assistance_status CHECK ("status_code" IN ('active','ended','expired','superseded')),
    CONSTRAINT ck_plugin_idc_core_assistance_reason CHECK (char_length("reason") BETWEEN 10 AND 512),
    CONSTRAINT ck_plugin_idc_core_assistance_window CHECK ("expires_at" > "started_at"),
    CONSTRAINT ck_plugin_idc_core_assistance_host_token CHECK (char_length("host_token_id") BETWEEN 16 AND 64 AND char_length("host_token_ciphertext") >= 64),
    CONSTRAINT uk_plugin_idc_core_assistance_no UNIQUE ("session_no"),
    CONSTRAINT uk_plugin_idc_core_assistance_token UNIQUE ("token_digest"),
    CONSTRAINT uk_plugin_idc_core_assistance_idempotency UNIQUE ("tenant_id", "staff_user_id", "idempotency_key_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_assistance_staff_active
    ON plugin_idc_core_customer_assistance_session ("tenant_id", "staff_user_id", "status_code", "expires_at" DESC, "id" DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_assistance_one_active_staff
    ON plugin_idc_core_customer_assistance_session ("tenant_id", "staff_user_id")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_assistance_customer
    ON plugin_idc_core_customer_assistance_session ("tenant_id", "customer_no", "started_at" DESC, "id" DESC);

-- Purpose: Immutable start, view, end, expiry, and supersession history for customer assistance.
-- 用途：保存客户协助事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_assistance_event (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "session_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "staff_user_id" BIGINT NOT NULL,
    "event_type" VARCHAR(16) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_assistance_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_assistance_event_type CHECK ("event_type" IN ('started','exchanged','ended','expired','superseded')),
    CONSTRAINT uk_plugin_idc_core_assistance_event_no UNIQUE ("event_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_assistance_event_session
    ON plugin_idc_core_customer_assistance_event ("tenant_id", "session_no", "occurred_at", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_assistance_event_customer
    ON plugin_idc_core_customer_assistance_event ("tenant_id", "customer_no", "occurred_at" DESC, "id" DESC);

COMMENT ON TABLE plugin_idc_core_customer_assistance_session IS 'Fifteen-minute single-use customer impersonation sessions; host access tokens are encrypted at rest';
COMMENT ON TABLE plugin_idc_core_customer_assistance_event IS 'Immutable start, view, end, expiry, and supersession history for customer assistance';
