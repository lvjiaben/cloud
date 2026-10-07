-- 022: Normalized cart, immutable quote/order snapshots, inventory holds, and fulfillment units.

-- These composite keys let commerce references prove that a selected child
-- belongs to the same immutable catalog version and inventory route.
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_option_version_group_no
    ON plugin_idc_core_catalog_configuration_option (
        "version_no", "group_no", "option_no"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_binding_version_no
    ON plugin_idc_core_catalog_resource_pool_binding (
        "version_no", "binding_no"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_inventory_binding_snapshot
    ON plugin_idc_core_catalog_inventory_snapshot (
        "binding_no", "snapshot_no"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_catalog_tax_version_currency_no
    ON plugin_idc_core_catalog_tax_policy (
        "version_no", "currency_code", "tax_policy_no"
    );

-- Purpose: Single-currency editable purchase intent owned by one guest token digest or customer.
-- 用途：保存交易购物车相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_cart (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cart_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40),
    "guest_owner_digest" CHAR(64),
    "currency_code" CHAR(3) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "latest_quote_no" VARCHAR(40),
    "merged_into_cart_no" VARCHAR(40),
    "expires_at" TIMESTAMPTZ NOT NULL,
    "closed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_cart_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_cart_owner CHECK (
        (("customer_no" IS NOT NULL)::INT + ("guest_owner_digest" IS NOT NULL)::INT) = 1
        AND ("guest_owner_digest" IS NULL OR "guest_owner_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_com_cart_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_com_cart_status CHECK (
        "status_code" IN ('active', 'checked_out', 'merged', 'expired', 'abandoned')
    ),
    CONSTRAINT ck_idc_com_cart_lifecycle CHECK (
        ("status_code" = 'active' AND "closed_at" IS NULL AND "merged_into_cart_no" IS NULL)
        OR ("status_code" IN ('checked_out', 'expired', 'abandoned')
            AND "closed_at" IS NOT NULL AND "merged_into_cart_no" IS NULL)
        OR ("status_code" = 'merged' AND "closed_at" IS NOT NULL
            AND "merged_into_cart_no" IS NOT NULL
            AND "merged_into_cart_no" <> "cart_no")
    ),
    CONSTRAINT ck_idc_com_cart_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT ck_idc_com_cart_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_com_cart_no UNIQUE ("cart_no"),
    CONSTRAINT uk_idc_com_cart_currency UNIQUE ("cart_no", "currency_code"),
    CONSTRAINT fk_idc_com_cart_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_cart_merge_target FOREIGN KEY ("merged_into_cart_no")
        REFERENCES plugin_idc_core_commerce_cart ("cart_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_cart_active_customer
    ON plugin_idc_core_commerce_cart ("tenant_id", "customer_no")
    WHERE "status_code" = 'active' AND "customer_no" IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_cart_active_guest
    ON plugin_idc_core_commerce_cart ("tenant_id", "guest_owner_digest")
    WHERE "status_code" = 'active' AND "guest_owner_digest" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_com_cart_expiry
    ON plugin_idc_core_commerce_cart (
        "tenant_id", "status_code", "expires_at", "id"
    );

-- Purpose: Append-only normalized server configuration selection header.
-- 用途：保存交易配置快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_configuration_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "source_revision_digest" CHAR(64) NOT NULL,
    "selection_digest" CHAR(64) NOT NULL,
    "selected_option_count" INT NOT NULL,
    "total_selected_quantity" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_config_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_config_digests CHECK (
        "source_revision_digest" ~ '^[0-9a-f]{64}$'
        AND "selection_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_config_counts CHECK (
        "selected_option_count" BETWEEN 1 AND 5000
        AND "total_selected_quantity" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT uk_idc_com_config_no UNIQUE ("configuration_snapshot_no"),
    CONSTRAINT uk_idc_com_config_version UNIQUE (
        "configuration_snapshot_no", "version_no"
    ),
    CONSTRAINT fk_idc_com_config_product_version FOREIGN KEY ("product_no", "version_no")
        REFERENCES plugin_idc_core_catalog_product_version (
            "product_no", "version_no"
        ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_config_product
    ON plugin_idc_core_commerce_configuration_snapshot (
        "tenant_id", "product_no", "version_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only selected catalog option and safe display/value snapshot.
-- 用途：保存交易配置快照明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_configuration_snapshot_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_item_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "group_no" VARCHAR(40) NOT NULL,
    "option_no" VARCHAR(40) NOT NULL,
    "group_code" VARCHAR(64) NOT NULL,
    "option_code" VARCHAR(64) NOT NULL,
    "component_type_code" VARCHAR(24) NOT NULL,
    "group_name_zh_cn" VARCHAR(128) NOT NULL,
    "group_name_en_us" VARCHAR(128) NOT NULL,
    "option_name_zh_cn" VARCHAR(128) NOT NULL,
    "option_name_en_us" VARCHAR(128) NOT NULL,
    "normalized_value" JSONB NOT NULL,
    "quantity" INT NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_config_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_config_item_component CHECK (
        "component_type_code" IN (
            'region', 'line', 'datacenter', 'cpu', 'memory', 'system_disk',
            'data_disk', 'ip', 'backup', 'image', 'traffic', 'bandwidth', 'other'
        )
    ),
    CONSTRAINT ck_idc_com_config_item_value CHECK (
        jsonb_typeof("normalized_value") = 'object'
    ),
    CONSTRAINT ck_idc_com_config_item_quantity CHECK (
        "quantity" BETWEEN 1 AND 100000
    ),
    CONSTRAINT uk_idc_com_config_item_no UNIQUE ("snapshot_item_no"),
    CONSTRAINT uk_idc_com_config_item_option UNIQUE (
        "configuration_snapshot_no", "option_no"
    ),
    CONSTRAINT fk_idc_com_config_item_snapshot FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_config_item_catalog FOREIGN KEY (
        "version_no", "group_no", "option_no"
    ) REFERENCES plugin_idc_core_catalog_configuration_option (
        "version_no", "group_no", "option_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_config_item_order
    ON plugin_idc_core_commerce_configuration_snapshot_item (
        "tenant_id", "configuration_snapshot_no", "sort_order", "id"
    );

-- Purpose: Editable cart line referencing one immutable configuration snapshot.
-- 用途：保存交易购物车明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_cart_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cart_item_no" VARCHAR(40) NOT NULL,
    "cart_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "server_quantity" INT NOT NULL DEFAULT 1,
    "purchase_cycles" INT NOT NULL DEFAULT 1,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'active',
    "issue_code" VARCHAR(64) NOT NULL DEFAULT '',
    "sort_order" INT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "removed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_cart_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_cart_item_quantity CHECK (
        "server_quantity" BETWEEN 1 AND 1000 AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_com_cart_item_status CHECK (
        "status_code" IN ('active', 'needs_reprice', 'unavailable', 'removed')
    ),
    CONSTRAINT ck_idc_com_cart_item_issue CHECK (
        ("status_code" = 'active' AND "issue_code" = '' AND "removed_at" IS NULL)
        OR ("status_code" IN ('needs_reprice', 'unavailable')
            AND "issue_code" ~ '^[a-z][a-z0-9_.-]{0,63}$' AND "removed_at" IS NULL)
        OR ("status_code" = 'removed'
            AND "issue_code" ~ '^[a-z][a-z0-9_.-]{0,63}$' AND "removed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_com_cart_item_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_com_cart_item_no UNIQUE ("cart_item_no"),
    CONSTRAINT uk_idc_com_cart_item_cart_no UNIQUE ("cart_no", "cart_item_no"),
    CONSTRAINT fk_idc_com_cart_item_cart FOREIGN KEY ("cart_no")
        REFERENCES plugin_idc_core_commerce_cart ("cart_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_cart_item_product_version FOREIGN KEY ("product_no", "version_no")
        REFERENCES plugin_idc_core_catalog_product_version (
            "product_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_cart_item_billing FOREIGN KEY ("version_no", "billing_rule_no")
        REFERENCES plugin_idc_core_catalog_billing_rule (
            "version_no", "billing_rule_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_cart_item_config FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_cart_item_order
    ON plugin_idc_core_commerce_cart_item (
        "tenant_id", "cart_no", "status_code", "sort_order", "id"
    );

-- Purpose: Append-only deterministic checkout quote header with local-only rule digests.
-- 用途：保存交易报价快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_no" VARCHAR(40) NOT NULL,
    "cart_no" VARCHAR(40),
    "customer_no" VARCHAR(40),
    "customer_subject_version" BIGINT,
    "purpose_code" VARCHAR(24) NOT NULL DEFAULT 'new_purchase',
    "source_code" VARCHAR(16) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "item_count" INT NOT NULL,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "input_digest" CHAR(64) NOT NULL,
    "pricing_rule_digest" CHAR(64) NOT NULL,
    "eligibility_digest" CHAR(64) NOT NULL,
    "inventory_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_by" INT NOT NULL DEFAULT 0,
    CONSTRAINT ck_idc_com_quote_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_quote_owner CHECK (
        "cart_no" IS NOT NULL OR "customer_no" IS NOT NULL
    ),
    CONSTRAINT ck_idc_com_quote_customer_version CHECK (
        ("customer_no" IS NULL AND "customer_subject_version" IS NULL)
        OR ("customer_no" IS NOT NULL AND "customer_subject_version" > 0)
    ),
    CONSTRAINT ck_idc_com_quote_purpose CHECK (
        "purpose_code" IN ('new_purchase', 'renewal', 'change', 'ip_change', 'addon')
    ),
    CONSTRAINT ck_idc_com_quote_source CHECK (
        "source_code" IN ('guest', 'customer', 'staff', 'channel', 'migration', 'system')
    ),
    CONSTRAINT ck_idc_com_quote_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_com_quote_amount CHECK (
        "item_count" BETWEEN 1 AND 1000
        AND "subtotal_minor" >= 0 AND "discount_minor" >= 0
        AND "discount_minor" <= "subtotal_minor"
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
    ),
    CONSTRAINT ck_idc_com_quote_digests CHECK (
        "input_digest" ~ '^[0-9a-f]{64}$'
        AND "pricing_rule_digest" ~ '^[0-9a-f]{64}$'
        AND "eligibility_digest" ~ '^[0-9a-f]{64}$'
        AND "inventory_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_quote_expiry CHECK ("expires_at" > "issued_at"),
    CONSTRAINT uk_idc_com_quote_no UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_com_quote_cart_no UNIQUE ("cart_no", "quote_no"),
    CONSTRAINT uk_idc_com_quote_currency UNIQUE ("quote_no", "currency_code"),
    CONSTRAINT uk_idc_com_quote_cart_currency UNIQUE (
        "quote_no", "cart_no", "currency_code"
    ),
    CONSTRAINT uk_idc_com_quote_customer_currency UNIQUE (
        "quote_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_com_quote_cart FOREIGN KEY ("cart_no", "currency_code")
        REFERENCES plugin_idc_core_commerce_cart (
            "cart_no", "currency_code"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_cart
    ON plugin_idc_core_commerce_quote_snapshot (
        "tenant_id", "cart_no", "issued_at" DESC, "id" DESC
    ) WHERE "cart_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_customer_expiry
    ON plugin_idc_core_commerce_quote_snapshot (
        "tenant_id", "customer_no", "expires_at", "id"
    ) WHERE "customer_no" IS NOT NULL;

-- Purpose: Append-only product, tax, route, inventory, policy, and amount quote item.
-- 用途：保存交易报价明细快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_item_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "cart_no" VARCHAR(40),
    "cart_item_no" VARCHAR(40),
    "line_number" INT NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "product_type_code" VARCHAR(24) NOT NULL,
    "product_name_zh_cn" VARCHAR(256) NOT NULL,
    "product_name_en_us" VARCHAR(256) NOT NULL,
    "server_quantity" INT NOT NULL,
    "purchase_cycles" INT NOT NULL,
    "charge_type_code" VARCHAR(16) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "requires_contract" BOOLEAN NOT NULL DEFAULT FALSE,
    "fulfillment_mode_code" VARCHAR(16) NOT NULL,
    "risk_decision_code" VARCHAR(16) NOT NULL,
    "binding_no" VARCHAR(40) NOT NULL,
    "inventory_snapshot_no" VARCHAR(40),
    "inventory_decision_code" VARCHAR(24) NOT NULL,
    "route_decision_digest" CHAR(64) NOT NULL,
    "tax_policy_no" VARCHAR(40) NOT NULL,
    "price_includes_tax" BOOLEAN NOT NULL,
    "tax_treatment_code" VARCHAR(16) NOT NULL,
    "tax_rate_basis_points" INT,
    "tax_category_code" VARCHAR(64) NOT NULL,
    "rounding_scope_code" VARCHAR(16) NOT NULL,
    "rounding_mode_code" VARCHAR(16) NOT NULL,
    "rounding_quantum_minor" BIGINT NOT NULL,
    "setup_subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    "period_subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_quote_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_quote_item_line CHECK (
        "line_number" BETWEEN 1 AND 1000
        AND "server_quantity" BETWEEN 1 AND 1000
        AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_com_quote_item_cart CHECK (
        ("cart_no" IS NULL AND "cart_item_no" IS NULL)
        OR ("cart_no" IS NOT NULL AND "cart_item_no" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_com_quote_item_product CHECK (
        "product_type_code" IN ('cloud_server', 'bare_metal')
    ),
    CONSTRAINT ck_idc_com_quote_item_billing CHECK (
        "charge_type_code" IN ('one_time', 'recurring', 'tiered', 'usage', 'trial')
        AND (
            ("period_unit_code" = 'once' AND "period_count" = 1 AND "purchase_cycles" = 1)
            OR ("period_unit_code" = 'hour' AND "period_count" BETWEEN 1 AND 87600)
            OR ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
            OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
            OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10)
        )
    ),
    CONSTRAINT ck_idc_com_quote_item_policy CHECK (
        "fulfillment_mode_code" IN ('automatic', 'manual_review')
        AND "risk_decision_code" IN ('approved', 'manual_review')
    ),
    CONSTRAINT ck_idc_com_quote_item_inventory CHECK (
        "inventory_decision_code" IN ('available', 'manual_confirmation')
        AND (
            ("inventory_decision_code" = 'available' AND "inventory_snapshot_no" IS NOT NULL)
            OR "inventory_decision_code" = 'manual_confirmation'
        )
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_quote_item_tax CHECK (
        "tax_treatment_code" IN ('taxable', 'zero_rated', 'exempt')
        AND (
            ("tax_treatment_code" = 'taxable' AND "tax_rate_basis_points" BETWEEN 0 AND 10000)
            OR ("tax_treatment_code" = 'zero_rated' AND "tax_rate_basis_points" = 0)
            OR ("tax_treatment_code" = 'exempt' AND "tax_rate_basis_points" IS NULL)
        )
        AND "tax_category_code" ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
        AND "rounding_scope_code" IN ('line', 'total')
        AND "rounding_mode_code" IN ('half_up', 'down', 'up')
        AND "rounding_quantum_minor" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_idc_com_quote_item_amount CHECK (
        "setup_subtotal_minor" >= 0 AND "period_subtotal_minor" >= 0
        AND "subtotal_minor" = "setup_subtotal_minor" + "period_subtotal_minor"
        AND "discount_minor" >= 0 AND "discount_minor" <= "subtotal_minor"
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_com_quote_item_no UNIQUE ("quote_item_no"),
    CONSTRAINT uk_idc_com_quote_item_line UNIQUE ("quote_no", "line_number"),
    CONSTRAINT uk_idc_com_quote_item_currency UNIQUE ("quote_item_no", "currency_code"),
    CONSTRAINT uk_idc_com_quote_item_config_no UNIQUE (
        "quote_item_no", "configuration_snapshot_no"
    ),
    CONSTRAINT uk_idc_com_quote_item_quote_currency UNIQUE (
        "quote_no", "quote_item_no", "currency_code"
    ),
    CONSTRAINT fk_idc_com_quote_item_quote FOREIGN KEY ("quote_no", "currency_code")
        REFERENCES plugin_idc_core_commerce_quote_snapshot (
            "quote_no", "currency_code"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_quote_cart FOREIGN KEY (
        "quote_no", "cart_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_snapshot (
        "quote_no", "cart_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_cart_item FOREIGN KEY (
        "cart_no", "cart_item_no"
    ) REFERENCES plugin_idc_core_commerce_cart_item (
        "cart_no", "cart_item_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_product_version FOREIGN KEY ("product_no", "version_no")
        REFERENCES plugin_idc_core_catalog_product_version (
            "product_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_billing FOREIGN KEY ("version_no", "billing_rule_no")
        REFERENCES plugin_idc_core_catalog_billing_rule (
            "version_no", "billing_rule_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_config FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_binding FOREIGN KEY ("version_no", "binding_no")
        REFERENCES plugin_idc_core_catalog_resource_pool_binding (
            "version_no", "binding_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_inventory FOREIGN KEY (
        "binding_no", "inventory_snapshot_no"
    ) REFERENCES plugin_idc_core_catalog_inventory_snapshot (
        "binding_no", "snapshot_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_item_tax FOREIGN KEY (
        "version_no", "currency_code", "tax_policy_no"
    ) REFERENCES plugin_idc_core_catalog_tax_policy (
        "version_no", "currency_code", "tax_policy_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_item_quote
    ON plugin_idc_core_commerce_quote_item_snapshot (
        "tenant_id", "quote_no", "line_number", "id"
    );

-- Purpose: Append-only base or selected-option price composition for one quote item.
-- 用途：保存交易报价费用快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_charge_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_charge_no" VARCHAR(40) NOT NULL,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "line_number" INT NOT NULL,
    "charge_kind_code" VARCHAR(16) NOT NULL,
    "charge_phase_code" VARCHAR(16) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40),
    "option_no" VARCHAR(40),
    "quantity" INT NOT NULL,
    "charge_amount_minor" BIGINT NOT NULL,
    "setup_fee_minor" BIGINT NOT NULL DEFAULT 0,
    "priced_amount_minor" BIGINT NOT NULL,
    "price_source_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_quote_charge_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_quote_charge_kind CHECK (
        ("charge_kind_code" = 'base'
            AND "configuration_snapshot_no" IS NULL AND "option_no" IS NULL)
        OR ("charge_kind_code" = 'option'
            AND "configuration_snapshot_no" IS NOT NULL AND "option_no" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_com_quote_charge_phase CHECK (
        "charge_phase_code" IN ('setup', 'periodic')
    ),
    CONSTRAINT ck_idc_com_quote_charge_amount CHECK (
        "line_number" BETWEEN 1 AND 4096 AND "quantity" BETWEEN 1 AND 100000
        AND "charge_amount_minor" >= 0 AND "setup_fee_minor" >= 0
        AND "priced_amount_minor" = "charge_amount_minor" + "setup_fee_minor"
        AND "price_source_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_com_quote_charge_no UNIQUE ("quote_charge_no"),
    CONSTRAINT uk_idc_com_quote_charge_line UNIQUE ("quote_item_no", "line_number"),
    CONSTRAINT fk_idc_com_quote_charge_item FOREIGN KEY ("quote_item_no", "currency_code")
        REFERENCES plugin_idc_core_commerce_quote_item_snapshot (
            "quote_item_no", "currency_code"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_charge_item_config FOREIGN KEY (
        "quote_item_no", "configuration_snapshot_no"
    ) REFERENCES plugin_idc_core_commerce_quote_item_snapshot (
        "quote_item_no", "configuration_snapshot_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_charge_option FOREIGN KEY (
        "configuration_snapshot_no", "option_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot_item (
        "configuration_snapshot_no", "option_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_charge_item
    ON plugin_idc_core_commerce_quote_charge_snapshot (
        "tenant_id", "quote_item_no", "line_number", "id"
    );

-- Purpose: Append-only promotion, customer, agent, or approved manual quote adjustment.
-- 用途：保存交易报价调整快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_adjustment_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_adjustment_no" VARCHAR(40) NOT NULL,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "application_order" INT NOT NULL,
    "adjustment_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "source_version_digest" CHAR(64) NOT NULL,
    "reservation_reference" VARCHAR(128),
    "amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_quote_adjustment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_quote_adjustment_type CHECK (
        "adjustment_type_code" IN (
            'campaign_discount', 'coupon', 'voucher', 'customer_price',
            'agent_price', 'manual_discount'
        )
    ),
    CONSTRAINT ck_idc_com_quote_adjustment_source CHECK (
        length(btrim("source_reference")) BETWEEN 1 AND 128
        AND "source_version_digest" ~ '^[0-9a-f]{64}$'
        AND ("reservation_reference" IS NULL
            OR length(btrim("reservation_reference")) BETWEEN 1 AND 128)
    ),
    CONSTRAINT ck_idc_com_quote_adjustment_amount CHECK (
        "application_order" BETWEEN 1 AND 4096 AND "amount_minor" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_com_quote_adjustment_no UNIQUE ("quote_adjustment_no"),
    CONSTRAINT uk_idc_com_quote_adjustment_order UNIQUE (
        "quote_item_no", "application_order"
    ),
    CONSTRAINT fk_idc_com_quote_adjustment_item FOREIGN KEY (
        "quote_item_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_item_snapshot (
        "quote_item_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_quote_adjustment_item
    ON plugin_idc_core_commerce_quote_adjustment_snapshot (
        "tenant_id", "quote_item_no", "application_order", "id"
    );

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_idc_com_cart_latest_quote'
          AND conrelid = 'plugin_idc_core_commerce_cart'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_cart
            ADD CONSTRAINT fk_idc_com_cart_latest_quote
            FOREIGN KEY ("cart_no", "latest_quote_no")
            REFERENCES plugin_idc_core_commerce_quote_snapshot (
                "cart_no", "quote_no"
            ) ON DELETE RESTRICT;
    END IF;
END $$;

-- Purpose: Customer checkout order protected by customer-level idempotency digests.
-- 用途：保存交易订单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "order_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_payment',
    "risk_decision_code" VARCHAR(16) NOT NULL,
    "requires_contract" BOOLEAN NOT NULL DEFAULT FALSE,
    "item_count" INT NOT NULL,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "checkout_idempotency_digest" CHAR(64) NOT NULL,
    "checkout_request_digest" CHAR(64) NOT NULL,
    "payment_due_at" TIMESTAMPTZ NOT NULL,
    "paid_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_order_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_order_source CHECK (
        "source_code" IN ('portal', 'admin', 'channel', 'migration')
    ),
    CONSTRAINT ck_idc_com_order_status CHECK (
        "status_code" IN (
            'pending_payment', 'payment_processing', 'paid', 'fulfilling',
            'partially_fulfilled', 'fulfilled', 'cancelled',
            'partially_refunded', 'refunded', 'manual_review'
        )
    ),
    CONSTRAINT ck_idc_com_order_risk CHECK (
        "risk_decision_code" IN ('approved', 'manual_review')
    ),
    CONSTRAINT ck_idc_com_order_amount CHECK (
        "item_count" BETWEEN 1 AND 1000
        AND "subtotal_minor" >= 0 AND "discount_minor" >= 0
        AND "discount_minor" <= "subtotal_minor"
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
    ),
    CONSTRAINT ck_idc_com_order_digests CHECK (
        "checkout_idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "checkout_request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_order_lifecycle CHECK (
        ("status_code" IN ('pending_payment', 'payment_processing')
            AND "paid_at" IS NULL AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
        OR ("status_code" IN ('paid', 'fulfilling', 'partially_fulfilled')
            AND "paid_at" IS NOT NULL AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'fulfilled' AND "paid_at" IS NOT NULL
            AND "completed_at" IS NOT NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'cancelled' AND "paid_at" IS NULL
            AND "completed_at" IS NULL AND "cancelled_at" IS NOT NULL)
        OR ("status_code" IN ('partially_refunded', 'refunded')
            AND "paid_at" IS NOT NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'manual_review'
            AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
    ),
    CONSTRAINT ck_idc_com_order_due CHECK ("payment_due_at" > "created_at"),
    CONSTRAINT ck_idc_com_order_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_com_order_no UNIQUE ("order_no"),
    CONSTRAINT uk_idc_com_order_quote UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_com_order_request UNIQUE (
        "customer_no", "checkout_idempotency_digest"
    ),
    CONSTRAINT uk_idc_com_order_quote_currency UNIQUE (
        "order_no", "quote_no", "currency_code"
    ),
    CONSTRAINT fk_idc_com_order_quote FOREIGN KEY (
        "quote_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_snapshot (
        "quote_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_order_customer
    ON plugin_idc_core_commerce_order (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_order_payment_expiry
    ON plugin_idc_core_commerce_order (
        "tenant_id", "status_code", "payment_due_at", "id"
    ) WHERE "status_code" IN ('pending_payment', 'payment_processing');

-- Purpose: Append-only commercial order item copied from one exact quote item.
-- 用途：保存交易订单明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "order_item_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "line_number" INT NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "product_type_code" VARCHAR(24) NOT NULL,
    "product_name_zh_cn" VARCHAR(256) NOT NULL,
    "product_name_en_us" VARCHAR(256) NOT NULL,
    "server_quantity" INT NOT NULL,
    "purchase_cycles" INT NOT NULL,
    "charge_type_code" VARCHAR(16) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "requires_contract" BOOLEAN NOT NULL DEFAULT FALSE,
    "fulfillment_mode_code" VARCHAR(16) NOT NULL,
    "risk_decision_code" VARCHAR(16) NOT NULL,
    "initial_binding_no" VARCHAR(40) NOT NULL,
    "inventory_snapshot_no" VARCHAR(40),
    "inventory_decision_code" VARCHAR(24) NOT NULL,
    "route_decision_digest" CHAR(64) NOT NULL,
    "tax_policy_no" VARCHAR(40) NOT NULL,
    "price_includes_tax" BOOLEAN NOT NULL,
    "tax_treatment_code" VARCHAR(16) NOT NULL,
    "tax_rate_basis_points" INT,
    "tax_category_code" VARCHAR(64) NOT NULL,
    "rounding_scope_code" VARCHAR(16) NOT NULL,
    "rounding_mode_code" VARCHAR(16) NOT NULL,
    "rounding_quantum_minor" BIGINT NOT NULL,
    "setup_subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    "period_subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_order_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_order_item_line CHECK (
        "line_number" BETWEEN 1 AND 1000
        AND "server_quantity" BETWEEN 1 AND 1000
        AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_com_order_item_product CHECK (
        "product_type_code" IN ('cloud_server', 'bare_metal')
    ),
    CONSTRAINT ck_idc_com_order_item_billing CHECK (
        "charge_type_code" IN ('one_time', 'recurring', 'tiered', 'usage', 'trial')
        AND (
            ("period_unit_code" = 'once' AND "period_count" = 1 AND "purchase_cycles" = 1)
            OR ("period_unit_code" = 'hour' AND "period_count" BETWEEN 1 AND 87600)
            OR ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
            OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
            OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10)
        )
    ),
    CONSTRAINT ck_idc_com_order_item_policy CHECK (
        "fulfillment_mode_code" IN ('automatic', 'manual_review')
        AND "risk_decision_code" IN ('approved', 'manual_review')
        AND "inventory_decision_code" IN ('available', 'manual_confirmation')
        AND (
            ("inventory_decision_code" = 'available' AND "inventory_snapshot_no" IS NOT NULL)
            OR "inventory_decision_code" = 'manual_confirmation'
        )
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_order_item_tax CHECK (
        "tax_treatment_code" IN ('taxable', 'zero_rated', 'exempt')
        AND (
            ("tax_treatment_code" = 'taxable' AND "tax_rate_basis_points" BETWEEN 0 AND 10000)
            OR ("tax_treatment_code" = 'zero_rated' AND "tax_rate_basis_points" = 0)
            OR ("tax_treatment_code" = 'exempt' AND "tax_rate_basis_points" IS NULL)
        )
        AND "tax_category_code" ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
        AND "rounding_scope_code" IN ('line', 'total')
        AND "rounding_mode_code" IN ('half_up', 'down', 'up')
        AND "rounding_quantum_minor" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_idc_com_order_item_amount CHECK (
        "setup_subtotal_minor" >= 0 AND "period_subtotal_minor" >= 0
        AND "subtotal_minor" = "setup_subtotal_minor" + "period_subtotal_minor"
        AND "discount_minor" >= 0 AND "discount_minor" <= "subtotal_minor"
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_com_order_item_no UNIQUE ("order_item_no"),
    CONSTRAINT uk_idc_com_order_item_line UNIQUE ("order_no", "line_number"),
    CONSTRAINT uk_idc_com_order_quote_item UNIQUE ("quote_item_no"),
    CONSTRAINT uk_idc_com_order_item_order_no UNIQUE ("order_no", "order_item_no"),
    CONSTRAINT fk_idc_com_order_item_order FOREIGN KEY (
        "order_no", "quote_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_order (
        "order_no", "quote_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_quote FOREIGN KEY (
        "quote_no", "quote_item_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_item_snapshot (
        "quote_no", "quote_item_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_product_version FOREIGN KEY ("product_no", "version_no")
        REFERENCES plugin_idc_core_catalog_product_version (
            "product_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_billing FOREIGN KEY ("version_no", "billing_rule_no")
        REFERENCES plugin_idc_core_catalog_billing_rule (
            "version_no", "billing_rule_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_config FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_binding FOREIGN KEY (
        "version_no", "initial_binding_no"
    ) REFERENCES plugin_idc_core_catalog_resource_pool_binding (
        "version_no", "binding_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_inventory FOREIGN KEY (
        "initial_binding_no", "inventory_snapshot_no"
    ) REFERENCES plugin_idc_core_catalog_inventory_snapshot (
        "binding_no", "snapshot_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_item_tax FOREIGN KEY (
        "version_no", "currency_code", "tax_policy_no"
    ) REFERENCES plugin_idc_core_catalog_tax_policy (
        "version_no", "currency_code", "tax_policy_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_order_item_order
    ON plugin_idc_core_commerce_order_item (
        "tenant_id", "order_no", "line_number", "id"
    );

-- Purpose: Local inventory hold lifecycle for one order item and complete inventory snapshot.
-- 用途：保存交易库存预留相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_inventory_reservation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reservation_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "binding_no" VARCHAR(40) NOT NULL,
    "inventory_snapshot_no" VARCHAR(40) NOT NULL,
    "reserved_quantity" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'held',
    "release_reason_code" VARCHAR(32) NOT NULL DEFAULT '',
    "content_digest" CHAR(64) NOT NULL,
    "hold_expires_at" TIMESTAMPTZ NOT NULL,
    "committed_at" TIMESTAMPTZ,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_inventory_res_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_inventory_res_quantity CHECK (
        "reserved_quantity" BETWEEN 1 AND 1000
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "hold_expires_at" > "created_at"
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_com_inventory_res_status CHECK (
        "status_code" IN ('held', 'committed', 'consumed', 'released', 'expired')
    ),
    CONSTRAINT ck_idc_com_inventory_res_lifecycle CHECK (
        ("status_code" = 'held' AND "committed_at" IS NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = '')
        OR ("status_code" = 'committed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = '')
        OR ("status_code" = 'consumed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NOT NULL AND "released_at" IS NULL
            AND "release_reason_code" = '')
        OR ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL
            AND "released_at" IS NOT NULL
            AND "release_reason_code" IN (
                'order_cancelled', 'payment_expired', 'rerouted',
                'fulfillment_failed', 'manual_release'
            ))
    ),
    CONSTRAINT uk_idc_com_inventory_res_no UNIQUE ("reservation_no"),
    CONSTRAINT fk_idc_com_inventory_res_order_item FOREIGN KEY (
        "order_no", "order_item_no"
    ) REFERENCES plugin_idc_core_commerce_order_item (
        "order_no", "order_item_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_inventory_res_snapshot FOREIGN KEY (
        "binding_no", "inventory_snapshot_no"
    ) REFERENCES plugin_idc_core_catalog_inventory_snapshot (
        "binding_no", "snapshot_no"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_inventory_res_active_item
    ON plugin_idc_core_commerce_inventory_reservation (
        "tenant_id", "order_item_no"
    ) WHERE "status_code" IN ('held', 'committed');

CREATE INDEX IF NOT EXISTS idx_idc_com_inventory_res_capacity
    ON plugin_idc_core_commerce_inventory_reservation (
        "tenant_id", "binding_no", "status_code", "hold_expires_at", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_inventory_res_expiry
    ON plugin_idc_core_commerce_inventory_reservation (
        "tenant_id", "status_code", "hold_expires_at", "id"
    ) WHERE "status_code" = 'held';

-- Purpose: Per-server fulfillment projection separated from order monetary facts.
-- 用途：保存交易履约单元相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_fulfillment_unit (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "fulfillment_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "unit_sequence" INT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_payment',
    "reservation_no" VARCHAR(40),
    "business_operation_no" VARCHAR(40),
    "service_no" VARCHAR(40),
    "last_evidence_digest" CHAR(64),
    "started_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_fulfillment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_fulfillment_sequence CHECK (
        "unit_sequence" BETWEEN 1 AND 1000 AND "version" > 0
    ),
    CONSTRAINT ck_idc_com_fulfillment_status CHECK (
        "status_code" IN (
            'pending_payment', 'ready', 'executing', 'succeeded', 'failed',
            'result_unknown', 'manual_review', 'cancelled'
        )
    ),
    CONSTRAINT ck_idc_com_fulfillment_evidence CHECK (
        "last_evidence_digest" IS NULL OR "last_evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_fulfillment_lifecycle CHECK (
        ("status_code" IN ('pending_payment', 'ready')
            AND "started_at" IS NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('executing', 'result_unknown', 'manual_review')
            AND "started_at" IS NOT NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('succeeded', 'failed')
            AND "started_at" IS NOT NULL AND "completed_at" IS NOT NULL)
        OR ("status_code" = 'cancelled' AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_com_fulfillment_success CHECK (
        "status_code" <> 'succeeded' OR "service_no" IS NOT NULL
    ),
    CONSTRAINT uk_idc_com_fulfillment_no UNIQUE ("fulfillment_no"),
    CONSTRAINT uk_idc_com_fulfillment_unit UNIQUE ("order_item_no", "unit_sequence"),
    CONSTRAINT fk_idc_com_fulfillment_order_item FOREIGN KEY (
        "order_no", "order_item_no"
    ) REFERENCES plugin_idc_core_commerce_order_item (
        "order_no", "order_item_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_fulfillment_reservation FOREIGN KEY ("reservation_no")
        REFERENCES plugin_idc_core_commerce_inventory_reservation (
            "reservation_no"
        ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_fulfillment_order
    ON plugin_idc_core_commerce_fulfillment_unit (
        "tenant_id", "order_no", "order_item_no", "unit_sequence", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_fulfillment_work
    ON plugin_idc_core_commerce_fulfillment_unit (
        "tenant_id", "status_code", "updated_at", "id"
    ) WHERE "status_code" IN ('ready', 'executing', 'result_unknown', 'manual_review');

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'commerce snapshots are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_com_config_snapshot_immutable
    ON plugin_idc_core_commerce_configuration_snapshot;
CREATE TRIGGER trg_idc_com_config_snapshot_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_configuration_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_config_item_immutable
    ON plugin_idc_core_commerce_configuration_snapshot_item;
CREATE TRIGGER trg_idc_com_config_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_configuration_snapshot_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_quote_snapshot_immutable
    ON plugin_idc_core_commerce_quote_snapshot;
CREATE TRIGGER trg_idc_com_quote_snapshot_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_quote_item_immutable
    ON plugin_idc_core_commerce_quote_item_snapshot;
CREATE TRIGGER trg_idc_com_quote_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_item_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_quote_charge_immutable
    ON plugin_idc_core_commerce_quote_charge_snapshot;
CREATE TRIGGER trg_idc_com_quote_charge_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_charge_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_quote_adjustment_immutable
    ON plugin_idc_core_commerce_quote_adjustment_snapshot;
CREATE TRIGGER trg_idc_com_quote_adjustment_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_adjustment_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

DROP TRIGGER IF EXISTS trg_idc_com_order_item_immutable
    ON plugin_idc_core_commerce_order_item;
CREATE TRIGGER trg_idc_com_order_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_order_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

COMMENT ON TABLE plugin_idc_core_commerce_cart
    IS 'Single-currency editable purchase intent owned by one guest token digest or customer';
COMMENT ON TABLE plugin_idc_core_commerce_configuration_snapshot
    IS 'Append-only normalized server configuration selection header';
COMMENT ON TABLE plugin_idc_core_commerce_configuration_snapshot_item
    IS 'Append-only selected catalog option and safe display/value snapshot';
COMMENT ON TABLE plugin_idc_core_commerce_cart_item
    IS 'Editable cart line referencing one immutable configuration snapshot';
COMMENT ON TABLE plugin_idc_core_commerce_quote_snapshot
    IS 'Append-only deterministic checkout quote header with local-only rule digests';
COMMENT ON TABLE plugin_idc_core_commerce_quote_item_snapshot
    IS 'Append-only product, tax, route, inventory, policy, and amount quote item';
COMMENT ON TABLE plugin_idc_core_commerce_quote_charge_snapshot
    IS 'Append-only base or selected-option price composition for one quote item';
COMMENT ON TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
    IS 'Append-only promotion, customer, agent, or approved manual quote adjustment';
COMMENT ON TABLE plugin_idc_core_commerce_order
    IS 'Customer checkout order protected by customer-level idempotency digests';
COMMENT ON TABLE plugin_idc_core_commerce_order_item
    IS 'Append-only commercial order item copied from one exact quote item';
COMMENT ON TABLE plugin_idc_core_commerce_inventory_reservation
    IS 'Local inventory hold lifecycle for one order item and complete inventory snapshot';
COMMENT ON TABLE plugin_idc_core_commerce_fulfillment_unit
    IS 'Per-server fulfillment projection separated from order monetary facts';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 购物车状态', 'idc_commerce_cart_status', 1, 1, FALSE,
     '单币种购物车生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 购物车项状态', 'idc_commerce_cart_item_status', 1, 1, FALSE,
     '购物车项目校验与移除状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 报价用途', 'idc_commerce_quote_purpose', 1, 1, FALSE,
     '确定报价所服务的业务用途', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 报价来源', 'idc_commerce_quote_source', 1, 1, FALSE,
     '确定报价的调用来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 库存决定', 'idc_commerce_inventory_decision', 1, 1, FALSE,
     '报价所采用的本地库存结论', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 履约模式', 'idc_commerce_fulfillment_mode', 1, 1, FALSE,
     '自动或人工审核履约策略', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 风险决定', 'idc_commerce_risk_decision', 1, 1, FALSE,
     '报价和订单的风险门禁结论', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单状态', 'idc_commerce_order_status', 1, 1, FALSE,
     '服务器订单生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单来源', 'idc_commerce_order_source', 1, 1, FALSE,
     '服务器订单创建渠道', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 库存预留状态', 'idc_commerce_reservation_status', 1, 1, FALSE,
     '本地库存预留生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 库存释放原因', 'idc_commerce_inventory_release_reason', 1, 1, FALSE,
     '库存预留释放或过期原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 履约状态', 'idc_commerce_fulfillment_status', 1, 1, FALSE,
     '逐台服务器履约状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 报价收费构成', 'idc_commerce_quote_charge_kind', 1, 1, FALSE,
     '基础商品或配置规格收费构成', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 报价优惠类型', 'idc_commerce_quote_adjustment_type', 1, 1, FALSE,
     '确定报价中的优惠来源类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_commerce_cart_status', '活动中', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_status', '已结算', 'checked_out', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_status', '已合并', 'merged', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_status', '已过期', 'expired', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_status', '已放弃', 'abandoned', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_item_status', '有效', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_item_status', '需要重新报价', 'needs_reprice', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_item_status', '暂不可用', 'unavailable', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cart_item_status', '已移除', 'removed', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_purpose', '新购', 'new_purchase', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_purpose', '续费', 'renewal', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_purpose', '变配', 'change', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_purpose', '更换 IP', 'ip_change', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_purpose', '附属能力', 'addon', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '游客', 'guest', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '客户', 'customer', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '员工', 'staff', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '下游渠道', 'channel', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '迁移', 'migration', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_source', '系统', 'system', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_decision', '可预留', 'available', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_decision', '需要人工确认', 'manual_confirmation', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_mode', '自动履约', 'automatic', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_mode', '人工审核', 'manual_review', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_risk_decision', '已通过', 'approved', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_risk_decision', '人工复核', 'manual_review', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '待支付', 'pending_payment', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '支付确认中', 'payment_processing', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '已支付', 'paid', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '履约中', 'fulfilling', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '部分完成', 'partially_fulfilled', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '已完成', 'fulfilled', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '已取消', 'cancelled', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '部分退款', 'partially_refunded', 8, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '已退款', 'refunded', 9, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_status', '需要人工处理', 'manual_review', 10, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_source', '客户门户', 'portal', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_source', '管理后台', 'admin', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_source', '下游渠道', 'channel', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_source', '数据迁移', 'migration', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_reservation_status', '已预留', 'held', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_reservation_status', '已承诺', 'committed', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_reservation_status', '已消耗', 'consumed', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_reservation_status', '已释放', 'released', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_reservation_status', '已过期', 'expired', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_release_reason', '订单取消', 'order_cancelled', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_release_reason', '付款超时', 'payment_expired', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_release_reason', '重新路由', 'rerouted', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_release_reason', '履约失败', 'fulfillment_failed', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_inventory_release_reason', '人工释放', 'manual_release', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '等待付款', 'pending_payment', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '等待执行', 'ready', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '执行中', 'executing', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '成功', 'succeeded', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '失败', 'failed', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '结果未知', 'result_unknown', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '人工处理', 'manual_review', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_fulfillment_status', '已取消', 'cancelled', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_charge_kind', '基础商品', 'base', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_charge_kind', '配置规格', 'option', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '活动优惠', 'campaign_discount', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '优惠券', 'coupon', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '代金券', 'voucher', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '客户价', 'customer_price', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '代理价', 'agent_price', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_quote_adjustment_type', '人工优惠', 'manual_discount', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
