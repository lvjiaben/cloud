-- Resolve one paid entitlement from its exact successful owner receipt before
-- reading the corrected task and mapping. Keep failed receipt history intact.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provision_outcome_identity
    ON plugin_idc_resource_provision_outcome_receipt
        (tenant_id, operation_no, customer_no, configuration_snapshot_no)
    WHERE status_code = 'succeeded';

-- Read the most recent complete or failed capability attempt without sorting
-- a channel's full snapshot history. Status-specific reads keep their index.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_cap_attempt_latest
    ON plugin_idc_resource_capability_snapshot
        (tenant_id, channel_no, observed_at DESC, id DESC);
