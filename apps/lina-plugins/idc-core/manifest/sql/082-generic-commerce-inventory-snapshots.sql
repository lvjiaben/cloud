-- 082: Freeze simple product inventory in commerce without requiring server
-- product types, resource-pool bindings, or provider inventory snapshots.

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_product,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_item_binding,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_item_inventory,
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_inventory;

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD CONSTRAINT ck_idc_com_quote_item_inventory CHECK (
        "inventory_decision_code" IN ('available','manual_confirmation')
        AND (
            ("binding_no" = '' AND "inventory_snapshot_no" IS NULL)
            OR ("binding_no" <> '' AND (
                ("inventory_decision_code"='available' AND "inventory_snapshot_no" IS NOT NULL)
                OR "inventory_decision_code"='manual_confirmation'
            ))
        )
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );

ALTER TABLE plugin_idc_core_commerce_order_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_product,
    DROP CONSTRAINT IF EXISTS fk_idc_com_order_item_binding,
    DROP CONSTRAINT IF EXISTS fk_idc_com_order_item_inventory,
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_policy;

ALTER TABLE plugin_idc_core_commerce_order_item
    ADD CONSTRAINT ck_idc_com_order_item_policy CHECK (
        "fulfillment_mode_code" IN ('automatic','manual_review')
        AND "risk_decision_code" IN ('approved','manual_review')
        AND "inventory_decision_code" IN ('available','manual_confirmation')
        AND (
            ("initial_binding_no" = '' AND "inventory_snapshot_no" IS NULL)
            OR ("initial_binding_no" <> '' AND (
                ("inventory_decision_code"='available' AND "inventory_snapshot_no" IS NOT NULL)
                OR "inventory_decision_code"='manual_confirmation'
            ))
        )
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );

-- Purpose: Stores Commerce Quote Item Inventory Snapshot records.
-- 用途：保存交易报价明细库存快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_quote_item_inventory_snapshot (
    "quote_item_no" VARCHAR(40) PRIMARY KEY,
    "version_no" VARCHAR(40) NOT NULL,
    "tracking_enabled" BOOLEAN NOT NULL,
    "inventory_version" BIGINT NOT NULL,
    "sellable_quantity" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_quote_inventory_values CHECK (
        "inventory_version" >= 0 AND "sellable_quantity" >= 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT fk_idc_com_quote_inventory_item FOREIGN KEY ("quote_item_no")
        REFERENCES plugin_idc_core_commerce_quote_item_snapshot("quote_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_quote_inventory_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

DROP TRIGGER IF EXISTS trg_idc_com_quote_inventory_immutable
    ON plugin_idc_core_commerce_quote_item_inventory_snapshot;
CREATE TRIGGER trg_idc_com_quote_inventory_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_quote_item_inventory_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

-- Purpose: Stores Commerce Order Item Inventory Snapshot records.
-- 用途：保存交易订单明细库存快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_item_inventory_snapshot (
    "order_item_no" VARCHAR(40) PRIMARY KEY,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "tracking_enabled" BOOLEAN NOT NULL,
    "inventory_version" BIGINT NOT NULL,
    "sellable_quantity" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_order_inventory_values CHECK (
        "inventory_version" >= 0 AND "sellable_quantity" >= 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT fk_idc_com_order_inventory_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item("order_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_inventory_quote FOREIGN KEY ("quote_item_no")
        REFERENCES plugin_idc_core_commerce_quote_item_inventory_snapshot("quote_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_order_inventory_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

DROP TRIGGER IF EXISTS trg_idc_com_order_inventory_immutable
    ON plugin_idc_core_commerce_order_item_inventory_snapshot;
CREATE TRIGGER trg_idc_com_order_inventory_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_order_item_inventory_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    ALTER COLUMN "binding_no" DROP NOT NULL,
    ALTER COLUMN "inventory_snapshot_no" DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS "version_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "inventory_version" BIGINT;

ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    DROP CONSTRAINT IF EXISTS fk_idc_com_inventory_res_snapshot,
    DROP CONSTRAINT IF EXISTS fk_idc_com_inventory_res_version,
    DROP CONSTRAINT IF EXISTS ck_idc_com_inventory_res_source;

ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    ADD CONSTRAINT ck_idc_com_inventory_res_source CHECK (
        ("binding_no" IS NOT NULL AND "inventory_snapshot_no" IS NOT NULL
            AND "version_no" IS NULL AND "inventory_version" IS NULL)
        OR ("binding_no" IS NULL AND "inventory_snapshot_no" IS NULL
            AND "version_no" IS NOT NULL AND "inventory_version" >= 1)
    ),
    ADD CONSTRAINT fk_idc_com_inventory_res_snapshot FOREIGN KEY (
        "binding_no","inventory_snapshot_no"
    ) REFERENCES plugin_idc_core_catalog_inventory_snapshot("binding_no","snapshot_no") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_com_inventory_res_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_idc_com_inventory_res_product_capacity
    ON plugin_idc_core_commerce_inventory_reservation(
        "version_no","status_code","hold_expires_at","id"
    ) WHERE "version_no" IS NOT NULL AND "status_code" IN ('held','committed');
