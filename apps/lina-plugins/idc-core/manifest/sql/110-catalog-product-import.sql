-- 110: Preserve bounded, typed upstream sales evidence and idempotent draft
-- imports. Evidence has no deletion lifecycle: retries recover the original
-- draft, and neither historical source facts nor import ownership may change.

CREATE OR REPLACE FUNCTION plugin_idc_core_import_text(value JSONB, maximum_length INT, allow_empty BOOLEAN DEFAULT FALSE)
RETURNS BOOLEAN AS $$
    SELECT COALESCE(jsonb_typeof(value) = 'string'
       AND length(value #>> '{}') BETWEEN CASE WHEN allow_empty THEN 0 ELSE 1 END AND maximum_length
       AND (value #>> '{}') = btrim(value #>> '{}')
       AND (value #>> '{}') !~ '[[:cntrl:]]', FALSE);
$$ LANGUAGE sql IMMUTABLE;

CREATE OR REPLACE FUNCTION plugin_idc_core_import_prices_valid(prices JSONB)
RETURNS BOOLEAN AS $$
DECLARE item JSONB;
BEGIN
    IF jsonb_typeof(prices) IS DISTINCT FROM 'array' OR jsonb_array_length(prices) > 120 THEN
        RETURN FALSE;
    END IF;
    FOR item IN SELECT value FROM jsonb_array_elements(prices) LOOP
        IF jsonb_typeof(item) IS DISTINCT FROM 'object'
           OR NOT jsonb_exists_all(item,ARRAY['currencyCode','period','periodCount','amount','setupFee'])
           OR item - ARRAY['currencyCode','period','periodCount','amount','setupFee'] <> '{}'::jsonb
           OR NOT plugin_idc_core_import_text(item->'currencyCode',3)
           OR item->>'currencyCode' !~ '^[A-Z]{3}$'
           OR jsonb_typeof(item->'period') IS DISTINCT FROM 'string'
           OR item->>'period' NOT IN ('once','hour','day','month','year')
           OR jsonb_typeof(item->'periodCount') IS DISTINCT FROM 'number'
           OR (item->>'periodCount') !~ '^[0-9]{1,3}$'
           OR (item->>'periodCount')::INT NOT BETWEEN 1 AND 120
           OR (item->>'period' = 'once' AND (item->>'periodCount')::INT <> 1)
           OR NOT plugin_idc_core_import_text(item->'amount',37)
           OR item->>'amount' !~ '^[0-9]{1,24}(\.[0-9]{1,12}){0,1}$'
           OR NOT plugin_idc_core_import_text(item->'setupFee',37)
           OR item->>'setupFee' !~ '^[0-9]{1,24}(\.[0-9]{1,12}){0,1}$' THEN
            RETURN FALSE;
        END IF;
    END LOOP;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION plugin_idc_core_import_source_valid(snapshot JSONB)
RETURNS BOOLEAN AS $$
DECLARE
    product JSONB;
    option_row JSONB;
    choice JSONB;
    configuration JSONB;
    choice_count INT := 0;
    value_key TEXT;
BEGIN
    IF snapshot IS NULL OR octet_length(snapshot::TEXT) > 1048576
       OR jsonb_typeof(snapshot) IS DISTINCT FROM 'object'
       OR NOT jsonb_exists_all(snapshot,ARRAY['product','basePrices','options','priceScheduleComplete','productConfiguration'])
       OR snapshot - ARRAY['product','basePrices','options','priceScheduleComplete','productConfiguration'] <> '{}'::jsonb
       OR jsonb_typeof(snapshot->'priceScheduleComplete') IS DISTINCT FROM 'boolean'
       OR NOT plugin_idc_core_import_prices_valid(snapshot->'basePrices')
       OR jsonb_typeof(snapshot->'options') IS DISTINCT FROM 'array'
       OR jsonb_array_length(snapshot->'options') > 128
       OR jsonb_typeof(snapshot->'productConfiguration') IS DISTINCT FROM 'array'
       OR jsonb_array_length(snapshot->'productConfiguration') > 128 THEN
        RETURN FALSE;
    END IF;
    product := snapshot->'product';
    IF (snapshot->>'priceScheduleComplete')::BOOLEAN AND jsonb_array_length(snapshot->'basePrices') = 0 THEN RETURN FALSE; END IF;
    IF jsonb_typeof(product) IS DISTINCT FROM 'object'
       OR NOT jsonb_exists_all(product,ARRAY['reference','name','description'])
       OR product - ARRAY['reference','name','description'] <> '{}'::jsonb
       OR NOT plugin_idc_core_import_text(product->'reference',256)
       OR NOT plugin_idc_core_import_text(product->'name',256)
       OR NOT plugin_idc_core_import_text(product->'description',1024,TRUE) THEN
        RETURN FALSE;
    END IF;
    FOR option_row IN SELECT value FROM jsonb_array_elements(snapshot->'options') LOOP
        IF jsonb_typeof(option_row) IS DISTINCT FROM 'object'
           OR NOT jsonb_exists_all(option_row,ARRAY['reference','name','choices'])
           OR option_row - ARRAY['reference','name','choices'] <> '{}'::jsonb
           OR NOT plugin_idc_core_import_text(option_row->'reference',256)
           OR NOT plugin_idc_core_import_text(option_row->'name',256)
           OR jsonb_typeof(option_row->'choices') IS DISTINCT FROM 'array'
           OR jsonb_array_length(option_row->'choices') > 512 THEN
            RETURN FALSE;
        END IF;
        choice_count := choice_count + jsonb_array_length(option_row->'choices');
        IF choice_count > 2048 THEN RETURN FALSE; END IF;
        FOR choice IN SELECT value FROM jsonb_array_elements(option_row->'choices') LOOP
            IF jsonb_typeof(choice) IS DISTINCT FROM 'object'
               OR NOT jsonb_exists_all(choice,ARRAY['reference','name','prices'])
               OR choice - ARRAY['reference','name','prices'] <> '{}'::jsonb
               OR NOT plugin_idc_core_import_text(choice->'reference',256)
               OR NOT plugin_idc_core_import_text(choice->'name',256)
               OR NOT plugin_idc_core_import_prices_valid(choice->'prices') THEN
                RETURN FALSE;
            END IF;
        END LOOP;
    END LOOP;
    FOR configuration IN SELECT value FROM jsonb_array_elements(snapshot->'productConfiguration') LOOP
        IF jsonb_typeof(configuration) IS DISTINCT FROM 'object'
           OR NOT plugin_idc_core_import_text(configuration->'fieldKey',64)
           OR configuration - ARRAY['fieldKey','textValue','integerValue','decimalValue','booleanValue','optionValue'] <> '{}'::jsonb
           OR (SELECT count(*) FROM jsonb_object_keys(configuration)) <> 2 THEN
            RETURN FALSE;
        END IF;
        SELECT key INTO value_key FROM jsonb_object_keys(configuration) key WHERE key <> 'fieldKey';
        IF (value_key = 'textValue' AND NOT plugin_idc_core_import_text(configuration->value_key,8192,TRUE))
           OR (value_key = 'optionValue' AND NOT plugin_idc_core_import_text(configuration->value_key,256))
           OR (value_key = 'decimalValue' AND (NOT plugin_idc_core_import_text(configuration->value_key,64)
               OR configuration->>value_key !~ '^[0-9]{1,24}(\.[0-9]{1,12}){0,1}$'))
           OR (value_key = 'integerValue' AND (jsonb_typeof(configuration->value_key) IS DISTINCT FROM 'number'
               OR configuration->>value_key !~ '^-{0,1}[0-9]{1,19}$'))
           OR (value_key = 'booleanValue' AND jsonb_typeof(configuration->value_key) IS DISTINCT FROM 'boolean') THEN
            RETURN FALSE;
        END IF;
    END LOOP;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE OR REPLACE FUNCTION plugin_idc_core_import_missing_valid(snapshot JSONB)
RETURNS BOOLEAN AS $$
DECLARE item JSONB;
BEGIN
    IF jsonb_typeof(snapshot) IS DISTINCT FROM 'array' OR jsonb_array_length(snapshot) > 256 THEN RETURN FALSE; END IF;
    FOR item IN SELECT value FROM jsonb_array_elements(snapshot) LOOP
        IF jsonb_typeof(item) IS DISTINCT FROM 'object'
           OR NOT jsonb_exists_all(item,ARRAY['fieldKey','nameZhCn','nameEnUs'])
           OR item - ARRAY['fieldKey','nameZhCn','nameEnUs'] <> '{}'::jsonb
           OR NOT plugin_idc_core_import_text(item->'fieldKey',128,TRUE)
           OR NOT plugin_idc_core_import_text(item->'nameZhCn',256)
           OR NOT plugin_idc_core_import_text(item->'nameEnUs',256) THEN RETURN FALSE; END IF;
    END LOOP;
    RETURN TRUE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_catalog_product_tenant_no
    ON plugin_idc_core_catalog_product (tenant_id,product_no);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_catalog_version_tenant_product_no
    ON plugin_idc_core_catalog_product_version (tenant_id,product_no,version_no);

-- Purpose: Immutable import request/source evidence, atomically bound to its newly created draft.
-- 用途：保存不可变的上游商品导入请求和安全来源证据，事务内绑定新建草稿。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_import (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0,
    request_key VARCHAR(128) NOT NULL,
    request_digest CHAR(64) NOT NULL,
    actor_user_id INT NOT NULL,
    interface_instance_no VARCHAR(40) NOT NULL,
    upstream_reference VARCHAR(256) NOT NULL,
    product_no VARCHAR(40),
    version_no VARCHAR(40),
    source_snapshot JSONB NOT NULL,
    missing_configuration_snapshot JSONB NOT NULL DEFAULT '[]',
    pricing_incomplete BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_catalog_import_identity CHECK (
        tenant_id = 0 AND actor_user_id > 0
        AND request_key = btrim(request_key) AND length(request_key) BETWEEN 1 AND 128
        AND request_key !~ '[[:cntrl:]]' AND request_digest ~ '^[0-9a-f]{64}$'
        AND interface_instance_no ~ '^pfi_[0-9a-f]{32}$'
        AND upstream_reference = btrim(upstream_reference)
        AND length(upstream_reference) BETWEEN 1 AND 256 AND upstream_reference !~ '[[:cntrl:]]'
    ),
    CONSTRAINT ck_plugin_idc_core_catalog_import_pair CHECK (
        (product_no IS NULL AND version_no IS NULL)
        OR (product_no IS NOT NULL AND version_no IS NOT NULL
            AND product_no ~ '^prd_[0-9a-f]{32}$' AND version_no ~ '^ver_[0-9a-f]{32}$')
    ),
    CONSTRAINT ck_plugin_idc_core_catalog_import_source CHECK (
        plugin_idc_core_import_source_valid(source_snapshot)
        AND source_snapshot->'product'->>'reference' = upstream_reference
        AND plugin_idc_core_import_missing_valid(missing_configuration_snapshot)
    ),
    CONSTRAINT uk_plugin_idc_core_catalog_import_request UNIQUE (tenant_id,actor_user_id,request_key),
    CONSTRAINT uk_plugin_idc_core_catalog_import_version UNIQUE (version_no),
    CONSTRAINT fk_plugin_idc_core_catalog_import_interface FOREIGN KEY (tenant_id,interface_instance_no)
        REFERENCES plugin_idc_core_product_automation_interface_instance (tenant_id,interface_instance_no) ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_catalog_import_product FOREIGN KEY (tenant_id,product_no)
        REFERENCES plugin_idc_core_catalog_product (tenant_id,product_no) ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_catalog_import_version FOREIGN KEY (tenant_id,product_no,version_no)
        REFERENCES plugin_idc_core_catalog_product_version (tenant_id,product_no,version_no) ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_catalog_import_interface
    ON plugin_idc_core_catalog_product_import (tenant_id,interface_instance_no,id);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_catalog_import_product
    ON plugin_idc_core_catalog_product_import (tenant_id,product_no);

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_catalog_import()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'catalog import evidence cannot be deleted'; END IF;
    IF TG_OP = 'UPDATE' AND (
        OLD.product_no IS NOT NULL OR OLD.version_no IS NOT NULL
        OR NEW.product_no IS NULL OR NEW.version_no IS NULL
        OR (to_jsonb(NEW) - ARRAY['product_no','version_no']) IS DISTINCT FROM
           (to_jsonb(OLD) - ARRAY['product_no','version_no'])
    ) THEN RAISE EXCEPTION 'catalog import evidence is immutable'; END IF;
    IF NEW.product_no IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM plugin_idc_core_catalog_product product
        JOIN plugin_idc_core_catalog_product_version version_row
          ON version_row.tenant_id=product.tenant_id AND version_row.product_no=product.product_no
        WHERE product.tenant_id=NEW.tenant_id AND product.product_no=NEW.product_no
          AND version_row.version_no=NEW.version_no
          AND product.status_code='draft' AND version_row.status_code='draft'
    ) THEN RAISE EXCEPTION 'catalog import must create an unpublished draft'; END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE OR REPLACE TRIGGER trg_plugin_idc_core_catalog_import_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_catalog_product_import
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_catalog_import();

-- A reservation is transaction-local only: even a caller that forgets the
-- final binding cannot commit a dangling idempotency record.
CREATE OR REPLACE FUNCTION plugin_idc_core_check_catalog_import_complete()
RETURNS TRIGGER AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM plugin_idc_core_catalog_product_import
        WHERE id=NEW.id AND (product_no IS NULL OR version_no IS NULL)) THEN
        RAISE EXCEPTION 'catalog import draft binding is incomplete';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
DO $constraint_trigger$
BEGIN
    PERFORM pg_advisory_xact_lock(
        hashtextextended('idc-core:trg_plugin_idc_core_catalog_import_complete', 0)
    );
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_catalog_product_import'::regclass
          AND conname = 'trg_plugin_idc_core_catalog_import_complete'
          AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row
              ON trigger_row.tgconstraint = constraint_row.oid
            WHERE constraint_row.conrelid = 'plugin_idc_core_catalog_product_import'::regclass
              AND constraint_row.conname = 'trg_plugin_idc_core_catalog_import_complete'
              AND constraint_row.contype = 't'
              AND constraint_row.condeferrable
              AND constraint_row.condeferred
              AND trigger_row.tgrelid = constraint_row.conrelid
              AND NOT trigger_row.tgisinternal
              AND trigger_row.tgname = 'trg_plugin_idc_core_catalog_import_complete'
              AND trigger_row.tgfoid = 'plugin_idc_core_check_catalog_import_complete()'::regprocedure
              AND trigger_row.tgtype = 21
              AND trigger_row.tgenabled = 'O'
              AND trigger_row.tgdeferrable
              AND trigger_row.tginitdeferred
        ) THEN
            RAISE EXCEPTION
                'constraint trigger trg_plugin_idc_core_catalog_import_complete catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgrelid = 'plugin_idc_core_catalog_product_import'::regclass
          AND tgname = 'trg_plugin_idc_core_catalog_import_complete'
          AND NOT tgisinternal
    ) THEN
        RAISE EXCEPTION
            'constraint trigger trg_plugin_idc_core_catalog_import_complete catalog contract is invalid'
            USING ERRCODE = '23514';
    ELSE
        CREATE CONSTRAINT TRIGGER trg_plugin_idc_core_catalog_import_complete
            AFTER INSERT OR UPDATE ON plugin_idc_core_catalog_product_import
            DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
            EXECUTE FUNCTION plugin_idc_core_check_catalog_import_complete();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_catalog_product_import IS 'Immutable bounded upstream catalog draft import evidence';
