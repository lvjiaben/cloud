-- 094: Price merchant-selected generic configuration candidates without
-- interpreting product-specific field meaning. Labels and candidate identity
-- are frozen when the merchant explicitly creates a price dimension.

-- Purpose: Stores Catalog Configuration Price Dimension records.
-- 用途：保存目录配置价格维度相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_configuration_price_dimension (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "dimension_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "configuration_version" BIGINT NOT NULL,
    "field_key" VARCHAR(128) NOT NULL,
    "candidate_value" VARCHAR(256) NOT NULL,
    "field_label_zh_cn" VARCHAR(128) NOT NULL,
    "field_label_en_us" VARCHAR(128) NOT NULL,
    "candidate_label_zh_cn" VARCHAR(128) NOT NULL,
    "candidate_label_en_us" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_config_price_dimension_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_config_price_dimension_identity CHECK (
        "dimension_no" ~ '^cpd_[0-9a-f]{32}$'
        AND "configuration_version" > 0
        AND length(btrim("field_key")) BETWEEN 1 AND 128
        AND length(btrim("candidate_value")) BETWEEN 1 AND 256
    ),
    CONSTRAINT ck_idc_catalog_config_price_dimension_labels CHECK (
        length(btrim("field_label_zh_cn")) BETWEEN 1 AND 128
        AND length(btrim("field_label_en_us")) BETWEEN 1 AND 128
        AND length(btrim("candidate_label_zh_cn")) BETWEEN 1 AND 128
        AND length(btrim("candidate_label_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_catalog_config_price_dimension_status CHECK ("status_code" IN ('active','inactive')),
    CONSTRAINT uk_idc_catalog_config_price_dimension_no UNIQUE ("dimension_no"),
    CONSTRAINT uk_idc_catalog_config_price_dimension_version_no UNIQUE ("version_no","dimension_no"),
    CONSTRAINT uk_idc_catalog_config_price_dimension_value UNIQUE ("version_no","field_key","candidate_value"),
    CONSTRAINT fk_idc_catalog_config_price_dimension_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_config_price_dimension_version
    ON plugin_idc_core_catalog_configuration_price_dimension
       ("tenant_id","version_no","status_code","field_key","candidate_value","id");

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_configuration_price_dimension()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'configuration price dimensions are immutable';
END;
$$;
DROP TRIGGER IF EXISTS trg_idc_guard_configuration_price_dimension
    ON plugin_idc_core_catalog_configuration_price_dimension;
CREATE TRIGGER trg_idc_guard_configuration_price_dimension
BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_configuration_price_dimension
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_configuration_price_dimension();

ALTER TABLE plugin_idc_core_catalog_price
    ADD COLUMN IF NOT EXISTS "configuration_price_dimension_no" VARCHAR(40);

ALTER TABLE plugin_idc_core_catalog_price
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_price_target;
ALTER TABLE plugin_idc_core_catalog_price
    ADD CONSTRAINT ck_idc_catalog_price_target CHECK (
        NOT ("option_no" IS NOT NULL AND "configuration_price_dimension_no" IS NOT NULL)
    );

ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD COLUMN IF NOT EXISTS "customer_value_digest" CHAR(64);
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_config_customer_value_digest;
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD CONSTRAINT ck_idc_com_config_customer_value_digest CHECK (
        "customer_value_digest" IS NULL OR "customer_value_digest" ~ '^[0-9a-f]{64}$'
    );

-- Existing development cart snapshots cannot prove which complete customer
-- values were quoted. Keep the rows as evidence but require the customer to
-- select the configuration again; never invent a digest from partial data.
UPDATE plugin_idc_core_commerce_cart_item item
SET "status_code"='needs_reprice', "issue_code"='configuration_reselection_required',
    "version"="version"+1, "updated_at"=CURRENT_TIMESTAMP
FROM plugin_idc_core_commerce_configuration_snapshot snapshot
WHERE snapshot."configuration_snapshot_no"=item."configuration_snapshot_no"
  AND snapshot."customer_value_digest" IS NULL
  AND item."status_code"='active';

UPDATE plugin_idc_core_commerce_cart cart
SET "latest_quote_no"=NULL, "version"="version"+1, "updated_at"=CURRENT_TIMESTAMP
WHERE cart."status_code"='active'
  AND cart."latest_quote_no" IS NOT NULL
  AND EXISTS (
      SELECT 1 FROM plugin_idc_core_commerce_cart_item item
      WHERE item."cart_no"=cart."cart_no"
        AND item."status_code"='needs_reprice'
        AND item."issue_code"='configuration_reselection_required'
  );

DROP INDEX IF EXISTS uk_idc_catalog_price_base_tier;
DROP INDEX IF EXISTS uk_idc_catalog_price_option_tier;
DROP INDEX IF EXISTS uk_idc_catalog_base_price_dimension;
DROP INDEX IF EXISTS uk_idc_catalog_option_price_dimension;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_price_base_tier
    ON plugin_idc_core_catalog_price("version_no","billing_rule_no","currency_code","minimum_quantity")
    WHERE "option_no" IS NULL AND "configuration_price_dimension_no" IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_price_option_tier
    ON plugin_idc_core_catalog_price("version_no","billing_rule_no","option_no","currency_code","minimum_quantity")
    WHERE "option_no" IS NOT NULL AND "configuration_price_dimension_no" IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_price_configuration_tier
    ON plugin_idc_core_catalog_price("version_no","billing_rule_no","configuration_price_dimension_no","currency_code","minimum_quantity")
    WHERE "option_no" IS NULL AND "configuration_price_dimension_no" IS NOT NULL;

ALTER TABLE plugin_idc_core_catalog_price
    DROP CONSTRAINT IF EXISTS fk_idc_catalog_price_configuration_dimension;
ALTER TABLE plugin_idc_core_catalog_price
    ADD CONSTRAINT fk_idc_catalog_price_configuration_dimension FOREIGN KEY
        ("version_no","configuration_price_dimension_no")
        REFERENCES plugin_idc_core_catalog_configuration_price_dimension("version_no","dimension_no")
        ON DELETE RESTRICT;

-- Purpose: Stores Commerce Configuration Price Selection records.
-- 用途：保存交易配置价格选择相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_configuration_price_selection (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "dimension_no" VARCHAR(40) NOT NULL,
    "quantity" INT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_config_price_selection_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_config_price_selection_quantity CHECK ("quantity" BETWEEN 1 AND 128),
    CONSTRAINT uk_idc_com_config_price_selection UNIQUE ("configuration_snapshot_no","dimension_no"),
    CONSTRAINT fk_idc_com_config_price_selection_snapshot FOREIGN KEY
        ("configuration_snapshot_no","version_no")
        REFERENCES plugin_idc_core_commerce_configuration_snapshot("configuration_snapshot_no","version_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_config_price_selection_dimension FOREIGN KEY
        ("version_no","dimension_no")
        REFERENCES plugin_idc_core_catalog_configuration_price_dimension("version_no","dimension_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_config_price_selection_snapshot
    ON plugin_idc_core_commerce_configuration_price_selection
       ("tenant_id","configuration_snapshot_no","dimension_no");

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_configuration_price_selection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'configuration price selections are immutable';
END;
$$;
DROP TRIGGER IF EXISTS trg_idc_guard_configuration_price_selection
    ON plugin_idc_core_commerce_configuration_price_selection;
CREATE TRIGGER trg_idc_guard_configuration_price_selection
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_configuration_price_selection
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_configuration_price_selection();

ALTER TABLE plugin_idc_core_commerce_quote_charge_snapshot
    ADD COLUMN IF NOT EXISTS "configuration_price_dimension_no" VARCHAR(40);

ALTER TABLE plugin_idc_core_commerce_quote_charge_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_charge_kind,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_charge_option,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_charge_configuration_dimension;

ALTER TABLE plugin_idc_core_commerce_quote_charge_snapshot
    ADD CONSTRAINT ck_idc_com_quote_charge_kind CHECK (
        ("charge_kind_code"='base' AND "configuration_snapshot_no" IS NULL
            AND "option_no" IS NULL AND "configuration_price_dimension_no" IS NULL)
        OR ("charge_kind_code"='option' AND "configuration_snapshot_no" IS NOT NULL
            AND (("option_no" IS NOT NULL AND "configuration_price_dimension_no" IS NULL)
                OR ("option_no" IS NULL AND "configuration_price_dimension_no" IS NOT NULL)))
    ),
    ADD CONSTRAINT fk_idc_com_quote_charge_option FOREIGN KEY
        ("configuration_snapshot_no","option_no")
        REFERENCES plugin_idc_core_commerce_configuration_snapshot_item("configuration_snapshot_no","option_no")
        ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_com_quote_charge_configuration_dimension FOREIGN KEY
        ("configuration_snapshot_no","configuration_price_dimension_no")
        REFERENCES plugin_idc_core_commerce_configuration_price_selection("configuration_snapshot_no","dimension_no")
        ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_charge_configuration_dimension
    ON plugin_idc_core_commerce_quote_charge_snapshot
       ("tenant_id","configuration_snapshot_no","configuration_price_dimension_no")
    WHERE "configuration_price_dimension_no" IS NOT NULL;

-- Drop dependent evidence links before rebuilding their composite parent key
-- so the whole migration remains safe to replay.
ALTER TABLE plugin_idc_core_service_price_component_snapshot
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_component_configuration_header,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_component_configuration_selection;

ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD COLUMN IF NOT EXISTS "structured_configuration_snapshot_no" VARCHAR(40);
ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS uk_idc_service_price_configuration_evidence;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT uk_idc_service_price_configuration_evidence UNIQUE
        ("price_snapshot_no","configuration_snapshot_no");
ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_structured_configuration;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT fk_idc_service_price_structured_configuration FOREIGN KEY
        ("structured_configuration_snapshot_no")
        REFERENCES plugin_idc_core_commerce_structured_configuration_snapshot("snapshot_no")
        ON DELETE RESTRICT;

ALTER TABLE plugin_idc_core_service_price_component_snapshot
    ADD COLUMN IF NOT EXISTS "configuration_price_dimension_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "configuration_snapshot_no" VARCHAR(40);
ALTER TABLE plugin_idc_core_service_price_component_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_component_type,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_component_configuration_dimension,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_component_configuration_header,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_component_configuration_selection;
ALTER TABLE plugin_idc_core_service_price_component_snapshot
    ADD CONSTRAINT ck_idc_service_price_component_type CHECK (
        "component_type_code" IN ('base','option','discount','tax')
        AND "charge_phase_code" IN ('setup','periodic','total')
        AND (("component_type_code"='option' AND
                (("option_no" IS NOT NULL AND "configuration_price_dimension_no" IS NULL
                  AND "configuration_snapshot_no" IS NULL)
                 OR ("option_no" IS NULL AND "configuration_price_dimension_no" IS NOT NULL
                     AND "configuration_snapshot_no" IS NOT NULL)))
             OR ("component_type_code"<>'option' AND "option_no" IS NULL
                 AND "configuration_price_dimension_no" IS NULL
                 AND "configuration_snapshot_no" IS NULL))
    ),
    ADD CONSTRAINT fk_idc_service_price_component_configuration_dimension FOREIGN KEY
        ("configuration_price_dimension_no")
        REFERENCES plugin_idc_core_catalog_configuration_price_dimension("dimension_no")
        ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_service_price_component_configuration_header FOREIGN KEY
        ("price_snapshot_no","configuration_snapshot_no")
        REFERENCES plugin_idc_core_service_price_snapshot("price_snapshot_no","configuration_snapshot_no")
        ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_service_price_component_configuration_selection FOREIGN KEY
        ("configuration_snapshot_no","configuration_price_dimension_no")
        REFERENCES plugin_idc_core_commerce_configuration_price_selection("configuration_snapshot_no","dimension_no")
        ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_idc_service_price_component_configuration_dimension
    ON plugin_idc_core_service_price_component_snapshot
       ("tenant_id","configuration_price_dimension_no","price_snapshot_no")
    WHERE "configuration_price_dimension_no" IS NOT NULL;
