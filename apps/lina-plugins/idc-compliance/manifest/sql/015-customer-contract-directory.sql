-- Adds the stable customer contract directory access path introduced in task 10.8.f.

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_contract_customer_directory
    ON plugin_idc_compliance_contract (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );
