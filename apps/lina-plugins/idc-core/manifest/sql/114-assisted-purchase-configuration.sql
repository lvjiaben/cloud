-- Immutable staff intent; configuration is revalidated before approval.
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_assisted_configuration (
    tenant_id BIGINT NOT NULL DEFAULT 0,
    request_item_no VARCHAR(40) NOT NULL,
    request_no VARCHAR(40) NOT NULL,
    purchase_input JSONB NOT NULL,
    PRIMARY KEY (tenant_id, request_item_no),
    FOREIGN KEY (request_item_no)
        REFERENCES plugin_idc_core_commerce_assisted_quote_request_item (request_item_no)
);
CREATE INDEX IF NOT EXISTS plugin_idc_core_assisted_configuration_request_idx
    ON plugin_idc_core_commerce_assisted_configuration (tenant_id, request_no);
DROP TRIGGER IF EXISTS trg_idc_assisted_configuration_immutable
    ON plugin_idc_core_commerce_assisted_configuration;
CREATE TRIGGER trg_idc_assisted_configuration_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_assisted_configuration
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();
