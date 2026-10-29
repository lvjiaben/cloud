-- Notification channels, bilingual templates, delivery evidence, preferences, and abuse fences.
-- PostgreSQL is authoritative for every lifecycle, retry, and resend decision.

-- Purpose: Persists business records for plugin_idc_notify_event_catalog.
-- 用途：持久化 plugin_idc_notify_event_catalog 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_event_catalog (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "event_code" VARCHAR(96) NOT NULL,
    "topic_code" VARCHAR(32) NOT NULL,
    "object_type" VARCHAR(48) NOT NULL,
    required_variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    optional_variables JSONB NOT NULL DEFAULT '[]'::jsonb,
    security_required BOOLEAN NOT NULL DEFAULT FALSE,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("topic_code" IN ('account','billing','service','security','support','marketing')),
    CHECK (jsonb_typeof(required_variables) = 'array'),
    CHECK (jsonb_typeof(optional_variables) = 'array'),
    PRIMARY KEY ("tenant_id", "event_code")
);

-- Purpose: Persists business records for plugin_idc_notify_channel.
-- 用途：持久化 plugin_idc_notify_channel 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_channel (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "channel_code" VARCHAR(32) NOT NULL,
    "channel_kind" VARCHAR(32) NOT NULL,
    "provider_code" VARCHAR(64),
    "display_name" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'unconfigured',
    "balance_state" VARCHAR(32) NOT NULL DEFAULT 'unknown',
    "configuration_generation" BIGINT NOT NULL DEFAULT 0 CHECK ("configuration_generation" >= 0),
    "configuration_version" BIGINT NOT NULL DEFAULT 1 CHECK ("configuration_version" > 0),
    non_secret_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    "credential_secret_ref" VARCHAR(256),
    "credential_masked_tail" VARCHAR(16) NOT NULL DEFAULT '',
    credential_updated_at TIMESTAMPTZ,
    "callback_verified_generation" BIGINT NOT NULL DEFAULT 0 CHECK ("callback_verified_generation" >= 0),
    callback_verified_at TIMESTAMPTZ,
    disabled_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("channel_kind" IN ('in_app','email','sms','wechat_official','signed_webhook','dingtalk','feishu')),
    CHECK ("status_code" IN ('unconfigured','configuring','enabled','disabled','configuration_failed')),
    CHECK ("balance_state" IN ('unknown','sufficient','insufficient')),
    CHECK (jsonb_typeof(non_secret_configuration) = 'object'),
    CHECK (("credential_secret_ref" IS NULL) = (credential_updated_at IS NULL)),
    PRIMARY KEY ("tenant_id", "channel_code")
);

ALTER TABLE plugin_idc_notify_channel
    ADD COLUMN IF NOT EXISTS "callback_verified_generation" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE plugin_idc_notify_channel
    ADD COLUMN IF NOT EXISTS callback_verified_at TIMESTAMPTZ;

