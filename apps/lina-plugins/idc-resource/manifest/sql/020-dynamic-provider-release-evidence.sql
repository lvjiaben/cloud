-- 020: freeze exact managed Provider release evidence on every resource channel

ALTER TABLE plugin_idc_resource_channel
    ADD COLUMN IF NOT EXISTS "provider_runtime_type" VARCHAR(16) NOT NULL DEFAULT 'source',
    ADD COLUMN IF NOT EXISTS "provider_contract_version" VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "provider_release_version" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "provider_release_digest" CHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "provider_declaration_digest" CHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_resource_channel
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_channel_provider_runtime;

ALTER TABLE plugin_idc_resource_channel
    ADD CONSTRAINT ck_plugin_idc_resource_channel_provider_runtime CHECK (
        (
            "provider_runtime_type" = 'source' AND
            "provider_contract_version" = '' AND
            "provider_release_version" = '' AND
            "provider_release_digest" = '' AND
            "provider_declaration_digest" = ''
        ) OR (
            "provider_runtime_type" = 'dynamic' AND
            "provider_contract_version" ~ '^v[1-9][0-9]*$' AND
            length("provider_release_version") > 0 AND
            "provider_release_digest" ~ '^[0-9a-f]{64}$' AND
            "provider_declaration_digest" ~ '^[0-9a-f]{64}$'
        )
    );

COMMENT ON COLUMN plugin_idc_resource_channel."provider_runtime_type" IS
    'Frozen Provider runtime type: source or dynamic';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_contract_version" IS
    'Exact owner contract version frozen when a dynamic Provider channel is created';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_release_version" IS
    'Exact managed plugin release version frozen when a dynamic Provider channel is created';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_release_digest" IS
    'Exact managed plugin release digest frozen when a dynamic Provider channel is created';
COMMENT ON COLUMN plugin_idc_resource_channel."provider_declaration_digest" IS
    'Exact resource-provider declaration digest frozen when a dynamic Provider channel is created';

COMMENT ON COLUMN plugin_idc_resource_channel."provider_plugin_id" IS
    'Plugin that owns the frozen Provider implementation';
