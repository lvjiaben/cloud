-- Retire the server-shaped support service association in favor of the
-- generic long-term entitlement owned by idc-core. The migration is safe to
-- rerun after older development migrations recreate their legacy objects.

ALTER TABLE plugin_idc_core_service_entitlement
    ADD COLUMN IF NOT EXISTS "support_association_reference" VARCHAR(36)
        GENERATED ALWAYS AS ('sar_' || md5('support-association:v1:' || "entitlement_no")) STORED;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_core_service_entitlement_support_association_reference
    ON plugin_idc_core_service_entitlement("tenant_id","support_association_reference");

CREATE INDEX IF NOT EXISTS idx_idc_core_service_entitlement_customer_support_directory
    ON plugin_idc_core_service_entitlement("tenant_id","customer_no","id" DESC)
    WHERE "status_code" <> 'terminated';

DROP TRIGGER IF EXISTS trg_idc_support_ticket_service_link_immutable
    ON plugin_idc_core_support_ticket;
DROP TRIGGER IF EXISTS trg_idc_support_ticket_entitlement_link_immutable
    ON plugin_idc_core_support_ticket;

ALTER TABLE plugin_idc_core_support_ticket
    DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_service_link,
    DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_entitlement_link;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_no'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_entitlement_no'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            RENAME COLUMN "related_service_no" TO "related_entitlement_no";
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_version'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_entitlement_version'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            RENAME COLUMN "related_service_version" TO "related_entitlement_version";
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_name_zh_cn'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_entitlement_name_zh_cn'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            RENAME COLUMN "related_service_name_zh_cn" TO "related_entitlement_name_zh_cn";
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_name_en_us'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_entitlement_name_en_us'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            RENAME COLUMN "related_service_name_en_us" TO "related_entitlement_name_en_us";
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_entitlement_status'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_entitlement_status'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            RENAME COLUMN "related_service_entitlement_status" TO "related_entitlement_status";
    END IF;
END $$;

-- Older migrations may recreate empty legacy columns on a later full rerun.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket'
          AND column_name='related_service_no'
    ) THEN
        EXECUTE $merge$
            UPDATE plugin_idc_core_support_ticket
            SET "related_entitlement_no"=COALESCE("related_entitlement_no","related_service_no"),
                "related_entitlement_version"=COALESCE("related_entitlement_version","related_service_version"),
                "related_entitlement_name_zh_cn"=COALESCE("related_entitlement_name_zh_cn","related_service_name_zh_cn"),
                "related_entitlement_name_en_us"=COALESCE("related_entitlement_name_en_us","related_service_name_en_us"),
                "related_entitlement_status"=COALESCE("related_entitlement_status","related_service_entitlement_status")
            WHERE "related_service_no" IS NOT NULL
        $merge$;
    END IF;
END $$;

ALTER TABLE plugin_idc_core_support_ticket
    DROP COLUMN IF EXISTS "related_service_runtime_status",
    DROP COLUMN IF EXISTS "related_service_entitlement_status",
    DROP COLUMN IF EXISTS "related_service_name_en_us",
    DROP COLUMN IF EXISTS "related_service_name_zh_cn",
    DROP COLUMN IF EXISTS "related_service_version",
    DROP COLUMN IF EXISTS "related_service_no";

ALTER TABLE plugin_idc_core_support_ticket
    ADD COLUMN IF NOT EXISTS "related_entitlement_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "related_entitlement_version" BIGINT,
    ADD COLUMN IF NOT EXISTS "related_entitlement_name_zh_cn" VARCHAR(256),
    ADD COLUMN IF NOT EXISTS "related_entitlement_name_en_us" VARCHAR(256),
    ADD COLUMN IF NOT EXISTS "related_entitlement_status" VARCHAR(24),
    ADD CONSTRAINT ck_idc_support_ticket_entitlement_link CHECK (
        ("related_entitlement_no" IS NULL AND "related_entitlement_version" IS NULL AND
         "related_entitlement_name_zh_cn" IS NULL AND "related_entitlement_name_en_us" IS NULL AND
         "related_entitlement_status" IS NULL)
        OR
        ("ticket_type"='customer' AND "customer_no" IS NOT NULL AND
         "related_entitlement_no" ~ '^[A-Za-z0-9_-]{1,40}$' AND "related_entitlement_version" > 0 AND
         length(btrim("related_entitlement_name_zh_cn")) > 0 AND
         length(btrim("related_entitlement_name_en_us")) > 0 AND
         "related_entitlement_status" <> 'terminated')
    );

DROP INDEX IF EXISTS idx_idc_support_ticket_service;
CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_entitlement
    ON plugin_idc_core_support_ticket(
        "tenant_id","related_entitlement_no","last_activity_at" DESC,"id" DESC
    ) WHERE "related_entitlement_no" IS NOT NULL;

