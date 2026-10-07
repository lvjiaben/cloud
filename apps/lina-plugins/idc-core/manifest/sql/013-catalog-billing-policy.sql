-- 013: Complete server billing-period and metered-usage policy semantics

ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD COLUMN IF NOT EXISTS "minimum_purchase_cycles" INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "maximum_renewal_cycles" INT,
    ADD COLUMN IF NOT EXISTS "meter_unit_code" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "tier_mode_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "usage_collection_unit_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "usage_collection_count" INT,
    ADD COLUMN IF NOT EXISTS "usage_rounding_mode_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "usage_rounding_quantity" BIGINT,
    ADD COLUMN IF NOT EXISTS "usage_minimum_quantity" BIGINT,
    ADD COLUMN IF NOT EXISTS "usage_maximum_quantity" BIGINT,
    ADD COLUMN IF NOT EXISTS "usage_missing_policy_code" VARCHAR(32);

ALTER TABLE plugin_idc_core_catalog_billing_rule
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_billing_period;
ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD CONSTRAINT ck_idc_catalog_billing_period CHECK (
        ("period_unit_code" = 'once' AND "period_count" = 1)
        OR ("period_unit_code" = 'hour' AND "period_count" BETWEEN 1 AND 87600)
        OR ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
        OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
        OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10)
    );

ALTER TABLE plugin_idc_core_catalog_billing_rule
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_billing_cycle_limits;
ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD CONSTRAINT ck_idc_catalog_billing_cycle_limits CHECK (
        "minimum_purchase_cycles" BETWEEN 1 AND 120
        AND (
            "maximum_renewal_cycles" IS NULL
            OR "maximum_renewal_cycles" BETWEEN 1 AND 120
        )
    );

ALTER TABLE plugin_idc_core_catalog_billing_rule
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_billing_semantics;
ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD CONSTRAINT ck_idc_catalog_billing_semantics CHECK (
        (
            "charge_type_code" = 'one_time'
            AND "period_unit_code" = 'once'
            AND "price_phase_code" = 'new_purchase'
            AND "quantity_mode_code" = 'fixed'
            AND "trial_quantity" = 0
            AND "minimum_purchase_cycles" = 1
            AND "maximum_renewal_cycles" IS NULL
        ) OR (
            "charge_type_code" = 'recurring'
            AND "period_unit_code" <> 'once'
            AND "quantity_mode_code" IN ('fixed', 'per_unit')
            AND "trial_quantity" = 0
        ) OR (
            "charge_type_code" = 'tiered'
            AND "period_unit_code" <> 'once'
            AND "quantity_mode_code" = 'tiered'
            AND "trial_quantity" = 0
        ) OR (
            "charge_type_code" = 'usage'
            AND "period_unit_code" <> 'once'
            AND "quantity_mode_code" = 'usage'
            AND "trial_quantity" = 0
        ) OR (
            "charge_type_code" = 'trial'
            AND "period_unit_code" <> 'once'
            AND "price_phase_code" = 'new_purchase'
            AND "quantity_mode_code" = 'fixed'
            AND "trial_quantity" BETWEEN 1 AND 100
            AND "minimum_purchase_cycles" = 1
            AND "maximum_renewal_cycles" IS NULL
        )
    );

ALTER TABLE plugin_idc_core_catalog_billing_rule
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_billing_renewal_limit;
ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD CONSTRAINT ck_idc_catalog_billing_renewal_limit CHECK (
        "price_phase_code" <> 'new_purchase' OR "maximum_renewal_cycles" IS NULL
    );

