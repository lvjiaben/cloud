-- 093: Inventory is a simple product-version setting. A shortage is not an
-- approval state: new-purchase quotes, orders, and fulfillment may only freeze
-- `available`. Renewal quotes do not sell a new unit and keep `not_required`.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_commerce_quote_item_snapshot
        WHERE "inventory_decision_code" NOT IN ('available', 'not_required')
    ) OR EXISTS (
        SELECT 1
        FROM plugin_idc_core_commerce_order_item
        WHERE "inventory_decision_code" <> 'available'
    ) THEN
        RAISE EXCEPTION
            'legacy manual inventory decisions exist; rebuild this development database from current migrations';
    END IF;
END $$;

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_inventory;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD CONSTRAINT ck_idc_com_quote_item_inventory CHECK (
        "inventory_decision_code" IN ('available', 'not_required')
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );

ALTER TABLE plugin_idc_core_commerce_order_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_policy;
ALTER TABLE plugin_idc_core_commerce_order_item
    ADD CONSTRAINT ck_idc_com_order_item_policy CHECK (
        "fulfillment_mode_code" IN ('automatic','manual_review')
        AND "risk_decision_code" IN ('approved','manual_review')
        AND "inventory_decision_code" = 'available'
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );

DELETE FROM sys_dict_data
WHERE "dict_type" = 'idc_commerce_inventory_decision'
  AND "value" = 'manual_confirmation';
