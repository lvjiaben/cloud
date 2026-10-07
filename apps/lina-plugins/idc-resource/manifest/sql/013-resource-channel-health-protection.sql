-- Persist the latest channel health window so every worker observes the same
-- write-protection decision. Health rows are operational state and immutable
-- observations, so neither table uses soft deletion.

ALTER TABLE plugin_idc_resource_health_snapshot
    ADD COLUMN IF NOT EXISTS "operation_kind" VARCHAR(24) NOT NULL DEFAULT 'write';

ALTER TABLE plugin_idc_resource_health_snapshot
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_health_operation;
ALTER TABLE plugin_idc_resource_health_snapshot
    ADD CONSTRAINT ck_plugin_idc_resource_health_operation CHECK (
        "operation_kind" IN ('authentication', 'query', 'write', 'provider_capacity')
    );

-- Purpose: Persists business records for plugin_idc_resource_channel_health_state.
-- 用途：持久化 plugin_idc_resource_channel_health_state 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_channel_health_state (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(64) NOT NULL,
    "sample_count" INT NOT NULL DEFAULT 0,
    "failure_count" INT NOT NULL DEFAULT 0,
    "unknown_count" INT NOT NULL DEFAULT 0,
    "consecutive_failures" INT NOT NULL DEFAULT 0,
    "circuit_open" BOOLEAN NOT NULL DEFAULT FALSE,
    "write_protected_until" TIMESTAMPTZ,
    "window_started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "last_summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    "last_observed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_health_state_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_health_state_counts CHECK (
        "sample_count" BETWEEN 0 AND 20 AND
        "failure_count" BETWEEN 0 AND "sample_count" AND
        "unknown_count" BETWEEN 0 AND "failure_count" AND
        "consecutive_failures" BETWEEN 0 AND 20
    ),
    CONSTRAINT ck_plugin_idc_resource_health_state_protection CHECK (
        NOT "circuit_open" OR "write_protected_until" IS NOT NULL
    ),
    CONSTRAINT ck_plugin_idc_resource_health_state_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_health_state_channel UNIQUE (
        "tenant_id", "channel_no"
    ),
    CONSTRAINT fk_plugin_idc_resource_health_state_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

DROP INDEX IF EXISTS idx_plugin_idc_resource_health_state_protection;
CREATE INDEX idx_plugin_idc_resource_health_state_protection
    ON plugin_idc_resource_channel_health_state (
        "tenant_id", "channel_no", "write_protected_until"
    ) WHERE "circuit_open" = TRUE AND "write_protected_until" IS NOT NULL;

COMMENT ON TABLE plugin_idc_resource_channel_health_state IS 'Database-authoritative bounded Provider health window and cross-worker write protection';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."last_summary_code" IS 'Stable non-sensitive machine code; raw upstream errors are forbidden';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."write_protected_until" IS 'Database-clock protection deadline; read-only task and resource reconciliation remains allowed';
