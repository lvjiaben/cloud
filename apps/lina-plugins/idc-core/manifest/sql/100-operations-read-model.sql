-- 100: Bounded operations read model and cross-domain health signals.
-- The core plugin owns the projection; other plugins publish redacted signals
-- through the operations service instead of being queried across boundaries.
-- Purpose: Stores the bounded operations workbench snapshot and its watermark.
-- 用途：保存受限的运营工作台快照及其水位。
CREATE TABLE IF NOT EXISTS plugin_idc_core_operations_read_model (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "model_key" VARCHAR(64) NOT NULL DEFAULT 'default',
    "generated_at" TIMESTAMPTZ NOT NULL,
    "watermark" TIMESTAMPTZ NOT NULL,
    "queues" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "currencies" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "revision" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_idc_operations_model_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_operations_model_key CHECK ("model_key" = 'default'),
    CONSTRAINT ck_idc_operations_model_revision CHECK ("revision" > 0),
    CONSTRAINT uk_idc_operations_model UNIQUE ("tenant_id", "model_key")
);
CREATE INDEX IF NOT EXISTS idx_idc_operations_model_watermark
    ON plugin_idc_core_operations_read_model ("tenant_id", "watermark" DESC);

-- Purpose: Stores redacted cross-domain health and performance signals for operations.
-- 用途：保存供运营使用的跨领域脱敏健康和性能信号。
CREATE TABLE IF NOT EXISTS plugin_idc_core_operations_signal (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "signal_kind" VARCHAR(32) NOT NULL,
    "signal_code" VARCHAR(96) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL,
    "count_value" BIGINT NOT NULL DEFAULT 0,
    "age_seconds" BIGINT NOT NULL DEFAULT 0,
    "value_minor" BIGINT NOT NULL DEFAULT 0,
    "threshold_minor" BIGINT NOT NULL DEFAULT 0,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "source_revision" VARCHAR(128) NOT NULL DEFAULT '',
    "details" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_operations_signal_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_operations_signal_kind CHECK ("signal_kind" IN ('queue_backlog', 'adapter_health', 'inventory_freshness', 'performance')),
    CONSTRAINT ck_idc_operations_signal_status CHECK ("status_code" IN ('ok', 'warning', 'critical', 'unknown')),
    CONSTRAINT ck_idc_operations_signal_counts CHECK ("count_value" >= 0 AND "age_seconds" >= 0 AND "value_minor" >= 0 AND "threshold_minor" >= 0),
    CONSTRAINT uk_idc_operations_signal UNIQUE ("tenant_id", "signal_kind", "signal_code")
);
CREATE INDEX IF NOT EXISTS idx_idc_operations_signal_alert
    ON plugin_idc_core_operations_signal ("tenant_id", "status_code", "observed_at" DESC);