-- Purpose: Persists business records for plugin_idc_notify_channel_operation.
-- 用途：持久化 plugin_idc_notify_channel_operation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_channel_operation (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "operation_no" VARCHAR(80) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "operation_code" VARCHAR(48) NOT NULL,
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    "reason_code" VARCHAR(64) NOT NULL,
    "before_version" BIGINT NOT NULL CHECK ("before_version" > 0),
    "after_version" BIGINT NOT NULL CHECK ("after_version" > "before_version"),
    affected_task_count INTEGER NOT NULL DEFAULT 0 CHECK (affected_task_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "operation_no"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code")
);

-- Purpose: Persists business records for plugin_idc_notify_channel_intent.
-- 用途：持久化 plugin_idc_notify_channel_intent 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_channel_intent (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "operation_no" VARCHAR(80) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "operation_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'prepared',
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    "base_status_code" VARCHAR(32) NOT NULL,
    "base_version" BIGINT NOT NULL CHECK ("base_version" > 0),
    "request_shape_digest" CHAR(64) NOT NULL,
    "desired_provider_code" VARCHAR(64),
    desired_non_secret_configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    "secret_operation_key" VARCHAR(80) NOT NULL,
    "credential_secret_ref" VARCHAR(256),
    secret_expected_updated_at TIMESTAMPTZ,
    secret_result_updated_at TIMESTAMPTZ,
    "secret_result_masked_tail" VARCHAR(16) NOT NULL DEFAULT '',
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token" VARCHAR(80),
    lease_expires_at TIMESTAMPTZ,
    "attempt_count" INTEGER NOT NULL DEFAULT 0 CHECK ("attempt_count" >= 0),
    "last_error_code" VARCHAR(64) NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("operation_code" IN ('configure_create','configure_replace','clear')),
    CHECK ("status_code" IN ('prepared','secret_applied','completed','failed')),
    CHECK ("base_status_code" IN ('unconfigured','enabled','disabled','configuration_failed')),
    CHECK (length("request_shape_digest") = 64),
    CHECK (jsonb_typeof(desired_non_secret_configuration) = 'object'),
    CHECK (("lease_token" IS NULL) = (lease_expires_at IS NULL)),
    CHECK (("status_code" = 'completed') = (completed_at IS NOT NULL)),
    CHECK (
        ("operation_code" = 'configure_create' AND secret_expected_updated_at IS NULL) OR
        ("operation_code" IN ('configure_replace','clear') AND "credential_secret_ref" IS NOT NULL AND secret_expected_updated_at IS NOT NULL)
    ),
    PRIMARY KEY ("tenant_id", "operation_no"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code")
);

-- Purpose: Persists business records for plugin_idc_notify_template_version.
-- 用途：持久化 plugin_idc_notify_template_version 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_template_version (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "template_no" VARCHAR(80) NOT NULL,
    "version" BIGINT NOT NULL CHECK ("version" > 0),
    "event_code" VARCHAR(96) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "locale_code" VARCHAR(16) NOT NULL,
    "subject_template" TEXT NOT NULL DEFAULT '',
    "body_template" TEXT NOT NULL,
    "provider_template_id" VARCHAR(128) NOT NULL DEFAULT '',
    provider_sms_variable_order JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider_wechat_data_mapping JSONB NOT NULL DEFAULT '{}'::jsonb,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'draft',
    "content_digest" CHAR(64) NOT NULL,
    "created_by" BIGINT NOT NULL CHECK ("created_by" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "template_no", "version"),
    UNIQUE ("tenant_id", "event_code", "channel_code", "locale_code", "version"),
    FOREIGN KEY ("tenant_id", "event_code")
        REFERENCES plugin_idc_notify_event_catalog("tenant_id", "event_code"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code"),
    CHECK ("locale_code" IN ('zh-CN','en-US')),
    CHECK ("status_code" IN ('draft','active','disabled')),
    CHECK (jsonb_typeof(provider_sms_variable_order) = 'array'),
    CHECK (jsonb_typeof(provider_wechat_data_mapping) = 'object'),
    CHECK (length("content_digest") = 64)
);

ALTER TABLE plugin_idc_notify_template_version
    ADD COLUMN IF NOT EXISTS "provider_template_id" VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_notify_template_version
    ADD COLUMN IF NOT EXISTS provider_sms_variable_order JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE plugin_idc_notify_template_version
    ADD COLUMN IF NOT EXISTS provider_wechat_data_mapping JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Purpose: Persists business records for plugin_idc_notify_unsubscribe_evidence.
-- 用途：持久化 plugin_idc_notify_unsubscribe_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_unsubscribe_evidence (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "evidence_no" VARCHAR(80) NOT NULL,
    "owner_user_id" BIGINT NOT NULL CHECK ("owner_user_id" > 0),
    "customer_no" VARCHAR(80) NOT NULL DEFAULT '',
    "contact_no" VARCHAR(80) NOT NULL DEFAULT '',
    "channel_code" VARCHAR(32) NOT NULL,
    "topic_code" VARCHAR(32) NOT NULL,
    "source_code" VARCHAR(32) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("topic_code" = 'marketing'),
    CHECK ("source_code" IN ('account_preference','provider_callback','signed_link','admin_import')),
    CHECK (length("evidence_digest") = 64),
    PRIMARY KEY ("tenant_id", "evidence_no"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code")
);

-- Purpose: Persists business records for plugin_idc_notify_delivery_chain.
-- 用途：持久化 plugin_idc_notify_delivery_chain 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_delivery_chain (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "chain_no" VARCHAR(80) NOT NULL,
    "root_task_no" VARCHAR(80) NOT NULL,
    "tail_task_no" VARCHAR(80) NOT NULL,
    "manual_resend_count" SMALLINT NOT NULL DEFAULT 0 CHECK ("manual_resend_count" BETWEEN 0 AND 3),
    last_manual_resend_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "chain_no"),
    UNIQUE ("tenant_id", "root_task_no"),
    UNIQUE ("tenant_id", "tail_task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_delivery_task.
-- 用途：持久化 plugin_idc_notify_delivery_task 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_delivery_task (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "task_no" VARCHAR(80) NOT NULL,
    "chain_no" VARCHAR(80) NOT NULL,
    "previous_task_no" VARCHAR(80),
    "event_code" VARCHAR(96) NOT NULL,
    "topic_code" VARCHAR(32) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "template_no" VARCHAR(80),
    "template_version" BIGINT,
    "locale_code" VARCHAR(16) NOT NULL,
    "owner_user_id" BIGINT CHECK ("owner_user_id" > 0),
    "customer_no" VARCHAR(80) NOT NULL DEFAULT '',
    "contact_no" VARCHAR(80) NOT NULL DEFAULT '',
    "recipient_mask" VARCHAR(128) NOT NULL DEFAULT '',
    "business_object_type" VARCHAR(48) NOT NULL,
    "business_object_no" VARCHAR(128) NOT NULL,
    "business_object_version" BIGINT NOT NULL DEFAULT 0 CHECK ("business_object_version" >= 0),
    "marketing_snapshot_no" VARCHAR(80) NOT NULL DEFAULT '',
    "marketing_decision_digest" CHAR(64),
    "marketing_consent_event_no" VARCHAR(80),
    "marketing_consent_version" BIGINT,
    "marketing_preference_version" BIGINT,
    "marketing_quiet_hours_version" BIGINT,
    marketing_evaluated_at TIMESTAMPTZ,
    "subject_content" TEXT NOT NULL DEFAULT '',
    "body_content" TEXT NOT NULL,
    provider_variables JSONB NOT NULL DEFAULT '{}'::jsonb,
    "content_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(40) NOT NULL DEFAULT 'queued',
    "result_class" VARCHAR(48) NOT NULL DEFAULT 'none',
    external_boundary_crossed BOOLEAN NOT NULL DEFAULT FALSE,
    "provider_message_ref" VARCHAR(256),
    provider_query_deadline TIMESTAMPTZ,
    "provider_query_deadline_source" VARCHAR(96) NOT NULL DEFAULT '',
    "retry_sequence" SMALLINT NOT NULL DEFAULT 0 CHECK ("retry_sequence" BETWEEN 0 AND 3),
    next_attempt_at TIMESTAMPTZ,
    "attempt_lease" VARCHAR(80),
    attempt_lease_until TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    queued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("locale_code" IN ('zh-CN','en-US')),
    CHECK ("status_code" IN ('queued','sending','retry_wait','submitted','delivered','succeeded','failed','result_unknown','manual_review','manual_unknown_closed','cancelled','template_error','preference_blocked')),
    CHECK ("result_class" IN ('none','accepted','delivered','definitive_not_received_temporary','definitive_not_received_permanent','invalid_recipient','unsubscribed','insufficient_balance','configuration_error','result_unknown','manually_closed_unknown','channel_disabled')),
    CHECK (jsonb_typeof(provider_variables) = 'object'),
    CHECK (length("content_digest") = 64),
    CHECK ("marketing_decision_digest" IS NULL OR length("marketing_decision_digest") = 64),
    CHECK (("template_no" IS NULL) = ("template_version" IS NULL)),
    PRIMARY KEY ("tenant_id", "task_no"),
    FOREIGN KEY ("tenant_id", "chain_no")
        REFERENCES plugin_idc_notify_delivery_chain("tenant_id", "chain_no"),
    FOREIGN KEY ("tenant_id", "previous_task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no"),
    FOREIGN KEY ("tenant_id", "event_code")
        REFERENCES plugin_idc_notify_event_catalog("tenant_id", "event_code"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code"),
    FOREIGN KEY ("tenant_id", "template_no", "template_version")
        REFERENCES plugin_idc_notify_template_version("tenant_id", "template_no", "version")
);

ALTER TABLE plugin_idc_notify_delivery_task
    ADD COLUMN IF NOT EXISTS provider_variables JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE plugin_idc_notify_delivery_chain
    DROP CONSTRAINT IF EXISTS plugin_idc_notify_delivery_chain_root_fk;
ALTER TABLE plugin_idc_notify_delivery_chain
    ADD CONSTRAINT plugin_idc_notify_delivery_chain_root_fk
    FOREIGN KEY ("tenant_id", "root_task_no")
    REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
    DEFERRABLE INITIALLY DEFERRED;
ALTER TABLE plugin_idc_notify_delivery_chain
    DROP CONSTRAINT IF EXISTS plugin_idc_notify_delivery_chain_tail_fk;
ALTER TABLE plugin_idc_notify_delivery_chain
    ADD CONSTRAINT plugin_idc_notify_delivery_chain_tail_fk
    FOREIGN KEY ("tenant_id", "tail_task_no")
    REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
    DEFERRABLE INITIALLY DEFERRED;

-- Purpose: Persists business records for plugin_idc_notify_delivery_attempt.
-- 用途：持久化 plugin_idc_notify_delivery_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_delivery_attempt (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "attempt_no" VARCHAR(80) NOT NULL,
    "task_no" VARCHAR(80) NOT NULL,
    "retry_sequence" SMALLINT NOT NULL CHECK ("retry_sequence" BETWEEN 0 AND 3),
    "attempt_fence" VARCHAR(80) NOT NULL,
    crossed_external_boundary BOOLEAN NOT NULL DEFAULT FALSE,
    "result_class" VARCHAR(48) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "provider_reference" VARCHAR(256),
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    UNIQUE ("tenant_id", "task_no", "retry_sequence"),
    UNIQUE ("tenant_id", "attempt_fence"),
    CHECK (length("evidence_digest") = 64),
    PRIMARY KEY ("tenant_id", "attempt_no"),
    FOREIGN KEY ("tenant_id", "task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_channel_test_operation.
-- 用途：持久化 plugin_idc_notify_channel_test_operation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_channel_test_operation (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "operation_no" VARCHAR(80) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "channel_version" BIGINT NOT NULL CHECK ("channel_version" > 0),
    "template_no" VARCHAR(80) NOT NULL,
    "template_version" BIGINT NOT NULL CHECK ("template_version" > 0),
    "target_mask" VARCHAR(128) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'prepared',
    "result_class" VARCHAR(48) NOT NULL DEFAULT 'result_unknown',
    crossed_external_boundary BOOLEAN NOT NULL DEFAULT FALSE,
    "provider_reference" VARCHAR(256),
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length("request_digest") = 64),
    CHECK ("status_code" IN ('prepared','completed')),
    CHECK ("result_class" IN ('accepted','delivered','definitive_not_received_temporary','definitive_not_received_permanent','invalid_recipient','insufficient_balance','configuration_error','result_unknown')),
    CHECK (("status_code" = 'completed') = (completed_at IS NOT NULL)),
    PRIMARY KEY ("tenant_id", "operation_no"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code"),
    FOREIGN KEY ("tenant_id", "template_no", "template_version")
        REFERENCES plugin_idc_notify_template_version("tenant_id", "template_no", "version")
);

-- Purpose: Persists business records for plugin_idc_notify_delivery_query.
-- 用途：持久化 plugin_idc_notify_delivery_query 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_delivery_query (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "query_no" VARCHAR(80) NOT NULL,
    "task_no" VARCHAR(80) NOT NULL,
    "source_code" VARCHAR(48) NOT NULL,
    "result_class" VARCHAR(48) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length("evidence_digest") = 64),
    PRIMARY KEY ("tenant_id", "query_no"),
    FOREIGN KEY ("tenant_id", "task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_wechat_callback_event.
-- 用途：持久化 plugin_idc_notify_wechat_callback_event 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_wechat_callback_event (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "callback_no" VARCHAR(80) NOT NULL,
    "provider_message_ref" VARCHAR(256) NOT NULL,
    "event_digest" CHAR(64) NOT NULL,
    "result_class" VARCHAR(48) NOT NULL,
    "applied_task_no" VARCHAR(80),
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (length("event_digest") = 64),
    CHECK ("result_class" IN ('delivered','definitive_not_received_temporary','definitive_not_received_permanent','invalid_recipient','result_unknown')),
    PRIMARY KEY ("tenant_id", "callback_no"),
    UNIQUE ("tenant_id", "event_digest"),
    FOREIGN KEY ("tenant_id", "applied_task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_manual_closure.
-- 用途：持久化 plugin_idc_notify_manual_closure 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_manual_closure (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "closure_no" VARCHAR(80) NOT NULL,
    "task_no" VARCHAR(80) NOT NULL,
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    "reason_text" VARCHAR(500) NOT NULL,
    "expected_task_version" BIGINT NOT NULL CHECK ("expected_task_version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "closure_no"),
    UNIQUE ("tenant_id", "task_no"),
    FOREIGN KEY ("tenant_id", "task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_manual_resend.
-- 用途：持久化 plugin_idc_notify_manual_resend 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_manual_resend (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "operation_no" VARCHAR(80) NOT NULL,
    "chain_no" VARCHAR(80) NOT NULL,
    "source_task_no" VARCHAR(80) NOT NULL,
    "created_task_no" VARCHAR(80) NOT NULL,
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" > 0),
    "reason_text" VARCHAR(500) NOT NULL,
    "source_task_version" BIGINT NOT NULL CHECK ("source_task_version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "operation_no"),
    UNIQUE ("tenant_id", "source_task_no"),
    UNIQUE ("tenant_id", "created_task_no"),
    FOREIGN KEY ("tenant_id", "chain_no")
        REFERENCES plugin_idc_notify_delivery_chain("tenant_id", "chain_no"),
    FOREIGN KEY ("tenant_id", "source_task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no"),
    FOREIGN KEY ("tenant_id", "created_task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

-- Purpose: Persists business records for plugin_idc_notify_delivery_audit.
-- 用途：持久化 plugin_idc_notify_delivery_audit 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_delivery_audit (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "audit_no" VARCHAR(80) NOT NULL,
    "task_no" VARCHAR(80) NOT NULL,
    "action_code" VARCHAR(64) NOT NULL,
    "actor_type" VARCHAR(24) NOT NULL,
    "actor_user_id" BIGINT,
    "reason_code" VARCHAR(64) NOT NULL,
    "previous_status" VARCHAR(40) NOT NULL,
    "current_status" VARCHAR(40) NOT NULL,
    "task_version" BIGINT NOT NULL CHECK ("task_version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("actor_type" IN ('system','staff','customer','provider')),
    PRIMARY KEY ("tenant_id", "audit_no"),
    FOREIGN KEY ("tenant_id", "task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

CREATE INDEX IF NOT EXISTS plugin_idc_notify_template_event_idx
    ON plugin_idc_notify_template_version ("tenant_id", "event_code", "channel_code", "locale_code", "status_code", "version" DESC);
CREATE UNIQUE INDEX IF NOT EXISTS plugin_idc_notify_template_active_idx
    ON plugin_idc_notify_template_version ("tenant_id", "event_code", "channel_code", "locale_code")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS plugin_idc_notify_delivery_due_idx
    ON plugin_idc_notify_delivery_task ("tenant_id", "status_code", next_attempt_at, queued_at, "task_no")
    WHERE "status_code" IN ('queued','retry_wait');
CREATE INDEX IF NOT EXISTS plugin_idc_notify_delivery_channel_pending_idx
    ON plugin_idc_notify_delivery_task ("tenant_id", "channel_code", "status_code", external_boundary_crossed, "task_no");
CREATE INDEX IF NOT EXISTS plugin_idc_notify_delivery_owner_idx
    ON plugin_idc_notify_delivery_task ("tenant_id", "owner_user_id", queued_at DESC, "task_no" DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_notify_delivery_object_idx
    ON plugin_idc_notify_delivery_task ("tenant_id", "business_object_type", "business_object_no", queued_at DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_notify_attempt_task_idx
    ON plugin_idc_notify_delivery_attempt ("tenant_id", "task_no", "retry_sequence");
CREATE INDEX IF NOT EXISTS plugin_idc_notify_channel_test_created_idx
    ON plugin_idc_notify_channel_test_operation ("tenant_id", "channel_code", created_at DESC, "operation_no");
CREATE INDEX IF NOT EXISTS plugin_idc_notify_query_task_idx
    ON plugin_idc_notify_delivery_query ("tenant_id", "task_no", observed_at DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_notify_wechat_callback_message_idx
    ON plugin_idc_notify_wechat_callback_event ("tenant_id", "provider_message_ref", received_at DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_notify_unsubscribe_owner_idx
    ON plugin_idc_notify_unsubscribe_evidence ("tenant_id", "owner_user_id", "channel_code", occurred_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS plugin_idc_notify_channel_intent_active_idx
    ON plugin_idc_notify_channel_intent ("tenant_id", "channel_code")
    WHERE "status_code" IN ('prepared','secret_applied');
CREATE INDEX IF NOT EXISTS plugin_idc_notify_channel_intent_due_idx
    ON plugin_idc_notify_channel_intent ("tenant_id", "status_code", available_at, "operation_no")
    WHERE "status_code" IN ('prepared','secret_applied');

INSERT INTO plugin_idc_notify_channel
    ("tenant_id", "channel_code", "channel_kind", "display_name", "status_code", "balance_state", non_secret_configuration)
VALUES
    (0, 'in_app', 'in_app', 'In-app message', 'enabled', 'sufficient', '{}'::jsonb),
    (0, 'email', 'email', 'Email', 'unconfigured', 'sufficient', '{}'::jsonb),
    (0, 'sms', 'sms', 'SMS', 'unconfigured', 'unknown', '{}'::jsonb),
    (0, 'wechat_official', 'wechat_official', 'WeChat Official Account', 'unconfigured', 'sufficient', '{}'::jsonb),
    (0, 'signed_webhook', 'signed_webhook', 'Signed Webhook', 'unconfigured', 'sufficient', '{}'::jsonb),
    (0, 'dingtalk', 'dingtalk', 'DingTalk', 'unconfigured', 'sufficient', '{}'::jsonb),
    (0, 'feishu', 'feishu', 'Feishu', 'unconfigured', 'sufficient', '{}'::jsonb)
ON CONFLICT ("tenant_id", "channel_code") DO NOTHING;

UPDATE plugin_idc_notify_channel
SET "balance_state" = 'sufficient'
WHERE "tenant_id" = 0
  AND "channel_code" IN ('in_app','email','wechat_official','signed_webhook','dingtalk','feishu')
  AND "balance_state" = 'unknown';

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id", "event_code", "topic_code", "object_type", required_variables, optional_variables, security_required)
VALUES
    (0, 'account.password_reset', 'security', 'account', '["reset_url","expires_minutes"]'::jsonb, '["display_name"]'::jsonb, TRUE),
    (0, 'support.ticket_updated', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["status","message"]'::jsonb, FALSE),
    (0, 'service.expiry_reminder', 'service', 'service', '["service_no","expires_at"]'::jsonb, '["service_name"]'::jsonb, FALSE),
    (0, 'billing.payment_result', 'billing', 'billing', '["invoice_no","status"]'::jsonb, '["amount","currency"]'::jsonb, FALSE),
    (0, 'marketing.campaign', 'marketing', 'marketing_campaign', '["campaign_name"]'::jsonb, '["unsubscribe_url"]'::jsonb, FALSE)
ON CONFLICT ("tenant_id", "event_code") DO NOTHING;
