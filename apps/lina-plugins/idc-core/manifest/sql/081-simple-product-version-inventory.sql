-- 081: Add the product-neutral inventory setting maintained with a product
-- version. This table deliberately has no plugin, approval, audit, reservation,
-- or provider fields.

-- Purpose: Stores Catalog Product Version Inventory records.
-- 用途：保存目录商品版本库存相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_inventory (
    "version_no" VARCHAR(40) PRIMARY KEY,
    "tracking_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
    "sellable_quantity" BIGINT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_idc_catalog_product_version_inventory_version
        FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no")
        ON DELETE CASCADE,
    CONSTRAINT ck_idc_catalog_product_version_inventory_quantity
        CHECK ("sellable_quantity" >= 0),
    CONSTRAINT ck_idc_catalog_product_version_inventory_version
        CHECK ("version" >= 1)
);

INSERT INTO plugin_idc_core_catalog_product_version_inventory(
    "version_no","tracking_enabled","sellable_quantity","version"
)
SELECT "version"."version_no",FALSE,0,1
FROM plugin_idc_core_catalog_product_version "version"
ON CONFLICT("version_no") DO NOTHING;

-- The primary key is the exact lookup, row-lock, and conditional-update index
-- used by concurrent stock changes. A second index beginning with version_no
-- would duplicate that access path and is intentionally not created.
