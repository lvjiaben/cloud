-- Contract policy is installation-local business state. The three levels stay
-- distinct so "inherit" can never be confused with an explicit "no contract".

ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "enterprise_contract_policy_code" VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS "enterprise_contract_template_no" VARCHAR(64);

ALTER TABLE plugin_idc_core_catalog_product_version
    ADD COLUMN IF NOT EXISTS "contract_policy_code" VARCHAR(16) NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS "contract_template_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "contract_policy_version" BIGINT NOT NULL DEFAULT 1;

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD COLUMN IF NOT EXISTS "contract_policy_code" VARCHAR(16) NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS "contract_template_no" VARCHAR(64);

ALTER TABLE plugin_idc_core_commerce_order_item
    ADD COLUMN IF NOT EXISTS "contract_policy_code" VARCHAR(16) NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS "contract_template_no" VARCHAR(64);

ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
    ADD COLUMN IF NOT EXISTS "contract_override_policy_code" VARCHAR(16) NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS "contract_override_template_no" VARCHAR(64);

ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
    DROP COLUMN IF EXISTS "contract_policy_code",
    DROP COLUMN IF EXISTS "contract_template_no";

ALTER TABLE plugin_idc_core_commerce_order
    ADD COLUMN IF NOT EXISTS "contract_override_policy_code" VARCHAR(16) NOT NULL DEFAULT 'inherit',
    ADD COLUMN IF NOT EXISTS "contract_override_template_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "enterprise_contract_policy_code" VARCHAR(16) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS "enterprise_contract_template_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "contract_decision_source_code" VARCHAR(16) NOT NULL DEFAULT 'default',
    ADD COLUMN IF NOT EXISTS "contract_template_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "contract_template_version_no" BIGINT,
    ADD COLUMN IF NOT EXISTS "contract_template_name" VARCHAR(200),
    ADD COLUMN IF NOT EXISTS "contract_language_code" VARCHAR(8),
    ADD COLUMN IF NOT EXISTS "contract_scope_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "contract_signing_order_code" VARCHAR(24);

ALTER TABLE plugin_idc_core_customer_subject
    DROP CONSTRAINT IF EXISTS ck_idc_customer_enterprise_contract_policy;
ALTER TABLE plugin_idc_core_catalog_product_version
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_version_contract_policy;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_contract_policy;
ALTER TABLE plugin_idc_core_commerce_order_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_contract_policy;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
    DROP CONSTRAINT IF EXISTS ck_idc_com_assisted_contract_policy;
