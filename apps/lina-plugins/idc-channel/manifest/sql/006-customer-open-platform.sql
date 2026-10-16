-- Immutable sanitized HTTP calls, isolated by the credential-bound account.
CREATE TABLE IF NOT EXISTS plugin_idc_channel_api_call (
 id BIGSERIAL PRIMARY KEY,
 channel_id VARCHAR(80) NOT NULL REFERENCES plugin_idc_channel_account(channel_id),
 method VARCHAR(16) NOT NULL,
 path VARCHAR(160) NOT NULL,
 succeeded BOOLEAN NOT NULL,
 duration_millis BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS plugin_idc_channel_api_call_owner_idx ON plugin_idc_channel_api_call(channel_id,id DESC);
