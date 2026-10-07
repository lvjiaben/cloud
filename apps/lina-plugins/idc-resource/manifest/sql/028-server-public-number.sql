-- 028: stable public server numbers for resource-owned customer views.
--
-- Public numbers and hidden action references use domain-separated inputs.
-- Customers can safely quote the public number without learning the routing
-- reference used by future resource actions.

ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD COLUMN IF NOT EXISTS server_no VARCHAR(36)
    GENERATED ALWAYS AS (
        'SVR-' || upper(md5('public-server-no:' || "entitlement_no"))
    ) STORED;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_server_public_no'
          AND conrelid = 'plugin_idc_resource_server_entitlement_projection'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_server_entitlement_projection
            ADD CONSTRAINT ck_plugin_idc_resource_server_public_no
            CHECK (server_no ~ '^SVR-[0-9A-F]{32}$');
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_server_public_no
    ON plugin_idc_resource_server_entitlement_projection (
        "tenant_id", server_no
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_server_customer_public_no
    ON plugin_idc_resource_server_entitlement_projection (
        "tenant_id", "customer_no", server_no
    );

COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection.server_no
    IS 'Stable customer-visible server business number; distinct from the hidden action reference';
