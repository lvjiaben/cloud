-- Merchant customer-service directories are navigation, independent from sales
-- categories and employee server classifications. Disable retains every fact;
-- no directory, membership or preference has a deletion lifecycle.
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_directory (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 directory_no VARCHAR(40) NOT NULL UNIQUE CHECK (directory_no<>'all'),
 name_zh_cn VARCHAR(128) NOT NULL CHECK (length(btrim(name_zh_cn))>0),
 name_en_us VARCHAR(128) NOT NULL CHECK (length(btrim(name_en_us))>0),
 sort_order INT NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 1000000),
 status_code VARCHAR(16) NOT NULL DEFAULT 'enabled' CHECK (status_code IN ('enabled','disabled')),
 version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
 created_by INT NOT NULL DEFAULT 0,
 updated_by INT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_idc_core_product_directory_navigation
 ON plugin_idc_core_product_directory(tenant_id,status_code,sort_order,id);
COMMENT ON TABLE plugin_idc_core_product_directory IS 'Merchant-defined customer service navigation; disable preserves service access';
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_directory_membership (
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 product_no VARCHAR(40) PRIMARY KEY REFERENCES plugin_idc_core_catalog_product(product_no) ON DELETE RESTRICT,
 directory_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_directory(directory_no) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_idc_core_product_directory_membership_directory
 ON plugin_idc_core_product_directory_membership(tenant_id,directory_no,product_no);
COMMENT ON TABLE plugin_idc_core_product_directory_membership IS 'One merchant directory per product; does not change sales category or service facts';
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_directory_visibility (
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 customer_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_customer_subject(customer_no) ON DELETE RESTRICT,
 directory_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_directory(directory_no) ON DELETE RESTRICT,
 hidden BOOLEAN NOT NULL DEFAULT FALSE,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(customer_no,directory_no)
);
COMMENT ON TABLE plugin_idc_core_product_directory_visibility IS 'Current customer navigation preference; absent preference means visible';
CREATE INDEX IF NOT EXISTS idx_idc_core_entitlement_customer_product_directory
 ON plugin_idc_core_service_entitlement(tenant_id,customer_no,product_no,id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_core_entitlement_customer_navigation
 ON plugin_idc_core_service_entitlement(tenant_id,customer_no,id DESC);
