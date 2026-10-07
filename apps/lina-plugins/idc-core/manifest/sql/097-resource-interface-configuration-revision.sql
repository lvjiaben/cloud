-- 097: Keep the resource owner's immutable endpoint/account/secret-reference
-- revision separate from the generic product configuration revision.

ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision
    ADD COLUMN IF NOT EXISTS "resource_owner_configuration_revision" VARCHAR(36) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_interface_resource_owner_configuration_revision'
          AND conrelid = 'plugin_idc_core_product_automation_interface_configuration_revision'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_product_automation_interface_configuration_revision
            ADD CONSTRAINT ck_idc_interface_resource_owner_configuration_revision CHECK (
                "resource_owner_configuration_revision" = ''
                OR "resource_owner_configuration_revision" ~ '^rci_[0-9a-f]{32}$'
            );
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_interface_resource_owner_configuration_revision
    ON plugin_idc_core_product_automation_interface_configuration_revision (
        "tenant_id", "interface_instance_no", "resource_owner_configuration_revision"
    )
    WHERE "resource_owner_configuration_revision" <> '';

COMMENT ON COLUMN plugin_idc_core_product_automation_interface_configuration_revision."resource_owner_configuration_revision" IS
    'Immutable resource-owner endpoint/account/secret-reference revision; empty for non-resource interfaces / 资源拥有方不可变接口配置修订，非资源接口为空';
