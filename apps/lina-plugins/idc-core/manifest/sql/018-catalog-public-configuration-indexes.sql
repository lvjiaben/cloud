-- Bounded public configurators filter only one published version and active rows.

CREATE INDEX IF NOT EXISTS idx_idc_catalog_option_public_configuration
    ON plugin_idc_core_catalog_configuration_option (
        "tenant_id", "version_no", "group_no", "sort_order", "id"
    ) WHERE "status_code" = 'active';

CREATE INDEX IF NOT EXISTS idx_idc_catalog_compat_public_configuration
    ON plugin_idc_core_catalog_compatibility_rule (
        "tenant_id", "version_no", "id"
    ) WHERE "status_code" = 'active';
