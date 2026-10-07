-- 033: Bounded customer and staff financial-directory access paths.

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_account_recent
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_account_status_recent
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "customer_no", "status_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_account_currency_recent
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_admin_recent
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_admin_status_recent
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_transaction_admin_recent
    ON plugin_idc_core_billing_ledger_transaction (
        "tenant_id", "posted_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_transaction_admin_type_recent
    ON plugin_idc_core_billing_ledger_transaction (
        "tenant_id", "transaction_type_code", "posted_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_transaction_admin_currency_recent
    ON plugin_idc_core_billing_ledger_transaction (
        "tenant_id", "currency_code", "posted_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjustment_admin_recent
    ON plugin_idc_core_billing_adjustment_request (
        "tenant_id", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjustment_admin_status_recent
    ON plugin_idc_core_billing_adjustment_request (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjustment_admin_customer_recent
    ON plugin_idc_core_billing_adjustment_request (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_account_renewal_recent
    ON plugin_idc_core_service_instance (
        "tenant_id", "customer_no", "updated_at" DESC, "id" DESC
    ) WHERE "period_unit_code" <> 'once' AND "entitlement_expires_at" IS NOT NULL;
