-- 009: Resource-owned upstream principal mappings and provision credentials.
--
-- Customer and service numbers are opaque cross-plugin references. No foreign
-- key crosses the plugin boundary, and plaintext credentials never enter this
-- schema.

-- Purpose: Persists business records for plugin_idc_resource_principal_mapping.
-- 用途：持久化 plugin_idc_resource_principal_mapping 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_principal_mapping (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_no" VARCHAR(64) NOT NULL,
    "customer_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "external_principal_ref" VARCHAR(256) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "evidence_digest" CHAR(64) NOT NULL,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_customer
        CHECK ("customer_no" = btrim("customer_no") AND length("customer_no") > 0),
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_external
        CHECK (
            "external_principal_ref" = btrim("external_principal_ref") AND
            length("external_principal_ref") > 0
        ),
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_status
        CHECK ("status_code" IN ('active', 'suspended', 'revoked')),
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_digest
        CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_resource_principal_mapping_version
        CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_principal_mapping_no UNIQUE ("mapping_no"),
    CONSTRAINT fk_plugin_idc_resource_principal_mapping_channel
        FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_principal_mapping_active_customer
    ON plugin_idc_resource_principal_mapping ("tenant_id", "channel_no", "customer_no")
    WHERE "status_code" = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_principal_mapping_active_external
    ON plugin_idc_resource_principal_mapping (
        "tenant_id", "channel_no", "external_principal_ref"
    ) WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_principal_mapping_lookup
    ON plugin_idc_resource_principal_mapping (
        "tenant_id", "customer_no", "status_code", "channel_no", "id"
    );

COMMENT ON TABLE plugin_idc_resource_principal_mapping
    IS 'Resource-owner mapping from an opaque core customer to one channel-local upstream principal';
COMMENT ON COLUMN plugin_idc_resource_principal_mapping."customer_no"
    IS 'Opaque core-owned customer reference with no cross-plugin database foreign key';
COMMENT ON COLUMN plugin_idc_resource_principal_mapping."external_principal_ref"
    IS 'Channel-local upstream client identifier; never inferred from the local customer number';
COMMENT ON COLUMN plugin_idc_resource_principal_mapping."evidence_digest"
    IS 'Secret-free digest of the authoritative mapping evidence';

-- Purpose: Persists business records for plugin_idc_resource_provision_credential.
-- 用途：持久化 plugin_idc_resource_provision_credential 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provision_credential (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "credential_no" VARCHAR(64) NOT NULL,
    "service_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "secret_ref" VARCHAR(256) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "revealed_at" TIMESTAMPTZ,
    "revoked_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_provision_credential_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_provision_credential_service
        CHECK ("service_no" = btrim("service_no") AND length("service_no") > 0),
    CONSTRAINT ck_plugin_idc_resource_provision_credential_secret
        CHECK ("secret_ref" = btrim("secret_ref") AND length("secret_ref") > 0),
    CONSTRAINT ck_plugin_idc_resource_provision_credential_status
        CHECK ("status_code" IN ('pending', 'revealable', 'revealed', 'revoked')),
    CONSTRAINT ck_plugin_idc_resource_provision_credential_lifecycle CHECK (
        ("status_code" IN ('pending', 'revealable') AND "revealed_at" IS NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revealed' AND "revealed_at" IS NOT NULL AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_provision_credential_version
        CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_provision_credential_no UNIQUE ("credential_no"),
    CONSTRAINT uk_plugin_idc_resource_provision_credential_task UNIQUE ("task_no"),
    CONSTRAINT fk_plugin_idc_resource_provision_credential_task_channel
        FOREIGN KEY ("task_no", "channel_no")
        REFERENCES plugin_idc_resource_task ("task_no", "channel_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_provision_credential_active_service
    ON plugin_idc_resource_provision_credential ("tenant_id", "service_no")
    WHERE "status_code" IN ('pending', 'revealable');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provision_credential_status
    ON plugin_idc_resource_provision_credential (
        "tenant_id", "status_code", "updated_at", "id"
    );

COMMENT ON TABLE plugin_idc_resource_provision_credential
    IS 'Owner-only retention of an encrypted initial credential handle for one provision task';
COMMENT ON COLUMN plugin_idc_resource_provision_credential."secret_ref"
    IS 'Sensitive secretcap handle reference; never returned, logged, audited, or sent as ordinary JSON';

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_request_capability
    ON plugin_idc_resource_task (
        "tenant_id", "request_id_digest", "capability_code", "created_at" DESC, "id" DESC
    );
