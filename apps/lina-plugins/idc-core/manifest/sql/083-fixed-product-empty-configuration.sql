-- 083: Fixed products freeze an explicit empty configuration snapshot instead
-- of inventing a server option. Plugin-owned structured values remain in the
-- separate structured configuration snapshot introduced by migration 075.

ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_config_counts;

ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD CONSTRAINT ck_idc_com_config_counts CHECK (
        "selected_option_count" BETWEEN 0 AND 5000
        AND "total_selected_quantity" BETWEEN 0 AND 1000000
        AND (("selected_option_count" = 0 AND "total_selected_quantity" = 0)
             OR ("selected_option_count" > 0 AND "total_selected_quantity" > 0))
    );
