-- 120: Freeze same-system sale contracts and whole-item price replacements.
-- Existing provider release and interface revision admission remain mandatory.

-- Keep the previous bounded legacy validator; accept the portable protocol only
-- through an additional, explicitly typed field rather than arbitrary metadata.
DO $body$
BEGIN
    IF to_regprocedure('plugin_idc_core_import_legacy_source_valid(jsonb)') IS NULL THEN
        ALTER FUNCTION plugin_idc_core_import_source_valid(JSONB) RENAME TO plugin_idc_core_import_legacy_source_valid;
    END IF;
END;
$body$;

CREATE OR REPLACE FUNCTION plugin_idc_core_import_source_valid(snapshot JSONB)
RETURNS BOOLEAN LANGUAGE plpgsql IMMUTABLE AS $$
DECLARE portable JSONB; legacy JSONB;
BEGIN
    IF snapshot IS NULL OR jsonb_typeof(snapshot) <> 'object' OR octet_length(snapshot::TEXT)>1048576 THEN RETURN FALSE; END IF;
    IF NOT jsonb_exists(snapshot,'importSnapshot') THEN RETURN plugin_idc_core_import_legacy_source_valid(snapshot); END IF;
    portable := snapshot->'importSnapshot';
    IF jsonb_typeof(portable) IS DISTINCT FROM 'object'
       OR portable->>'protocolVersion' IS DISTINCT FROM '1'
       OR jsonb_typeof(portable->'pricingComplete') IS DISTINCT FROM 'boolean'
       OR (portable->>'pricingComplete')::BOOLEAN IS NOT TRUE
       OR jsonb_typeof(portable->'prices') IS DISTINCT FROM 'array'
       OR jsonb_array_length(portable->'prices') NOT BETWEEN 1 AND 4096
       OR jsonb_typeof(portable->'mappings') IS DISTINCT FROM 'array'
       OR jsonb_array_length(portable->'mappings')>128
       OR jsonb_typeof(portable->'currencies') IS DISTINCT FROM 'array'
       OR jsonb_array_length(portable->'currencies') NOT BETWEEN 1 AND 64
       OR NOT plugin_idc_core_import_text(portable->'configurationRevision',4096)
    THEN RETURN FALSE; END IF;
    legacy := jsonb_set(snapshot - ARRAY['importSnapshot','peerPurchase'],'{priceScheduleComplete}','false'::JSONB);
    RETURN plugin_idc_core_import_legacy_source_valid(legacy);
END;
$$;

ALTER TABLE plugin_idc_core_catalog_product_import DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_catalog_import_source;
ALTER TABLE plugin_idc_core_catalog_product_import ADD CONSTRAINT ck_plugin_idc_core_catalog_import_source CHECK (
    plugin_idc_core_import_source_valid(source_snapshot)
    AND COALESCE(source_snapshot->'product'->>'reference', source_snapshot->'importSnapshot'->'product'->>'publicSlug')=upstream_reference
    AND plugin_idc_core_import_missing_valid(missing_configuration_snapshot)
);

-- Purpose: immutable version-owned portable catalog and trusted read evidence.
-- 用途：冻结按商品版本绑定的完整同系统销售契约和可信接口读取证据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_import_contract (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    version_no VARCHAR(40) NOT NULL UNIQUE REFERENCES plugin_idc_core_catalog_product_version(version_no) ON DELETE RESTRICT,
    interface_instance_no VARCHAR(40) NOT NULL,
    snapshot JSONB NOT NULL CHECK(jsonb_typeof(snapshot)='object' AND octet_length(snapshot::TEXT)<=1048576),
    snapshot_digest CHAR(64) NOT NULL CHECK(snapshot_digest ~ '^[0-9a-f]{64}$'),
    read_evidence JSONB NOT NULL CHECK(jsonb_typeof(read_evidence)='object' AND octet_length(read_evidence::TEXT)<=16384),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id,interface_instance_no) REFERENCES plugin_idc_core_product_automation_interface_instance(tenant_id,interface_instance_no) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_idc_import_contract_interface ON plugin_idc_core_catalog_import_contract(tenant_id,interface_instance_no,version_no);

-- Purpose: immutable dynamic candidate purchase-token evidence per imported version.
-- 用途：保存动态配置候选的上游购买令牌，避免按不透明选项值猜测令牌。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_import_candidate (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
    version_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_catalog_import_contract(version_no) ON DELETE RESTRICT,
    field_key VARCHAR(128) NOT NULL,
    candidate_value VARCHAR(256) NOT NULL,
    option_token VARCHAR(4096) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(version_no,field_key,candidate_value),
    CHECK(length(field_key)>0 AND length(candidate_value)>0 AND length(option_token)>0)
);

-- Purpose: frozen local billing-rule mapping and whole-item pricing semantics.
-- 用途：保存导入计费周期的上游引用和本地整件商品价格规则，配置差价与安装费不被错误摊分。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_import_term (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK(tenant_id=0),
    version_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_catalog_product_version(version_no) ON DELETE RESTRICT,
    billing_rule_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_catalog_billing_rule(billing_rule_no) ON DELETE RESTRICT,
    currency_code VARCHAR(3) NOT NULL CHECK(currency_code ~ '^[A-Z]{3}$'),
    upstream_term_reference VARCHAR(256) NOT NULL CHECK(length(upstream_term_reference) BETWEEN 1 AND 256),
    whole_item_rule JSONB,
    markup_percent VARCHAR(128) NOT NULL CHECK(markup_percent ~ '^[0-9]+$' OR markup_percent ~ '^[0-9]+\.[0-9]+$'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(version_no,billing_rule_no,currency_code),
    CHECK(whole_item_rule IS NULL OR (jsonb_typeof(whole_item_rule)='object' AND
        ((whole_item_rule->>'kind'='fixed' AND jsonb_typeof(whole_item_rule->'fixedAmountMinor')='number' AND (whole_item_rule->>'fixedAmountMinor')::NUMERIC>=0)
         OR (whole_item_rule->>'kind'='discount' AND jsonb_typeof(whole_item_rule->'discountBasisPoints')='number' AND (whole_item_rule->>'discountBasisPoints')::INT BETWEEN 0 AND 10000))))
);

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_portable_import()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP <> 'INSERT' THEN RAISE EXCEPTION 'portable import evidence is immutable'; END IF;
    RETURN NEW;
END;
$$;
CREATE OR REPLACE TRIGGER trg_idc_import_contract_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_import_contract
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_portable_import();
CREATE OR REPLACE TRIGGER trg_idc_import_term_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_import_term
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_portable_import();
CREATE OR REPLACE TRIGGER trg_idc_import_candidate_immutable BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_import_candidate
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_portable_import();
