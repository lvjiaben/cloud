-- 010: Versioned general product catalog and pricing foundation
--
-- Draft catalog rows are recoverable and published versions remain immutable
-- history. The server configuration, supply, inventory, and cost tables later
-- in this migration are read-only migration sources until FB-52 moves them
-- behind server-plugin contracts; they are not fields of the general catalog.

-- Purpose: Recoverable merchant-defined general product category.
-- 用途：保存目录分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_category (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "category_no" VARCHAR(40) NOT NULL,
    "parent_category_no" VARCHAR(40),
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT ck_idc_catalog_category_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_category_status CHECK (
        "status_code" IN ('active', 'inactive')
    ),
    CONSTRAINT ck_idc_catalog_category_parent CHECK (
        "parent_category_no" IS NULL OR "parent_category_no" <> "category_no"
    ),
    CONSTRAINT uk_idc_catalog_category_no UNIQUE ("category_no"),
    CONSTRAINT fk_idc_catalog_category_parent FOREIGN KEY ("parent_category_no")
        REFERENCES plugin_idc_core_catalog_category ("category_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_category_tree
    ON plugin_idc_core_catalog_category (
        "tenant_id", "parent_category_no", "status_code", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL;

-- Purpose: Recoverable general product identity independent of fulfillment plugins.
-- 用途：保存目录商品相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "product_no" VARCHAR(40) NOT NULL,
    "category_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "public_slug" VARCHAR(128) NOT NULL,
    "name_zh_cn" VARCHAR(256) NOT NULL,
    "name_en_us" VARCHAR(256) NOT NULL,
    "summary_zh_cn" VARCHAR(1024) NOT NULL DEFAULT '',
    "summary_en_us" VARCHAR(1024) NOT NULL DEFAULT '',
    "current_version_no" VARCHAR(40),
    "sort_order" INT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at" TIMESTAMPTZ,
    CONSTRAINT ck_idc_catalog_product_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_product_status CHECK (
        "status_code" IN ('draft', 'active', 'suspended', 'retired')
    ),
    CONSTRAINT uk_idc_catalog_product_no UNIQUE ("product_no"),
    CONSTRAINT fk_idc_catalog_product_category FOREIGN KEY ("category_no")
        REFERENCES plugin_idc_core_catalog_category ("category_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_product_slug
    ON plugin_idc_core_catalog_product ("tenant_id", "public_slug")
    WHERE "deleted_at" IS NULL;
CREATE INDEX IF NOT EXISTS idx_idc_catalog_product_list
    ON plugin_idc_core_catalog_product (
        "tenant_id", "category_no", "status_code", "sort_order", "id"
    ) WHERE "deleted_at" IS NULL;

-- Purpose: Immutable-after-publication product delivery and quote version.
-- 用途：保存目录商品版本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "version_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_number" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "source_code" VARCHAR(16) NOT NULL DEFAULT 'manual',
    "source_version" VARCHAR(128) NOT NULL DEFAULT '',
    "source_fingerprint" CHAR(64) NOT NULL DEFAULT '',
    "change_summary" VARCHAR(1024) NOT NULL DEFAULT '',
    "requires_contract" BOOLEAN NOT NULL DEFAULT FALSE,
    "published_at" TIMESTAMPTZ,
    "retired_at" TIMESTAMPTZ,
    "created_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_version_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_version_number CHECK ("version_number" > 0),
    CONSTRAINT ck_idc_catalog_version_status CHECK (
        "status_code" IN ('draft', 'published', 'superseded', 'retired')
    ),
    CONSTRAINT ck_idc_catalog_version_source CHECK (
        "source_code" IN ('manual', 'migration', 'copy')
    ),
    CONSTRAINT ck_idc_catalog_version_fingerprint CHECK (
        "source_fingerprint" = '' OR "source_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_catalog_version_lifecycle CHECK (
        ("status_code" = 'draft' AND "published_at" IS NULL AND "retired_at" IS NULL)
        OR ("status_code" = 'published' AND "published_at" IS NOT NULL AND "retired_at" IS NULL)
        OR ("status_code" = 'superseded' AND "published_at" IS NOT NULL AND "retired_at" IS NULL)
        OR ("status_code" = 'retired' AND "published_at" IS NOT NULL AND "retired_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_catalog_version_no UNIQUE ("version_no"),
    CONSTRAINT uk_idc_catalog_product_version UNIQUE ("product_no", "version_number"),
    CONSTRAINT uk_idc_catalog_product_version_no UNIQUE ("product_no", "version_no"),
    CONSTRAINT fk_idc_catalog_version_product FOREIGN KEY ("product_no")
        REFERENCES plugin_idc_core_catalog_product ("product_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_version_list
    ON plugin_idc_core_catalog_product_version (
        "tenant_id", "product_no", "status_code", "version_number" DESC, "id"
    );
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_one_published_version
    ON plugin_idc_core_catalog_product_version ("tenant_id", "product_no")
    WHERE "status_code" = 'published';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_idc_catalog_product_current_version'
          AND conrelid = 'plugin_idc_core_catalog_product'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_catalog_product
            ADD CONSTRAINT fk_idc_catalog_product_current_version
            FOREIGN KEY ("product_no", "current_version_no")
            REFERENCES plugin_idc_core_catalog_product_version ("product_no", "version_no")
            ON DELETE RESTRICT;
    END IF;
END $$;

-- Purpose: Legacy server configuration migration source; generic writes are disabled.
-- 用途：保存目录配置分组相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_configuration_group (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "group_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "group_code" VARCHAR(64) NOT NULL,
    "selection_mode_code" VARCHAR(16) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "minimum_selected" INT NOT NULL DEFAULT 0,
    "maximum_selected" INT NOT NULL DEFAULT 1,
    "sort_order" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_group_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_group_mode CHECK (
        "selection_mode_code" IN ('single', 'multiple', 'quantity')
    ),
    CONSTRAINT ck_idc_catalog_group_selection CHECK (
        "minimum_selected" >= 0 AND "maximum_selected" >= "minimum_selected"
        AND "maximum_selected" <= 1000
    ),
    CONSTRAINT uk_idc_catalog_group_no UNIQUE ("group_no"),
    CONSTRAINT uk_idc_catalog_group_code UNIQUE ("version_no", "group_code"),
    CONSTRAINT uk_idc_catalog_version_group UNIQUE ("version_no", "group_no"),
    CONSTRAINT fk_idc_catalog_group_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_group_order
    ON plugin_idc_core_catalog_configuration_group (
        "tenant_id", "version_no", "sort_order", "id"
    );

-- Purpose: Legacy server option migration source; generic writes are disabled.
-- 用途：保存目录配置选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_configuration_option (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "option_no" VARCHAR(40) NOT NULL,
    "group_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "option_code" VARCHAR(64) NOT NULL,
    "component_type_code" VARCHAR(24) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "normalized_value" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "minimum_quantity" INT NOT NULL DEFAULT 0,
    "maximum_quantity" INT NOT NULL DEFAULT 1,
    "default_quantity" INT NOT NULL DEFAULT 0,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_option_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_option_component CHECK (
        "component_type_code" IN (
            'region', 'line', 'datacenter', 'cpu', 'memory', 'system_disk',
            'data_disk', 'ip', 'backup', 'image', 'traffic', 'bandwidth', 'other'
        )
    ),
    CONSTRAINT ck_idc_catalog_option_quantity CHECK (
        "minimum_quantity" >= 0 AND "maximum_quantity" >= "minimum_quantity"
        AND "maximum_quantity" <= 100000
        AND "default_quantity" BETWEEN "minimum_quantity" AND "maximum_quantity"
    ),
    CONSTRAINT ck_idc_catalog_option_status CHECK (
        "status_code" IN ('active', 'inactive')
    ),
    CONSTRAINT ck_idc_catalog_option_value CHECK (jsonb_typeof("normalized_value") = 'object'),
    CONSTRAINT uk_idc_catalog_option_no UNIQUE ("option_no"),
    CONSTRAINT uk_idc_catalog_option_code UNIQUE ("group_no", "option_code"),
    CONSTRAINT uk_idc_catalog_version_option UNIQUE ("version_no", "option_no"),
    CONSTRAINT fk_idc_catalog_option_group FOREIGN KEY ("version_no", "group_no")
        REFERENCES plugin_idc_core_catalog_configuration_group ("version_no", "group_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_option_order
    ON plugin_idc_core_catalog_configuration_option (
        "tenant_id", "group_no", "status_code", "sort_order", "id"
    );

-- Purpose: Legacy server compatibility migration source; generic writes are disabled.
-- 用途：保存目录兼容规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_compatibility_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "rule_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "rule_type_code" VARCHAR(16) NOT NULL,
    "source_option_no" VARCHAR(40) NOT NULL,
    "target_option_no" VARCHAR(40) NOT NULL,
    "minimum_target_quantity" INT NOT NULL DEFAULT 0,
    "maximum_target_quantity" INT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_compat_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_compat_type CHECK (
        "rule_type_code" IN ('requires', 'excludes', 'allows')
    ),
    CONSTRAINT ck_idc_catalog_compat_distinct CHECK ("source_option_no" <> "target_option_no"),
    CONSTRAINT ck_idc_catalog_compat_quantity CHECK (
        "minimum_target_quantity" >= 0
        AND ("maximum_target_quantity" IS NULL OR "maximum_target_quantity" >= "minimum_target_quantity")
    ),
    CONSTRAINT ck_idc_catalog_compat_status CHECK ("status_code" IN ('active', 'inactive')),
    CONSTRAINT uk_idc_catalog_compat_no UNIQUE ("rule_no"),
    CONSTRAINT uk_idc_catalog_compat_pair UNIQUE (
        "version_no", "rule_type_code", "source_option_no", "target_option_no"
    ),
    CONSTRAINT fk_idc_catalog_compat_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_catalog_compat_source FOREIGN KEY ("version_no", "source_option_no")
        REFERENCES plugin_idc_core_catalog_configuration_option ("version_no", "option_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_catalog_compat_target FOREIGN KEY ("version_no", "target_option_no")
        REFERENCES plugin_idc_core_catalog_configuration_option ("version_no", "option_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_compat_source
    ON plugin_idc_core_catalog_compatibility_rule (
        "tenant_id", "version_no", "source_option_no", "status_code", "id"
    );

-- Purpose: Versioned charge period and quantity semantics.
-- 用途：保存目录账务规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_billing_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "rule_code" VARCHAR(64) NOT NULL,
    "charge_type_code" VARCHAR(16) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL DEFAULT 1,
    "price_phase_code" VARCHAR(16) NOT NULL DEFAULT 'both',
    "quantity_mode_code" VARCHAR(16) NOT NULL DEFAULT 'fixed',
    "trial_quantity" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_billing_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_billing_charge CHECK (
        "charge_type_code" IN ('one_time', 'recurring', 'tiered', 'usage', 'trial')
    ),
    CONSTRAINT ck_idc_catalog_billing_period CHECK (
        "period_unit_code" IN ('once', 'hour', 'day', 'month', 'year')
        AND "period_count" BETWEEN 1 AND 120
        AND (("period_unit_code" = 'once' AND "period_count" = 1) OR "period_unit_code" <> 'once')
    ),
    CONSTRAINT ck_idc_catalog_billing_phase CHECK (
        "price_phase_code" IN ('new_purchase', 'renewal', 'both')
    ),
    CONSTRAINT ck_idc_catalog_billing_quantity CHECK (
        "quantity_mode_code" IN ('fixed', 'per_unit', 'tiered', 'usage')
        AND "trial_quantity" >= 0
    ),
    CONSTRAINT ck_idc_catalog_billing_status CHECK ("status_code" IN ('active', 'inactive')),
    CONSTRAINT uk_idc_catalog_billing_no UNIQUE ("billing_rule_no"),
    CONSTRAINT uk_idc_catalog_billing_code UNIQUE ("version_no", "rule_code"),
    CONSTRAINT uk_idc_catalog_version_billing UNIQUE ("version_no", "billing_rule_no"),
    CONSTRAINT fk_idc_catalog_billing_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_billing_version
    ON plugin_idc_core_catalog_billing_rule (
        "tenant_id", "version_no", "status_code", "id"
    );

-- Purpose: Explicit minor-unit price for one currency and billing dimension.
-- 用途：保存目录价格相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_price (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "price_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "option_no" VARCHAR(40),
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "setup_fee_minor" BIGINT NOT NULL DEFAULT 0,
    "renewal_amount_minor" BIGINT,
    "minimum_quantity" BIGINT NOT NULL DEFAULT 0,
    "maximum_quantity" BIGINT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_price_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_price_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_catalog_price_amount CHECK (
        "amount_minor" >= 0 AND "setup_fee_minor" >= 0
        AND ("renewal_amount_minor" IS NULL OR "renewal_amount_minor" >= 0)
    ),
    CONSTRAINT ck_idc_catalog_price_quantity CHECK (
        "minimum_quantity" >= 0
        AND ("maximum_quantity" IS NULL OR "maximum_quantity" >= "minimum_quantity")
    ),
    CONSTRAINT ck_idc_catalog_price_status CHECK ("status_code" IN ('active', 'inactive')),
    CONSTRAINT uk_idc_catalog_price_no UNIQUE ("price_no"),
    CONSTRAINT fk_idc_catalog_price_billing FOREIGN KEY ("version_no", "billing_rule_no")
        REFERENCES plugin_idc_core_catalog_billing_rule ("version_no", "billing_rule_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_catalog_price_option FOREIGN KEY ("version_no", "option_no")
        REFERENCES plugin_idc_core_catalog_configuration_option ("version_no", "option_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_base_price_dimension
    ON plugin_idc_core_catalog_price (
        "version_no", "billing_rule_no", "currency_code", "minimum_quantity"
    ) WHERE "option_no" IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_option_price_dimension
    ON plugin_idc_core_catalog_price (
        "version_no", "billing_rule_no", "option_no", "currency_code", "minimum_quantity"
    ) WHERE "option_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_catalog_price_quote
    ON plugin_idc_core_catalog_price (
        "tenant_id", "version_no", "billing_rule_no", "currency_code", "status_code", "option_no", "minimum_quantity"
    );

-- Purpose: Legacy server pool-binding migration source; generic writes are disabled.
-- 用途：保存目录资源资源池绑定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_resource_pool_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "binding_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "pool_no" VARCHAR(64) NOT NULL,
    "region_code" VARCHAR(64) NOT NULL DEFAULT '',
    "line_code" VARCHAR(64) NOT NULL DEFAULT '',
    "priority" INT NOT NULL DEFAULT 100,
    "weight" INT NOT NULL DEFAULT 100,
    "new_order_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_pool_binding_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_pool_binding_route CHECK (
        "priority" BETWEEN 0 AND 100000 AND "weight" BETWEEN 0 AND 100000
    ),
    CONSTRAINT ck_idc_catalog_pool_binding_status CHECK ("status_code" IN ('active', 'inactive')),
    CONSTRAINT uk_idc_catalog_pool_binding_no UNIQUE ("binding_no"),
    CONSTRAINT uk_idc_catalog_pool_binding_route UNIQUE (
        "version_no", "pool_no", "region_code", "line_code"
    ),
    CONSTRAINT fk_idc_catalog_pool_binding_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_pool_binding_route
    ON plugin_idc_core_catalog_resource_pool_binding (
        "tenant_id", "version_no", "status_code", "new_order_enabled", "priority", "id"
    );

-- Purpose: Legacy server inventory migration source; generic writes are disabled.
-- 用途：保存目录库存快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_inventory_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "binding_no" VARCHAR(40) NOT NULL,
    "source_snapshot_no" VARCHAR(64) NOT NULL,
    "source_fingerprint" CHAR(64) NOT NULL,
    "availability_code" VARCHAR(16) NOT NULL,
    "available_quantity" BIGINT,
    "safety_margin" BIGINT NOT NULL DEFAULT 0,
    "sellable_quantity" BIGINT,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_inventory_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_inventory_fingerprint CHECK ("source_fingerprint" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_catalog_inventory_availability CHECK (
        "availability_code" IN ('available', 'limited', 'unavailable', 'unknown', 'stale')
    ),
    CONSTRAINT ck_idc_catalog_inventory_quantity CHECK (
        ("available_quantity" IS NULL OR "available_quantity" >= 0)
        AND "safety_margin" >= 0
        AND ("sellable_quantity" IS NULL OR "sellable_quantity" >= 0)
        AND ("available_quantity" IS NULL OR "sellable_quantity" IS NULL OR "sellable_quantity" <= "available_quantity")
    ),
    CONSTRAINT ck_idc_catalog_inventory_window CHECK ("expires_at" > "observed_at"),
    CONSTRAINT uk_idc_catalog_inventory_no UNIQUE ("snapshot_no"),
    CONSTRAINT uk_idc_catalog_inventory_source UNIQUE ("binding_no", "source_snapshot_no"),
    CONSTRAINT fk_idc_catalog_inventory_binding FOREIGN KEY ("binding_no")
        REFERENCES plugin_idc_core_catalog_resource_pool_binding ("binding_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_inventory_latest
    ON plugin_idc_core_catalog_inventory_snapshot (
        "tenant_id", "binding_no", "observed_at" DESC, "id" DESC
    );

-- Purpose: Legacy server cost migration source; generic writes are disabled.
-- 用途：保存目录成本快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_cost_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cost_snapshot_no" VARCHAR(40) NOT NULL,
    "binding_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(16) NOT NULL,
    "source_snapshot_no" VARCHAR(64) NOT NULL DEFAULT '',
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL DEFAULT 1,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "valid_until" TIMESTAMPTZ,
    "created_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_cost_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_cost_source CHECK ("source_code" IN ('manual', 'provider')),
    CONSTRAINT ck_idc_catalog_cost_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_catalog_cost_amount CHECK ("amount_minor" >= 0),
    CONSTRAINT ck_idc_catalog_cost_period CHECK (
        "period_unit_code" IN ('once', 'hour', 'day', 'month', 'year')
        AND "period_count" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_catalog_cost_source_ref CHECK (
        ("source_code" = 'manual' AND "source_snapshot_no" = '')
        OR ("source_code" = 'provider' AND "source_snapshot_no" <> '')
    ),
    CONSTRAINT ck_idc_catalog_cost_window CHECK ("valid_until" IS NULL OR "valid_until" > "observed_at"),
    CONSTRAINT uk_idc_catalog_cost_no UNIQUE ("cost_snapshot_no"),
    CONSTRAINT fk_idc_catalog_cost_binding FOREIGN KEY ("binding_no")
        REFERENCES plugin_idc_core_catalog_resource_pool_binding ("binding_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_cost_latest
    ON plugin_idc_core_catalog_cost_snapshot (
        "tenant_id", "binding_no", "currency_code", "observed_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_catalog_category IS 'Recoverable merchant-defined general product category';
COMMENT ON TABLE plugin_idc_core_catalog_product IS 'Recoverable general product identity independent of fulfillment plugins';
COMMENT ON TABLE plugin_idc_core_catalog_product_version IS 'Immutable-after-publication product delivery and quote version';
COMMENT ON TABLE plugin_idc_core_catalog_configuration_group IS 'Legacy server configuration migration source; generic writes are disabled';
COMMENT ON TABLE plugin_idc_core_catalog_configuration_option IS 'Legacy server option migration source; generic writes are disabled';
COMMENT ON COLUMN plugin_idc_core_catalog_configuration_option."normalized_value" IS 'Business-normalized value object; raw Provider responses and credentials are forbidden';
COMMENT ON TABLE plugin_idc_core_catalog_compatibility_rule IS 'Legacy server compatibility migration source; generic writes are disabled';
COMMENT ON TABLE plugin_idc_core_catalog_billing_rule IS 'Versioned charge period and quantity semantics';
COMMENT ON TABLE plugin_idc_core_catalog_price IS 'Explicit minor-unit price for one currency and billing dimension';
COMMENT ON COLUMN plugin_idc_core_catalog_price."option_no" IS 'Null for the product base price, otherwise a version-owned option business number';
COMMENT ON TABLE plugin_idc_core_catalog_resource_pool_binding IS 'Legacy server pool-binding migration source; generic writes are disabled';
COMMENT ON COLUMN plugin_idc_core_catalog_resource_pool_binding."pool_no" IS 'Opaque resource-owner pool business number; no cross-plugin foreign key';
COMMENT ON TABLE plugin_idc_core_catalog_inventory_snapshot IS 'Legacy server inventory migration source; generic writes are disabled';
COMMENT ON TABLE plugin_idc_core_catalog_cost_snapshot IS 'Legacy server cost migration source; generic writes are disabled';

INSERT INTO sys_dict_type (
    "tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at"
)
VALUES
    (0, 'IDC 商品状态', 'idc_catalog_product_status', 1, 1, FALSE, '商品生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 商品版本状态', 'idc_catalog_version_status', 1, 1, FALSE, '商品发布版本生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 商品版本来源', 'idc_catalog_version_source', 1, 1, FALSE, '商品草稿与版本来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 计费类型', 'idc_catalog_charge_type', 1, 1, FALSE, '商品一次性、周期、阶梯、按量和试用计费', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 计费周期单位', 'idc_catalog_period_unit', 1, 1, FALSE, '商品计费周期单位', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 价格阶段', 'idc_catalog_price_phase', 1, 1, FALSE, '新购与续费价格适用阶段', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 计费数量模式', 'idc_catalog_quantity_mode', 1, 1, FALSE, '固定、按数量、阶梯和按量计费方式', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 商品配置状态', 'idc_catalog_activation_status', 1, 1, FALSE, '商品配置记录启停状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at"
)
VALUES
    (0, 'idc_catalog_product_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_product_status', '销售中', 'active', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_product_status', '已暂停', 'suspended', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_product_status', '已退市', 'retired', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_status', '已发布', 'published', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_status', '已被新版本替代', 'superseded', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_status', '已退役', 'retired', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_source', '人工创建', 'manual', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_source', '数据迁移', 'migration', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_version_source', '复制商品', 'copy', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_charge_type', '一次性', 'one_time', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_charge_type', '周期', 'recurring', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_charge_type', '阶梯', 'tiered', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_charge_type', '按量', 'usage', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_charge_type', '试用', 'trial', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_period_unit', '一次', 'once', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_period_unit', '小时', 'hour', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_period_unit', '天', 'day', 3, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_period_unit', '月', 'month', 4, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_period_unit', '年', 'year', 5, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_price_phase', '仅新购', 'new_purchase', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_price_phase', '仅续费', 'renewal', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_price_phase', '新购与续费', 'both', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_quantity_mode', '固定金额', 'fixed', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_quantity_mode', '按数量', 'per_unit', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_quantity_mode', '阶梯', 'tiered', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_quantity_mode', '按量', 'usage', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_activation_status', '启用', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_activation_status', '停用', 'inactive', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
