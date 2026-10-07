-- Preserve the generic product/version references carried by the immutable
-- entitlement-created event. Existing development rows remain intact and are
-- completed by replaying their durable core outbox event.
ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD COLUMN IF NOT EXISTS "product_no" varchar(40),
    ADD COLUMN IF NOT EXISTS "product_version_no" varchar(40);

COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_no" IS
    'Opaque generic product reference frozen by the entitlement-created event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_version_no" IS
    'Opaque generic product version reference frozen by the entitlement-created event';

ALTER TABLE plugin_idc_resource_server_entitlement_projection
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_server_entitlement_product_reference;
ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD CONSTRAINT ck_plugin_idc_resource_server_entitlement_product_reference CHECK (
        ("product_no" IS NULL AND "product_version_no" IS NULL) OR
        ("product_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
         "product_version_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$')
    );
