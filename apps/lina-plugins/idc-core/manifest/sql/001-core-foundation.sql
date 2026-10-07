-- 001: IDC core foundation
--
-- This migration establishes the single-merchant identity, customer,
-- reliability, idempotency, and audit invariants used by later domains.

-- The installation owns exactly one merchant profile in tenant 0.
-- Purpose: Single IDC merchant operating context for this installation.
-- 用途：保存商家资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_merchant_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "merchant_no" VARCHAR(40) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL DEFAULT '',
    "legal_name" VARCHAR(256) NOT NULL DEFAULT '',
    "country_code" CHAR(2) NOT NULL DEFAULT 'CN',
    "default_locale" VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    "default_currency" CHAR(3) NOT NULL DEFAULT 'CNY',
    "timezone" VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_merchant_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_plugin_idc_core_merchant_tenant UNIQUE ("tenant_id"),
    CONSTRAINT uk_plugin_idc_core_merchant_no UNIQUE ("merchant_no")
);

COMMENT ON TABLE plugin_idc_core_merchant_profile IS 'Single IDC merchant operating context for this installation';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."tenant_id" IS 'Host tenant context; the single-merchant edition requires tenant 0';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."merchant_no" IS 'Opaque public merchant business number';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."display_name" IS 'Merchant display name';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."legal_name" IS 'Merchant legal entity name';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."country_code" IS 'ISO 3166-1 alpha-2 country code';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."default_locale" IS 'Default IETF locale tag';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."default_currency" IS 'Default ISO 4217 currency code';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."timezone" IS 'Default IANA time zone';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."enabled" IS 'Whether the merchant operating context accepts normal business traffic';
COMMENT ON COLUMN plugin_idc_core_merchant_profile."version" IS 'Optimistic concurrency version';