ALTER TABLE plugin_idc_core_catalog_billing_rule
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_billing_usage_policy;
ALTER TABLE plugin_idc_core_catalog_billing_rule
    ADD CONSTRAINT ck_idc_catalog_billing_usage_policy CHECK (
        (
            "charge_type_code" = 'usage'
            AND "meter_unit_code" IS NOT NULL
            AND "meter_unit_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
            AND "tier_mode_code" IS NOT NULL
            AND "tier_mode_code" = 'graduated'
            AND "usage_collection_unit_code" IS NOT NULL
            AND "usage_collection_unit_code" IN ('hour', 'day', 'month')
            AND "usage_collection_count" IS NOT NULL
            AND "usage_collection_count" BETWEEN 1 AND 120
            AND "usage_rounding_mode_code" IS NOT NULL
            AND "usage_rounding_mode_code" IN ('down', 'up', 'nearest')
            AND "usage_rounding_quantity" IS NOT NULL
            AND "usage_rounding_quantity" > 0
            AND "usage_minimum_quantity" IS NOT NULL
            AND "usage_minimum_quantity" >= 0
            AND "usage_missing_policy_code" IS NOT NULL
            AND (
                "usage_maximum_quantity" IS NULL
                OR "usage_maximum_quantity" >= "usage_minimum_quantity"
            )
            AND "usage_missing_policy_code" IN ('pending_recalculation', 'estimate_previous_period')
        ) OR (
            "charge_type_code" = 'tiered'
            AND "meter_unit_code" IS NULL
            AND "tier_mode_code" IS NOT NULL
            AND "tier_mode_code" = 'graduated'
            AND "usage_collection_unit_code" IS NULL
            AND "usage_collection_count" IS NULL
            AND "usage_rounding_mode_code" IS NULL
            AND "usage_rounding_quantity" IS NULL
            AND "usage_minimum_quantity" IS NULL
            AND "usage_maximum_quantity" IS NULL
            AND "usage_missing_policy_code" IS NULL
        ) OR (
            "charge_type_code" NOT IN ('usage', 'tiered')
            AND "meter_unit_code" IS NULL
            AND "tier_mode_code" IS NULL
            AND "usage_collection_unit_code" IS NULL
            AND "usage_collection_count" IS NULL
            AND "usage_rounding_mode_code" IS NULL
            AND "usage_rounding_quantity" IS NULL
            AND "usage_minimum_quantity" IS NULL
            AND "usage_maximum_quantity" IS NULL
            AND "usage_missing_policy_code" IS NULL
        )
    );

ALTER TABLE plugin_idc_core_catalog_price
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_price_quantity;
ALTER TABLE plugin_idc_core_catalog_price
    ADD CONSTRAINT ck_idc_catalog_price_quantity CHECK (
        "minimum_quantity" >= 0
        AND ("maximum_quantity" IS NULL OR "maximum_quantity" > "minimum_quantity")
    );

COMMENT ON COLUMN plugin_idc_core_catalog_billing_rule."minimum_purchase_cycles"
    IS 'Minimum number of this explicit billing period accepted for a new purchase';
COMMENT ON COLUMN plugin_idc_core_catalog_billing_rule."maximum_renewal_cycles"
    IS 'Optional maximum number of this explicit billing period accepted per renewal quote';
COMMENT ON COLUMN plugin_idc_core_catalog_billing_rule."meter_unit_code"
    IS 'Merchant-defined normalized authoritative meter unit for usage billing';
COMMENT ON COLUMN plugin_idc_core_catalog_billing_rule."usage_missing_policy_code"
    IS 'Fail-closed pending recalculation or explicit previous-period estimate policy';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 阶梯计价模式', 'idc_catalog_tier_mode', 1, 1, FALSE,
     '阶梯价格的区间计算方式', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 用量采集周期单位', 'idc_catalog_usage_collection_unit', 1, 1, FALSE,
     '权威计量数据的采集周期单位', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 用量舍入模式', 'idc_catalog_usage_rounding_mode', 1, 1, FALSE,
     '按量计费数量的整数舍入方式', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 用量缺失策略', 'idc_catalog_usage_missing_policy', 1, 1, FALSE,
     '权威用量缺失时的失败关闭或明确暂估策略', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_catalog_tier_mode', '分段累进', 'graduated', 1, 'primary',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_collection_unit', '小时', 'hour', 1, 'processing',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_collection_unit', '天', 'day', 2, 'cyan',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_collection_unit', '月', 'month', 3, 'primary',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_rounding_mode', '向下舍入', 'down', 1, 'default',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_rounding_mode', '向上舍入', 'up', 2, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_rounding_mode', '四舍五入', 'nearest', 3, 'primary',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_missing_policy', '等待补算', 'pending_recalculation', 1, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_usage_missing_policy', '按上一完整周期暂估', 'estimate_previous_period', 2, 'processing',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
