-- 144: Add explicit marginal quantity pricing without converting old prices,
-- dimensions or immutable order selections. No data or generated shape changes.
ALTER TABLE plugin_idc_core_catalog_configuration_price_dimension
 DROP CONSTRAINT IF EXISTS ck_idc_catalog_config_price_mode;
ALTER TABLE plugin_idc_core_catalog_configuration_price_dimension
 ADD CONSTRAINT ck_idc_catalog_config_price_mode CHECK (
 pricing_mode_code IN ('fixed','per_unit','band_total','graduated')
 );
