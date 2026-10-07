-- Purpose: Persists business records for plugin_idc_notify_wecom_oauth_intent.
-- 用途：持久化 plugin_idc_notify_wecom_oauth_intent 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_wecom_oauth_intent (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "intent_no" VARCHAR(80) NOT NULL,
    "state_digest" CHAR(64) NOT NULL,
    "host_user_id" BIGINT NOT NULL CHECK ("host_user_id" > 0),
    "staff_version" BIGINT NOT NULL CHECK ("staff_version" > 0),
    "session_digest" CHAR(64) NOT NULL,
    "channel_generation" BIGINT NOT NULL CHECK ("channel_generation" > 0),
    "channel_version" BIGINT NOT NULL CHECK ("channel_version" > 0),
    "corp_id" VARCHAR(64) NOT NULL,
    "agent_id" BIGINT NOT NULL CHECK ("agent_id" > 0),
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "lease_token" VARCHAR(80),
    lease_expires_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length("state_digest") = 64),
    CHECK (length("session_digest") = 64),
    CHECK ("status_code" IN ('pending','exchanging','completed','failed')),
    CHECK (("status_code" = 'completed') = (completed_at IS NOT NULL)),
    PRIMARY KEY ("tenant_id", "intent_no"),
    UNIQUE ("tenant_id", "state_digest")
);

-- Purpose: Persists business records for plugin_idc_notify_wecom_employee_binding.
-- 用途：持久化 plugin_idc_notify_wecom_employee_binding 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_wecom_employee_binding (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "binding_no" VARCHAR(80) NOT NULL,
    "host_user_id" BIGINT NOT NULL CHECK ("host_user_id" > 0),
    "staff_version" BIGINT NOT NULL CHECK ("staff_version" > 0),
    "corp_id" VARCHAR(64) NOT NULL,
    "channel_generation" BIGINT NOT NULL CHECK ("channel_generation" > 0),
    "identity_digest" CHAR(64) NOT NULL,
    "identity_secret_ref" VARCHAR(256),
    identity_secret_updated_at TIMESTAMPTZ,
    "secret_operation_key" VARCHAR(80) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'configuring',
    "terminal_status_code" VARCHAR(24) NOT NULL DEFAULT '',
    "request_no" VARCHAR(80) NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token" VARCHAR(80),
    lease_expires_at TIMESTAMPTZ,
    "attempt_count" INTEGER NOT NULL DEFAULT 0 CHECK ("attempt_count" >= 0),
    "last_error_code" VARCHAR(64) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    bound_at TIMESTAMPTZ,
    terminal_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length("identity_digest") = 64),
    CHECK ("status_code" IN ('configuring','active','revoking','unbound','invalidated')),
    CHECK ("terminal_status_code" IN ('','unbound','invalidated')),
    CHECK (("identity_secret_ref" IS NULL) = (identity_secret_updated_at IS NULL)),
    CHECK ("status_code" <> 'active' OR (bound_at IS NOT NULL AND "identity_secret_ref" IS NOT NULL)),
    CHECK ("status_code" NOT IN ('unbound','invalidated') OR terminal_at IS NOT NULL),
    CHECK ("status_code" IN ('unbound','invalidated') OR terminal_at IS NULL),
    CHECK ("status_code" <> 'revoking' OR ("terminal_status_code" <> '' AND "identity_secret_ref" IS NOT NULL)),
    PRIMARY KEY ("tenant_id", "binding_no"),
    UNIQUE ("tenant_id", "request_no")
);

-- Purpose: Persists business records for plugin_idc_notify_wecom_binding_audit.
-- 用途：持久化 plugin_idc_notify_wecom_binding_audit 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_wecom_binding_audit (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "audit_no" VARCHAR(80) NOT NULL,
    "binding_no" VARCHAR(80) NOT NULL,
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    "action_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "previous_status" VARCHAR(24) NOT NULL,
    "current_status" VARCHAR(24) NOT NULL,
    "binding_version" BIGINT NOT NULL CHECK ("binding_version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("action_code" IN ('bind','unbind','invalidate','test')),
    PRIMARY KEY ("tenant_id", "audit_no"),
    FOREIGN KEY ("tenant_id", "binding_no")
        REFERENCES plugin_idc_notify_wecom_employee_binding("tenant_id", "binding_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS plugin_idc_notify_wecom_binding_host_active_idx
    ON plugin_idc_notify_wecom_employee_binding ("tenant_id", "host_user_id")
    WHERE "status_code" IN ('configuring','active','revoking');
CREATE UNIQUE INDEX IF NOT EXISTS plugin_idc_notify_wecom_binding_identity_active_idx
    ON plugin_idc_notify_wecom_employee_binding ("tenant_id", "corp_id", "identity_digest")
    WHERE "status_code" IN ('configuring','active','revoking');
CREATE INDEX IF NOT EXISTS plugin_idc_notify_wecom_binding_recovery_idx
    ON plugin_idc_notify_wecom_employee_binding ("tenant_id", "status_code", available_at, "binding_no")
    WHERE "status_code" IN ('configuring','revoking');
CREATE INDEX IF NOT EXISTS plugin_idc_notify_wecom_oauth_staff_idx
    ON plugin_idc_notify_wecom_oauth_intent ("tenant_id", "host_user_id", created_at DESC, "intent_no" DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_notify_wecom_oauth_expiry_idx
    ON plugin_idc_notify_wecom_oauth_intent ("tenant_id", "status_code", expires_at, "intent_no")
    WHERE "status_code" IN ('pending','exchanging');
CREATE INDEX IF NOT EXISTS plugin_idc_notify_wecom_binding_audit_target_idx
    ON plugin_idc_notify_wecom_binding_audit ("tenant_id", "binding_no", created_at, "audit_no");
