-- Merchant-wide current payment routing. Saving replaces current settings;
-- there is no publication step or soft deletion of this singleton setting.
CREATE TABLE IF NOT EXISTS plugin_idc_payment_public_address (
    tenant_id INT PRIMARY KEY DEFAULT 0 CHECK (tenant_id=0),
    notification_origin VARCHAR(2048) NOT NULL DEFAULT '',
    customer_return_origin VARCHAR(2048) NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
    updated_by INT NOT NULL CHECK (updated_by>0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
