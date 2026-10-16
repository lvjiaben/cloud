-- Purpose: Receipts and offline payout evidence for the customer business loop.
-- 用途：保存实收计佣幂等凭据与线下打款证据，不改变客户消费余额。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commission_order_receipt (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    order_no VARCHAR(40) UNIQUE REFERENCES plugin_idc_core_commerce_order(order_no),
    invoice_no VARCHAR(40) NOT NULL UNIQUE REFERENCES plugin_idc_core_billing_invoice(invoice_no),
    settlement_no VARCHAR(40) NOT NULL,
    source_digest CHAR(64) NOT NULL CHECK (source_digest ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

ALTER TABLE plugin_idc_core_commission_accrual ALTER COLUMN order_no DROP NOT NULL;
ALTER TABLE plugin_idc_core_commission_order_receipt ALTER COLUMN order_no DROP NOT NULL;
ALTER TABLE plugin_idc_core_commission_order_receipt ADD COLUMN IF NOT EXISTS invoice_no VARCHAR(40) REFERENCES plugin_idc_core_billing_invoice(invoice_no);
UPDATE plugin_idc_core_commission_order_receipt r
 SET invoice_no=s.invoice_no
 FROM plugin_idc_core_financial_settlement s
 WHERE r.invoice_no IS NULL AND r.order_no=s.order_no;
ALTER TABLE plugin_idc_core_commission_order_receipt ALTER COLUMN invoice_no SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_receipt_invoice ON plugin_idc_core_commission_order_receipt(invoice_no);
DROP TRIGGER IF EXISTS trg_idc_core_commission_receipt_immutable ON plugin_idc_core_commission_order_receipt;
CREATE TRIGGER trg_idc_core_commission_receipt_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_commission_order_receipt
 FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();

CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_offline_payout (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    payout_no VARCHAR(40) NOT NULL UNIQUE,
    account_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_commission_account(account_no),
    currency_code CHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    evidence_reference VARCHAR(512) NOT NULL CHECK (length(evidence_reference) BETWEEN 6 AND 512),
    paid_at TIMESTAMPTZ NOT NULL,
    recorded_by INT NOT NULL CHECK (recorded_by > 0),
    request_digest CHAR(64) NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_offline_payout_account
    ON plugin_idc_core_sales_commission_offline_payout(account_no, id DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_offline_payout_list
    ON plugin_idc_core_sales_commission_offline_payout(tenant_id, id DESC);
DROP TRIGGER IF EXISTS trg_idc_core_offline_payout_immutable ON plugin_idc_core_sales_commission_offline_payout;
CREATE TRIGGER trg_idc_core_offline_payout_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_commission_offline_payout
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
COMMENT ON TABLE plugin_idc_core_commission_order_receipt IS 'Immutable receipt for payment-time commission attribution';
COMMENT ON TABLE plugin_idc_core_sales_commission_offline_payout IS 'Append-only administrator evidence of an offline commission payment';

-- Purpose: Attribute each offline payout to exact matured accruals for refunds.
-- 用途：逐笔关联已支付佣金，保证退款能追踪待扣回来源。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_offline_payout_item (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    payout_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_commission_offline_payout(payout_no),
    commission_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_commission_accrual(commission_no),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_core_offline_payout_item UNIQUE(payout_no, commission_no)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_offline_payout_commission
    ON plugin_idc_core_sales_commission_offline_payout_item(commission_no);
DROP TRIGGER IF EXISTS trg_idc_core_offline_payout_item_immutable ON plugin_idc_core_sales_commission_offline_payout_item;
CREATE TRIGGER trg_idc_core_offline_payout_item_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_commission_offline_payout_item
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_renewal_poll
 ON plugin_idc_core_billing_invoice(settled_at,id)
 WHERE tenant_id=0 AND source_code='renewal' AND settled_at IS NOT NULL AND paid_amount_minor>0;

-- Paid commission remains historical cash paid; recoverable debt has a separate append-only ledger.
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_recovery (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
 recovery_no VARCHAR(40) NOT NULL UNIQUE,
 account_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_commission_account(account_no),
 commission_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_commission_accrual(commission_no),
 refund_no VARCHAR(40) NOT NULL,
 reversal_no VARCHAR(40) NOT NULL UNIQUE REFERENCES plugin_idc_core_commission_refund_reversal(reversal_no),
 amount_minor BIGINT NOT NULL CHECK(amount_minor>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_recovery_account ON plugin_idc_core_sales_commission_recovery(account_no,id);
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_recovery_offset (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
 offset_no VARCHAR(40) NOT NULL UNIQUE,
 recovery_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_commission_recovery(recovery_no),
 commission_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_commission_accrual(commission_no),
 account_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_commission_account(account_no),
 amount_minor BIGINT NOT NULL CHECK(amount_minor>0),
 entry_no VARCHAR(40) NOT NULL UNIQUE REFERENCES plugin_idc_core_sales_commission_account_entry(entry_no),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_offset_recovery ON plugin_idc_core_sales_commission_recovery_offset(recovery_no,id);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_offset_account ON plugin_idc_core_sales_commission_recovery_offset(account_no,id);
DROP TRIGGER IF EXISTS trg_idc_core_commission_recovery_immutable ON plugin_idc_core_sales_commission_recovery;
CREATE TRIGGER trg_idc_core_commission_recovery_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_commission_recovery FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
DROP TRIGGER IF EXISTS trg_idc_core_commission_offset_immutable ON plugin_idc_core_sales_commission_recovery_offset;
CREATE TRIGGER trg_idc_core_commission_offset_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_commission_recovery_offset FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
