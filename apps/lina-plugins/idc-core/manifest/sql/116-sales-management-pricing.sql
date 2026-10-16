-- Purpose: Stores immutable customer-specific and agent-tier prices per product,
-- currency and billing period. A later version replaces new quotes only.
-- 用途：保存客户专属价、代理等级价的逐商品、币种和计费周期版本。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_purchase_price (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 price_no VARCHAR(40) NOT NULL,
 customer_no VARCHAR(40),
 level_no VARCHAR(40),
 product_no VARCHAR(40) NOT NULL,
 currency_code CHAR(3) NOT NULL,
 charge_type_code VARCHAR(24) NOT NULL,
 period_unit_code VARCHAR(16) NOT NULL DEFAULT '',
 period_count INT NOT NULL DEFAULT 0,
 adjustment_type_code VARCHAR(24) NOT NULL,
 fixed_price_minor BIGINT,
 discount_basis_points INT,
 status_code VARCHAR(16) NOT NULL DEFAULT 'active',
 version BIGINT NOT NULL,
 content_digest CHAR(64) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT ck_idc_core_sales_purchase_price CHECK (
  tenant_id = 0 AND version > 0 AND currency_code ~ '^[A-Z]{3}$'
  AND ((customer_no IS NOT NULL AND level_no IS NULL) OR (customer_no IS NULL AND level_no IS NOT NULL))
  AND status_code IN ('active','inactive')
  AND charge_type_code IN ('one_time','recurring','trial','usage','tiered')
  AND period_count >= 0
  AND content_digest ~ '^[0-9a-f]{64}$'
  AND ((adjustment_type_code='fixed_price' AND fixed_price_minor >= 0 AND discount_basis_points IS NULL)
    OR (adjustment_type_code='discount_basis_points' AND fixed_price_minor IS NULL AND discount_basis_points BETWEEN 0 AND 10000))
 ),
 CONSTRAINT uk_idc_core_sales_purchase_price_version UNIQUE(price_no,version),
 CONSTRAINT fk_idc_core_sales_purchase_price_customer FOREIGN KEY(customer_no) REFERENCES plugin_idc_core_customer_subject(customer_no),
 CONSTRAINT fk_idc_core_sales_purchase_price_level FOREIGN KEY(level_no) REFERENCES plugin_idc_core_sales_agent_level(level_no),
 CONSTRAINT fk_idc_core_sales_purchase_price_product FOREIGN KEY(product_no) REFERENCES plugin_idc_core_catalog_product(product_no)
);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_purchase_price_customer ON plugin_idc_core_sales_purchase_price(tenant_id,customer_no,product_no,currency_code,version DESC);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_purchase_price_level ON plugin_idc_core_sales_purchase_price(tenant_id,level_no,product_no,currency_code,version DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_core_sales_purchase_price_customer_scope ON plugin_idc_core_sales_purchase_price(customer_no,product_no,currency_code,charge_type_code,period_unit_code,period_count,version) WHERE customer_no IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_core_sales_purchase_price_level_scope ON plugin_idc_core_sales_purchase_price(level_no,product_no,currency_code,charge_type_code,period_unit_code,period_count,version) WHERE level_no IS NOT NULL;
DROP TRIGGER IF EXISTS trg_idc_core_sales_purchase_price_immutable ON plugin_idc_core_sales_purchase_price;
CREATE TRIGGER trg_idc_core_sales_purchase_price_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_purchase_price FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();

-- Purpose: Keeps administrator-assigned referral ownership without fabricating
-- referral clicks or links. A null referrer explicitly clears manual ownership.
-- 用途：保存后台明确指定的推广归属及历史，禁止伪造推荐链接点击。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_customer_referrer (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 customer_no VARCHAR(40) NOT NULL,
 referrer_customer_no VARCHAR(40),
 version BIGINT NOT NULL,
 effective_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 created_by INT NOT NULL,
 CONSTRAINT ck_idc_core_sales_customer_referrer CHECK(tenant_id=0 AND version>0 AND created_by>0 AND (referrer_customer_no IS NULL OR referrer_customer_no<>customer_no)),
 CONSTRAINT uk_idc_core_sales_customer_referrer_version UNIQUE(customer_no,version),
 CONSTRAINT fk_idc_core_sales_customer_referrer_customer FOREIGN KEY(customer_no) REFERENCES plugin_idc_core_customer_subject(customer_no),
 CONSTRAINT fk_idc_core_sales_customer_referrer_owner FOREIGN KEY(referrer_customer_no) REFERENCES plugin_idc_core_customer_subject(customer_no)
);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_customer_referrer_history ON plugin_idc_core_sales_customer_referrer(tenant_id,customer_no,effective_at DESC,id DESC);
DROP TRIGGER IF EXISTS trg_idc_core_sales_customer_referrer_immutable ON plugin_idc_core_sales_customer_referrer;
CREATE TRIGGER trg_idc_core_sales_customer_referrer_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_customer_referrer FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();

-- A matching price can equal the public price and must still freeze its source
-- and prevent marketing stacking. Other adjustment kinds remain strictly positive.
ALTER TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
 DROP CONSTRAINT IF EXISTS ck_idc_com_quote_adjustment_amount;
ALTER TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
 ADD CONSTRAINT ck_idc_com_quote_adjustment_amount CHECK (
 application_order BETWEEN 1 AND 4096 AND
 (amount_minor > 0 OR (adjustment_type_code='agent_price' AND amount_minor=0))
 AND content_digest ~ '^[0-9a-f]{64}$');

-- Original catalog and negotiated periodic prices are retained independently
-- from discount amounts, including negotiated prices above the public price.
ALTER TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
 ADD COLUMN IF NOT EXISTS original_period_amount_minor BIGINT,
 ADD COLUMN IF NOT EXISTS target_period_amount_minor BIGINT;
ALTER TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
 DROP CONSTRAINT IF EXISTS ck_idc_com_agent_price_source;
ALTER TABLE plugin_idc_core_commerce_quote_adjustment_snapshot
 ADD CONSTRAINT ck_idc_com_agent_price_source CHECK (
 (original_period_amount_minor IS NULL AND target_period_amount_minor IS NULL) OR
 (adjustment_type_code='agent_price' AND amount_minor=0 AND original_period_amount_minor>=0 AND target_period_amount_minor>=0
 AND original_period_amount_minor IS NOT NULL AND target_period_amount_minor IS NOT NULL));
