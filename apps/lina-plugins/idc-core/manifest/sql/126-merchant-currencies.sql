-- Merchant-wide currency admission preserves money history when configuration is removed.
CREATE TABLE IF NOT EXISTS plugin_idc_core_merchant_currency (
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 currency_code CHAR(3) NOT NULL CHECK (currency_code ~ '^[A-Z]{3}$'),
 enabled BOOLEAN NOT NULL DEFAULT TRUE,
 version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
 created_by INT NOT NULL DEFAULT 0,
 updated_by INT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY (tenant_id,currency_code)
);
COMMENT ON TABLE plugin_idc_core_merchant_currency IS 'Merchant currency admission for new quotes; historical money remains unchanged';
COMMENT ON COLUMN plugin_idc_core_merchant_currency.currency_code IS 'ISO 4217 currency code';
COMMENT ON COLUMN plugin_idc_core_merchant_currency.enabled IS 'Whether new quotes may use this currency';
COMMENT ON COLUMN plugin_idc_core_merchant_currency.version IS 'Optimistic concurrency version';
INSERT INTO plugin_idc_core_merchant_currency (tenant_id,currency_code)
SELECT 0,currency_code FROM (
 SELECT default_currency AS currency_code FROM plugin_idc_core_merchant_profile WHERE tenant_id=0
 UNION SELECT currency_code FROM plugin_idc_core_catalog_price WHERE tenant_id=0
 UNION SELECT currency_code FROM plugin_idc_core_billing_invoice WHERE tenant_id=0
 UNION SELECT currency_code FROM plugin_idc_core_billing_ledger_account WHERE tenant_id=0
 UNION SELECT 'CNY'
) currencies
ON CONFLICT (tenant_id,currency_code) DO NOTHING;

ALTER TABLE plugin_idc_core_merchant_currency ADD COLUMN IF NOT EXISTS deleted BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN plugin_idc_core_merchant_currency.deleted IS 'Whether the configuration is removed; retain version and audit continuity';
