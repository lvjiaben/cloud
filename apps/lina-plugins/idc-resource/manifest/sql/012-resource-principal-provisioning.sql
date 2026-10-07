-- 012: Durable upstream-principal provisioning and reconciliation
--
-- Customer numbers remain opaque. Username/profile material and the generated
-- password are retained only as channel-scoped secretcap handles. A create
-- lease that expires is permanently moved to lookup-only verification.

-- Purpose: Persists business records for plugin_idc_resource_principal_task.
-- 用途：持久化 plugin_idc_resource_principal_task 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_principal_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(64) NOT NULL,
    "customer_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "username_secret_ref" VARCHAR(256) NOT NULL,
    "password_secret_ref" VARCHAR(256) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_lookup',
    "expected_external_principal_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "result_unknown" BOOLEAN NOT NULL DEFAULT FALSE,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "observation_attempt_count" INT NOT NULL DEFAULT 0,
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token_digest" CHAR(64) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "evidence_digest" CHAR(64),
    "finished_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_principal_task_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_principal_task_customer
        CHECK ("customer_no" = btrim("customer_no") AND length("customer_no") > 0),
    CONSTRAINT ck_plugin_idc_resource_principal_task_secret_refs
        CHECK (
            "username_secret_ref" = btrim("username_secret_ref") AND
            length("username_secret_ref") > 0 AND
            "password_secret_ref" = btrim("password_secret_ref") AND
            length("password_secret_ref") > 0
        ),
    CONSTRAINT ck_plugin_idc_resource_principal_task_status
        CHECK ("status_code" IN (
            'pending_lookup', 'lookup_leased', 'ready_create', 'create_leased',
            'verify_wait', 'verify_leased', 'cleanup_pending', 'cleanup_leased',
            'manual_cleanup_pending', 'manual_cleanup_leased',
            'succeeded', 'manual_review'
        )),
    CONSTRAINT ck_plugin_idc_resource_principal_task_attempts
        CHECK ("attempt_count" BETWEEN 0 AND 3 AND "observation_attempt_count" >= 0),
    CONSTRAINT ck_plugin_idc_resource_principal_task_lease CHECK (
        ("status_code" IN (
            'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
            'manual_cleanup_leased'
         ) AND
         "lease_token_digest" ~ '^[0-9a-f]{64}$' AND "lease_expires_at" IS NOT NULL) OR
        ("status_code" NOT IN (
            'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
            'manual_cleanup_leased'
         ) AND
         "lease_token_digest" = '' AND "lease_expires_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_principal_task_unknown CHECK (
        NOT "result_unknown" OR "status_code" NOT IN (
            'pending_lookup', 'lookup_leased', 'ready_create', 'create_leased'
        )
    ),
    CONSTRAINT ck_plugin_idc_resource_principal_task_evidence CHECK (
        "evidence_digest" IS NULL OR "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_principal_task_finish CHECK (
        ("status_code" IN ('succeeded', 'manual_review') AND "finished_at" IS NOT NULL) OR
        ("status_code" NOT IN ('succeeded', 'manual_review') AND "finished_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_principal_task_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_principal_task_no UNIQUE ("task_no"),
    CONSTRAINT uk_plugin_idc_resource_principal_task_customer
        UNIQUE ("tenant_id", "channel_no", "customer_no"),
    CONSTRAINT fk_plugin_idc_resource_principal_task_channel
        FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

-- This migration remains safe when an earlier development revision already
-- created the table: replace the two state constraints and partial indexes
-- whose predicates evolve with the durable state machine.
ALTER TABLE plugin_idc_resource_principal_task
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_principal_task_status;
ALTER TABLE plugin_idc_resource_principal_task
    ADD CONSTRAINT ck_plugin_idc_resource_principal_task_status
    CHECK ("status_code" IN (
        'pending_lookup', 'lookup_leased', 'ready_create', 'create_leased',
        'verify_wait', 'verify_leased', 'cleanup_pending', 'cleanup_leased',
        'manual_cleanup_pending', 'manual_cleanup_leased',
        'succeeded', 'manual_review'
    ));
ALTER TABLE plugin_idc_resource_principal_task
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_principal_task_lease;
ALTER TABLE plugin_idc_resource_principal_task
    ADD CONSTRAINT ck_plugin_idc_resource_principal_task_lease CHECK (
        ("status_code" IN (
            'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
            'manual_cleanup_leased'
         ) AND
         "lease_token_digest" ~ '^[0-9a-f]{64}$' AND "lease_expires_at" IS NOT NULL) OR
        ("status_code" NOT IN (
            'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
            'manual_cleanup_leased'
         ) AND
         "lease_token_digest" = '' AND "lease_expires_at" IS NULL)
    );

DROP INDEX IF EXISTS idx_plugin_idc_resource_principal_task_claim;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_principal_task_claim
    ON plugin_idc_resource_principal_task (
        "tenant_id", "status_code", "available_at", "id"
    ) WHERE "status_code" IN (
        'pending_lookup', 'ready_create', 'verify_wait', 'cleanup_pending',
        'manual_cleanup_pending'
    );
DROP INDEX IF EXISTS idx_plugin_idc_resource_principal_task_lease;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_principal_task_lease
    ON plugin_idc_resource_principal_task (
        "tenant_id", "lease_expires_at", "id"
    ) WHERE "status_code" IN (
        'lookup_leased', 'create_leased', 'verify_leased', 'cleanup_leased',
        'manual_cleanup_leased'
    );

COMMENT ON TABLE plugin_idc_resource_principal_task
    IS 'Resource-owner state machine for exact lookup, bounded lookup-fenced creates, verification, and secret cleanup';
COMMENT ON COLUMN plugin_idc_resource_principal_task."username_secret_ref"
    IS 'Channel-scoped secretcap handle; never a username or customer profile field';
COMMENT ON COLUMN plugin_idc_resource_principal_task."password_secret_ref"
    IS 'Channel-scoped secretcap handle; plaintext is never persisted in the resource schema';
COMMENT ON COLUMN plugin_idc_resource_principal_task."result_unknown"
    IS 'Hard replay fence: when true, only exact lookup or manual review is allowed';
COMMENT ON COLUMN plugin_idc_resource_principal_task."attempt_count"
    IS 'Count of write-eligible claims; unknown writes are never made claimable again';
COMMENT ON COLUMN plugin_idc_resource_principal_task."observation_attempt_count"
    IS 'Read-only exact lookup attempts; never authorizes another create';
COMMENT ON COLUMN plugin_idc_resource_principal_task."finished_at"
    IS 'Confirmed mappings revoke both handles; manual review revokes the password and retains only its encrypted username handle for operator reconciliation';
