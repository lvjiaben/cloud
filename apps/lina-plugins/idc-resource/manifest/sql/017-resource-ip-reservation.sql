-- 017: Serialize customer IP-change stock reservations inside the resource owner.

-- Purpose: Persists business records for plugin_idc_resource_ip_reservation.
-- 用途：持久化 plugin_idc_resource_ip_reservation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_ip_reservation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reservation_no" VARCHAR(64) NOT NULL,
    "service_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "pool_no" VARCHAR(64) NOT NULL,
    "capacity_observation_no" VARCHAR(64) NOT NULL,
    "capacity_kind" VARCHAR(24) NOT NULL DEFAULT 'stock',
    "quantity" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'reserved',
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "provider_task_no" VARCHAR(64),
    "release_reason_code" VARCHAR(64),
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_idc_resource_ip_reservation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_resource_ip_reservation_quantity CHECK ("quantity" BETWEEN 1 AND 64),
    CONSTRAINT ck_idc_resource_ip_reservation_capacity_kind CHECK ("capacity_kind" = 'stock'),
    CONSTRAINT ck_idc_resource_ip_reservation_status CHECK (
        "status_code" IN ('reserved', 'consumed', 'released', 'expired')
    ),
    CONSTRAINT ck_idc_resource_ip_reservation_digests CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_resource_ip_reservation_lifecycle CHECK (
        ("status_code" = 'reserved' AND "provider_task_no" IS NULL AND "consumed_at" IS NULL AND "released_at" IS NULL)
        OR ("status_code" = 'consumed' AND "provider_task_no" IS NOT NULL AND "consumed_at" IS NOT NULL AND "released_at" IS NULL AND "release_reason_code" IS NULL)
        OR ("status_code" IN ('released', 'expired') AND "provider_task_no" IS NULL AND "consumed_at" IS NULL AND "released_at" IS NOT NULL AND "release_reason_code" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_resource_ip_reservation_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT ck_idc_resource_ip_reservation_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_resource_ip_reservation_no UNIQUE ("reservation_no"),
    CONSTRAINT uk_idc_resource_ip_reservation_request UNIQUE ("idempotency_digest"),
    CONSTRAINT fk_idc_resource_ip_reservation_pool FOREIGN KEY ("channel_no", "pool_no")
        REFERENCES plugin_idc_resource_pool ("channel_no", "pool_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_resource_ip_reservation_observation FOREIGN KEY ("capacity_observation_no", "capacity_kind")
        REFERENCES plugin_idc_resource_provider_capacity_observation ("observation_no", "capacity_kind") ON DELETE RESTRICT
);

-- Preserve local validation rows created before the provider-capacity naming
-- was made explicit. This is an in-place development-data migration, not a
-- runtime compatibility alias.
DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_resource_ip_reservation'
           AND column_name = 'snapshot_no'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_resource_ip_reservation'
           AND column_name = 'capacity_observation_no'
    ) THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            RENAME COLUMN "snapshot_no" TO "capacity_observation_no";
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_resource_ip_reservation'
           AND column_name = 'inventory_kind'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_resource_ip_reservation'
           AND column_name = 'capacity_kind'
    ) THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            RENAME COLUMN inventory_kind TO "capacity_kind";
    END IF;
END;
$migration$;

CREATE INDEX IF NOT EXISTS idx_idc_resource_ip_reservation_active
    ON plugin_idc_resource_ip_reservation (
        "tenant_id", "channel_no", "pool_no", "capacity_observation_no", "expires_at", "id"
    ) WHERE "status_code" = 'reserved';
CREATE INDEX IF NOT EXISTS idx_idc_resource_ip_reservation_service
    ON plugin_idc_resource_ip_reservation (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_resource_ip_reservation
    IS 'Resource-owner serialized stock fence for one server IP-change intent';
