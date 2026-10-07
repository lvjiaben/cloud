-- Purpose: Stores Support Upstream Channel records.
-- 用途：保存支持上游渠道相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_channel (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(40) NOT NULL DEFAULT 'legacy-finance-ticket',
    "base_url" VARCHAR(2048) NOT NULL,
    "auth_mode" VARCHAR(32) NOT NULL,
    "credential_secret_ref" VARCHAR(256) NOT NULL,
    "credential_masked_tail" VARCHAR(16) NOT NULL DEFAULT '',
    "credential_updated_at" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'disabled',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_upstream_channel UNIQUE ("tenant_id", "channel_no"),
    CONSTRAINT ck_idc_support_upstream_channel_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_support_upstream_auth_mode CHECK ("auth_mode" IN ('zjmf_api_login','resource_login')),
    CONSTRAINT ck_idc_support_upstream_status CHECK ("status_code" IN ('enabled','disabled')),
    CONSTRAINT ck_idc_support_upstream_channel_version CHECK ("version">0)
);

-- Purpose: Stores Support Upstream Service Map records.
-- 用途：保存支持上游服务映射相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_service_map (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "service_no" VARCHAR(40) NOT NULL,
    "upstream_service_ref" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_upstream_service_map UNIQUE("tenant_id","service_no"),
    CONSTRAINT ck_idc_support_upstream_service_tenant CHECK("tenant_id"=0),
    CONSTRAINT ck_idc_support_upstream_service_status CHECK("status_code" IN ('active','disabled')),
    CONSTRAINT ck_idc_support_upstream_service_version CHECK("version">0)
);

-- Purpose: Stores Support Upstream Department Map records.
-- 用途：保存支持上游部门映射相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_department_map (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "department_no" VARCHAR(40) NOT NULL,
    "upstream_department_id" BIGINT NOT NULL,
    "upstream_department_name" VARCHAR(128) NOT NULL DEFAULT '',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_upstream_department_map UNIQUE ("tenant_id","department_no"),
    CONSTRAINT ck_idc_support_upstream_department_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_support_upstream_department_id CHECK ("upstream_department_id">0),
    CONSTRAINT ck_idc_support_upstream_department_status CHECK ("status_code" IN ('active','disabled')),
    CONSTRAINT ck_idc_support_upstream_department_version CHECK ("version">0),
    CONSTRAINT fk_idc_support_upstream_department_local FOREIGN KEY ("department_no") REFERENCES plugin_idc_core_support_department("department_no")
);

ALTER TABLE plugin_idc_core_support_ticket_upstream_forward
    ADD COLUMN IF NOT EXISTS "operation_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "request_digest" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "upstream_department_id" BIGINT,
    ADD COLUMN IF NOT EXISTS "service_upstream_ref" VARCHAR(128),
    ADD COLUMN IF NOT EXISTS "attempt_count" INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "last_attempt_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "next_query_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "confirmed_reason" VARCHAR(512),
    ADD COLUMN IF NOT EXISTS "last_evidence_code" VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_support_upstream_operation_no
    ON plugin_idc_core_support_ticket_upstream_forward("operation_no") WHERE "operation_no" IS NOT NULL;

-- Purpose: Stores Support Upstream Attempt records.
-- 用途：保存支持上游尝试相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_attempt (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "attempt_no" VARCHAR(64) NOT NULL,
    "forward_no" VARCHAR(40) NOT NULL,
    "attempt_seq" INT NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "result_classification" VARCHAR(32) NOT NULL,
    "http_status" INT,
    "evidence_code" VARCHAR(64) NOT NULL,
    "provider_ticket_reference" VARCHAR(128),
    "started_at" TIMESTAMPTZ NOT NULL,
    "finished_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT uk_idc_support_upstream_attempt_no UNIQUE("attempt_no"),
    CONSTRAINT uk_idc_support_upstream_attempt_seq UNIQUE("forward_no","attempt_seq"),
    CONSTRAINT ck_idc_support_upstream_attempt_tenant CHECK("tenant_id"=0),
    CONSTRAINT ck_idc_support_upstream_attempt_action CHECK("action_code" IN ('create','retry','query','sync_reply')),
    CONSTRAINT ck_idc_support_upstream_attempt_result CHECK("result_classification" IN ('accepted','definitive_not_received','result_unknown')),
    CONSTRAINT fk_idc_support_upstream_attempt_forward FOREIGN KEY("forward_no") REFERENCES plugin_idc_core_support_ticket_upstream_forward("forward_no")
);

-- Purpose: Stores Support Upstream Reply records.
-- 用途：保存支持上游回复相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_reply (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "sync_no" VARCHAR(64) NOT NULL,
    "forward_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "provider_reply_reference" VARCHAR(128) NOT NULL,
    "upstream_content_digest" VARCHAR(64) NOT NULL,
    "local_message_no" VARCHAR(40) NOT NULL,
    "synced_at" TIMESTAMPTZ NOT NULL,
    "published_message_no" VARCHAR(40),
    "published_by" BIGINT,
    "published_at" TIMESTAMPTZ,
    CONSTRAINT uk_idc_support_upstream_reply_sync UNIQUE("sync_no"),
    CONSTRAINT uk_idc_support_upstream_reply_provider UNIQUE("forward_no","provider_reply_reference"),
    CONSTRAINT ck_idc_support_upstream_reply_tenant CHECK("tenant_id"=0),
    CONSTRAINT fk_idc_support_upstream_reply_forward FOREIGN KEY("forward_no") REFERENCES plugin_idc_core_support_ticket_upstream_forward("forward_no"),
    CONSTRAINT fk_idc_support_upstream_reply_ticket FOREIGN KEY("ticket_no") REFERENCES plugin_idc_core_support_ticket("ticket_no")
);
