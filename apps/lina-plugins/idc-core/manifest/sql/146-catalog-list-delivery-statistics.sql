-- Bounded catalog-page delivery statistics use completed product intents.
-- This index changes no business data and is safe to apply repeatedly.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_fulfillment_product_completed
 ON plugin_idc_core_product_fulfillment_intent(tenant_id,product_no)
 INCLUDE(intent_no,granularity_code,quantity) WHERE status_code='completed';
