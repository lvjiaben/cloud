-- Commercial configuration belongs to a product version. Automation is an
-- optional exact relationship, never a placeholder supplier for manual goods.
-- This stage changes no historical schema, values, prices or order amounts.

ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    ALTER COLUMN interface_instance_no DROP NOT NULL,
    ALTER COLUMN interface_configuration_revision DROP NOT NULL;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    DROP CONSTRAINT IF EXISTS fk_idc_product_version_configuration_binding,
    DROP CONSTRAINT IF EXISTS ck_idc_product_version_configuration_revision;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    ADD CONSTRAINT ck_idc_product_version_configuration_revision CHECK (
        version > 0 AND (
            (interface_instance_no IS NULL AND interface_configuration_revision IS NULL)
            OR (interface_instance_no IS NOT NULL AND interface_configuration_revision IS NOT NULL
                AND interface_configuration_revision > 0)
        )
    );

-- The existing version identity is unique. This equivalent composite key
-- permits a nullable relationship while also requiring the exact interface.
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_binding_version_interface
    ON plugin_idc_core_catalog_product_version_fulfillment_binding
       (tenant_id, version_no, interface_instance_no);
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_configuration_optional_binding;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
    ADD CONSTRAINT fk_plugin_idc_core_configuration_optional_binding
    FOREIGN KEY (tenant_id, version_no, interface_instance_no)
    REFERENCES plugin_idc_core_catalog_product_version_fulfillment_binding
               (tenant_id, version_no, interface_instance_no) ON DELETE RESTRICT;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_product_version_configuration()
RETURNS TRIGGER AS $$
DECLARE
    target_version_status VARCHAR(16);
    binding_interface_no VARCHAR(40);
BEGIN
    SELECT status_code INTO target_version_status
      FROM plugin_idc_core_catalog_product_version
     WHERE tenant_id = COALESCE(NEW.tenant_id, OLD.tenant_id)
       AND version_no = COALESCE(NEW.version_no, OLD.version_no)
     FOR UPDATE;
    IF target_version_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'published product version configuration is immutable';
    END IF;
    SELECT interface_instance_no INTO binding_interface_no
      FROM plugin_idc_core_catalog_product_version_fulfillment_binding
     WHERE tenant_id = COALESCE(NEW.tenant_id, OLD.tenant_id)
       AND version_no = COALESCE(NEW.version_no, OLD.version_no)
     FOR UPDATE;
    -- Choosing an interface does not discard or attest a previously independent
    -- offer definition. It acquires an exact reference only after configuration
    -- is validated and saved against that interface.
    IF TG_OP <> 'DELETE' AND NEW.interface_instance_no IS NOT NULL
       AND binding_interface_no IS DISTINCT FROM NEW.interface_instance_no THEN
        RAISE EXCEPTION 'product version configuration does not match its fulfillment binding';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        NEW.changed_at := CURRENT_TIMESTAMP;
        IF NEW.id IS DISTINCT FROM OLD.id
           OR NEW.tenant_id IS DISTINCT FROM OLD.tenant_id
           OR NEW.version_no IS DISTINCT FROM OLD.version_no
           OR NEW.version <> OLD.version + 1 THEN
            RAISE EXCEPTION 'product version configuration revision is invalid';
        END IF;
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot
    ALTER COLUMN interface_instance_no DROP NOT NULL,
    ALTER COLUMN interface_configuration_revision DROP NOT NULL;
ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_structured_snapshot_revision;
ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot
    ADD CONSTRAINT ck_idc_com_structured_snapshot_revision CHECK (
        (interface_instance_no IS NULL AND interface_configuration_revision IS NULL)
        OR (interface_instance_no IS NOT NULL AND interface_configuration_revision IS NOT NULL
            AND interface_configuration_revision > 0)
    );

COMMENT ON TABLE plugin_idc_core_catalog_product_version_configuration IS
    'Commercial purchase configuration with an optional exact automation binding';

-- A commercial selection snapshot also belongs to manually delivered goods.
-- Provider evidence remains empty; automatic intents retain their exact gates.
ALTER TABLE plugin_idc_core_product_fulfillment_intent
    DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_plugin_evidence;
ALTER TABLE plugin_idc_core_product_fulfillment_intent
    ADD CONSTRAINT ck_idc_product_fulfillment_plugin_evidence CHECK (
        ("strategy_code" = 'manual' AND "provision_method" = '' AND "query_method" = '' AND (
            ("interface_instance_no" = '' AND "plugin_id" = '' AND "interface_configuration_revision" = 0
             AND ("configuration_snapshot_no" = '' OR "configuration_snapshot_no" ~ '^SCF_[0-9a-f]{32}$')
             AND "plugin_release_version" = '' AND "contract_version" = ''
             AND "release_digest" = '' AND "declaration_digest" = '') OR
            ("interface_instance_no" ~ '^pfi_[0-9a-f]{32}$' AND "interface_configuration_revision" > 0
             AND "configuration_snapshot_no" <> ''
             AND "plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
             AND btrim("plugin_release_version") <> '' AND btrim("contract_version") <> ''
             AND "release_digest" ~ '^[0-9a-f]{64}$' AND "declaration_digest" ~ '^[0-9a-f]{64}$')
         )) OR
        ("strategy_code" <> 'manual' AND "interface_instance_no" ~ '^pfi_[0-9a-f]{32}$'
         AND "interface_configuration_revision" > 0 AND "configuration_snapshot_no" <> ''
         AND "plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
         AND btrim("plugin_release_version") <> '' AND btrim("contract_version") <> ''
         AND "provision_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "query_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
         AND "release_digest" ~ '^[0-9a-f]{64}$' AND "declaration_digest" ~ '^[0-9a-f]{64}$')
    );
