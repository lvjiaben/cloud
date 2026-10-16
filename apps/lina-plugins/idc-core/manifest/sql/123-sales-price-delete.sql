-- Purpose: Allow immutable price deletion revisions without erasing financial history.
-- 用途：允许追加价格删除版本，历史报价和订单快照保持不变。
ALTER TABLE plugin_idc_core_sales_purchase_price DROP CONSTRAINT IF EXISTS ck_idc_core_sales_purchase_price;
ALTER TABLE plugin_idc_core_sales_purchase_price ADD CONSTRAINT ck_idc_core_sales_purchase_price CHECK (
  tenant_id = 0 AND version > 0 AND currency_code ~ '^[A-Z]{3}$'
  AND ((customer_no IS NOT NULL AND level_no IS NULL) OR (customer_no IS NULL AND level_no IS NOT NULL))
  AND status_code IN ('active','inactive','deleted')
  AND charge_type_code IN ('one_time','recurring','trial','usage','tiered')
  AND period_count >= 0
  AND content_digest ~ '^[0-9a-f]{64}$'
  AND ((adjustment_type_code='fixed_price' AND fixed_price_minor >= 0 AND discount_basis_points IS NULL)
    OR (adjustment_type_code='discount_basis_points' AND fixed_price_minor IS NULL AND discount_basis_points BETWEEN 0 AND 10000))
 );

-- Product-local pricing pages use the product key before latest-version checks.
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_purchase_price_product
 ON plugin_idc_core_sales_purchase_price(tenant_id,product_no,id DESC);
