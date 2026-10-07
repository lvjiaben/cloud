CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_ind_profile_identity_digest
    ON plugin_idc_core_customer_individual_profile (
        "tenant_id", "identity_lookup_digest"
    ) WHERE "identity_lookup_digest" <> '';
