-- Append-only, body-free Provider invocation history; no editable or soft-delete state.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provider_api_log (
 id BIGSERIAL PRIMARY KEY,
 tenant_id BIGINT NOT NULL DEFAULT 0,
 channel_no VARCHAR(64) NOT NULL,
 provider_plugin_id VARCHAR(64) NOT NULL,
 method_code VARCHAR(96) NOT NULL,
 succeeded BOOLEAN NOT NULL,
 error_code VARCHAR(128) NOT NULL DEFAULT '',
 duration_ms BIGINT NOT NULL CHECK (duration_ms >= 0),
 started_at TIMESTAMPTZ NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS plugin_idc_resource_provider_api_log_tenant_id_idx ON plugin_idc_resource_provider_api_log(tenant_id,id DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_resource_provider_api_log_channel_idx ON plugin_idc_resource_provider_api_log(tenant_id,channel_no,id DESC);
