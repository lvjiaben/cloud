-- 029: bind one generic product-automation interface to one resource-owned
-- server supplier channel. The product core keeps only the opaque pfi number;
-- endpoint, credentials, protocol settings, pools and routing stay here.

-- Purpose: Persists the immutable product-interface to server-channel ownership binding.
-- 用途：持久化商品自动化接口与服务器供应商渠道的唯一归属关系。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_interface_channel_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "binding_version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_interface_channel_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_interface_channel_no CHECK (
        "interface_instance_no" ~ '^pfi_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_channel_version CHECK ("binding_version" > 0),
    CONSTRAINT uk_plugin_idc_resource_interface_channel_interface UNIQUE (
        "tenant_id", "interface_instance_no"
    ),
    CONSTRAINT uk_plugin_idc_resource_interface_channel_channel UNIQUE (
        "tenant_id", "channel_no"
    ),
    CONSTRAINT fk_plugin_idc_resource_interface_channel_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

COMMENT ON TABLE plugin_idc_resource_interface_channel_binding IS
    'Immutable one-to-one ownership binding from a generic product interface number to one resource-owned supplier channel';
COMMENT ON COLUMN plugin_idc_resource_interface_channel_binding."interface_instance_no" IS
    'Opaque product-automation interface number; no endpoint, credential or pool configuration crosses into product core';
COMMENT ON COLUMN plugin_idc_resource_interface_channel_binding."binding_version" IS
    'Frozen binding contract version; an existing interface number is never silently retargeted';
