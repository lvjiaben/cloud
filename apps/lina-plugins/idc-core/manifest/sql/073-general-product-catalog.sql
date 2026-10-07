-- 073: Remove the fixed server-family discriminator from the general catalog.
--
-- Existing products, versions, configuration rows, resource-pool bindings,
-- and their stable business numbers are deliberately preserved. Legacy
-- category numbers that do not satisfy the general catalog contract are
-- atomically rewritten together with every catalog and marketing reference;
-- the old-to-new mapping remains as migration evidence.
-- The legacy server-specific child tables remain migration sources for the
-- plugin-contract migration performed by the later FB-52 tasks.

DROP INDEX IF EXISTS idx_idc_catalog_product_management_type;
DROP INDEX IF EXISTS idx_idc_catalog_product_management_type_status;
DROP INDEX IF EXISTS idx_idc_catalog_product_public_type;

-- Purpose: Stores Legacy Server Category records.
-- 用途：保存旧版服务器分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_legacy_server_category (
    "tenant_id" BIGINT NOT NULL,
    "category_no" VARCHAR(64) NOT NULL,
    "category_type_code" VARCHAR(24) NOT NULL,
    "captured_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "category_no"),
    CONSTRAINT ck_idc_legacy_server_category_type CHECK (
        "category_type_code" IN ('cloud_server', 'bare_metal')
    )
);

DO $$
DECLARE
    "source_count" BIGINT;
    "evidence_count" BIGINT;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_core_catalog_category'
          AND column_name = 'category_type_code'
    ) THEN
        SELECT count(*) INTO "source_count"
        FROM plugin_idc_core_catalog_category;
        INSERT INTO plugin_idc_core_legacy_server_category (
            "tenant_id", "category_no", "category_type_code"
        )
        SELECT "tenant_id", "category_no", "category_type_code"
        FROM plugin_idc_core_catalog_category
        ON CONFLICT ("tenant_id", "category_no") DO UPDATE SET
            "category_type_code" = EXCLUDED."category_type_code";
        SELECT count(*) INTO "evidence_count"
        FROM plugin_idc_core_legacy_server_category;
        IF "evidence_count" <> "source_count" THEN
            RAISE EXCEPTION 'legacy category evidence mismatch: source %, evidence %', "source_count", "evidence_count";
        END IF;
    END IF;
END $$;

-- Preserve the exact old server discriminator before removing it from the
-- generic product identity. The later server-plugin migration must use this
-- evidence and must never guess from names or categories.
-- Purpose: Stores Legacy Server Product records.
-- 用途：保存旧版服务器商品相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_legacy_server_product (
    "tenant_id" BIGINT NOT NULL,
    "product_no" VARCHAR(64) NOT NULL,
    "category_no" VARCHAR(64) NOT NULL,
    "product_type_code" VARCHAR(24) NOT NULL,
    "configuration_mode_code" VARCHAR(24) NOT NULL,
    "captured_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "product_no"),
    CONSTRAINT ck_idc_legacy_server_product_type CHECK (
        "product_type_code" IN ('cloud_server', 'bare_metal')
    )
);

DO $$
DECLARE
    "source_count" BIGINT;
    "evidence_count" BIGINT;
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_core_catalog_product'
          AND column_name = 'product_type_code'
    ) THEN
        SELECT count(*) INTO "source_count"
        FROM plugin_idc_core_catalog_product;
        INSERT INTO plugin_idc_core_legacy_server_product (
            "tenant_id", "product_no", "category_no", "product_type_code",
            "configuration_mode_code"
        )
        SELECT "tenant_id", "product_no", "category_no", "product_type_code",
               "configuration_mode_code"
        FROM plugin_idc_core_catalog_product
        WHERE "product_type_code" IN ('cloud_server', 'bare_metal')
        ON CONFLICT ("tenant_id", "product_no") DO UPDATE SET
            "category_no" = EXCLUDED."category_no",
            "product_type_code" = EXCLUDED."product_type_code",
            "configuration_mode_code" = EXCLUDED."configuration_mode_code";
        SELECT count(*) INTO "evidence_count"
        FROM plugin_idc_core_legacy_server_product;
        IF "evidence_count" <> "source_count" THEN
            RAISE EXCEPTION 'legacy product evidence mismatch: source %, evidence %', "source_count", "evidence_count";
        END IF;
    END IF;
END $$;

ALTER TABLE plugin_idc_core_catalog_category
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_category_type;
ALTER TABLE plugin_idc_core_catalog_category
    DROP COLUMN IF EXISTS "category_type_code";

ALTER TABLE plugin_idc_core_catalog_product
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_product_type;
ALTER TABLE plugin_idc_core_catalog_product
    DROP COLUMN IF EXISTS "product_type_code";
ALTER TABLE plugin_idc_core_catalog_product
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_product_mode;
ALTER TABLE plugin_idc_core_catalog_product
    DROP COLUMN IF EXISTS "configuration_mode_code";

