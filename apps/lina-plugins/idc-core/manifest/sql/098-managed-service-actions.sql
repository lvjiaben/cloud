-- 098: Durable, provider-version-frozen actions for existing long-lived services.
-- Provider payloads remain bounded JSON. Certificate bytes are kept only in
-- private plugin storage and are released through a short-lived, one-use ticket.

-- Purpose: Store durable, version-frozen actions for existing managed services.
-- 用途：保存现有受管服务的持久且版本冻结的操作任务。
CREATE TABLE IF NOT EXISTS plugin_idc_core_managed_service_action_task (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "id" BIGSERIAL UNIQUE,
    "task_no" VARCHAR(40) PRIMARY KEY,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "action_key" VARCHAR(64) NOT NULL,
    "audience_code" VARCHAR(16) NOT NULL,
    "actor_user_id" BIGINT NOT NULL,
    "idempotency_key" VARCHAR(256) NOT NULL,
    "input_snapshot" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "input_digest" CHAR(64) NOT NULL,
    "plugin_id" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "contract_version" VARCHAR(32) NOT NULL,
    "release_digest" CHAR(64) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "interface_configuration_revision" BIGINT NOT NULL,
    "source_intent_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "external_reference" VARCHAR(256) NOT NULL,
    "action_method" VARCHAR(128) NOT NULL,
    "query_method" VARCHAR(128) NOT NULL,
    "query_action_key" VARCHAR(64) NOT NULL,
    "confirmed_entitlement" VARCHAR(24) NOT NULL DEFAULT '',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_query',
    "phase_code" VARCHAR(24) NOT NULL DEFAULT 'query_first',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "query_attempt_count" INT NOT NULL DEFAULT 0,
    "submit_attempt_count" INT NOT NULL DEFAULT 0,
    "lease_token" VARCHAR(64) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "safe_projection" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "last_result_code" VARCHAR(64) NOT NULL DEFAULT '',
    "takeover_requested" BOOLEAN NOT NULL DEFAULT FALSE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_managed_service_action_idempotency UNIQUE ("customer_no", "entitlement_no", "action_key", "idempotency_key"),
    CONSTRAINT ck_idc_managed_service_action_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_managed_service_action_audience CHECK ("audience_code" IN ('customer','admin')),
    CONSTRAINT ck_idc_managed_service_action_status CHECK ("status_code" IN ('pending_query','pending_submit','querying','submitting','retry_wait','succeeded','failed','result_unknown','manual_attention')),
    CONSTRAINT ck_idc_managed_service_action_phase CHECK ("phase_code" IN ('query_first','submit_allowed','post_submit_query','final')),
    CONSTRAINT ck_idc_managed_service_action_entitlement CHECK ("confirmed_entitlement" IN ('','cancelled')),
    CONSTRAINT ck_idc_managed_service_action_attempts CHECK ("attempt_count" BETWEEN 0 AND 13 AND "query_attempt_count" BETWEEN 0 AND 10 AND "submit_attempt_count" BETWEEN 0 AND 3),
    CONSTRAINT ck_idc_managed_service_action_digests CHECK ("input_digest" ~ '^[0-9a-f]{64}$' AND "release_digest" ~ '^[0-9a-f]{64}$' AND "declaration_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_managed_service_action_entitlement FOREIGN KEY("entitlement_no") REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_managed_service_action_intent FOREIGN KEY("source_intent_no") REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_managed_service_action_task
    ADD COLUMN IF NOT EXISTS "confirmed_entitlement" VARCHAR(24) NOT NULL DEFAULT '';

DO $constraint$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_managed_service_action_task'::regclass AND conname='ck_idc_managed_service_action_entitlement') THEN
    ALTER TABLE plugin_idc_core_managed_service_action_task
        ADD CONSTRAINT ck_idc_managed_service_action_entitlement CHECK ("confirmed_entitlement" IN ('','cancelled'));
END IF;
END $constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_managed_service_action_claim
    ON plugin_idc_core_managed_service_action_task("tenant_id","next_attempt_at","id")
    WHERE "status_code" IN ('pending_query','pending_submit','retry_wait');
CREATE INDEX IF NOT EXISTS idx_idc_managed_service_action_customer
    ON plugin_idc_core_managed_service_action_task("tenant_id","customer_no","id" DESC);

-- Purpose: Store bounded query and submit evidence for each managed-service action attempt.
-- 用途：保存每次受管服务操作的有限查询与提交证据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_managed_service_action_attempt (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "action_code" VARCHAR(16) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "result_digest" CHAR(64) NOT NULL DEFAULT repeat('0',64),
    "result_code" VARCHAR(64) NOT NULL DEFAULT '',
    "safe_evidence" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "finished_at" TIMESTAMPTZ,
    PRIMARY KEY("task_no","attempt_no"),
    CONSTRAINT ck_idc_managed_service_attempt_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_managed_service_attempt_action CHECK ("action_code" IN ('query','submit')),
    CONSTRAINT ck_idc_managed_service_attempt_digests CHECK ("request_digest" ~ '^[0-9a-f]{64}$' AND "result_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_managed_service_attempt_task FOREIGN KEY("task_no") REFERENCES plugin_idc_core_managed_service_action_task("task_no") ON DELETE RESTRICT
);

-- Purpose: Store short-lived, one-use tickets for private managed-service downloads.
-- 用途：保存受管服务私有文件下载使用的短期单次票据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_download_ticket (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "id" BIGSERIAL UNIQUE,
    "ticket_digest" CHAR(64) PRIMARY KEY,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "audience_code" VARCHAR(16) NOT NULL,
    "actor_user_id" BIGINT NOT NULL,
    "action_key" VARCHAR(64) NOT NULL,
    "object_path" VARCHAR(512) NOT NULL,
    "object_digest" CHAR(64) NOT NULL,
    "file_name" VARCHAR(255) NOT NULL,
    "content_type" VARCHAR(128) NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_download_ticket_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_idc_service_download_ticket_audience CHECK ("audience_code" IN ('customer','admin')),
    CONSTRAINT ck_idc_service_download_ticket_digest CHECK ("ticket_digest" ~ '^[0-9a-f]{64}$' AND "object_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_service_download_ticket_entitlement FOREIGN KEY("entitlement_no") REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_download_ticket_expiry
    ON plugin_idc_core_service_download_ticket("tenant_id","expires_at")
    WHERE "consumed_at" IS NULL;
