-- Preserve only the frozen customer-safe product labels delivered through the
-- generic entitlement event. Existing projections are completed by replaying
-- their durable core outbox event; idc-resource never reads core tables.
ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD COLUMN IF NOT EXISTS "product_name_zh_cn" varchar(256),
    ADD COLUMN IF NOT EXISTS "product_name_en_us" varchar(256);

ALTER TABLE plugin_idc_resource_server_entitlement_projection
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_server_entitlement_safe_labels;
ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD CONSTRAINT ck_plugin_idc_resource_server_entitlement_safe_labels CHECK (
        ("product_name_zh_cn" IS NULL AND "product_name_en_us" IS NULL) OR
        (length(btrim("product_name_zh_cn")) BETWEEN 1 AND 256 AND
         length(btrim("product_name_en_us")) BETWEEN 1 AND 256)
    );

COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_name_zh_cn" IS
    'Frozen customer-safe Simplified Chinese product label delivered by the entitlement owner event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_name_en_us" IS
    'Frozen customer-safe English product label delivered by the entitlement owner event';
