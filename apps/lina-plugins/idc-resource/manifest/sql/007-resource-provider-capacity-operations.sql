-- 007: Audited automatic provider_capacity synchronization operations.
--
-- The existing observation header remains the sole durable operation record. A
-- worker claims one due collecting observation at a time and advances exactly one
-- Provider page outside the claim transaction. Raw Provider data, endpoints,
-- credentials, and lease tokens are never persisted here.

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "trigger_code" VARCHAR(16) NOT NULL DEFAULT 'system';

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "requested_by" BIGINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "attempt_count" SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "next_attempt_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "lease_token_digest" CHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "lease_expires_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "failure_code" VARCHAR(64) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_trigger'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_trigger CHECK (
                ("trigger_code" = 'system' AND "requested_by" = 0) OR
                ("trigger_code" = 'manual' AND "requested_by" > 0)
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_attempt'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_attempt
            CHECK ("attempt_count" BETWEEN 0 AND 8);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_lease'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_lease CHECK (
                (
                    "lease_token_digest" = '' AND "lease_expires_at" IS NULL
                ) OR (
                    "status_code" = 'collecting' AND
                    "lease_token_digest" ~ '^[0-9a-f]{64}$' AND
                    "lease_expires_at" IS NOT NULL
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_failure'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_failure CHECK (
                ("status_code" = 'failed' AND "failure_code" ~ '^[a-z][a-z0-9_]{0,63}$') OR
                ("status_code" <> 'failed' AND "failure_code" = '')
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_schedule'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_schedule CHECK (
                "next_attempt_at" IS NULL OR "status_code" = 'collecting'
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_due
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "next_attempt_at", "id"
    )
    WHERE "status_code" = 'collecting' AND "next_attempt_at" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_operations
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_operations_channel
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "channel_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_operations_kind
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "capacity_kind", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provider_capacity_operations_status
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."trigger_code"
    IS 'Trusted operation source: system or direct staff manual request';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."requested_by"
    IS 'Trusted host staff user identifier for a manual request; zero for system work';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."attempt_count"
    IS 'Bounded number of claimed Provider page attempts for this observation';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."next_attempt_at"
    IS 'PostgreSQL due time for the next single-page attempt; null means not scheduled';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."lease_token_digest"
    IS 'SHA-256 digest of an ephemeral worker lease token; the token itself is never stored';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."lease_expires_at"
    IS 'PostgreSQL time after which another worker may reclaim this collecting observation';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."failure_code"
    IS 'Stable non-secret terminal failure code; raw Provider errors are forbidden';
