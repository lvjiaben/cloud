-- Purpose: Persists business records for plugin_idc_notify_contact_verification.
-- 用途：持久化 plugin_idc_notify_contact_verification 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_contact_verification (
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "verification_no" VARCHAR(48) NOT NULL,
    "purpose_code" VARCHAR(32) NOT NULL CHECK ("purpose_code" = 'support.presales'),
    "operation_code" VARCHAR(16) NOT NULL CHECK ("operation_code" IN ('create', 'renew')),
    "destination_kind" VARCHAR(16) NOT NULL CHECK ("destination_kind" IN ('email', 'sms')),
    "destination_digest" CHAR(64) NOT NULL,
    "browser_digest" CHAR(64) NOT NULL,
    "ticket_digest" CHAR(64) NOT NULL,
    "locale_code" VARCHAR(10) NOT NULL CHECK ("locale_code" IN ('zh-CN', 'en-US')),
    "template_no" VARCHAR(80) NOT NULL,
    "template_version" BIGINT NOT NULL CHECK ("template_version" > 0),
    "request_shape_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "code_digest" CHAR(64) NOT NULL,
    "masked_destination" VARCHAR(320) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL CHECK ("status_code" IN ('prepared', 'sent', 'result_unknown', 'delivery_failed', 'verified', 'expired', 'superseded', 'locked')),
    "delivery_state" VARCHAR(24) NOT NULL CHECK ("delivery_state" IN ('prepared', 'accepted', 'delivered', 'result_unknown', 'failed')),
    failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    retry_after_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    provider_boundary_crossed BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "verification_no"),
    UNIQUE ("tenant_id", "idempotency_digest"),
    FOREIGN KEY ("tenant_id", "template_no", "template_version")
        REFERENCES plugin_idc_notify_template_version ("tenant_id", "template_no", "version"),
    CHECK ("destination_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("browser_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("ticket_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("request_shape_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("idempotency_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("code_digest" ~ '^[0-9a-f]{64}$'),
    CHECK (expires_at > created_at),
    CHECK (retry_after_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_idc_notify_contact_verification_qualification
    ON plugin_idc_notify_contact_verification
    ("tenant_id", "purpose_code", "destination_digest", "browser_digest", "status_code", expires_at DESC);
CREATE INDEX IF NOT EXISTS idx_idc_notify_contact_verification_expiry
    ON plugin_idc_notify_contact_verification ("tenant_id", "status_code", expires_at);

-- Purpose: Persists business records for plugin_idc_notify_rate_window.
-- 用途：持久化 plugin_idc_notify_rate_window 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_rate_window (
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "dimension_code" VARCHAR(24) NOT NULL CHECK ("dimension_code" IN ('destination', 'account', 'browser', 'ip', 'template', 'sms_global')),
    "subject_digest" CHAR(64) NOT NULL CHECK ("subject_digest" ~ '^[0-9a-f]{64}$'),
    window_seconds INTEGER NOT NULL CHECK (window_seconds IN (60, 3600, 86400)),
    window_started_at TIMESTAMPTZ NOT NULL,
    counter INTEGER NOT NULL CHECK (counter > 0),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "dimension_code", "subject_digest", window_seconds, window_started_at),
    CHECK (expires_at > window_started_at)
);

CREATE INDEX IF NOT EXISTS idx_idc_notify_rate_window_expiry
    ON plugin_idc_notify_rate_window ("tenant_id", expires_at);

-- Purpose: Persists business records for plugin_idc_notify_presales_operation.
-- 用途：持久化 plugin_idc_notify_presales_operation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_presales_operation (
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "operation_digest" CHAR(64) NOT NULL CHECK ("operation_digest" ~ '^[0-9a-f]{64}$'),
    "operation_code" VARCHAR(16) NOT NULL CHECK ("operation_code" IN ('create', 'renew')),
    "verification_no" VARCHAR(48) NOT NULL,
    "destination_kind" VARCHAR(16) NOT NULL CHECK ("destination_kind" IN ('email', 'sms')),
    "destination_digest" CHAR(64) NOT NULL CHECK ("destination_digest" ~ '^[0-9a-f]{64}$'),
    "browser_digest" CHAR(64) NOT NULL CHECK ("browser_digest" ~ '^[0-9a-f]{64}$'),
    "ticket_digest" CHAR(64) NOT NULL CHECK ("ticket_digest" ~ '^[0-9a-f]{64}$'),
    "request_digest" CHAR(64) NOT NULL CHECK ("request_digest" ~ '^[0-9a-f]{64}$'),
    "status_code" VARCHAR(24) NOT NULL CHECK ("status_code" IN ('prepared', 'core_completed', 'completed')),
    "ticket_no" VARCHAR(48),
    access_expires_at TIMESTAMPTZ,
    "link_delivery_state" VARCHAR(24) NOT NULL CHECK ("link_delivery_state" IN ('not_started', 'prepared', 'accepted', 'delivered', 'result_unknown', 'failed', 'unavailable')),
    provider_boundary_crossed BOOLEAN NOT NULL DEFAULT FALSE,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "operation_digest"),
    UNIQUE ("tenant_id", "verification_no"),
    FOREIGN KEY ("tenant_id", "verification_no")
        REFERENCES plugin_idc_notify_contact_verification ("tenant_id", "verification_no"),
    CHECK ("ticket_no" IS NULL OR "ticket_no" ~ '^TKT_[A-Za-z0-9_-]{1,36}$')
);

CREATE INDEX IF NOT EXISTS idx_idc_notify_presales_operation_ticket
    ON plugin_idc_notify_presales_operation ("tenant_id", "ticket_no", "status_code");

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id", "event_code", "topic_code", "object_type", required_variables, optional_variables, security_required, enabled, "version")
VALUES
    (0, 'support.presales_verification_code', 'support', 'support_ticket', '["code","expires_minutes"]'::jsonb, '[]'::jsonb, TRUE, TRUE, 1)
ON CONFLICT ("tenant_id", "event_code") DO UPDATE SET
    "topic_code" = EXCLUDED."topic_code",
    "object_type" = EXCLUDED."object_type",
    required_variables = EXCLUDED.required_variables,
    optional_variables = EXCLUDED.optional_variables,
    security_required = EXCLUDED.security_required,
    enabled = EXCLUDED.enabled;

INSERT INTO plugin_idc_notify_template_version
    ("tenant_id", "template_no", "version", "event_code", "channel_code", "locale_code",
     "subject_template", "body_template", "provider_template_id", provider_sms_variable_order,
     provider_wechat_data_mapping, "status_code", "content_digest", "created_by")
VALUES
    (0, 'ntpl_presales_verification_email_zh', 1, 'support.presales_verification_code', 'email', 'zh-CN',
     '售前咨询验证码', '您的验证码是 {{code}}，{{expires_minutes}} 分钟内有效。请勿转发给他人。', '', '[]'::jsonb, '{}'::jsonb,
     'active', 'b461128fae8564158512c317dff7555c8bd61e58e9235033b729c0ae497f3b67', 1),
    (0, 'ntpl_presales_verification_email_en', 1, 'support.presales_verification_code', 'email', 'en-US',
     'Presales verification code', 'Your verification code is {{code}}. It expires in {{expires_minutes}} minutes. Do not share it.', '', '[]'::jsonb, '{}'::jsonb,
     'active', 'e9585656cd487c045c5c2ad146fa711d5512fd0f3e4ffe3f42f96bbb0efbd4f2', 1)
ON CONFLICT ("tenant_id", "template_no", "version") DO NOTHING;
