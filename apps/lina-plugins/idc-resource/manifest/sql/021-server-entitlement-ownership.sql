-- 021: Server entitlement ownership projection.
--
-- idc-resource owns this projection and never reads idc-core tables. The
-- generic entitlement and fulfillment owner must deliver the frozen values
-- through the versioned resource-owner command boundary. Only a succeeded
-- provision task with an active resource mapping can be bound.

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_mapping_identity
    ON plugin_idc_resource_mapping ("tenant_id", "mapping_no", "local_resource_no");

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_task_identity
    ON plugin_idc_resource_task ("tenant_id", "task_no", "local_resource_no");

-- Purpose: Persists business records for plugin_idc_resource_server_entitlement_projection.
-- 用途：持久化 plugin_idc_resource_server_entitlement_projection 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_server_entitlement_projection (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entitlement_no" VARCHAR(40) PRIMARY KEY,
    "source_intent_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "product_version_no" VARCHAR(40) NOT NULL,
    "product_name_zh_cn" VARCHAR(256) NOT NULL,
    "product_name_en_us" VARCHAR(256) NOT NULL,
    "local_resource_no" VARCHAR(64) NOT NULL,
    "mapping_no" VARCHAR(64) NOT NULL,
    "provider_task_no" VARCHAR(64) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "provider_plugin_id" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "fulfillment_contract_version" VARCHAR(32) NOT NULL,
    "server_contract_version" VARCHAR(32) NOT NULL,
    "release_digest" CHAR(64) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "bound_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_server_entitlement_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_server_entitlement_identity CHECK (
        "entitlement_no" ~ '^ent_[0-9a-f]{32}$' AND
        "source_intent_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        "customer_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        "product_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        "product_version_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        length(btrim("product_name_zh_cn")) BETWEEN 1 AND 256 AND
        length(btrim("product_name_en_us")) BETWEEN 1 AND 256 AND
        "local_resource_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_server_entitlement_plugin CHECK (
        "interface_instance_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        "provider_plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND
        length(btrim("plugin_release_version")) > 0 AND
        length(btrim("fulfillment_contract_version")) > 0 AND
        "server_contract_version" = 'v1'
    ),
    CONSTRAINT ck_plugin_idc_resource_server_entitlement_digest CHECK (
        "release_digest" ~ '^[0-9a-f]{64}$' AND
        "declaration_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_server_entitlement_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_resource_server_entitlement_intent UNIQUE ("source_intent_no"),
    CONSTRAINT uk_plugin_idc_resource_server_entitlement_resource UNIQUE ("local_resource_no"),
    CONSTRAINT uk_plugin_idc_resource_server_entitlement_mapping UNIQUE ("mapping_no"),
    CONSTRAINT fk_plugin_idc_resource_server_entitlement_mapping FOREIGN KEY (
        "tenant_id", "mapping_no", "local_resource_no"
    ) REFERENCES plugin_idc_resource_mapping (
        "tenant_id", "mapping_no", "local_resource_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_resource_server_entitlement_task FOREIGN KEY (
        "tenant_id", "provider_task_no", "local_resource_no"
    ) REFERENCES plugin_idc_resource_task (
        "tenant_id", "task_no", "local_resource_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_server_entitlement_customer
    ON plugin_idc_resource_server_entitlement_projection (
        "tenant_id", "customer_no", "entitlement_no"
    );

COMMENT ON TABLE plugin_idc_resource_server_entitlement_projection
    IS 'Resource-owner projection binding one generic long-lived entitlement to one proven server resource and customer';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."customer_no"
    IS 'Frozen customer owner delivered through the owner command boundary; never derived from JSON or another plugin table';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_no"
    IS 'Opaque generic product reference frozen by the entitlement-created event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_version_no"
    IS 'Opaque generic product version reference frozen by the entitlement-created event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_name_zh_cn"
    IS 'Frozen customer-safe Simplified Chinese product label delivered by the entitlement owner event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."product_name_en_us"
    IS 'Frozen customer-safe English product label delivered by the entitlement owner event';
COMMENT ON COLUMN plugin_idc_resource_server_entitlement_projection."local_resource_no"
    IS 'Deterministic generic entitlement number used as the resource-local identity';