ALTER TABLE plugin_idc_core_commerce_order
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_contract_policy;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_customer_enterprise_contract_policy'
          AND conrelid = 'plugin_idc_core_customer_subject'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_customer_subject
            ADD CONSTRAINT ck_idc_customer_enterprise_contract_policy CHECK (
                (
                    "customer_type_code" = 'enterprise' AND
                    "enterprise_contract_policy_code" IN ('none', 'required') AND
                    (
                        ("enterprise_contract_policy_code" = 'none' AND "enterprise_contract_template_no" IS NULL) OR
                        ("enterprise_contract_policy_code" = 'required' AND "enterprise_contract_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                    )
                ) OR (
                    "customer_type_code" = 'individual' AND
                    "enterprise_contract_policy_code" = 'none' AND
                    "enterprise_contract_template_no" IS NULL
                )
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_order_item_contract_policy'
          AND conrelid = 'plugin_idc_core_commerce_order_item'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order_item
            ADD CONSTRAINT ck_idc_com_order_item_contract_policy CHECK (
                "contract_policy_code" IN ('inherit', 'none', 'required') AND
                (
                    ("contract_policy_code" IN ('inherit', 'none') AND "contract_template_no" IS NULL) OR
                    ("contract_policy_code" = 'required' AND "contract_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                )
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_catalog_version_contract_policy'
          AND conrelid = 'plugin_idc_core_catalog_product_version'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_catalog_product_version
            ADD CONSTRAINT ck_idc_catalog_version_contract_policy CHECK (
                "contract_policy_code" IN ('inherit', 'none', 'required') AND
                "contract_policy_version" >= 1 AND
                (
                    ("contract_policy_code" IN ('inherit', 'none') AND "contract_template_no" IS NULL) OR
                    ("contract_policy_code" = 'required' AND "contract_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                )
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_quote_item_contract_policy'
          AND conrelid = 'plugin_idc_core_commerce_quote_item_snapshot'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
            ADD CONSTRAINT ck_idc_com_quote_item_contract_policy CHECK (
                "contract_policy_code" IN ('inherit', 'none', 'required') AND
                (
                    ("contract_policy_code" IN ('inherit', 'none') AND "contract_template_no" IS NULL) OR
                    ("contract_policy_code" = 'required' AND "contract_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                )
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_assisted_contract_policy'
          AND conrelid = 'plugin_idc_core_commerce_assisted_quote_request'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
            ADD CONSTRAINT ck_idc_com_assisted_contract_policy CHECK (
                "contract_override_policy_code" IN ('inherit', 'none', 'required') AND
                (
                    ("contract_override_policy_code" IN ('inherit', 'none') AND "contract_override_template_no" IS NULL) OR
                    ("contract_override_policy_code" = 'required' AND "contract_override_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                )
            ) NOT VALID;
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_order_contract_policy'
          AND conrelid = 'plugin_idc_core_commerce_order'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order
            ADD CONSTRAINT ck_idc_com_order_contract_policy CHECK (
                "contract_override_policy_code" IN ('inherit', 'none', 'required') AND
                (
                    ("contract_override_policy_code" IN ('inherit', 'none') AND "contract_override_template_no" IS NULL) OR
                    ("contract_override_policy_code" = 'required' AND "contract_override_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                ) AND
                "enterprise_contract_policy_code" IN ('none', 'required') AND
                (
                    ("enterprise_contract_policy_code" = 'none' AND "enterprise_contract_template_no" IS NULL) OR
                    ("enterprise_contract_policy_code" = 'required' AND "enterprise_contract_template_no" ~ '^ctpl_[0-9a-f]{32}$')
                ) AND
                "contract_decision_source_code" IN ('default', 'enterprise', 'product', 'order') AND
                (
                    (
                        "requires_contract" = FALSE AND
                        "contract_template_no" IS NULL AND "contract_template_version_no" IS NULL AND
                        "contract_template_name" IS NULL AND
                        "contract_language_code" IS NULL AND "contract_scope_code" IS NULL AND
                        "contract_signing_order_code" IS NULL
                    ) OR (
                        "requires_contract" = TRUE AND
                        "contract_template_no" ~ '^ctpl_[0-9a-f]{32}$' AND
                        "contract_template_version_no" > 0 AND
                        LENGTH(BTRIM("contract_template_name")) BETWEEN 1 AND 200 AND
                        "contract_language_code" IN ('zh-CN', 'en-US') AND
                        "contract_scope_code" IN ('personal', 'enterprise', 'all_customers') AND
                        "contract_signing_order_code" IN ('customer_first', 'merchant_first', 'parallel')
                    )
                )
            ) NOT VALID;
    END IF;
END $$;

ALTER TABLE plugin_idc_core_customer_subject
    VALIDATE CONSTRAINT ck_idc_customer_enterprise_contract_policy;
ALTER TABLE plugin_idc_core_catalog_product_version
    VALIDATE CONSTRAINT ck_idc_catalog_version_contract_policy;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    VALIDATE CONSTRAINT ck_idc_com_quote_item_contract_policy;
ALTER TABLE plugin_idc_core_commerce_order_item
    VALIDATE CONSTRAINT ck_idc_com_order_item_contract_policy;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
    VALIDATE CONSTRAINT ck_idc_com_assisted_contract_policy;
ALTER TABLE plugin_idc_core_commerce_order
    VALIDATE CONSTRAINT ck_idc_com_order_contract_policy;

CREATE INDEX IF NOT EXISTS idx_idc_customer_enterprise_contract_policy
    ON plugin_idc_core_customer_subject (
        "tenant_id", "enterprise_contract_policy_code", "enterprise_contract_template_no", "id"
    ) WHERE "customer_type_code" = 'enterprise';

CREATE INDEX IF NOT EXISTS idx_idc_catalog_version_contract_policy
    ON plugin_idc_core_catalog_product_version (
        "tenant_id", "contract_policy_code", "contract_template_no", "id"
    );

COMMENT ON COLUMN plugin_idc_core_customer_subject."enterprise_contract_policy_code" IS
    'Enterprise default: none or required; it affects only orders created after the setting changes';
COMMENT ON COLUMN plugin_idc_core_catalog_product_version."contract_policy_code" IS
    'Product choice: inherit enterprise, explicit none, or required';
COMMENT ON COLUMN plugin_idc_core_commerce_assisted_quote_request."contract_override_policy_code" IS
    'Administrator order choice captured before assisted checkout';
COMMENT ON COLUMN plugin_idc_core_commerce_order."contract_override_policy_code" IS
    'Order choice preserved separately: inherit product, explicit none, or required';
COMMENT ON COLUMN plugin_idc_core_commerce_order."contract_decision_source_code" IS
    'Winning rule source after order > product > enterprise > default resolution';