-- One host identity can have exactly one mutually exclusive audience.
-- Purpose: Mutually exclusive customer or staff audience bound to a host identity.
-- 用途：保存主体受众相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_principal_audience (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "sys_user_id" INT NOT NULL,
    "audience_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "bound_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_audience_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_audience_code CHECK ("audience_code" IN ('customer', 'staff')),
    CONSTRAINT ck_plugin_idc_core_audience_status CHECK ("status_code" IN ('active', 'revoked')),
    CONSTRAINT uk_plugin_idc_core_audience_user UNIQUE ("sys_user_id")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audience_scope
    ON plugin_idc_core_principal_audience ("tenant_id", "audience_code", "status_code", "id");

COMMENT ON TABLE plugin_idc_core_principal_audience IS 'Mutually exclusive customer or staff audience bound to a host identity';
COMMENT ON COLUMN plugin_idc_core_principal_audience."sys_user_id" IS 'Host sys_user identity ID validated through host capabilities';
COMMENT ON COLUMN plugin_idc_core_principal_audience."audience_code" IS 'Audience code: customer or staff';
COMMENT ON COLUMN plugin_idc_core_principal_audience."status_code" IS 'Binding status code: active or revoked';
COMMENT ON COLUMN plugin_idc_core_principal_audience."bound_at" IS 'Time when the audience binding became active';
COMMENT ON COLUMN plugin_idc_core_principal_audience."revoked_at" IS 'Time when the audience binding was revoked';

-- Customer subjects remain visible to historical business records and are
-- anonymized on approved closure instead of being soft-deleted.
-- Purpose: Personal or enterprise customer business subject.
-- 用途：保存客户主体相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_subject (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'pending_activation',
    "display_name" VARCHAR(256) NOT NULL DEFAULT '',
    "country_code" CHAR(2) NOT NULL DEFAULT 'CN',
    "preferred_locale" VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    "version" BIGINT NOT NULL DEFAULT 1,
    "anonymized_at" TIMESTAMPTZ,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_customer_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_customer_type CHECK ("customer_type_code" IN ('individual', 'enterprise')),
    CONSTRAINT ck_plugin_idc_core_customer_status CHECK ("status_code" IN ('pending_activation', 'active', 'restricted', 'disabled', 'anonymized')),
    CONSTRAINT uk_plugin_idc_core_customer_no UNIQUE ("customer_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_status
    ON plugin_idc_core_customer_subject ("tenant_id", "status_code", "created_at" DESC, "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_type
    ON plugin_idc_core_customer_subject ("tenant_id", "customer_type_code", "status_code", "id");

COMMENT ON TABLE plugin_idc_core_customer_subject IS 'Personal or enterprise customer business subject';
COMMENT ON COLUMN plugin_idc_core_customer_subject."customer_no" IS 'Opaque public customer business number';
COMMENT ON COLUMN plugin_idc_core_customer_subject."customer_type_code" IS 'Customer type code: individual or enterprise';
COMMENT ON COLUMN plugin_idc_core_customer_subject."status_code" IS 'Customer lifecycle status code';
COMMENT ON COLUMN plugin_idc_core_customer_subject."display_name" IS 'Customer display name without verified legal identity evidence';
COMMENT ON COLUMN plugin_idc_core_customer_subject."preferred_locale" IS 'Preferred IETF locale tag';
COMMENT ON COLUMN plugin_idc_core_customer_subject."anonymized_at" IS 'Time when approved account closure anonymization completed';

-- Public onboarding tokens and browser nonces are persisted only as digests.
-- Purpose: Short-lived single-use customer onboarding intent.
-- 用途：保存客户开户意图相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_onboarding_intent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "intent_no" VARCHAR(40) NOT NULL,
    "token_digest" CHAR(64) NOT NULL,
    "browser_nonce_digest" CHAR(64) NOT NULL,
    "normalized_username_hash" CHAR(64) NOT NULL,
    "contact_channel_code" VARCHAR(16) NOT NULL,
    "contact_hash" CHAR(64) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "return_path" VARCHAR(512) NOT NULL DEFAULT '/account',
    "host_registration_user_id" INT,
    "consumed_by_user_id" INT,
    "result_customer_no" VARCHAR(40) NOT NULL DEFAULT '',
    "verified_contact_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_onboarding_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_onboarding_channel CHECK ("contact_channel_code" IN ('email', 'phone')),
    CONSTRAINT ck_plugin_idc_core_onboarding_type CHECK ("customer_type_code" IN ('individual', 'enterprise')),
    CONSTRAINT ck_plugin_idc_core_onboarding_status CHECK ("status_code" IN ('pending', 'verified', 'consumed', 'expired', 'cancelled')),
    CONSTRAINT ck_plugin_idc_core_onboarding_attempt CHECK ("attempt_count" >= 0),
    CONSTRAINT uk_plugin_idc_core_onboarding_no UNIQUE ("intent_no"),
    CONSTRAINT uk_plugin_idc_core_onboarding_token UNIQUE ("token_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_onboarding_pending
    ON plugin_idc_core_customer_onboarding_intent ("tenant_id", "status_code", "expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_onboarding_contact
    ON plugin_idc_core_customer_onboarding_intent ("tenant_id", "contact_hash", "created_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_onboarding_user
    ON plugin_idc_core_customer_onboarding_intent ("tenant_id", "host_registration_user_id", "status_code")
    WHERE "host_registration_user_id" IS NOT NULL;

COMMENT ON TABLE plugin_idc_core_customer_onboarding_intent IS 'Short-lived single-use customer onboarding intent';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."intent_no" IS 'Opaque onboarding intent business number';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."token_digest" IS 'SHA-256 digest of the single-use opaque token';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."browser_nonce_digest" IS 'SHA-256 digest of the browser-bound nonce';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."normalized_username_hash" IS 'Digest of the normalized requested username';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."contact_hash" IS 'Digest of the verified contact destination';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."return_path" IS 'Validated same-origin path used after onboarding';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."host_registration_user_id" IS 'Host identity returned by native registration';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."result_customer_no" IS 'Customer business number created by successful redemption';
COMMENT ON COLUMN plugin_idc_core_customer_onboarding_intent."expires_at" IS 'Intent expiry time';

-- The first release permits one host identity for one customer subject.
-- Purpose: Exclusive link between a host identity and one customer subject.
-- 用途：保存客户身份绑定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_identity_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "binding_no" VARCHAR(40) NOT NULL,
    "sys_user_id" INT NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL,
    "source_reference_no" VARCHAR(64) NOT NULL DEFAULT '',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "activated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_binding_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_binding_source CHECK ("source_code" IN ('onboarding', 'admin_invitation', 'migration')),
    CONSTRAINT ck_plugin_idc_core_binding_status CHECK ("status_code" IN ('active', 'revoked')),
    CONSTRAINT uk_plugin_idc_core_binding_no UNIQUE ("binding_no"),
    CONSTRAINT uk_plugin_idc_core_binding_user UNIQUE ("sys_user_id"),
    CONSTRAINT uk_plugin_idc_core_binding_customer UNIQUE ("customer_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_binding_scope
    ON plugin_idc_core_customer_identity_binding ("tenant_id", "status_code", "customer_no");

COMMENT ON TABLE plugin_idc_core_customer_identity_binding IS 'Exclusive link between a host identity and one customer subject';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."sys_user_id" IS 'Host sys_user identity ID validated through host capabilities';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."customer_no" IS 'Bound customer business number';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."source_code" IS 'Binding source: onboarding, admin invitation, or migration';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."source_reference_no" IS 'Opaque onboarding, invitation, or migration reference';

-- Business numbers are random opaque identifiers reserved idempotently.
-- Purpose: Idempotent reservation ledger for opaque public business numbers.
-- 用途：保存业务编号相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_business_number (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "number_type" VARCHAR(64) NOT NULL,
    "business_no" VARCHAR(64) NOT NULL,
    "reservation_scope_digest" CHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "owner_type" VARCHAR(64) NOT NULL DEFAULT '',
    "owner_internal_id" BIGINT,
    "assigned_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_number_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_plugin_idc_core_business_no UNIQUE ("business_no"),
    CONSTRAINT uk_plugin_idc_core_number_request UNIQUE ("tenant_id", "number_type", "reservation_scope_digest", "idempotency_key_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_number_type
    ON plugin_idc_core_business_number ("tenant_id", "number_type", "created_at" DESC, "id");

COMMENT ON TABLE plugin_idc_core_business_number IS 'Idempotent reservation ledger for opaque public business numbers';
COMMENT ON COLUMN plugin_idc_core_business_number."number_type" IS 'Stable business number namespace';
COMMENT ON COLUMN plugin_idc_core_business_number."business_no" IS 'Cryptographically random opaque business number';
COMMENT ON COLUMN plugin_idc_core_business_number."reservation_scope_digest" IS 'Digest of the normalized caller or command scope';
COMMENT ON COLUMN plugin_idc_core_business_number."idempotency_key_digest" IS 'SHA-256 digest of the number reservation idempotency key';
COMMENT ON COLUMN plugin_idc_core_business_number."owner_type" IS 'Owning aggregate type after assignment';
COMMENT ON COLUMN plugin_idc_core_business_number."owner_internal_id" IS 'Internal owner ID that is never exposed across plugins';

-- Outbox events are written in the same transaction as authoritative state.
-- Purpose: Transactional outbox event awaiting reliable asynchronous delivery.
-- 用途：保存发件箱事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_outbox_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "event_type" VARCHAR(128) NOT NULL,
    "event_version" INT NOT NULL DEFAULT 1,
    "topic" VARCHAR(128) NOT NULL DEFAULT '',
    "aggregate_type" VARCHAR(64) NOT NULL,
    "aggregate_no" VARCHAR(64) NOT NULL,
    "correlation_no" VARCHAR(64) NOT NULL DEFAULT '',
    "deduplication_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "payload" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "metadata" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "max_attempts" INT NOT NULL DEFAULT 12,
    "published_at" TIMESTAMPTZ,
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "last_error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_outbox_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_outbox_version CHECK ("event_version" > 0),
    CONSTRAINT ck_plugin_idc_core_outbox_status CHECK ("status_code" IN ('pending', 'processing', 'retry_wait', 'published', 'dead_letter', 'result_unknown')),
    CONSTRAINT ck_plugin_idc_core_outbox_attempt CHECK ("attempt_count" >= 0 AND "max_attempts" > 0),
    CONSTRAINT uk_plugin_idc_core_outbox_no UNIQUE ("event_no"),
    CONSTRAINT uk_plugin_idc_core_outbox_dedupe UNIQUE ("tenant_id", "event_type", "deduplication_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_outbox_claim
    ON plugin_idc_core_outbox_event ("tenant_id", "status_code", "available_at", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_outbox_aggregate
    ON plugin_idc_core_outbox_event ("tenant_id", "aggregate_type", "aggregate_no", "created_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_outbox_lease
    ON plugin_idc_core_outbox_event ("tenant_id", "status_code", "lease_expires_at")
    WHERE "lease_expires_at" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_outbox_correlation
    ON plugin_idc_core_outbox_event ("tenant_id", "correlation_no", "created_at" DESC)
    WHERE "correlation_no" <> '';

COMMENT ON TABLE plugin_idc_core_outbox_event IS 'Transactional outbox event awaiting reliable asynchronous delivery';
COMMENT ON COLUMN plugin_idc_core_outbox_event."event_no" IS 'Opaque outbox event business number';
COMMENT ON COLUMN plugin_idc_core_outbox_event."event_version" IS 'Positive version of the published event contract';
COMMENT ON COLUMN plugin_idc_core_outbox_event."topic" IS 'Stable logical delivery topic';
COMMENT ON COLUMN plugin_idc_core_outbox_event."correlation_no" IS 'Opaque business correlation number';
COMMENT ON COLUMN plugin_idc_core_outbox_event."deduplication_digest" IS 'Digest that prevents duplicate events of the same type';
COMMENT ON COLUMN plugin_idc_core_outbox_event."payload" IS 'Versioned redacted event payload without secrets';
COMMENT ON COLUMN plugin_idc_core_outbox_event."metadata" IS 'Versioned delivery metadata without secrets';
COMMENT ON COLUMN plugin_idc_core_outbox_event."available_at" IS 'Earliest time at which the event may be claimed';
COMMENT ON COLUMN plugin_idc_core_outbox_event."lease_expires_at" IS 'Claim lease expiry used for crash recovery';

-- Business tasks track local asynchronous orchestration without holding
-- external calls inside a checkout or state-change transaction.
-- Purpose: Reliable asynchronous business task and recovery state.
-- 用途：保存异步任务相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_async_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "task_type" VARCHAR(128) NOT NULL,
    "subject_type" VARCHAR(64) NOT NULL,
    "subject_no" VARCHAR(64) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL DEFAULT '',
    "external_reference_no" VARCHAR(128) NOT NULL DEFAULT '',
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "priority" SMALLINT NOT NULL DEFAULT 0,
    "payload" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "result_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deadline_at" TIMESTAMPTZ,
    "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "max_attempts" INT NOT NULL DEFAULT 12,
    "completed_at" TIMESTAMPTZ,
    "manual_takeover_at" TIMESTAMPTZ,
    "manual_assignee_user_id" INT,
    "manual_reason" VARCHAR(1024) NOT NULL DEFAULT '',
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "last_error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_task_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_task_status CHECK ("status_code" IN ('pending', 'running', 'retry_wait', 'succeeded', 'failed', 'dead_letter', 'result_unknown', 'manual_takeover')),
    CONSTRAINT ck_plugin_idc_core_task_attempt CHECK ("attempt_count" >= 0 AND "max_attempts" > 0),
    CONSTRAINT uk_plugin_idc_core_task_no UNIQUE ("task_no"),
    CONSTRAINT uk_plugin_idc_core_task_request UNIQUE ("tenant_id", "task_type", "subject_type", "subject_no", "idempotency_key_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_task_claim
    ON plugin_idc_core_async_task ("tenant_id", "status_code", "available_at", "priority" DESC, "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_task_subject
    ON plugin_idc_core_async_task ("tenant_id", "subject_type", "subject_no", "created_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_task_customer
    ON plugin_idc_core_async_task ("tenant_id", "customer_no", "created_at" DESC, "id")
    WHERE "customer_no" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_task_lease
    ON plugin_idc_core_async_task ("tenant_id", "status_code", "lease_expires_at")
    WHERE "lease_expires_at" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_task_external
    ON plugin_idc_core_async_task ("tenant_id", "external_reference_no")
    WHERE "external_reference_no" <> '';

COMMENT ON TABLE plugin_idc_core_async_task IS 'Reliable asynchronous business task and recovery state';
COMMENT ON COLUMN plugin_idc_core_async_task."task_no" IS 'Opaque task business number';
COMMENT ON COLUMN plugin_idc_core_async_task."customer_no" IS 'Validated customer ownership projection for database-side permission filtering';
COMMENT ON COLUMN plugin_idc_core_async_task."external_reference_no" IS 'Current normalized external task or operation reference';
COMMENT ON COLUMN plugin_idc_core_async_task."idempotency_key_digest" IS 'SHA-256 digest of the task idempotency key';
COMMENT ON COLUMN plugin_idc_core_async_task."payload" IS 'Versioned redacted task input without secrets';
COMMENT ON COLUMN plugin_idc_core_async_task."result_snapshot" IS 'Redacted normalized result used for status and recovery';
COMMENT ON COLUMN plugin_idc_core_async_task."manual_takeover_at" IS 'Time when automated execution yielded to an operator';
COMMENT ON COLUMN plugin_idc_core_async_task."manual_assignee_user_id" IS 'Host staff identity assigned to manual takeover';
COMMENT ON COLUMN plugin_idc_core_async_task."manual_reason" IS 'Redacted operator-facing reason for manual takeover';

-- Purpose: Append-oriented execution attempt evidence for an asynchronous task.
-- 用途：保存异步任务尝试相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_async_task_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "worker_id" VARCHAR(128) NOT NULL DEFAULT '',
    "external_reference_no" VARCHAR(128) NOT NULL DEFAULT '',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'running',
    "request_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "response_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ended_at" TIMESTAMPTZ,
    "error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "error_summary" VARCHAR(1024) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_attempt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_attempt_number CHECK ("attempt_no" > 0),
    CONSTRAINT ck_plugin_idc_core_attempt_status CHECK ("status_code" IN ('running', 'succeeded', 'failed', 'result_unknown')),
    CONSTRAINT uk_plugin_idc_core_attempt_number UNIQUE ("task_no", "attempt_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_attempt_task
    ON plugin_idc_core_async_task_attempt ("tenant_id", "task_no", "attempt_no" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_attempt_status
    ON plugin_idc_core_async_task_attempt ("tenant_id", "status_code", "started_at" DESC, "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_attempt_external
    ON plugin_idc_core_async_task_attempt ("tenant_id", "external_reference_no")
    WHERE "external_reference_no" <> '';

COMMENT ON TABLE plugin_idc_core_async_task_attempt IS 'Append-oriented execution attempt evidence for an asynchronous task';
COMMENT ON COLUMN plugin_idc_core_async_task_attempt."attempt_no" IS 'Monotonic attempt number within one task';
COMMENT ON COLUMN plugin_idc_core_async_task_attempt."external_reference_no" IS 'External reference returned for this execution attempt';
COMMENT ON COLUMN plugin_idc_core_async_task_attempt."request_snapshot" IS 'Redacted normalized request evidence';
COMMENT ON COLUMN plugin_idc_core_async_task_attempt."response_snapshot" IS 'Redacted normalized response evidence';

-- Idempotency records safely converge repeated and concurrent commands.
-- Purpose: Request idempotency state and replay-safe normalized response.
-- 用途：保存幂等记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_idempotency_record (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "scope" VARCHAR(128) NOT NULL,
    "caller_scope_digest" CHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'in_progress',
    "lock_owner" VARCHAR(128) NOT NULL DEFAULT '',
    "lock_expires_at" TIMESTAMPTZ,
    "resource_type" VARCHAR(64) NOT NULL DEFAULT '',
    "resource_no" VARCHAR(64) NOT NULL DEFAULT '',
    "response_status_code" INT,
    "response_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_idempotency_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_idempotency_status CHECK ("status_code" IN ('in_progress', 'succeeded', 'failed', 'result_unknown')),
    CONSTRAINT uk_plugin_idc_core_idempotency_key UNIQUE ("tenant_id", "scope", "caller_scope_digest", "idempotency_key_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_idempotency_expiry
    ON plugin_idc_core_idempotency_record ("tenant_id", "status_code", "expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_idempotency_resource
    ON plugin_idc_core_idempotency_record ("tenant_id", "resource_type", "resource_no", "created_at" DESC)
    WHERE "resource_type" <> '' AND "resource_no" <> '';

COMMENT ON TABLE plugin_idc_core_idempotency_record IS 'Request idempotency state and replay-safe normalized response';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."scope" IS 'Stable command or endpoint idempotency namespace';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."caller_scope_digest" IS 'Digest of the normalized customer, staff, channel, or system caller scope';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."idempotency_key_digest" IS 'SHA-256 digest of the caller idempotency key';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."request_fingerprint" IS 'SHA-256 digest of canonical request semantics';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."response_snapshot" IS 'Redacted normalized response used for safe replay';
COMMENT ON COLUMN plugin_idc_core_idempotency_record."expires_at" IS 'Earliest time at which retention cleanup may evaluate the record';

-- Business audit events are append-only evidence. Sensitive before and after
-- snapshots must be redacted before insertion.
-- Purpose: Append-only redacted business audit evidence.
-- 用途：保存业务审计事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_business_audit_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "audit_no" VARCHAR(40) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "actor_type" VARCHAR(32) NOT NULL,
    "actor_user_id" INT,
    "acting_user_id" INT,
    "actor_audience_code" VARCHAR(16) NOT NULL DEFAULT '',
    "actor_customer_no" VARCHAR(40) NOT NULL DEFAULT '',
    "customer_no" VARCHAR(40) NOT NULL DEFAULT '',
    "operation_code" VARCHAR(128) NOT NULL,
    "target_type" VARCHAR(64) NOT NULL,
    "target_no" VARCHAR(64) NOT NULL,
    "result_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(128) NOT NULL DEFAULT '',
    "risk_code" VARCHAR(16) NOT NULL DEFAULT 'low',
    "before_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "after_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "context_snapshot" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "request_id" VARCHAR(128) NOT NULL DEFAULT '',
    "trace_id" VARCHAR(128) NOT NULL DEFAULT '',
    "correlation_no" VARCHAR(64) NOT NULL DEFAULT '',
    "ip_digest" CHAR(64) NOT NULL DEFAULT '',
    "user_agent_digest" CHAR(64) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_audit_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_audit_actor_type CHECK ("actor_type" IN ('user', 'system', 'external')),
    CONSTRAINT ck_plugin_idc_core_audit_audience CHECK ("actor_audience_code" IN ('', 'customer', 'staff')),
    CONSTRAINT ck_plugin_idc_core_audit_result CHECK ("result_code" IN ('succeeded', 'denied', 'failed', 'result_unknown')),
    CONSTRAINT ck_plugin_idc_core_audit_risk CHECK ("risk_code" IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT uk_plugin_idc_core_audit_no UNIQUE ("audit_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_timeline
    ON plugin_idc_core_business_audit_event ("tenant_id", "occurred_at" DESC, "id" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_target
    ON plugin_idc_core_business_audit_event ("tenant_id", "target_type", "target_no", "occurred_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_actor
    ON plugin_idc_core_business_audit_event ("tenant_id", "actor_user_id", "occurred_at" DESC)
    WHERE "actor_user_id" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_acting
    ON plugin_idc_core_business_audit_event ("tenant_id", "acting_user_id", "occurred_at" DESC)
    WHERE "acting_user_id" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_actor_customer
    ON plugin_idc_core_business_audit_event ("tenant_id", "actor_customer_no", "occurred_at" DESC, "id" DESC)
    WHERE "actor_customer_no" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_customer
    ON plugin_idc_core_business_audit_event ("tenant_id", "customer_no", "occurred_at" DESC, "id" DESC)
    WHERE "customer_no" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_correlation
    ON plugin_idc_core_business_audit_event ("tenant_id", "correlation_no", "occurred_at" DESC)
    WHERE "correlation_no" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_operation
    ON plugin_idc_core_business_audit_event ("tenant_id", "operation_code", "occurred_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_result
    ON plugin_idc_core_business_audit_event ("tenant_id", "result_code", "occurred_at" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_risk
    ON plugin_idc_core_business_audit_event ("tenant_id", "risk_code", "occurred_at" DESC);

COMMENT ON TABLE plugin_idc_core_business_audit_event IS 'Append-only redacted business audit evidence';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."audit_no" IS 'Opaque audit event business number';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."actor_type" IS 'Authenticated user, system, or external actor type';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."actor_user_id" IS 'Effective host identity responsible for the action';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."acting_user_id" IS 'Actual host identity acting through assistance or delegation';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."actor_customer_no" IS 'Customer subject represented by the actor when the actor audience is customer';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."customer_no" IS 'Validated customer subject affected by the audited operation';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."operation_code" IS 'Stable audited business operation code';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."target_no" IS 'Opaque target business number';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."before_snapshot" IS 'Redacted state before the operation';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."after_snapshot" IS 'Redacted state after the operation';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."context_snapshot" IS 'Redacted permission, reason, and risk context';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."ip_digest" IS 'Keyed digest of the source IP when policy permits';
COMMENT ON COLUMN plugin_idc_core_business_audit_event."user_agent_digest" IS 'Digest of the user agent when policy permits';

-- Stable enum values are registered in the host dictionary module. The host
-- keeps database labels for its default zh-CN locale, while plugin i18n
-- resources provide both zh-CN ownership and en-US projections.
INSERT INTO sys_dict_type ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 身份受众', 'idc_principal_audience', 1, 1, FALSE, '互斥的客户与员工身份受众', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 绑定状态', 'idc_binding_status', 1, 1, FALSE, '身份与客户绑定生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户类型', 'idc_customer_type', 1, 1, FALSE, '个人与企业客户类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户状态', 'idc_customer_status', 1, 1, FALSE, '客户主体生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 联系渠道', 'idc_contact_channel', 1, 1, FALSE, '开户时已验证的联系渠道', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 开户状态', 'idc_onboarding_status', 1, 1, FALSE, '客户开户意图生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 绑定来源', 'idc_binding_source', 1, 1, FALSE, '客户身份绑定来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 事务发件箱状态', 'idc_outbox_status', 1, 1, FALSE, '事务发件箱生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 异步任务状态', 'idc_async_task_status', 1, 1, FALSE, '异步业务任务生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 任务尝试状态', 'idc_task_attempt_status', 1, 1, FALSE, '异步任务执行结果', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 幂等状态', 'idc_idempotency_status', 1, 1, FALSE, '请求幂等生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 审计操作者类型', 'idc_audit_actor_type', 1, 1, FALSE, '业务审计操作者类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 审计结果', 'idc_audit_result', 1, 1, FALSE, '业务审计操作结果', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 审计风险', 'idc_audit_risk', 1, 1, FALSE, '业务审计风险等级', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_principal_audience', '客户', 'customer', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_principal_audience', '员工', 'staff', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_binding_status', '生效', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_binding_status', '已撤销', 'revoked', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_type', '个人', 'individual', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_type', '企业', 'enterprise', 2, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_status', '待激活', 'pending_activation', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_status', '正常', 'active', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_status', '受限', 'restricted', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_status', '停用', 'disabled', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_status', '已匿名化', 'anonymized', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_channel', '电子邮箱', 'email', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_channel', '手机', 'phone', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_onboarding_status', '待处理', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_onboarding_status', '已验证', 'verified', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_onboarding_status', '已使用', 'consumed', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_onboarding_status', '已过期', 'expired', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_onboarding_status', '已取消', 'cancelled', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_binding_source', '门户开户', 'onboarding', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_binding_source', '管理员邀请', 'admin_invitation', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_binding_source', '数据迁移', 'migration', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '待处理', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '处理中', 'processing', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '等待重试', 'retry_wait', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '已发布', 'published', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '死信', 'dead_letter', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_outbox_status', '结果未知', 'result_unknown', 6, 'orange', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '待处理', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '执行中', 'running', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '等待重试', 'retry_wait', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '成功', 'succeeded', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '失败', 'failed', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '死信', 'dead_letter', 6, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '结果未知', 'result_unknown', 7, 'orange', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_async_task_status', '人工接管', 'manual_takeover', 8, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_task_attempt_status', '执行中', 'running', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_task_attempt_status', '成功', 'succeeded', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_task_attempt_status', '失败', 'failed', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_task_attempt_status', '结果未知', 'result_unknown', 4, 'orange', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_idempotency_status', '处理中', 'in_progress', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_idempotency_status', '成功', 'succeeded', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_idempotency_status', '失败', 'failed', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_idempotency_status', '结果未知', 'result_unknown', 4, 'orange', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_actor_type', '用户', 'user', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_actor_type', '系统', 'system', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_actor_type', '外部调用方', 'external', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_result', '成功', 'succeeded', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_result', '已拒绝', 'denied', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_result', '失败', 'failed', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_result', '结果未知', 'result_unknown', 4, 'orange', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_risk', '低', 'low', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_risk', '中', 'medium', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_risk', '高', 'high', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_audit_risk', '严重', 'critical', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
