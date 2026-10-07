-- Retire the abandoned resource/Provider snapshot to catalog-draft direction.
-- Development databases without provider_import product versions are cleaned
-- in place. Databases that still contain those obsolete versions fail closed:
-- export any test data that must be kept, rebuild the development database from
-- current migrations, then restore only data that does not depend on the retired path.

DO $provider_catalog_retirement$
BEGIN
    IF to_regclass('plugin_idc_core_catalog_product_version') IS NOT NULL
       AND EXISTS (
           SELECT 1
             FROM plugin_idc_core_catalog_product_version
            WHERE "source_code" = 'provider_import'
       ) THEN
        RAISE EXCEPTION 'cannot retire Provider catalog drafts while provider_import product versions still exist'
            USING HINT = 'Export any test data that must be kept, rebuild this development database from current migrations, then restore only data that does not depend on provider_import versions. Do not relabel obsolete versions as merchant-created or migration versions.';
    END IF;
END
$provider_catalog_retirement$;

DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft_candidate_decision CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft_currency CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft_completion CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft_candidate CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_catalog_provider_draft CASCADE;

DROP FUNCTION IF EXISTS plugin_idc_core_reject_provider_review_mutation() CASCADE;

DELETE FROM sys_dict_data
 WHERE "tenant_id" = 0
   AND (
       "dict_type" IN (
           'idc_catalog_provider_draft_status',
           'idc_catalog_provider_source_scope',
           'idc_catalog_provider_tax_treatment',
           'idc_catalog_provider_fulfillment_policy',
           'idc_catalog_provider_risk_policy',
           'idc_catalog_provider_compatibility_decision',
           'idc_catalog_provider_candidate_decision'
       )
       OR ("dict_type" = 'idc_catalog_version_source' AND "value" = 'provider_import')
   );

DELETE FROM sys_dict_type
 WHERE "tenant_id" = 0
   AND "type" IN (
       'idc_catalog_provider_draft_status',
       'idc_catalog_provider_source_scope',
       'idc_catalog_provider_tax_treatment',
       'idc_catalog_provider_fulfillment_policy',
       'idc_catalog_provider_risk_policy',
       'idc_catalog_provider_compatibility_decision',
       'idc_catalog_provider_candidate_decision'
   );

ALTER TABLE IF EXISTS plugin_idc_core_catalog_product_version
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_version_source;

ALTER TABLE IF EXISTS plugin_idc_core_catalog_product_version
    ADD CONSTRAINT ck_idc_catalog_version_source CHECK (
        "source_code" IN ('manual', 'migration', 'copy')
    );