-- Purpose: Stores Catalog Category No Migration records.
-- 用途：保存目录分类编号迁移相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_category_no_migration (
    "tenant_id" BIGINT NOT NULL,
    "old_category_no" VARCHAR(64) NOT NULL,
    "new_category_no" VARCHAR(40) NOT NULL,
    "migrated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("tenant_id", "old_category_no"),
    CONSTRAINT uk_idc_catalog_category_no_migration_new UNIQUE ("new_category_no"),
    CONSTRAINT ck_idc_catalog_category_no_migration_new CHECK (
        "new_category_no" ~ '^ctg_[0-9a-f]{32}$'
    )
);

INSERT INTO plugin_idc_core_catalog_category_no_migration (
    "tenant_id", "old_category_no", "new_category_no"
)
SELECT "tenant_id", "category_no",
       'ctg_' || md5("tenant_id"::text || ':' || "category_no")
FROM plugin_idc_core_catalog_category
WHERE "category_no" !~ '^ctg_[0-9a-f]{32}$'
ON CONFLICT ("tenant_id", "old_category_no") DO NOTHING;

ALTER TABLE plugin_idc_core_catalog_product
    DROP CONSTRAINT IF EXISTS fk_idc_catalog_product_category;
ALTER TABLE plugin_idc_core_catalog_category
    DROP CONSTRAINT IF EXISTS fk_idc_catalog_category_parent;

UPDATE plugin_idc_core_catalog_product AS product
SET "category_no" = mapping."new_category_no"
FROM plugin_idc_core_catalog_category_no_migration AS mapping
WHERE product."tenant_id" = mapping."tenant_id"
  AND product."category_no" = mapping."old_category_no";

UPDATE plugin_idc_core_catalog_category AS "category"
SET "parent_category_no" = mapping."new_category_no"
FROM plugin_idc_core_catalog_category_no_migration AS mapping
WHERE "category"."tenant_id" = mapping."tenant_id"
  AND "category"."parent_category_no" = mapping."old_category_no";

-- Marketing category scopes were introduced after the original catalog. An
-- older development database can therefore reach this migration before that
-- optional table exists. When it does exist, keep the business references in
-- lockstep and remove only exact duplicates before rewriting the old number.
DO $$
BEGIN
    IF to_regclass('plugin_idc_core_marketing_campaign_scope') IS NOT NULL THEN
        DELETE FROM plugin_idc_core_marketing_campaign_scope AS old_scope
        USING plugin_idc_core_catalog_category_no_migration AS mapping,
              plugin_idc_core_marketing_campaign_scope AS current_scope
        WHERE old_scope."tenant_id" = mapping."tenant_id"
          AND old_scope."scope_kind_code" = 'category'
          AND old_scope."reference_code" = mapping."old_category_no"
          AND current_scope."tenant_id" = old_scope."tenant_id"
          AND current_scope."campaign_no" = old_scope."campaign_no"
          AND current_scope."scope_kind_code" = old_scope."scope_kind_code"
          AND current_scope."match_mode_code" = old_scope."match_mode_code"
          AND current_scope."reference_code" = mapping."new_category_no";

        UPDATE plugin_idc_core_marketing_campaign_scope AS "scope"
        SET "reference_code" = mapping."new_category_no"
        FROM plugin_idc_core_catalog_category_no_migration AS mapping
        WHERE "scope"."tenant_id" = mapping."tenant_id"
          AND "scope"."scope_kind_code" = 'category'
          AND "scope"."reference_code" = mapping."old_category_no";
    END IF;
END $$;

UPDATE plugin_idc_core_catalog_category AS "category"
SET "category_no" = mapping."new_category_no"
FROM plugin_idc_core_catalog_category_no_migration AS mapping
WHERE "category"."tenant_id" = mapping."tenant_id"
  AND "category"."category_no" = mapping."old_category_no";

ALTER TABLE plugin_idc_core_catalog_category
    ADD CONSTRAINT fk_idc_catalog_category_parent FOREIGN KEY ("parent_category_no")
        REFERENCES plugin_idc_core_catalog_category ("category_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_catalog_product
    ADD CONSTRAINT fk_idc_catalog_product_category FOREIGN KEY ("category_no")
        REFERENCES plugin_idc_core_catalog_category ("category_no") ON DELETE RESTRICT;

DO $$
DECLARE
    invalid_categories BIGINT;
    unmigrated_products BIGINT;
BEGIN
    SELECT count(*) INTO invalid_categories
    FROM plugin_idc_core_catalog_category
    WHERE "category_no" !~ '^ctg_[0-9a-f]{32}$'
       OR ("parent_category_no" IS NOT NULL AND
           "parent_category_no" !~ '^ctg_[0-9a-f]{32}$');
    SELECT count(*) INTO unmigrated_products
    FROM plugin_idc_core_catalog_product
    WHERE "category_no" !~ '^ctg_[0-9a-f]{32}$';
    IF invalid_categories <> 0 OR unmigrated_products <> 0 THEN
        RAISE EXCEPTION 'catalog category number migration incomplete: categories %, products %',
            invalid_categories, unmigrated_products;
    END IF;
END $$;

DELETE FROM sys_dict_data
WHERE "dict_type" = 'idc_catalog_category_type';

DELETE FROM sys_dict_type
WHERE "type" = 'idc_catalog_category_type';
