-- Native imported units are channel-owned and have no server resource pool.
-- Existing server mappings retain both their pool requirement and foreign key.
ALTER TABLE plugin_idc_resource_mapping ALTER COLUMN pool_no DROP NOT NULL;
ALTER TABLE plugin_idc_resource_mapping
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_mapping_pool_scope;
ALTER TABLE plugin_idc_resource_mapping
    ADD CONSTRAINT ck_plugin_idc_resource_mapping_pool_scope CHECK (
        (left(local_resource_no, 4) = 'NAT_' AND pool_no IS NULL)
        OR (left(local_resource_no, 4) <> 'NAT_' AND pool_no IS NOT NULL)
    );
