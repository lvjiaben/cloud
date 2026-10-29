-- 134: Merchant-owned purchase controls, independent of provider parameter types.
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    ADD COLUMN IF NOT EXISTS customer_presentation_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    DROP CONSTRAINT IF EXISTS ck_idc_customer_presentation_object;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    ADD CONSTRAINT ck_idc_customer_presentation_object CHECK (
        jsonb_typeof(customer_presentation_snapshot) = 'object'
        AND octet_length(customer_presentation_snapshot::text) <= 131072
    );
