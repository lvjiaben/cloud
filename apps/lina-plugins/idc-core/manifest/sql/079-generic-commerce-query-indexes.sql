-- 079: Query indexes for generic commerce quantity snapshots.
-- The order directory and fulfillment assembly load all item quantities for
-- one tenant and order; keep that lookup independent of total table size.

CREATE INDEX IF NOT EXISTS idx_idc_core_order_item_quantity_order
    ON plugin_idc_core_commerce_order_item_quantity_snapshot("tenant_id", "order_no", "order_item_no");
