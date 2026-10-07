-- 016: align the opaque resource-owner pool reference with its 64-byte owner contract.

ALTER TABLE plugin_idc_core_catalog_resource_pool_binding
    ALTER COLUMN "pool_no" TYPE VARCHAR(64);
