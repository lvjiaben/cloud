-- Bounded keyset and literal-prefix indexes for the staff catalog workbench.

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_management_order
    ON plugin_idc_core_catalog_product (
        "tenant_id", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_management_status
    ON plugin_idc_core_catalog_product (
        "tenant_id", "status_code", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_management_name_zh
    ON plugin_idc_core_catalog_product (
        "tenant_id", LOWER("name_zh_cn") text_pattern_ops
    ) WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_management_name_en
    ON plugin_idc_core_catalog_product (
        "tenant_id", LOWER("name_en_us") text_pattern_ops
    ) WHERE "deleted_at" IS NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_management_slug
    ON plugin_idc_core_catalog_product (
        "tenant_id", "public_slug" text_pattern_ops
    ) WHERE "deleted_at" IS NULL;
