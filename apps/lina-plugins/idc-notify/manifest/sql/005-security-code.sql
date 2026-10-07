-- Purpose: Persists business records for plugin_idc_notify_security_code_purpose.
-- 用途：持久化 plugin_idc_notify_security_code_purpose 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_security_code_purpose (
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "caller_plugin_id" VARCHAR(128) NOT NULL,
    "purpose_code" VARCHAR(64) NOT NULL,
    "event_code" VARCHAR(128) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "caller_plugin_id", "purpose_code"),
    FOREIGN KEY ("tenant_id", "event_code")
        REFERENCES plugin_idc_notify_event_catalog ("tenant_id", "event_code")
);

-- Purpose: Persists business records for plugin_idc_notify_security_code_verification.
-- 用途：持久化 plugin_idc_notify_security_code_verification 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_security_code_verification (
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "verification_no" VARCHAR(48) NOT NULL,
    "caller_plugin_id" VARCHAR(128) NOT NULL,
    "purpose_code" VARCHAR(64) NOT NULL,
    "purpose_version" BIGINT NOT NULL CHECK ("purpose_version" > 0),
    "subject_digest" CHAR(64) NOT NULL,
    "destination_digest" CHAR(64) NOT NULL,
    "browser_digest" CHAR(64) NOT NULL,
    "locale_code" VARCHAR(10) NOT NULL CHECK ("locale_code" IN ('zh-CN', 'en-US')),
    "template_no" VARCHAR(80) NOT NULL,
    "template_version" BIGINT NOT NULL CHECK ("template_version" > 0),
    "request_shape_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "code_digest" CHAR(64) NOT NULL,
    "masked_destination" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL CHECK ("status_code" IN ('prepared', 'sent', 'result_unknown', 'delivery_failed', 'verified', 'expired', 'superseded', 'locked')),
    "delivery_state" VARCHAR(24) NOT NULL CHECK ("delivery_state" IN ('prepared', 'accepted', 'delivered', 'result_unknown', 'failed')),
    failed_attempts INTEGER NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
    "verification_version" BIGINT NOT NULL DEFAULT 0 CHECK ("verification_version" BETWEEN 0 AND 1),
    retry_after_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    provider_boundary_crossed BOOLEAN NOT NULL DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "verification_no"),
    UNIQUE ("tenant_id", "caller_plugin_id", "idempotency_digest"),
    FOREIGN KEY ("tenant_id", "caller_plugin_id", "purpose_code")
        REFERENCES plugin_idc_notify_security_code_purpose ("tenant_id", "caller_plugin_id", "purpose_code"),
    FOREIGN KEY ("tenant_id", "template_no", "template_version")
        REFERENCES plugin_idc_notify_template_version ("tenant_id", "template_no", "version"),
    CHECK ("subject_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("destination_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("browser_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("request_shape_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("idempotency_digest" ~ '^[0-9a-f]{64}$'),
    CHECK ("code_digest" ~ '^[0-9a-f]{64}$'),
    CHECK (expires_at > created_at),
    CHECK (retry_after_at >= created_at),
    CHECK (("status_code" = 'verified') = ("verification_version" = 1 AND verified_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_idc_notify_security_code_qualification
    ON plugin_idc_notify_security_code_verification
    ("tenant_id", "caller_plugin_id", "purpose_code", "subject_digest", "destination_digest", "browser_digest", "status_code", expires_at DESC);
CREATE INDEX IF NOT EXISTS idx_idc_notify_security_code_expiry
    ON plugin_idc_notify_security_code_verification ("tenant_id", "status_code", expires_at);

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id", "event_code", "topic_code", "object_type", required_variables, optional_variables, security_required, enabled, "version")
VALUES
    (0, 'compliance.contract_signer_phone_verification', 'security', 'contract_signer',
     '["code","expires_minutes"]'::jsonb, '[]'::jsonb, TRUE, TRUE, 1)
ON CONFLICT ("tenant_id", "event_code") DO UPDATE SET
    "topic_code" = EXCLUDED."topic_code",
    "object_type" = EXCLUDED."object_type",
    required_variables = EXCLUDED.required_variables,
    optional_variables = EXCLUDED.optional_variables,
    security_required = EXCLUDED.security_required,
    enabled = EXCLUDED.enabled;

INSERT INTO plugin_idc_notify_security_code_purpose
    ("tenant_id", "caller_plugin_id", "purpose_code", "event_code", enabled, "version")
VALUES
    (0, 'idc-compliance', 'contract_signer_phone',
     'compliance.contract_signer_phone_verification', TRUE, 1)
ON CONFLICT ("tenant_id", "caller_plugin_id", "purpose_code") DO UPDATE SET
    "event_code" = EXCLUDED."event_code",
    enabled = EXCLUDED.enabled,
    "version" = plugin_idc_notify_security_code_purpose."version" + 1,
    updated_at = CURRENT_TIMESTAMP;
