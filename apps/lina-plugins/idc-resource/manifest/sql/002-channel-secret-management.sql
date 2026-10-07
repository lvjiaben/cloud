-- 002: recoverable channel-secret management states

ALTER TABLE plugin_idc_resource_channel_secret_binding
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_binding_status;

ALTER TABLE plugin_idc_resource_channel_secret_binding
    ADD CONSTRAINT ck_plugin_idc_resource_binding_status CHECK (
        "status_code" IN (
            'preparing', 'retryable_failed', 'active', 'superseded', 'revoked'
        )
    );

ALTER TABLE plugin_idc_resource_channel_secret_binding
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_binding_lifecycle;

ALTER TABLE plugin_idc_resource_channel_secret_binding
    ADD CONSTRAINT ck_plugin_idc_resource_binding_lifecycle CHECK (
        ("status_code" IN ('preparing', 'retryable_failed', 'active')
            AND "superseded_at" IS NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'superseded'
            AND "superseded_at" IS NOT NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_binding_recovery
    ON plugin_idc_resource_channel_secret_binding (
        "tenant_id", "status_code", "updated_at", "id"
    ) WHERE "status_code" IN ('preparing', 'retryable_failed');

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_binding_pending
    ON plugin_idc_resource_channel_secret_binding (
        "tenant_id", "channel_no", "binding_name"
    ) WHERE "status_code" IN ('preparing', 'retryable_failed');

COMMENT ON COLUMN plugin_idc_resource_channel_secret_binding."status_code" IS
    'Recoverable owner lifecycle: preparing, retryable_failed, active, superseded, or revoked';
