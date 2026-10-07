-- 027: stable opaque action references for the resource-owned server directory.
--
-- The source entitlement number already carries 128 bits of randomness. The
-- generated digest keeps that identifier out of future HTTP DTOs while
-- remaining deterministic, idempotent, and indexable for ownership checks.

ALTER TABLE plugin_idc_resource_server_entitlement_projection
    ADD COLUMN IF NOT EXISTS action_reference VARCHAR(36)
    GENERATED ALWAYS AS ('srv_' || md5("entitlement_no")) STORED;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_server_entitlement_action_ref'
          AND conrelid = 'plugin_idc_resource_server_entitlement_projection'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_server_entitlement_projection
            ADD CONSTRAINT ck_plugin_idc_resource_server_entitlement_action_ref
            CHECK (action_reference ~ '^srv_[0-9a-f]{32}$');
    END IF;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_server_entitlement_action_ref
    ON plugin_idc_resource_server_entitlement_projection (
        "tenant_id", action_reference
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_server_directory_customer
    ON plugin_idc_resource_server_entitlement_projection (
        "tenant_id", "customer_no", action_reference
    );

COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection.action_reference
    IS 'Stable opaque resource-owner action reference; safe for routing but never rendered as customer-visible text';
