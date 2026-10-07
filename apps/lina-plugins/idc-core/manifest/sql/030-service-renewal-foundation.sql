-- 030: Establish the core-owned service entitlement, immutable commercial
-- snapshots, renewal policy, and unified business-operation foundation.

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_catalog_binding_service_snapshot'
          AND conrelid = 'plugin_idc_core_catalog_resource_pool_binding'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_catalog_resource_pool_binding
            ADD CONSTRAINT uk_idc_catalog_binding_service_snapshot UNIQUE (
                "binding_no", "version_no", "pool_no"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_catalog_tax_service_snapshot'
          AND conrelid = 'plugin_idc_core_catalog_tax_policy'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_catalog_tax_policy
            ADD CONSTRAINT uk_idc_catalog_tax_service_snapshot UNIQUE (
                "tax_policy_no", "version_no", "currency_code"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_catalog_billing_service_snapshot'
          AND conrelid = 'plugin_idc_core_catalog_billing_rule'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_catalog_billing_rule
            ADD CONSTRAINT uk_idc_catalog_billing_service_snapshot UNIQUE (
                "version_no", "billing_rule_no", "period_unit_code", "period_count"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_com_fulfillment_service_snapshot'
          AND conrelid = 'plugin_idc_core_commerce_fulfillment_unit'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
            ADD CONSTRAINT uk_idc_com_fulfillment_service_snapshot UNIQUE (
                "fulfillment_no", "order_no", "order_item_no"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_com_order_item_service_snapshot'
          AND conrelid = 'plugin_idc_core_commerce_order_item'::regclass
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_core_commerce_order_item'
          AND column_name = 'product_type_code'
    ) AND EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_core_commerce_order_item'
          AND column_name = 'initial_binding_no'
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order_item
            ADD CONSTRAINT uk_idc_com_order_item_service_snapshot UNIQUE (
                "order_no", "order_item_no", "product_no", "version_no",
                "billing_rule_no", "configuration_snapshot_no",
                "product_type_code", "currency_code", "initial_binding_no",
                "period_unit_code", "period_count"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_com_quote_item_service_price_source'
          AND conrelid = 'plugin_idc_core_commerce_quote_item_snapshot'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
            ADD CONSTRAINT uk_idc_com_quote_item_service_price_source UNIQUE (
                "quote_no", "quote_item_no", "currency_code", "product_no",
                "version_no", "billing_rule_no", "configuration_snapshot_no",
                "purchase_cycles", "period_unit_code", "period_count", "tax_policy_no"
            );
    END IF;
END;
$constraint$;

-- Purpose: Core-owned customer entitlement and runtime projection; Provider truth remains resource-owned.
-- 用途：保存服务实例相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_instance (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(16) NOT NULL,
    "source_fulfillment_no" VARCHAR(40),
    "order_no" VARCHAR(40),
    "order_item_no" VARCHAR(40),
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "product_type_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "resource_owner_code" VARCHAR(64) NOT NULL DEFAULT 'idc-resource',
    "binding_no" VARCHAR(40) NOT NULL,
    "pool_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64),
    "entitlement_status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_activation',
    "runtime_status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_provision',
    "provider_task_status_code" VARCHAR(24) NOT NULL DEFAULT 'none',
    "renewal_mode_code" VARCHAR(24) NOT NULL DEFAULT 'unresolved',
    "renewal_capability_evidence_no" VARCHAR(64),
    "renewal_capability_digest" CHAR(64),
    "renewal_capability_observed_at" TIMESTAMPTZ,
    "renewal_capability_expires_at" TIMESTAMPTZ,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "entitlement_started_at" TIMESTAMPTZ,
    "entitlement_expires_at" TIMESTAMPTZ,
    "current_price_snapshot_no" VARCHAR(40),
    "current_renewal_policy_no" VARCHAR(40),
    "current_renewal_policy_version" BIGINT,
    "last_business_operation_no" VARCHAR(40),
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_source CHECK (
        "source_code" IN ('order', 'migration', 'manual')
        AND (
            ("source_code" = 'order' AND "source_fulfillment_no" IS NOT NULL
                AND "order_no" IS NOT NULL AND "order_item_no" IS NOT NULL)
            OR ("source_code" <> 'order' AND "source_fulfillment_no" IS NULL
                AND "order_no" IS NULL AND "order_item_no" IS NULL)
        )
    ),
    CONSTRAINT ck_idc_service_product CHECK (
        "product_type_code" IN ('cloud_server', 'bare_metal')
        AND "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_service_resource_owner CHECK (
        "resource_owner_code" = 'idc-resource'
        AND length(btrim("pool_no")) BETWEEN 1 AND 64
        AND ("channel_no" IS NULL OR length(btrim("channel_no")) BETWEEN 1 AND 64)
    ),
    CONSTRAINT ck_idc_service_entitlement_status CHECK (
        "entitlement_status_code" IN (
            'pending_activation', 'active', 'grace', 'expired',
            'terminating', 'terminated', 'cancelled'
        )
    ),
    CONSTRAINT ck_idc_service_runtime_status CHECK (
        "runtime_status_code" IN (
            'pending_provision', 'provisioning', 'running', 'stopped',
            'suspended', 'resuming', 'changing', 'terminating',
            'terminated', 'unknown'
        )
    ),
    CONSTRAINT ck_idc_service_provider_task_status CHECK (
        "provider_task_status_code" IN (
            'none', 'pending', 'executing', 'succeeded', 'failed',
            'result_unknown', 'manual_review'
        )
    ),
    CONSTRAINT ck_idc_service_renewal_mode CHECK (
        "renewal_mode_code" IN (
            'unresolved', 'provider_confirmed', 'local_entitlement'
        )
        AND (
            ("renewal_mode_code" = 'unresolved'
                AND "renewal_capability_evidence_no" IS NULL
                AND "renewal_capability_digest" IS NULL
                AND "renewal_capability_observed_at" IS NULL
                AND "renewal_capability_expires_at" IS NULL)
            OR ("renewal_mode_code" <> 'unresolved'
                AND "channel_no" IS NOT NULL
                AND "renewal_capability_evidence_no" IS NOT NULL
                AND length(btrim("renewal_capability_evidence_no")) BETWEEN 1 AND 64
                AND "renewal_capability_digest" IS NOT NULL
                AND "renewal_capability_digest" ~ '^[0-9a-f]{64}$'
                AND "renewal_capability_observed_at" IS NOT NULL
                AND "renewal_capability_expires_at" > "renewal_capability_observed_at")
        )
    ),
    CONSTRAINT ck_idc_service_period CHECK (
        (("period_unit_code" = 'once' AND "period_count" = 1)
            OR ("period_unit_code" = 'hour' AND "period_count" BETWEEN 1 AND 87600)
            OR ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
            OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
            OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10))
        AND (
            ("entitlement_status_code" IN ('pending_activation', 'cancelled')
                AND "entitlement_started_at" IS NULL
                AND "entitlement_expires_at" IS NULL)
            OR ("entitlement_status_code" NOT IN ('pending_activation', 'cancelled')
                AND "entitlement_started_at" IS NOT NULL
                AND (
                    ("period_unit_code" = 'once' AND "entitlement_expires_at" IS NULL)
                    OR ("period_unit_code" <> 'once'
                        AND "entitlement_expires_at" > "entitlement_started_at")
                ))
        )
    ),
    CONSTRAINT ck_idc_service_policy_pointer CHECK (
        ("current_renewal_policy_no" IS NULL
            AND "current_renewal_policy_version" IS NULL)
        OR ("current_renewal_policy_no" IS NOT NULL
            AND "current_renewal_policy_version" > 0)
    ),
    CONSTRAINT ck_idc_service_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_service_no UNIQUE ("service_no"),
    CONSTRAINT uk_idc_service_owner_currency UNIQUE (
        "service_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_service_owner UNIQUE (
        "service_no", "customer_no"
    ),
    CONSTRAINT uk_idc_service_fulfillment UNIQUE ("source_fulfillment_no"),
    CONSTRAINT fk_idc_service_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_product_version FOREIGN KEY ("product_no", "version_no")
        REFERENCES plugin_idc_core_catalog_product_version (
            "product_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_billing_rule FOREIGN KEY (
        "version_no", "billing_rule_no", "period_unit_code", "period_count"
    )
        REFERENCES plugin_idc_core_catalog_billing_rule (
            "version_no", "billing_rule_no", "period_unit_code", "period_count"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_configuration FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_binding FOREIGN KEY (
        "binding_no", "version_no", "pool_no"
    ) REFERENCES plugin_idc_core_catalog_resource_pool_binding (
        "binding_no", "version_no", "pool_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_fulfillment FOREIGN KEY (
        "source_fulfillment_no", "order_no", "order_item_no"
    )
        REFERENCES plugin_idc_core_commerce_fulfillment_unit (
            "fulfillment_no", "order_no", "order_item_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_order_item FOREIGN KEY (
        "order_no", "order_item_no", "product_no", "version_no",
        "billing_rule_no", "configuration_snapshot_no", "product_type_code",
        "currency_code", "binding_no", "period_unit_code", "period_count"
    )
        REFERENCES plugin_idc_core_commerce_order_item (
            "order_no", "order_item_no", "product_no", "version_no",
            "billing_rule_no", "configuration_snapshot_no", "product_type_code",
            "currency_code", "initial_binding_no", "period_unit_code", "period_count"
        ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_customer_detail
    ON plugin_idc_core_service_instance (
        "tenant_id", "customer_no", "entitlement_status_code",
        "entitlement_expires_at", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_expiry_work
    ON plugin_idc_core_service_instance (
        "tenant_id", "entitlement_status_code", "entitlement_expires_at", "id"
    ) WHERE "entitlement_status_code" IN ('active', 'grace');

CREATE INDEX IF NOT EXISTS idx_idc_service_resource_projection
    ON plugin_idc_core_service_instance (
        "tenant_id", "resource_owner_code", "channel_no", "pool_no", "service_no"
    );

-- Purpose: Append-only new-purchase or renewal commercial snapshot bound to one service.
-- 用途：保存服务价格快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_price_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "price_snapshot_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "phase_code" VARCHAR(24) NOT NULL,
    "source_quote_no" VARCHAR(40),
    "source_quote_item_no" VARCHAR(40),
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "purchase_cycles" INT NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "renewal_mode_code" VARCHAR(24) NOT NULL,
    "capability_evidence_no" VARCHAR(64),
    "capability_evidence_digest" CHAR(64),
    "capability_evidence_observed_at" TIMESTAMPTZ,
    "capability_evidence_expires_at" TIMESTAMPTZ,
    "original_expires_at" TIMESTAMPTZ,
    "projected_expires_at" TIMESTAMPTZ,
    "tax_policy_no" VARCHAR(40) NOT NULL,
    "price_includes_tax" BOOLEAN NOT NULL,
    "tax_treatment_code" VARCHAR(16) NOT NULL,
    "tax_rate_basis_points" INT,
    "tax_category_code" VARCHAR(64) NOT NULL,
    "rounding_scope_code" VARCHAR(16) NOT NULL,
    "rounding_mode_code" VARCHAR(16) NOT NULL,
    "rounding_quantum_minor" BIGINT NOT NULL,
    "component_count" INT NOT NULL,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "pricing_rule_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_price_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_price_phase CHECK (
        "phase_code" IN ('new_purchase', 'renewal')
        AND (
            ("source_quote_no" IS NULL AND "source_quote_item_no" IS NULL)
            OR ("source_quote_no" IS NOT NULL AND "source_quote_item_no" IS NOT NULL)
        )
    ),
    CONSTRAINT ck_idc_service_price_period CHECK (
        "purchase_cycles" BETWEEN 1 AND 120
        AND (("period_unit_code" = 'once' AND "period_count" = 1
                AND "purchase_cycles" = 1)
            OR ("period_unit_code" = 'hour' AND "period_count" BETWEEN 1 AND 87600)
            OR ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
            OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
            OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10))
    ),
    CONSTRAINT ck_idc_service_price_renewal_mode CHECK (
        "renewal_mode_code" IN (
            'unresolved', 'provider_confirmed', 'local_entitlement'
        )
        AND (
            ("phase_code" = 'new_purchase' AND "renewal_mode_code" = 'unresolved'
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL
                AND "capability_evidence_observed_at" IS NULL
                AND "capability_evidence_expires_at" IS NULL
                AND "original_expires_at" IS NULL)
            OR ("phase_code" = 'renewal' AND "renewal_mode_code" <> 'unresolved'
                AND "capability_evidence_no" IS NOT NULL
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NOT NULL
                AND "projected_expires_at" > "original_expires_at")
        )
    ),
    CONSTRAINT ck_idc_service_price_tax CHECK (
        "tax_treatment_code" IN ('taxable', 'zero_rated', 'exempt')
        AND (("tax_treatment_code" = 'taxable'
                AND "tax_rate_basis_points" BETWEEN 0 AND 10000)
            OR ("tax_treatment_code" = 'zero_rated'
                AND "tax_rate_basis_points" = 0)
            OR ("tax_treatment_code" = 'exempt'
                AND "tax_rate_basis_points" IS NULL))
        AND "tax_category_code" ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
        AND "rounding_scope_code" IN ('line', 'total')
        AND "rounding_mode_code" IN ('half_up', 'down', 'up')
        AND "rounding_quantum_minor" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_idc_service_price_amount CHECK (
        "component_count" BETWEEN 1 AND 5000
        AND "subtotal_minor" >= 0
        AND "discount_minor" BETWEEN 0 AND "subtotal_minor"
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "pricing_rule_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_price_no UNIQUE ("price_snapshot_no"),
    CONSTRAINT uk_idc_service_price_service_no UNIQUE (
        "price_snapshot_no", "service_no"
    ),
    CONSTRAINT uk_idc_service_price_service_currency UNIQUE (
        "price_snapshot_no", "service_no", "currency_code"
    ),
    CONSTRAINT fk_idc_service_price_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_price_catalog FOREIGN KEY (
        "product_no", "version_no"
    ) REFERENCES plugin_idc_core_catalog_product_version (
        "product_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_price_billing FOREIGN KEY (
        "version_no", "billing_rule_no"
    ) REFERENCES plugin_idc_core_catalog_billing_rule (
        "version_no", "billing_rule_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_price_configuration FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_price_quote FOREIGN KEY (
        "source_quote_no", "source_quote_item_no", "currency_code", "product_no",
        "version_no", "billing_rule_no", "configuration_snapshot_no",
        "purchase_cycles", "period_unit_code", "period_count", "tax_policy_no"
    ) REFERENCES plugin_idc_core_commerce_quote_item_snapshot (
        "quote_no", "quote_item_no", "currency_code", "product_no",
        "version_no", "billing_rule_no", "configuration_snapshot_no",
        "purchase_cycles", "period_unit_code", "period_count", "tax_policy_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_price_tax_policy FOREIGN KEY (
        "tax_policy_no", "version_no", "currency_code"
    ) REFERENCES plugin_idc_core_catalog_tax_policy (
        "tax_policy_no", "version_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_price_history
    ON plugin_idc_core_service_price_snapshot (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only bounded price composition for one service commercial snapshot.
-- 用途：保存服务价格组成项快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_price_component_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "component_snapshot_no" VARCHAR(40) NOT NULL,
    "price_snapshot_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "line_number" INT NOT NULL,
    "component_type_code" VARCHAR(16) NOT NULL,
    "charge_phase_code" VARCHAR(16) NOT NULL,
    "option_no" VARCHAR(40),
    "source_reference" VARCHAR(128) NOT NULL,
    "quantity" INT NOT NULL,
    "unit_amount_minor" BIGINT NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "source_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_price_component_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_price_component_type CHECK (
        "component_type_code" IN ('base', 'option', 'discount', 'tax')
        AND "charge_phase_code" IN ('setup', 'periodic', 'total')
        AND (("component_type_code" = 'option' AND "option_no" IS NOT NULL)
            OR ("component_type_code" <> 'option' AND "option_no" IS NULL))
    ),
    CONSTRAINT ck_idc_service_price_component_value CHECK (
        "line_number" BETWEEN 1 AND 5000
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
        AND "quantity" BETWEEN 1 AND 1000000
        AND "unit_amount_minor" >= 0 AND "amount_minor" >= 0
        AND "source_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_price_component_no UNIQUE ("component_snapshot_no"),
    CONSTRAINT uk_idc_service_price_component_line UNIQUE (
        "price_snapshot_no", "line_number"
    ),
    CONSTRAINT fk_idc_service_price_component_header FOREIGN KEY (
        "price_snapshot_no", "service_no"
    ) REFERENCES plugin_idc_core_service_price_snapshot (
        "price_snapshot_no", "service_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_price_component_order
    ON plugin_idc_core_service_price_component_snapshot (
        "tenant_id", "price_snapshot_no", "line_number", "id"
    );

-- Purpose: Append-only per-service automatic renewal policy versions.
-- 用途：保存服务续费策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_renewal_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "auto_renew_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
    "renewal_cycles" INT NOT NULL DEFAULT 1,
    "lead_seconds" BIGINT NOT NULL DEFAULT 604800,
    "maximum_attempts" INT NOT NULL DEFAULT 3,
    "retry_interval_seconds" BIGINT NOT NULL DEFAULT 21600,
    "failure_notification_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "configured_by" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_renewal_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_renewal_policy_values CHECK (
        "policy_version" > 0 AND "renewal_cycles" BETWEEN 1 AND 120
        AND "lead_seconds" BETWEEN 3600 AND 31536000
        AND "maximum_attempts" BETWEEN 1 AND 32
        AND "retry_interval_seconds" BETWEEN 300 AND 2592000
        AND "configured_by" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_renewal_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_service_renewal_policy_no_version UNIQUE (
        "policy_no", "policy_version"
    ),
    CONSTRAINT uk_idc_service_renewal_policy_owner_version UNIQUE (
        "policy_no", "policy_version", "entitlement_no"
    ),
    CONSTRAINT uk_idc_service_renewal_policy_version UNIQUE (
        "entitlement_no", "policy_version"
    ),
    CONSTRAINT uk_idc_service_renewal_policy_entitlement_no UNIQUE (
        "policy_no", "entitlement_no"
    )
);

-- Older development databases created this identity as service_no. The
-- project has no production compatibility contract, but local validation data
-- must survive an in-place upgrade to the generic entitlement model.
DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_policy'
           AND column_name = 'service_no'
    ) AND NOT EXISTS (
        SELECT 1
          FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_policy'
           AND column_name = 'entitlement_no'
    ) THEN
        ALTER TABLE plugin_idc_core_service_renewal_policy
            RENAME COLUMN "service_no" TO "entitlement_no";
    END IF;
END;
$migration$;

CREATE INDEX IF NOT EXISTS idx_idc_service_renewal_policy_current
    ON plugin_idc_core_service_renewal_policy (
        "tenant_id", "entitlement_no", "policy_version" DESC, "id" DESC
    );

-- Purpose: Unified idempotent business saga projection with immutable intent.
-- 用途：保存业务操作相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_business_operation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_no" VARCHAR(40) NOT NULL,
    "operation_type_code" VARCHAR(24) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "price_snapshot_no" VARCHAR(40),
    "quote_no" VARCHAR(40),
    "order_no" VARCHAR(40),
    "invoice_no" VARCHAR(40),
    "status_code" VARCHAR(32) NOT NULL,
    "financial_settlement_required" BOOLEAN NOT NULL,
    "execution_mode_code" VARCHAR(24) NOT NULL,
    "resource_owner_code" VARCHAR(64) NOT NULL DEFAULT 'idc-resource',
    "channel_no" VARCHAR(64),
    "pool_no" VARCHAR(64) NOT NULL,
    "provider_task_no" VARCHAR(64),
    "capability_evidence_no" VARCHAR(64),
    "capability_evidence_digest" CHAR(64),
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "financially_settled_at" TIMESTAMPTZ,
    "execution_started_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_business_operation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_business_operation_type CHECK (
        "operation_type_code" IN (
            'new_purchase', 'renewal', 'change', 'ip_change', 'addon',
            'refund', 'suspend', 'resume', 'terminate'
        ) AND "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_business_operation_status CHECK (
        "status_code" IN (
            'quoted', 'awaiting_financial_settlement', 'financially_settled',
            'ready', 'executing', 'succeeded', 'failed', 'compensating',
            'refunding', 'manual_review', 'cancelled'
        )
    ),
    CONSTRAINT ck_idc_business_operation_financial CHECK (
        (("financial_settlement_required" AND "operation_type_code" IN (
                'new_purchase', 'renewal', 'change', 'ip_change', 'addon'
            )) OR (NOT "financial_settlement_required"))
        AND ("status_code" NOT IN (
                'financially_settled', 'ready', 'executing', 'succeeded',
                'failed', 'compensating', 'refunding'
            ) OR NOT "financial_settlement_required"
            OR "financially_settled_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_business_operation_execution CHECK (
        "execution_mode_code" IN (
            'provider_confirmed', 'local_entitlement', 'manual_review'
        )
        AND ("execution_mode_code" <> 'local_entitlement'
            OR "operation_type_code" = 'renewal')
        AND ("execution_mode_code" = 'manual_review'
            OR "channel_no" IS NOT NULL)
        AND (("execution_mode_code" = 'manual_review'
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL)
            OR ("execution_mode_code" <> 'manual_review'
                AND "capability_evidence_no" IS NOT NULL
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'))
        AND ("provider_task_no" IS NULL
            OR "execution_mode_code" = 'provider_confirmed')
    ),
    CONSTRAINT ck_idc_business_operation_reference CHECK (
        ("operation_type_code" IN ('new_purchase', 'renewal', 'change', 'ip_change', 'addon')
            AND "price_snapshot_no" IS NOT NULL)
        OR ("operation_type_code" IN ('refund', 'suspend', 'resume', 'terminate')
            AND "price_snapshot_no" IS NULL)
    ),
    CONSTRAINT ck_idc_business_operation_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "expected_service_version" > 0 AND "version" > 0
        AND ("last_error_code" = ''
            OR "last_error_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
    ),
    CONSTRAINT ck_idc_business_operation_timestamps CHECK (
        ("status_code" NOT IN ('executing', 'succeeded', 'failed')
            OR "execution_started_at" IS NOT NULL)
        AND ("status_code" NOT IN ('succeeded', 'failed', 'cancelled')
            OR "completed_at" IS NOT NULL)
        AND ("completed_at" IS NULL OR "completed_at" >= "created_at")
    ),
    CONSTRAINT uk_idc_business_operation_no UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_business_operation_idempotency UNIQUE (
        "customer_no", "operation_type_code", "idempotency_digest"
    ),
    CONSTRAINT uk_idc_business_operation_service_no UNIQUE (
        "operation_no", "service_no"
    ),
    CONSTRAINT fk_idc_business_operation_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_business_operation_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_business_operation_price FOREIGN KEY (
        "price_snapshot_no", "service_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_price_snapshot (
        "price_snapshot_no", "service_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_business_operation_quote FOREIGN KEY (
        "quote_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_snapshot (
        "quote_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_business_operation_order FOREIGN KEY (
        "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_order (
        "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_business_operation_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_service_history
    ON plugin_idc_core_business_operation (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_work
    ON plugin_idc_core_business_operation (
        "tenant_id", "status_code", "updated_at", "id"
    ) WHERE "status_code" IN (
        'financially_settled', 'ready', 'executing', 'compensating',
        'refunding', 'manual_review'
    );

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_invoice
    ON plugin_idc_core_business_operation (
        "tenant_id", "invoice_no", "id"
    ) WHERE "invoice_no" IS NOT NULL;

-- Purpose: Append-only business-operation transition evidence.
-- 用途：保存业务操作事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_business_operation_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "event_sequence" INT NOT NULL,
    "from_status_code" VARCHAR(32),
    "to_status_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(128) NOT NULL,
    "actor_type_code" VARCHAR(16) NOT NULL,
    "actor_reference" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_business_operation_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_business_operation_event_status CHECK (
        ("from_status_code" IS NULL OR "from_status_code" IN (
            'quoted', 'awaiting_financial_settlement', 'financially_settled',
            'ready', 'executing', 'succeeded', 'failed', 'compensating',
            'refunding', 'manual_review', 'cancelled'
        ))
        AND "to_status_code" IN (
            'quoted', 'awaiting_financial_settlement', 'financially_settled',
            'ready', 'executing', 'succeeded', 'failed', 'compensating',
            'refunding', 'manual_review', 'cancelled'
        )
        AND "from_status_code" IS DISTINCT FROM "to_status_code"
    ),
    CONSTRAINT ck_idc_business_operation_event_values CHECK (
        "event_sequence" BETWEEN 1 AND 1000000
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "actor_type_code" IN ('customer', 'staff', 'system', 'channel', 'migration')
        AND length(btrim("actor_reference")) BETWEEN 1 AND 64
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_business_operation_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_idc_business_operation_event_sequence UNIQUE (
        "operation_no", "event_sequence"
    ),
    CONSTRAINT fk_idc_business_operation_event_operation FOREIGN KEY (
        "operation_no", "service_no"
    ) REFERENCES plugin_idc_core_business_operation (
        "operation_no", "service_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_event_history
    ON plugin_idc_core_business_operation_event (
        "tenant_id", "operation_no", "event_sequence", "id"
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_service_current_price'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT fk_idc_service_current_price FOREIGN KEY (
                "current_price_snapshot_no", "service_no"
            ) REFERENCES plugin_idc_core_service_price_snapshot (
                "price_snapshot_no", "service_no"
            ) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_service_last_operation'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT fk_idc_service_last_operation FOREIGN KEY (
                "last_business_operation_no", "service_no"
            ) REFERENCES plugin_idc_core_business_operation (
                "operation_no", "service_no"
            ) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_com_fulfillment_service'
          AND conrelid = 'plugin_idc_core_commerce_fulfillment_unit'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
            ADD CONSTRAINT fk_idc_com_fulfillment_service FOREIGN KEY ("service_no")
                REFERENCES plugin_idc_core_service_instance ("service_no")
                ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_com_fulfillment_business_operation'
          AND conrelid = 'plugin_idc_core_commerce_fulfillment_unit'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
            ADD CONSTRAINT fk_idc_com_fulfillment_business_operation
                FOREIGN KEY ("business_operation_no")
                REFERENCES plugin_idc_core_business_operation ("operation_no")
                ON DELETE RESTRICT;
    END IF;
END;
$constraint$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_service_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'service commercial history is append-only';
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."operation_no", OLD."operation_type_code",
        OLD."customer_no", OLD."service_no", OLD."currency_code",
        OLD."price_snapshot_no", OLD."quote_no",
        OLD."order_no", OLD."invoice_no", OLD."financial_settlement_required",
        OLD."execution_mode_code", OLD."resource_owner_code", OLD."channel_no",
        OLD."pool_no", OLD."capability_evidence_no", OLD."capability_evidence_digest",
        OLD."idempotency_digest", OLD."request_digest", OLD."content_digest",
        OLD."expected_service_version", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."operation_no", NEW."operation_type_code",
        NEW."customer_no", NEW."service_no", NEW."currency_code",
        NEW."price_snapshot_no", NEW."quote_no",
        NEW."order_no", NEW."invoice_no", NEW."financial_settlement_required",
        NEW."execution_mode_code", NEW."resource_owner_code", NEW."channel_no",
        NEW."pool_no", NEW."capability_evidence_no", NEW."capability_evidence_digest",
        NEW."idempotency_digest", NEW."request_digest", NEW."content_digest",
        NEW."expected_service_version", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_service_price_components()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    "target_price_snapshot_no" VARCHAR(40);
    expected_component_count INT;
    expected_subtotal BIGINT;
    expected_discount BIGINT;
    expected_tax BIGINT;
    actual_component_count BIGINT;
    actual_subtotal NUMERIC;
    actual_discount NUMERIC;
    actual_tax NUMERIC;
BEGIN
    "target_price_snapshot_no" := COALESCE(NEW."price_snapshot_no", OLD."price_snapshot_no");
    SELECT "component_count", "subtotal_minor", "discount_minor", "tax_amount_minor"
    INTO expected_component_count, expected_subtotal, expected_discount, expected_tax
    FROM plugin_idc_core_service_price_snapshot
    WHERE "price_snapshot_no" = "target_price_snapshot_no";
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT
        COUNT(*),
        COALESCE(SUM("amount_minor") FILTER (
            WHERE "component_type_code" IN ('base', 'option')
        ), 0),
        COALESCE(SUM("amount_minor") FILTER (
            WHERE "component_type_code" = 'discount'
        ), 0),
        COALESCE(SUM("amount_minor") FILTER (
            WHERE "component_type_code" = 'tax'
        ), 0)
    INTO actual_component_count, actual_subtotal, actual_discount, actual_tax
    FROM plugin_idc_core_service_price_component_snapshot
    WHERE "price_snapshot_no" = "target_price_snapshot_no";
    IF actual_component_count <> expected_component_count
        OR actual_subtotal <> expected_subtotal
        OR actual_discount <> expected_discount
        OR actual_tax <> expected_tax THEN
        RAISE EXCEPTION 'service price components do not match snapshot %',
            "target_price_snapshot_no";
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_business_operation_events()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_operation_no VARCHAR(40);
    operation_status VARCHAR(32);
    event_count BIGINT;
    maximum_sequence INT;
    latest_status VARCHAR(32);
    invalid_transitions BIGINT;
BEGIN
    target_operation_no := COALESCE(NEW."operation_no", OLD."operation_no");
    SELECT "status_code" INTO operation_status
    FROM plugin_idc_core_business_operation
    WHERE "operation_no" = target_operation_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT COUNT(*), COALESCE(MAX("event_sequence"), 0)
    INTO event_count, maximum_sequence
    FROM plugin_idc_core_business_operation_event
    WHERE "operation_no" = target_operation_no;
    SELECT "to_status_code" INTO latest_status
    FROM plugin_idc_core_business_operation_event
    WHERE "operation_no" = target_operation_no
    ORDER BY "event_sequence" DESC
    LIMIT 1;
    SELECT COUNT(*) INTO invalid_transitions
    FROM (
        SELECT "event_sequence", "from_status_code",
               LAG("to_status_code") OVER (ORDER BY "event_sequence") AS "previous_status"
        FROM plugin_idc_core_business_operation_event
        WHERE "operation_no" = target_operation_no
    ) events
    WHERE ("event_sequence" = 1 AND "from_status_code" IS NOT NULL)
       OR ("event_sequence" > 1
           AND "from_status_code" IS DISTINCT FROM "previous_status");
    IF event_count = 0 OR event_count <> maximum_sequence
        OR latest_status IS DISTINCT FROM operation_status
        OR invalid_transitions <> 0 THEN
        RAISE EXCEPTION 'business operation events do not match operation %',
            target_operation_no;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_service_price_immutable
    ON plugin_idc_core_service_price_snapshot;
CREATE TRIGGER trg_idc_service_price_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_price_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_price_component_immutable
    ON plugin_idc_core_service_price_component_snapshot;
CREATE TRIGGER trg_idc_service_price_component_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_price_component_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_renewal_policy_immutable
    ON plugin_idc_core_service_renewal_policy;
CREATE TRIGGER trg_idc_service_renewal_policy_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_renewal_policy
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_business_operation_event_immutable
    ON plugin_idc_core_business_operation_event;
CREATE TRIGGER trg_idc_business_operation_event_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_business_operation_event
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_business_operation_intent_guard
    ON plugin_idc_core_business_operation;
CREATE TRIGGER trg_idc_business_operation_intent_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_business_operation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_business_operation_intent();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_service_price_snapshot'::regclass
           AND conname = 'trg_idc_service_price_component_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_service_price_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_service_price_component_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_service_price_component_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_service_price_components()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_service_price_component_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_service_price_component_composition
        AFTER INSERT ON plugin_idc_core_service_price_snapshot
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_service_price_components();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_service_price_component_snapshot'::regclass
           AND conname = 'trg_idc_service_price_component_item_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_service_price_component_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_service_price_component_item_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_service_price_component_item_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_service_price_components()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_service_price_component_item_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_service_price_component_item_composition
        AFTER INSERT ON plugin_idc_core_service_price_component_snapshot
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_service_price_components();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_business_operation'::regclass
           AND conname = 'trg_idc_business_operation_event_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_business_operation'::regclass
               AND constraint_row.conname = 'trg_idc_business_operation_event_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_business_operation_event_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_business_operation_events()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_business_operation_event_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_business_operation_event_composition
        AFTER INSERT OR UPDATE ON plugin_idc_core_business_operation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_business_operation_events();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_business_operation_event'::regclass
           AND conname = 'trg_idc_business_operation_event_item_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_business_operation_event'::regclass
               AND constraint_row.conname = 'trg_idc_business_operation_event_item_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_business_operation_event_item_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_business_operation_events()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_business_operation_event_item_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_business_operation_event_item_composition
        AFTER INSERT ON plugin_idc_core_business_operation_event
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_business_operation_events();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_service_instance
    IS 'Core-owned customer entitlement and runtime projection; Provider truth remains resource-owned';
COMMENT ON TABLE plugin_idc_core_service_price_snapshot
    IS 'Append-only new-purchase or renewal commercial snapshot bound to one service';
COMMENT ON TABLE plugin_idc_core_service_price_component_snapshot
    IS 'Append-only bounded price composition for one service commercial snapshot';
COMMENT ON TABLE plugin_idc_core_service_renewal_policy
    IS 'Append-only per-service automatic renewal policy versions';
COMMENT ON TABLE plugin_idc_core_business_operation
    IS 'Unified idempotent business saga projection with immutable intent';
COMMENT ON TABLE plugin_idc_core_business_operation_event
    IS 'Append-only business-operation transition evidence';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 服务权益状态', 'idc_service_entitlement_status', 1, 1, FALSE,
     '客户服务权益生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 服务运行状态', 'idc_service_runtime_status', 1, 1, FALSE,
     '服务器本地运行状态投影', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC Provider 任务状态', 'idc_service_provider_task_status', 1, 1, FALSE,
     '服务关联 Provider 任务的脱敏状态投影', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 续费履约模式', 'idc_service_renewal_mode', 1, 1, FALSE,
     '等待 Provider 确认或只推进本地权益', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 业务操作类型', 'idc_business_operation_type', 1, 1, FALSE,
     '统一服务业务操作类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 业务操作状态', 'idc_business_operation_status', 1, 1, FALSE,
     '统一业务操作 Saga 生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 服务价格快照阶段', 'idc_service_price_phase', 1, 1, FALSE,
     '新购或续费不可变价格快照', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_service_entitlement_status', '待生效', 'pending_activation', 1, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '有效', 'active', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '宽限期', 'grace', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '已到期', 'expired', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '终止中', 'terminating', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '已终止', 'terminated', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_entitlement_status', '已取消', 'cancelled', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '待开通', 'pending_provision', 1, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '开通中', 'provisioning', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '运行中', 'running', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '已关机', 'stopped', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '已暂停', 'suspended', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '恢复中', 'resuming', 6, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '变更中', 'changing', 7, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '终止中', 'terminating', 8, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '已终止', 'terminated', 9, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_runtime_status', '未知', 'unknown', 10, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '无任务', 'none', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '等待执行', 'pending', 2, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '执行中', 'executing', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '已成功', 'succeeded', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '已失败', 'failed', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '结果未知', 'result_unknown', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_provider_task_status', '人工处理', 'manual_review', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_mode', '尚未确定', 'unresolved', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_mode', 'Provider 确认', 'provider_confirmed', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_mode', '本地权益', 'local_entitlement', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '新购', 'new_purchase', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '续费', 'renewal', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '变配', 'change', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '更换 IP', 'ip_change', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '附属能力', 'addon', 5, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '退款', 'refund', 6, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '暂停', 'suspend', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '恢复', 'resume', 8, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '终止', 'terminate', 9, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '已报价', 'quoted', 1, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '等待财务结算', 'awaiting_financial_settlement', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '财务已结算', 'financially_settled', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '就绪', 'ready', 4, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '执行中', 'executing', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '成功', 'succeeded', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '失败', 'failed', 7, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '补偿中', 'compensating', 8, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '退款中', 'refunding', 9, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '人工处理', 'manual_review', 10, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_status', '已取消', 'cancelled', 11, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_price_phase', '新购', 'new_purchase', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_price_phase', '续费', 'renewal', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
