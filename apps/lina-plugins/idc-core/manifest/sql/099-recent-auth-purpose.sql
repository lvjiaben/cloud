-- Extend the durable recent-authentication replay fence for plugin-managed
-- sensitive material and customer server operations.
ALTER TABLE plugin_idc_core_recent_auth_consumption
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_reauth_purpose;

ALTER TABLE plugin_idc_core_recent_auth_consumption
    ADD CONSTRAINT ck_plugin_idc_core_reauth_purpose CHECK (
        "purpose_code" IN (
            'secret.access',
            'payment.config',
            'funds.high-value',
            'resource.bulk',
            'export.sensitive',
            'material.sensitive',
            'permission.change',
            'resource.server-operation'
        )
    );