DROP FUNCTION IF EXISTS plugin_idc_core_support_ticket_service_link_immutable();
CREATE OR REPLACE FUNCTION plugin_idc_core_support_ticket_entitlement_link_immutable()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."related_entitlement_no" IS DISTINCT FROM NEW."related_entitlement_no" OR
       OLD."related_entitlement_version" IS DISTINCT FROM NEW."related_entitlement_version" OR
       OLD."related_entitlement_name_zh_cn" IS DISTINCT FROM NEW."related_entitlement_name_zh_cn" OR
       OLD."related_entitlement_name_en_us" IS DISTINCT FROM NEW."related_entitlement_name_en_us" OR
       OLD."related_entitlement_status" IS DISTINCT FROM NEW."related_entitlement_status" THEN
        RAISE EXCEPTION 'support ticket entitlement link is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_idc_support_ticket_entitlement_link_immutable
    BEFORE UPDATE ON plugin_idc_core_support_ticket
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_support_ticket_entitlement_link_immutable();

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket_upstream_forward'
          AND column_name='service_upstream_ref'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket_upstream_forward'
          AND column_name='entitlement_upstream_ref'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket_upstream_forward
            RENAME COLUMN "service_upstream_ref" TO "entitlement_upstream_ref";
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_ticket_upstream_forward'
          AND column_name='service_upstream_ref'
    ) THEN
        EXECUTE $merge$
            UPDATE plugin_idc_core_support_ticket_upstream_forward
            SET "entitlement_upstream_ref"=COALESCE("entitlement_upstream_ref","service_upstream_ref")
            WHERE "service_upstream_ref" IS NOT NULL
        $merge$;
        ALTER TABLE plugin_idc_core_support_ticket_upstream_forward
            DROP COLUMN "service_upstream_ref";
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('plugin_idc_core_support_upstream_service_map') IS NOT NULL
       AND to_regclass('plugin_idc_core_support_upstream_entitlement_map') IS NULL THEN
        ALTER TABLE plugin_idc_core_support_upstream_service_map
            RENAME TO plugin_idc_core_support_upstream_entitlement_map;
    END IF;
END $$;

-- Purpose: Stores Support Upstream Entitlement Map records.
-- 用途：保存支持上游权益映射相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_upstream_entitlement_map (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "upstream_reference" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_upstream_entitlement_map'
          AND column_name='service_no'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_upstream_entitlement_map'
          AND column_name='entitlement_no'
    ) THEN
        ALTER TABLE plugin_idc_core_support_upstream_entitlement_map
            RENAME COLUMN "service_no" TO "entitlement_no";
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_upstream_entitlement_map'
          AND column_name='upstream_service_ref'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema() AND table_name='plugin_idc_core_support_upstream_entitlement_map'
          AND column_name='upstream_reference'
    ) THEN
        ALTER TABLE plugin_idc_core_support_upstream_entitlement_map
            RENAME COLUMN "upstream_service_ref" TO "upstream_reference";
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass('plugin_idc_core_support_upstream_service_map') IS NOT NULL THEN
        EXECUTE $merge$
            INSERT INTO plugin_idc_core_support_upstream_entitlement_map(
                "tenant_id","entitlement_no","upstream_reference","status_code","version",
                "created_by","updated_by","created_at","updated_at"
            )
            SELECT legacy."tenant_id",legacy."service_no",legacy."upstream_service_ref",legacy."status_code",legacy."version",
                   legacy."created_by",legacy."updated_by",legacy."created_at",legacy."updated_at"
            FROM plugin_idc_core_support_upstream_service_map legacy
            WHERE NOT EXISTS (
                SELECT 1
                FROM plugin_idc_core_support_upstream_entitlement_map current_mapping
                WHERE current_mapping."tenant_id"=legacy."tenant_id"
                  AND current_mapping."entitlement_no"=legacy."service_no"
            )
        $merge$;
    END IF;
END $$;

DROP TABLE IF EXISTS plugin_idc_core_support_upstream_service_map;

ALTER TABLE plugin_idc_core_support_upstream_entitlement_map
    DROP CONSTRAINT IF EXISTS uk_idc_support_upstream_service_map,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_service_tenant,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_service_status,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_service_version,
    DROP CONSTRAINT IF EXISTS uk_idc_support_upstream_entitlement_map,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_entitlement_tenant,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_entitlement_status,
    DROP CONSTRAINT IF EXISTS ck_idc_support_upstream_entitlement_version,
    DROP CONSTRAINT IF EXISTS fk_idc_support_upstream_entitlement;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_support_upstream_entitlement_map mapping
        LEFT JOIN plugin_idc_core_service_entitlement entitlement
          ON entitlement."entitlement_no"=mapping."entitlement_no"
        WHERE entitlement."entitlement_no" IS NULL
    ) THEN
        RAISE EXCEPTION 'support upstream mapping contains a legacy service reference; export any needed development fixtures and rebuild the development database';
    END IF;
END $$;

ALTER TABLE plugin_idc_core_support_upstream_entitlement_map
    ADD CONSTRAINT uk_idc_support_upstream_entitlement_map UNIQUE("tenant_id","entitlement_no"),
    ADD CONSTRAINT ck_idc_support_upstream_entitlement_tenant CHECK("tenant_id"=0),
    ADD CONSTRAINT ck_idc_support_upstream_entitlement_status CHECK("status_code" IN ('active','disabled')),
    ADD CONSTRAINT ck_idc_support_upstream_entitlement_version CHECK("version">0),
    ADD CONSTRAINT fk_idc_support_upstream_entitlement FOREIGN KEY("entitlement_no")
        REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT;
