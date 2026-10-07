-- Purpose: Store immutable configuration revisions for resource supplier interfaces.
-- 用途：保存资源供应商接口的不可变配置修订。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_interface_configuration_revision (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "revision_no" VARCHAR(40) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "endpoint" VARCHAR(2048) NOT NULL,
    "non_secret_settings" JSONB NOT NULL,
    "secret_bindings" JSONB NOT NULL DEFAULT '{}'::jsonb,
    "created_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_interface_revision_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_plugin_idc_resource_interface_revision_no CHECK ("revision_no" ~ '^rci_[0-9a-f]{32}$'),
    CONSTRAINT ck_plugin_idc_resource_interface_revision_settings CHECK (
        jsonb_typeof("non_secret_settings")='object' AND jsonb_typeof("secret_bindings")='object'
    ),
    CONSTRAINT uk_plugin_idc_resource_interface_revision_no UNIQUE ("tenant_id","revision_no"),
    CONSTRAINT fk_plugin_idc_resource_interface_revision_binding FOREIGN KEY ("tenant_id","interface_instance_no")
        REFERENCES plugin_idc_resource_interface_channel_binding("tenant_id","interface_instance_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_resource_interface_channel_binding
    ADD COLUMN IF NOT EXISTS "current_configuration_revision" VARCHAR(40);

COMMENT ON TABLE plugin_idc_resource_interface_configuration_revision IS
    'Append-only endpoint, non-secret settings and opaque secret-reference snapshot for one resource interface';
