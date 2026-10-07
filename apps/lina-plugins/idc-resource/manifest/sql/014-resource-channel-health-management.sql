-- Add merchant-managed safe health thresholds and a bounded current projection
-- for the administration page. The current state remains operational and is
-- never soft-deleted; policy changes are preserved by the business audit owner.

ALTER TABLE plugin_idc_resource_channel_health_state
    ADD COLUMN IF NOT EXISTS "window_sample_limit" INT NOT NULL DEFAULT 20,
    ADD COLUMN IF NOT EXISTS "minimum_samples_to_protect" INT NOT NULL DEFAULT 5,
    ADD COLUMN IF NOT EXISTS "failure_limit" INT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS "unknown_result_limit" INT NOT NULL DEFAULT 2,
    ADD COLUMN IF NOT EXISTS "consecutive_failure_limit" INT NOT NULL DEFAULT 3,
    ADD COLUMN IF NOT EXISTS "protection_duration_seconds" INT NOT NULL DEFAULT 300,
    ADD COLUMN IF NOT EXISTS "policy_version" BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "policy_updated_by" INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "policy_updated_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "last_authentication_health_code" VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS "last_authentication_summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "last_authentication_latency_ms" INT,
    ADD COLUMN IF NOT EXISTS "last_authentication_observed_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "last_query_health_code" VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS "last_query_summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "last_query_latency_ms" INT,
    ADD COLUMN IF NOT EXISTS "last_query_observed_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "last_write_health_code" VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS "last_write_summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "last_write_latency_ms" INT,
    ADD COLUMN IF NOT EXISTS "last_write_observed_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "last_provider_capacity_health_code" VARCHAR(16) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS "last_provider_capacity_summary_code" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "last_provider_capacity_latency_ms" INT,
    ADD COLUMN IF NOT EXISTS "last_provider_capacity_observed_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_resource_channel_health_state
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_health_state_counts;
ALTER TABLE plugin_idc_resource_channel_health_state
    ADD CONSTRAINT ck_plugin_idc_resource_health_state_counts CHECK (
        "sample_count" BETWEEN 0 AND "window_sample_limit" AND
        "failure_count" BETWEEN 0 AND "sample_count" AND
        "unknown_count" BETWEEN 0 AND "failure_count" AND
        "consecutive_failures" BETWEEN 0 AND "window_sample_limit"
    );

ALTER TABLE plugin_idc_resource_channel_health_state
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_health_state_policy;
ALTER TABLE plugin_idc_resource_channel_health_state
    ADD CONSTRAINT ck_plugin_idc_resource_health_state_policy CHECK (
        "window_sample_limit" BETWEEN 5 AND 100 AND
        "minimum_samples_to_protect" BETWEEN 3 AND "window_sample_limit" AND
        "failure_limit" BETWEEN 1 AND "window_sample_limit" AND
        "unknown_result_limit" BETWEEN 1 AND "failure_limit" AND
        "consecutive_failure_limit" BETWEEN 1 AND "window_sample_limit" AND
        "protection_duration_seconds" BETWEEN 60 AND 86400 AND
        "policy_version" > 0
    );

ALTER TABLE plugin_idc_resource_channel_health_state
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_health_state_projection;
ALTER TABLE plugin_idc_resource_channel_health_state
    ADD CONSTRAINT ck_plugin_idc_resource_health_state_projection CHECK (
        "last_authentication_health_code" IN ('healthy', 'degraded', 'unavailable', 'unknown') AND
        "last_query_health_code" IN ('healthy', 'degraded', 'unavailable', 'unknown') AND
        "last_write_health_code" IN ('healthy', 'degraded', 'unavailable', 'unknown') AND
        "last_provider_capacity_health_code" IN ('healthy', 'degraded', 'unavailable', 'unknown') AND
        ("last_authentication_latency_ms" IS NULL OR "last_authentication_latency_ms" BETWEEN 0 AND 3600000) AND
        ("last_query_latency_ms" IS NULL OR "last_query_latency_ms" BETWEEN 0 AND 3600000) AND
        ("last_write_latency_ms" IS NULL OR "last_write_latency_ms" BETWEEN 0 AND 3600000) AND
        ("last_provider_capacity_latency_ms" IS NULL OR "last_provider_capacity_latency_ms" BETWEEN 0 AND 3600000)
    );

COMMENT ON COLUMN plugin_idc_resource_channel_health_state."window_sample_limit" IS 'Merchant policy: bounded write sample window from 5 through 100';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."minimum_samples_to_protect" IS 'Merchant policy: minimum write observations before protection may open';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."protection_duration_seconds" IS 'Merchant policy: automatic write protection duration from 60 through 86400 seconds';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."policy_updated_at" IS 'PostgreSQL-clock time of the latest audited merchant policy change';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."last_authentication_summary_code" IS 'Stable non-sensitive latest authentication summary; raw upstream errors are forbidden';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."last_query_summary_code" IS 'Stable non-sensitive latest query summary; raw upstream errors are forbidden';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."last_write_summary_code" IS 'Stable non-sensitive latest write summary; raw upstream errors are forbidden';
COMMENT ON COLUMN plugin_idc_resource_channel_health_state."last_provider_capacity_summary_code" IS 'Stable non-sensitive latest provider_capacity summary; raw upstream errors are forbidden';
