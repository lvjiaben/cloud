-- Schema-only, repeatable indexes for bounded all-currency credit pages.
-- Existing currency-prefixed indexes continue serving currency-specific queries.
CREATE INDEX IF NOT EXISTS idx_idc_credit_application_page
    ON plugin_idc_core_credit_application (tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_credit_application_customer_page
    ON plugin_idc_core_credit_application (tenant_id, customer_no, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_credit_account_page
    ON plugin_idc_core_credit_account (tenant_id, updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_credit_account_customer_page
    ON plugin_idc_core_credit_account (tenant_id, customer_no, updated_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_credit_statement_page
    ON plugin_idc_core_credit_statement (tenant_id, issued_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_credit_statement_customer_page
    ON plugin_idc_core_credit_statement (tenant_id, customer_no, issued_at DESC, id DESC);
