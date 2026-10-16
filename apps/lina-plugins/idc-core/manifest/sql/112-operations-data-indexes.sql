-- Support currency-local payment windows and stable operating-data drill-downs.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_order_paid_window
    ON plugin_idc_core_commerce_order (tenant_id, currency_code, paid_at DESC, order_no DESC)
    WHERE paid_at IS NOT NULL;

-- Registration metrics do not filter customer lifecycle status.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_registered
    ON plugin_idc_core_customer_subject (tenant_id, created_at DESC, customer_no DESC);
