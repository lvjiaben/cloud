-- Bounded public catalog reads only scan active products with published pointers.

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_public_order
    ON plugin_idc_core_catalog_product (
        "tenant_id", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL
        AND "status_code" = 'active'
        AND "current_version_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_public_category
    ON plugin_idc_core_catalog_product (
        "tenant_id", "category_no", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL
        AND "status_code" = 'active'
        AND "current_version_no" IS NOT NULL;
