-- 016: Persist immutable, field-whitelisted per-server accessory directories.
-- Provider identifiers remain resource-owner private and are never exposed as
-- customer or administrator object identifiers.

-- Purpose: Persists business records for plugin_idc_resource_accessory_snapshot.
-- 用途：持久化 plugin_idc_resource_accessory_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_accessory_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "mapping_no" VARCHAR(64) NOT NULL,
    "local_resource_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'complete',
    "snapshot_fingerprint" CHAR(64) NOT NULL,
    "item_count" INT NOT NULL,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_accessory_snapshot_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_accessory_snapshot_status
        CHECK ("status_code" IN ('complete', 'superseded')),
    CONSTRAINT ck_plugin_idc_resource_accessory_snapshot_digest
        CHECK ("snapshot_fingerprint" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_resource_accessory_snapshot_count
        CHECK ("item_count" BETWEEN 0 AND 10000),
    CONSTRAINT ck_plugin_idc_resource_accessory_snapshot_time
        CHECK ("expires_at" > "observed_at"),
    CONSTRAINT uk_plugin_idc_resource_accessory_snapshot_no
        UNIQUE ("snapshot_no"),
    CONSTRAINT fk_plugin_idc_resource_accessory_snapshot_mapping
        FOREIGN KEY ("mapping_no")
        REFERENCES plugin_idc_resource_mapping ("mapping_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_resource_accessory_snapshot_channel
        FOREIGN KEY ("channel_no")
        REFERENCES plugin_idc_resource_channel ("channel_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_accessory_snapshot_current
    ON plugin_idc_resource_accessory_snapshot (
        "tenant_id", "local_resource_no"
    ) WHERE "status_code" = 'complete';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_accessory_snapshot_latest
    ON plugin_idc_resource_accessory_snapshot (
        "tenant_id", "local_resource_no", "status_code", "observed_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_resource_accessory_snapshot_item.
-- 用途：持久化 plugin_idc_resource_accessory_snapshot_item 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_accessory_snapshot_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(64) NOT NULL,
    "object_no" VARCHAR(64) NOT NULL,
    "accessory_kind" VARCHAR(24) NOT NULL,
    "external_ref" VARCHAR(256) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL DEFAULT '',
    "state_code" VARCHAR(32) NOT NULL,
    "projection" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_accessory_item_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_accessory_item_kind CHECK (
        "accessory_kind" IN (
            'ip', 'disk', 'snapshot', 'backup', 'security_group', 'nat',
            'virtual_media', 'traffic'
        )
    ),
    CONSTRAINT ck_plugin_idc_resource_accessory_item_ref
        CHECK (length("external_ref") BETWEEN 1 AND 256),
    CONSTRAINT ck_plugin_idc_resource_accessory_item_state
        CHECK ("state_code" IN ('available', 'attached', 'mounted', 'active', 'inactive', 'unknown')),
    CONSTRAINT ck_plugin_idc_resource_accessory_item_projection
        CHECK (jsonb_typeof("projection") = 'object'),
    CONSTRAINT uk_plugin_idc_resource_accessory_item_object
        UNIQUE ("snapshot_no", "object_no"),
    CONSTRAINT uk_plugin_idc_resource_accessory_item_external
        UNIQUE ("snapshot_no", "accessory_kind", "external_ref"),
    CONSTRAINT fk_plugin_idc_resource_accessory_item_snapshot
        FOREIGN KEY ("snapshot_no")
        REFERENCES plugin_idc_resource_accessory_snapshot ("snapshot_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_accessory_item_directory
    ON plugin_idc_resource_accessory_snapshot_item (
        "tenant_id", "snapshot_no", "accessory_kind", "display_name", "id"
    );

-- Purpose: Persists business records for plugin_idc_resource_accessory_refresh_state.
-- 用途：持久化 plugin_idc_resource_accessory_refresh_state 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_accessory_refresh_state (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_no" VARCHAR(64) NOT NULL,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "attempt_count" INT NOT NULL DEFAULT 0,
    "lease_token_digest" CHAR(64),
    "lease_expires_at" TIMESTAMPTZ,
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_accessory_refresh_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_accessory_refresh_attempt CHECK ("attempt_count" BETWEEN 0 AND 1000000),
    CONSTRAINT ck_plugin_idc_resource_accessory_refresh_lease CHECK (
        ("lease_token_digest" IS NULL AND "lease_expires_at" IS NULL) OR
        ("lease_token_digest" ~ '^[0-9a-f]{64}$' AND "lease_expires_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_resource_accessory_refresh_failure CHECK ("failure_code" ~ '^[a-z0-9_]{0,64}$'),
    CONSTRAINT uk_plugin_idc_resource_accessory_refresh_mapping UNIQUE ("mapping_no"),
    CONSTRAINT fk_plugin_idc_resource_accessory_refresh_mapping FOREIGN KEY ("mapping_no")
        REFERENCES plugin_idc_resource_mapping ("mapping_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_accessory_refresh_due
    ON plugin_idc_resource_accessory_refresh_state ("tenant_id", "next_attempt_at", "id");

CREATE OR REPLACE FUNCTION plugin_idc_resource_reject_accessory_item_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'resource accessory snapshot items are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_resource_accessory_item_immutable
    ON plugin_idc_resource_accessory_snapshot_item;
CREATE TRIGGER trg_plugin_idc_resource_accessory_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_resource_accessory_snapshot_item
FOR EACH ROW EXECUTE FUNCTION
    plugin_idc_resource_reject_accessory_item_mutation();

COMMENT ON TABLE plugin_idc_resource_accessory_snapshot
    IS 'Immutable complete field-whitelisted accessory directory for one active resource mapping';
COMMENT ON TABLE plugin_idc_resource_accessory_snapshot_item
    IS 'Owner-private Provider references and safe projections for one accessory directory snapshot';
COMMENT ON TABLE plugin_idc_resource_accessory_refresh_state
    IS 'Database-clock lease and retry state for managed accessory directory refreshes';
COMMENT ON COLUMN plugin_idc_resource_accessory_snapshot_item."object_no"
    IS 'Local opaque object number exposed to callers instead of Provider identifiers';
COMMENT ON COLUMN plugin_idc_resource_accessory_snapshot_item."external_ref"
    IS 'Resource-owner private Provider reference; never returned, logged, or audited';
