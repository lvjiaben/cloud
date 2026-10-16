-- Purpose: Preserve immutable agent-tier-wide price percentages independently of tier administration.
-- 用途：独立保存代理等级总价格百分比版本，清除规则不删除历史。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_level_price (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0,
 price_no VARCHAR(40) NOT NULL,
 level_no VARCHAR(40) NOT NULL,
 percentage_basis_points INT NOT NULL,
 status_code VARCHAR(16) NOT NULL,
 version BIGINT NOT NULL,
 content_digest CHAR(64) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT ck_idc_core_sales_level_price CHECK(tenant_id=0 AND percentage_basis_points BETWEEN 0 AND 10000 AND status_code IN ('active','cleared') AND version>0 AND content_digest ~ '^[0-9a-f]{64}$'),
 CONSTRAINT uk_idc_core_sales_level_price_version UNIQUE(level_no,version),
 CONSTRAINT uk_idc_core_sales_level_price_number UNIQUE(price_no,version),
 CONSTRAINT fk_idc_core_sales_level_price_level FOREIGN KEY(level_no) REFERENCES plugin_idc_core_sales_agent_level(level_no)
);
CREATE INDEX IF NOT EXISTS idx_idc_core_sales_level_price_latest ON plugin_idc_core_sales_level_price(tenant_id,level_no,version DESC);
DROP TRIGGER IF EXISTS trg_idc_core_sales_level_price_immutable ON plugin_idc_core_sales_level_price;
CREATE TRIGGER trg_idc_core_sales_level_price_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_sales_level_price FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation();
