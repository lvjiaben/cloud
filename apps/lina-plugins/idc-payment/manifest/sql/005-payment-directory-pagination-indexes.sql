-- Purpose: Keep exact administrative payment pages ordered without full-directory sorts.
-- 用途：为支付目录分页提供匹配过滤范围与稳定排序的索引；不修改业务数据。
CREATE INDEX IF NOT EXISTS idx_idc_payment_channel_health_page
    ON plugin_idc_payment_channel (display_name, channel_no)
    WHERE tenant_id = 0 AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_idc_payment_batch_channel_page
    ON plugin_idc_payment_reconciliation_batch (channel_no, business_date DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_idc_payment_batch_all_page
    ON plugin_idc_payment_reconciliation_batch (business_date DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_idc_payment_difference_page
    ON plugin_idc_payment_reconciliation_line (batch_no, id)
    WHERE difference_code <> 'none';

CREATE INDEX IF NOT EXISTS idx_idc_payment_exception_pending_page
    ON plugin_idc_payment_exception_fund (created_at, id)
    WHERE status_code IN ('pending', 'disputed');
