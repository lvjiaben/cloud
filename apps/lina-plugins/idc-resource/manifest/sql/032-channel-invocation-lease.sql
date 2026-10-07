-- Purpose: Store cluster-safe invocation leases for supplier channels with shared state.
-- 用途：保存共享状态供应商通道的集群安全调用租约。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_channel_invocation_lease (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(64) NOT NULL,
    "lease_token" VARCHAR(64) NOT NULL,
    "lease_until" TIMESTAMPTZ NOT NULL,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pk_plugin_idc_resource_channel_invocation_lease PRIMARY KEY ("tenant_id","channel_no"),
    CONSTRAINT ck_plugin_idc_resource_channel_invocation_lease_tenant CHECK ("tenant_id"=0),
    CONSTRAINT ck_plugin_idc_resource_channel_invocation_lease_token CHECK ("lease_token" ~ '^ril_[0-9a-f]{32}$'),
    CONSTRAINT fk_plugin_idc_resource_channel_invocation_lease_channel FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel("channel_no") ON DELETE RESTRICT
);

COMMENT ON TABLE plugin_idc_resource_channel_invocation_lease IS
    'Recoverable cross-process lease that serializes one complete managed connector invocation for a channel';
