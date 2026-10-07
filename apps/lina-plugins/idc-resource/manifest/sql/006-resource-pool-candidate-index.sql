-- 006: bound enabled resource-pool candidate listing in display order.

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_pool_candidate
    ON plugin_idc_resource_pool (
        "tenant_id", "resource_kind", "status_code", "display_name", "pool_no"
    )
    WHERE "deleted_at" IS NULL;
