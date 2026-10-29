-- Native imported purchase pricing evidence.

-- Native imported carts retain compiler output before any supplier quote.
-- Historical snapshots remain unchanged; only native pricing requires the map.
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD COLUMN IF NOT EXISTS typed_value_snapshot JSONB;
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_core_commerce_configuration_typed_values;
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD CONSTRAINT ck_idc_core_commerce_configuration_typed_values CHECK (
        typed_value_snapshot IS NULL OR (
            jsonb_typeof(typed_value_snapshot) = 'object'
            AND octet_length(typed_value_snapshot::TEXT) <= 1048576
        )
    );
COMMENT ON COLUMN plugin_idc_core_commerce_configuration_snapshot.typed_value_snapshot
    IS 'Compiler-produced customer values for native imported pricing; NULL for historical snapshots';

-- Only merchant pricing may change on an unpublished imported version. The
-- source contract, period identity and historical published rows stay frozen.
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_import_term_pricing()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE version_status TEXT;
BEGIN
    IF TG_OP <> 'UPDATE' THEN
        RAISE EXCEPTION 'imported period evidence cannot be deleted';
    END IF;
    IF (to_jsonb(NEW) - ARRAY['markup_percent','whole_item_rule'])
       IS DISTINCT FROM (to_jsonb(OLD) - ARRAY['markup_percent','whole_item_rule']) THEN
        RAISE EXCEPTION 'imported period identity is immutable';
    END IF;
    SELECT status_code INTO version_status
    FROM plugin_idc_core_catalog_product_version
    WHERE tenant_id=OLD.tenant_id AND version_no=OLD.version_no
    FOR SHARE;
    IF version_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'published imported prices are immutable';
    END IF;
    RETURN NEW;
END;
$$;
CREATE OR REPLACE TRIGGER trg_idc_import_term_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_import_term
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_import_term_pricing();

-- Price commands have their own actor-scoped receipt. A source import still
-- owns exactly one immutable row per version; saving prices does not reimport.
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_import_price_command (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    actor_user_id INT NOT NULL CHECK (actor_user_id>0),
    request_key VARCHAR(128) NOT NULL CHECK (length(request_key) BETWEEN 16 AND 128 AND request_key=btrim(request_key)),
    request_digest CHAR(64) NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    version_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_catalog_product_version(version_no) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_core_import_price_command_request UNIQUE (tenant_id,actor_user_id,request_key)
);
CREATE INDEX IF NOT EXISTS idx_idc_core_import_price_command_version
    ON plugin_idc_core_catalog_import_price_command (tenant_id,version_no,id);
CREATE OR REPLACE TRIGGER trg_idc_core_import_price_command_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_catalog_import_price_command
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_portable_import();
COMMENT ON TABLE plugin_idc_core_catalog_import_price_command
    IS 'Immutable actor-scoped receipts for merchant pricing changes on imported drafts';
