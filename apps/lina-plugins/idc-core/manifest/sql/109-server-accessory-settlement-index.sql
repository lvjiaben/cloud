-- Purpose: Keep strict persisted-addon classification bounded by the original order.
-- This non-unique index changes neither ownership nor settlement eligibility.
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_addon_order
    ON plugin_idc_core_business_operation (tenant_id, order_no)
    WHERE operation_type_code = 'addon' AND order_no IS NOT NULL;

-- Fixed traffic packages reuse immutable accessory price versions and the
-- original-server checkout. Existing per-GB policies remain historical only.
ALTER TABLE plugin_idc_core_server_accessory_price_policy
    ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS stock_quantity BIGINT;
ALTER TABLE plugin_idc_core_server_accessory_price_policy
    DROP CONSTRAINT IF EXISTS uk_idc_server_accessory_policy_kind_currency;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_accessory_policy_kind_currency
    ON plugin_idc_core_server_accessory_price_policy (accessory_kind, currency_code)
    WHERE accessory_kind <> 'traffic';
ALTER TABLE plugin_idc_core_server_accessory_price_policy
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_traffic_package_stock;
ALTER TABLE plugin_idc_core_server_accessory_price_policy
    ADD CONSTRAINT ck_plugin_idc_core_traffic_package_stock CHECK (
        stock_quantity IS NULL OR stock_quantity BETWEEN 0 AND 1000000000
    );

ALTER TABLE plugin_idc_core_server_accessory_price_policy_version
    ADD COLUMN IF NOT EXISTS traffic_gb INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS name_zh_cn VARCHAR(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS name_en_us VARCHAR(120) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_server_accessory_price_policy_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_traffic_package_capacity;
ALTER TABLE plugin_idc_core_server_accessory_price_policy_version
    ADD CONSTRAINT ck_plugin_idc_core_traffic_package_capacity CHECK (
        traffic_gb BETWEEN 0 AND 1000000
        AND (traffic_gb = 0 OR (accessory_kind = 'traffic'
            AND length(btrim(name_zh_cn)) > 0 AND length(btrim(name_en_us)) > 0))
    );

-- Purpose: Frozen merchant-selected products eligible for one package version.
-- 用途：保留各流量套餐版本适用商品，不删除历史适用关系。
CREATE TABLE IF NOT EXISTS plugin_idc_core_traffic_package_product (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    policy_no VARCHAR(40) NOT NULL,
    policy_version BIGINT NOT NULL CHECK (policy_version > 0),
    product_no VARCHAR(40) NOT NULL,
    CONSTRAINT uk_plugin_idc_core_traffic_package_product
        UNIQUE (policy_no, policy_version, product_no),
    CONSTRAINT fk_plugin_idc_core_traffic_package_version
        FOREIGN KEY (policy_no, policy_version)
        REFERENCES plugin_idc_core_server_accessory_price_policy_version (policy_no, policy_version),
    CONSTRAINT fk_plugin_idc_core_traffic_package_product
        FOREIGN KEY (product_no) REFERENCES plugin_idc_core_catalog_product (product_no)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_traffic_package_product
    ON plugin_idc_core_traffic_package_product (tenant_id, product_no, policy_no, policy_version);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_accessory_quote_policy
    ON plugin_idc_core_server_accessory_quote (tenant_id, policy_no, quote_no);

-- Product eligibility is part of the same immutable merchant version as price
-- and capacity. Reuse the existing history guard instead of permitting rewrites.
DROP TRIGGER IF EXISTS trg_plugin_idc_core_traffic_package_product_immutable
    ON plugin_idc_core_traffic_package_product;
CREATE TRIGGER trg_plugin_idc_core_traffic_package_product_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_traffic_package_product
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_accessory_pricing_history_mutation();
