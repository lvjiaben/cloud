-- Preserve explicit channel protocols. No endpoint is rewritten by migration.
ALTER TABLE plugin_idc_resource_channel
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_channel_endpoint;
ALTER TABLE plugin_idc_resource_channel
    ADD CONSTRAINT ck_plugin_idc_resource_channel_endpoint CHECK (
        "endpoint" = btrim("endpoint") AND (
            left(lower("endpoint"), 8) = 'https://' OR
            left(lower("endpoint"), 7) = 'http://'
        )
    );
COMMENT ON COLUMN plugin_idc_resource_channel."endpoint"
    IS 'Administrator-approved explicit HTTP(S) API origin without credentials';

-- Reconstructible, secret-free option projection and refresh work queue.
-- Exact configuration/release identity is included in request_key. No money,
-- credentials, availability guarantees or business outcome is stored here.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_interface_candidate_snapshot (
    request_key CHAR(64) PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    interface_instance_no VARCHAR(64) NOT NULL,
    configuration_revision VARCHAR(64) NOT NULL,
    request_payload JSONB NOT NULL,
    result_payload JSONB,
    observed_at TIMESTAMPTZ,
    refresh_after TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lease_until TIMESTAMPTZ,
    last_error_code VARCHAR(96) NOT NULL DEFAULT '',
    CONSTRAINT ck_plugin_idc_resource_candidate_request CHECK (
        jsonb_typeof(request_payload) = 'object' AND octet_length(request_payload::text) <= 16384
    ),
    CONSTRAINT ck_plugin_idc_resource_candidate_result CHECK (
        result_payload IS NULL OR jsonb_typeof(result_payload) = 'object'
    )
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_candidate_refresh
    ON plugin_idc_resource_interface_candidate_snapshot (refresh_after, request_key);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_candidate_interface
    ON plugin_idc_resource_interface_candidate_snapshot (tenant_id, interface_instance_no, configuration_revision);
COMMENT ON TABLE plugin_idc_resource_interface_candidate_snapshot
    IS 'Reconstructible supplier option labels and bounded asynchronous refresh queue; never authoritative stock';
