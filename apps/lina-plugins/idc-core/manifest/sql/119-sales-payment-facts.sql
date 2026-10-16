-- Purpose: Freeze payment-time sales ownership independently of commission rules.
-- 用途：独立保存到账与归属事实；零元结清保留笔数，历史已处理记录不猜测回填。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_payment_fact (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    invoice_no VARCHAR(40) NOT NULL UNIQUE REFERENCES plugin_idc_core_billing_invoice(invoice_no),
    order_no VARCHAR(40) UNIQUE REFERENCES plugin_idc_core_commerce_order(order_no),
    customer_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_customer_subject(customer_no),
    currency_code CHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
    operation_type_code VARCHAR(20) NOT NULL CHECK (operation_type_code IN ('new_purchase','renewal')),
    received_amount_minor BIGINT NOT NULL CHECK (received_amount_minor>=0),
    received_at TIMESTAMPTZ NOT NULL,
    settlement_no VARCHAR(40) NOT NULL,
    source_digest CHAR(64) NOT NULL CHECK (source_digest ~ '^[0-9a-f]{64}$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_core_sales_payment_order CHECK (
        (operation_type_code='new_purchase' AND order_no IS NOT NULL) OR
        (operation_type_code='renewal' AND order_no IS NULL)
    )
);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_payment_received
    ON plugin_idc_core_sales_payment_fact(tenant_id,received_at,invoice_no);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_payment_customer
    ON plugin_idc_core_sales_payment_fact(tenant_id,customer_no,received_at);

CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_payment_owner (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    invoice_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_sales_payment_fact(invoice_no),
    beneficiary_type_code VARCHAR(20) NOT NULL CHECK (beneficiary_type_code IN ('sales','affiliate')),
    beneficiary_reference VARCHAR(40) NOT NULL CHECK (length(beneficiary_reference)>0),
    sales_assignment_no VARCHAR(40) REFERENCES plugin_idc_core_sales_customer_assignment(assignment_no),
    affiliate_attribution_no VARCHAR(40) REFERENCES plugin_idc_core_sales_affiliate_attribution(attribution_no),
    manual_referrer_id BIGINT REFERENCES plugin_idc_core_sales_customer_referrer(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_core_sales_payment_owner UNIQUE(invoice_no,beneficiary_type_code),
    CONSTRAINT ck_idc_core_sales_payment_owner_source CHECK (
        (beneficiary_type_code='sales' AND sales_assignment_no IS NOT NULL AND affiliate_attribution_no IS NULL AND manual_referrer_id IS NULL) OR
        (beneficiary_type_code='affiliate' AND sales_assignment_no IS NULL AND num_nonnulls(affiliate_attribution_no,manual_referrer_id)=1)
    )
);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_payment_beneficiary
    ON plugin_idc_core_sales_payment_owner(tenant_id,beneficiary_type_code,beneficiary_reference,invoice_no);
DROP TRIGGER IF EXISTS trg_idc_core_sales_payment_fact_immutable ON plugin_idc_core_sales_payment_fact;
CREATE TRIGGER trg_idc_core_sales_payment_fact_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_payment_fact
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
DROP TRIGGER IF EXISTS trg_idc_core_sales_payment_owner_immutable ON plugin_idc_core_sales_payment_owner;
CREATE TRIGGER trg_idc_core_sales_payment_owner_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_payment_owner
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
COMMENT ON TABLE plugin_idc_core_sales_payment_fact IS 'Immutable actual payment or lawful zero-amount settlement, independent of commission rules';
COMMENT ON TABLE plugin_idc_core_sales_payment_owner IS 'Immutable payment-time sales and referral ownership shared by performance and commission';
