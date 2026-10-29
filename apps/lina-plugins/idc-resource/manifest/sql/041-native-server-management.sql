-- Reconstructible, whitelisted last observations. Mapping ownership remains
-- authoritative; this table never grants an action or stores credentials.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_server_runtime_fact (
    mapping_no VARCHAR(64) PRIMARY KEY REFERENCES plugin_idc_resource_mapping(mapping_no) ON DELETE RESTRICT,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    facts JSONB NOT NULL DEFAULT '{}'::jsonb,
    observed_at TIMESTAMPTZ,
    next_refresh_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lease_until TIMESTAMPTZ,
    lease_version BIGINT NOT NULL DEFAULT 0,
    last_error_code VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT ck_plugin_idc_resource_runtime_facts CHECK (jsonb_typeof(facts) = 'object' AND octet_length(facts::text) <= 65536)
);
ALTER TABLE plugin_idc_resource_server_runtime_fact ADD COLUMN IF NOT EXISTS mapping_version BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_runtime_due
    ON plugin_idc_resource_server_runtime_fact(next_refresh_at, mapping_no);
COMMENT ON TABLE plugin_idc_resource_server_runtime_fact IS 'Non-secret historical runtime facts; never an authorization or current availability guarantee';

-- Durable correlation between a native commercial intent and the existing
-- resource task. No separate provider executor or secret payload is introduced.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_native_commercial_task (
    operation_no VARCHAR(40) PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    entitlement_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    task_no VARCHAR(64) NOT NULL UNIQUE REFERENCES plugin_idc_resource_task(task_no) ON DELETE RESTRICT,
    local_resource_no VARCHAR(64) NOT NULL,
    channel_no VARCHAR(64) NOT NULL,
    pool_no VARCHAR(64) NOT NULL,
    external_resource_ref VARCHAR(256) NOT NULL,
    action_code VARCHAR(64) NOT NULL,
    parameters JSONB NOT NULL CHECK (jsonb_typeof(parameters)='object' AND octet_length(parameters::TEXT)<=16384),
    request_digest CHAR(64) NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    intent_digest CHAR(64) NOT NULL CHECK (intent_digest ~ '^[0-9a-f]{64}$'),
    entitlement_version BIGINT NOT NULL CHECK (entitlement_version>0),
    admitted_at TIMESTAMPTZ,
    admitted_attempt INT,
    result_status VARCHAR(24) NOT NULL DEFAULT '',
    next_report_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_native_commercial_pending
    ON plugin_idc_resource_native_commercial_task(next_report_at,operation_no) WHERE completed_at IS NULL;

-- A task-bound IP reservation survives its quote expiry until an authoritative
-- outcome releases or consumes it. Keep the bound task on failure for audit.
ALTER TABLE plugin_idc_resource_ip_reservation DROP CONSTRAINT IF EXISTS ck_idc_resource_ip_reservation_status;
ALTER TABLE plugin_idc_resource_ip_reservation ADD COLUMN IF NOT EXISTS commercial_operation_no VARCHAR(40);
ALTER TABLE plugin_idc_resource_ip_reservation ADD CONSTRAINT ck_idc_resource_ip_reservation_status
    CHECK (status_code IN ('reserved','commercial_hold','in_flight','consumed','released','expired'));
ALTER TABLE plugin_idc_resource_ip_reservation DROP CONSTRAINT IF EXISTS ck_idc_resource_ip_reservation_lifecycle;
ALTER TABLE plugin_idc_resource_ip_reservation ADD CONSTRAINT ck_idc_resource_ip_reservation_lifecycle CHECK (
    (status_code='reserved' AND provider_task_no IS NULL AND consumed_at IS NULL AND released_at IS NULL)
    OR (status_code='commercial_hold' AND commercial_operation_no IS NOT NULL AND provider_task_no IS NULL AND consumed_at IS NULL AND released_at IS NULL AND release_reason_code IS NULL)
    OR (status_code='in_flight' AND provider_task_no IS NOT NULL AND consumed_at IS NULL AND released_at IS NULL AND release_reason_code IS NULL)
    OR (status_code='consumed' AND provider_task_no IS NOT NULL AND consumed_at IS NOT NULL AND released_at IS NULL AND release_reason_code IS NULL)
    OR (status_code='released' AND consumed_at IS NULL AND released_at IS NOT NULL AND release_reason_code IS NOT NULL)
    OR (status_code='expired' AND provider_task_no IS NULL AND consumed_at IS NULL AND released_at IS NOT NULL AND release_reason_code IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_ip_in_flight
    ON plugin_idc_resource_ip_reservation(channel_no,pool_no) WHERE status_code IN ('commercial_hold','in_flight');

-- Successful free deletion must eventually stop the core object's renewal even
-- after a process restart or a pending payment-close response.
CREATE TABLE IF NOT EXISTS plugin_idc_resource_native_accessory_receipt (
    task_no VARCHAR(64) PRIMARY KEY REFERENCES plugin_idc_resource_task(task_no) ON DELETE RESTRICT,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    operation_no VARCHAR(40) NOT NULL,
    entitlement_no VARCHAR(40) NOT NULL,
    customer_no VARCHAR(40) NOT NULL,
    object_no VARCHAR(64) NOT NULL,
    accessory_kind VARCHAR(24) NOT NULL CHECK (accessory_kind IN ('snapshot','backup')),
    next_report_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_native_accessory_due
    ON plugin_idc_resource_native_accessory_receipt(next_report_at,task_no) WHERE completed_at IS NULL;
