-- 014: Local inventory projection and deterministic resource-pool routing policy

ALTER TABLE plugin_idc_core_catalog_resource_pool_binding
    ADD COLUMN IF NOT EXISTS "maintenance_mode_code" VARCHAR(16) NOT NULL DEFAULT 'normal',
    ADD COLUMN IF NOT EXISTS "minimum_sellable_quantity" BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "uncertain_sale_policy_code" VARCHAR(32) NOT NULL DEFAULT 'stop_sale';

ALTER TABLE plugin_idc_core_catalog_resource_pool_binding
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_pool_binding_inventory_policy;
ALTER TABLE plugin_idc_core_catalog_resource_pool_binding
    ADD CONSTRAINT ck_idc_catalog_pool_binding_inventory_policy CHECK (
        "maintenance_mode_code" IN ('normal', 'drain', 'maintenance')
        AND "minimum_sellable_quantity" BETWEEN 1 AND 1000000000
        AND "uncertain_sale_policy_code" IN ('stop_sale', 'manual_confirmation')
    );

ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    ADD COLUMN IF NOT EXISTS "source_completed_at" TIMESTAMPTZ;

UPDATE plugin_idc_core_catalog_inventory_snapshot
SET "source_completed_at" = "observed_at"
WHERE "source_completed_at" IS NULL;

ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    ALTER COLUMN "source_completed_at" SET NOT NULL;

ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_inventory_quantity;
ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    ADD CONSTRAINT ck_idc_catalog_inventory_quantity CHECK (
        "safety_margin" >= 0
        AND (
            "availability_code" IN ('available', 'limited')
            AND "available_quantity" IS NOT NULL
            AND "available_quantity" >= 0
            AND "sellable_quantity" IS NOT NULL
            AND "sellable_quantity" = GREATEST("available_quantity" - "safety_margin", 0)
        OR
            "availability_code" = 'unavailable'
            AND "available_quantity" = 0
            AND "sellable_quantity" = 0
        OR
            "availability_code" IN ('unknown', 'stale')
            AND "available_quantity" IS NULL
            AND "sellable_quantity" IS NULL
        )
    );

ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_inventory_source_completion;
ALTER TABLE plugin_idc_core_catalog_inventory_snapshot
    ADD CONSTRAINT ck_idc_catalog_inventory_source_completion CHECK (
        "source_completed_at" >= "observed_at"
        AND "source_completed_at" < "expires_at"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_catalog_inventory_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'catalog inventory snapshots are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_catalog_inventory_immutable
    ON plugin_idc_core_catalog_inventory_snapshot;
CREATE TRIGGER trg_idc_catalog_inventory_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_inventory_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_catalog_inventory_mutation();

COMMENT ON COLUMN plugin_idc_core_catalog_resource_pool_binding."maintenance_mode_code"
    IS 'Normal, drain, or maintenance mode for new-order routing only';
COMMENT ON COLUMN plugin_idc_core_catalog_resource_pool_binding."minimum_sellable_quantity"
    IS 'Local sellable quantity threshold required before automatic fulfillment is promised';
COMMENT ON COLUMN plugin_idc_core_catalog_resource_pool_binding."uncertain_sale_policy_code"
    IS 'Fail-closed stop-sale or explicit manual-confirmation policy for uncertain inventory';
COMMENT ON COLUMN plugin_idc_core_catalog_inventory_snapshot."source_completed_at"
    IS 'Completion time of the authoritative source snapshot used for this local projection';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 资源池维护模式', 'idc_catalog_pool_maintenance_mode', 1, 1, FALSE,
     '资源池对新订单的正常、排空或维护状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 不确定库存销售策略', 'idc_catalog_uncertain_sale_policy', 1, 1, FALSE,
     '本地库存未知或过期时停止销售或转人工确认', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_catalog_pool_maintenance_mode', '正常', 'normal', 1, 'success',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_pool_maintenance_mode', '排空', 'drain', 2, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_pool_maintenance_mode', '维护', 'maintenance', 3, 'error',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_uncertain_sale_policy', '停止销售', 'stop_sale', 1, 'error',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_catalog_uncertain_sale_policy', '人工确认', 'manual_confirmation', 2, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
