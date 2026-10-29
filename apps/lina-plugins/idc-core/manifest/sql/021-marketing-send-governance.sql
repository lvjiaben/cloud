-- Marketing delivery consent is fail-closed and independent from mandatory service notices.

-- Purpose: Append-only per-contact marketing opt-in or withdrawal evidence; raw unsubscribe evidence remains notify-owned.
-- 用途：保存营销渠道同意事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_channel_consent_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "channel_code" VARCHAR(24) NOT NULL,
    "action_code" VARCHAR(16) NOT NULL,
    "policy_reference" VARCHAR(256) NOT NULL,
    "policy_version" VARCHAR(64) NOT NULL,
    "locale" VARCHAR(16) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_consent_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_consent_event_channel CHECK (
        "channel_code" IN ('in_app', 'email', 'sms', 'wechat_official')
    ),
    CONSTRAINT ck_idc_marketing_consent_event_action CHECK (
        "action_code" IN ('granted', 'withdrawn')
    ),
    CONSTRAINT ck_idc_marketing_consent_event_policy CHECK (
        length(btrim("policy_reference")) BETWEEN 1 AND 256 AND
        length(btrim("policy_version")) BETWEEN 1 AND 64
    ),
    CONSTRAINT ck_idc_marketing_consent_event_locale CHECK (
        "locale" IN ('zh-CN', 'en-US')
    ),
    CONSTRAINT ck_idc_marketing_consent_event_source CHECK (
        "source_code" IN ('portal', 'admin', 'migration', 'notify')
    ),
    CONSTRAINT ck_idc_marketing_consent_event_digest CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$' AND
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_marketing_consent_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_idc_marketing_consent_event_owner UNIQUE (
        "event_no", "customer_no", "contact_no", "channel_code"
    ),
    CONSTRAINT uk_idc_marketing_consent_event_idempotency UNIQUE (
        "customer_no", "contact_no", "channel_code", "idempotency_key_digest"
    ),
    CONSTRAINT fk_idc_marketing_consent_event_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_consent_event_contact FOREIGN KEY (
        "customer_no", "contact_no"
    ) REFERENCES plugin_idc_core_customer_contact (
        "customer_no", "contact_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_consent_event_history
    ON plugin_idc_core_marketing_channel_consent_event (
        "tenant_id", "customer_no", "contact_no", "channel_code",
        "occurred_at" DESC, "id" DESC
    );

-- Purpose: Fail-closed current marketing channel consent projection derived from append-only events.
-- 用途：保存营销渠道同意相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_channel_consent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "channel_code" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL,
    "current_event_no" VARCHAR(40) NOT NULL,
    "consent_occurred_at" TIMESTAMPTZ NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_consent_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_consent_channel CHECK (
        "channel_code" IN ('in_app', 'email', 'sms', 'wechat_official')
    ),
    CONSTRAINT ck_idc_marketing_consent_status CHECK (
        "status_code" IN ('opted_in', 'opted_out')
    ),
    CONSTRAINT ck_idc_marketing_consent_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_consent_target UNIQUE (
        "customer_no", "contact_no", "channel_code"
    ),
    CONSTRAINT fk_idc_marketing_consent_event FOREIGN KEY (
        "current_event_no", "customer_no", "contact_no", "channel_code"
    ) REFERENCES plugin_idc_core_marketing_channel_consent_event (
        "event_no", "customer_no", "contact_no", "channel_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_consent_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_consent_contact FOREIGN KEY (
        "customer_no", "contact_no"
    ) REFERENCES plugin_idc_core_customer_contact (
        "customer_no", "contact_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_consent_gate
    ON plugin_idc_core_marketing_channel_consent (
        "tenant_id", "customer_no", "contact_no", "channel_code", "status_code"
    );

-- Purpose: Customer-local marketing quiet-hours projection; mandatory service and security notifications do not use this gate.
-- 用途：保存营销免打扰时段相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_quiet_hours (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "timezone_name" VARCHAR(64) NOT NULL,
    "start_minute" SMALLINT NOT NULL,
    "end_minute" SMALLINT NOT NULL,
    "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_quiet_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_quiet_timezone CHECK (
        length(btrim("timezone_name")) BETWEEN 1 AND 64 AND
        "timezone_name" ~ '^[A-Za-z0-9_+.-]+(/[A-Za-z0-9_+.-]+)*$'
    ),
    CONSTRAINT ck_idc_marketing_quiet_minutes CHECK (
        "start_minute" BETWEEN 0 AND 1439 AND "end_minute" BETWEEN 0 AND 1439 AND
        ("enabled" = FALSE OR "start_minute" <> "end_minute")
    ),
    CONSTRAINT ck_idc_marketing_quiet_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_quiet_customer UNIQUE ("customer_no"),
    CONSTRAINT fk_idc_marketing_quiet_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

COMMENT ON TABLE plugin_idc_core_marketing_channel_consent_event
    IS 'Append-only per-contact marketing opt-in or withdrawal evidence; raw unsubscribe evidence remains notify-owned';
COMMENT ON TABLE plugin_idc_core_marketing_channel_consent
    IS 'Fail-closed current marketing channel consent projection derived from append-only events';
COMMENT ON TABLE plugin_idc_core_marketing_quiet_hours
    IS 'Customer-local marketing quiet-hours projection; mandatory service and security notifications do not use this gate';
