-- Freeze transferred file tokens per paid procurement intent so retries reuse
-- the original upstream quote input. This immutable evidence is not soft deleted.
CREATE TABLE IF NOT EXISTS plugin_idc_core_peer_purchase_file (
 id BIGSERIAL PRIMARY KEY,
 tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
 intent_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_product_fulfillment_intent(intent_no),
 field_key VARCHAR(64) NOT NULL CHECK (length(field_key)>0),
 file_sequence INT NOT NULL CHECK (file_sequence>=0 AND file_sequence<128),
 sha256_hex CHAR(64) NOT NULL CHECK (sha256_hex ~ '^[a-f0-9]{64}$'),
 upstream_file_token VARCHAR(4096) NOT NULL CHECK (length(upstream_file_token)>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT uk_plugin_idc_core_peer_purchase_file UNIQUE(intent_no,field_key,file_sequence)
);
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_peer_purchase_file()
RETURNS TRIGGER AS $$
BEGIN
 RAISE EXCEPTION 'peer purchase file evidence is immutable' USING ERRCODE='23514';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_plugin_idc_core_peer_purchase_file ON plugin_idc_core_peer_purchase_file;
CREATE TRIGGER trg_plugin_idc_core_peer_purchase_file BEFORE UPDATE OR DELETE ON plugin_idc_core_peer_purchase_file
 FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_peer_purchase_file();
