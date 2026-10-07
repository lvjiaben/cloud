-- 080: Generic delivery records and optional long-lived service entitlements.
-- Inventory remains a product-version setting; these tables contain no
-- server type, resource pool, provider runtime status, or inventory audit.

ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy
    ADD COLUMN IF NOT EXISTS "ongoing_management" BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE plugin_idc_core_product_fulfillment_intent
    ADD COLUMN IF NOT EXISTS "ongoing_management" BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_intent_order_item_status
    ON plugin_idc_core_product_fulfillment_intent("tenant_id", "order_item_no", "status_code");

-- Purpose: Stores Product Delivery Record records.
-- 用途：保存商品交付记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_delivery_record (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "id" BIGSERIAL UNIQUE,
    "delivery_no" VARCHAR(40) PRIMARY KEY,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "quantity" INT NOT NULL,
    "strategy_code" VARCHAR(32) NOT NULL,
    "policy_digest" CHAR(64) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL DEFAULT '',
    "interface_instance_no" VARCHAR(40) NOT NULL DEFAULT '',
    "plugin_id" VARCHAR(64) NOT NULL DEFAULT '',
    "plugin_release_version" VARCHAR(32) NOT NULL DEFAULT '',
    "contract_version" VARCHAR(32) NOT NULL DEFAULT '',
    "declaration_digest" CHAR(64) NOT NULL DEFAULT '',
    "ongoing_management" BOOLEAN NOT NULL DEFAULT FALSE,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'awaiting_settlement',
    "started_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_product_delivery_order_item UNIQUE ("order_item_no"),
    CONSTRAINT ck_idc_product_delivery_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_delivery_quantity CHECK ("quantity" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_product_delivery_strategy CHECK (
        "strategy_code" IN ('manual','approval_then_plugin','payment_confirmed_plugin')
    ),
    CONSTRAINT ck_idc_product_delivery_status CHECK (
        "status_code" IN ('awaiting_settlement','review_pending','queued','processing','manual_pending','completed','incomplete','canceled')
    ),
    CONSTRAINT ck_idc_product_delivery_digests CHECK (
        "policy_digest" ~ '^[0-9a-f]{64}$' AND "content_digest" ~ '^[0-9a-f]{64}$' AND
        ("declaration_digest" = '' OR "declaration_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_product_delivery_plugin_evidence CHECK (
        ("strategy_code" = 'manual' AND "interface_instance_no" = '' AND "plugin_id" = '' AND
         "plugin_release_version" = '' AND "contract_version" = '' AND "declaration_digest" = '' AND
         "ongoing_management" = FALSE) OR
        ("strategy_code" <> 'manual' AND "interface_instance_no" <> '' AND "plugin_id" <> '' AND
         "plugin_release_version" <> '' AND "contract_version" <> '' AND "declaration_digest" <> '')
    ),
    CONSTRAINT ck_idc_product_delivery_time CHECK (
        ("completed_at" IS NULL AND "status_code" NOT IN ('completed','incomplete','canceled')) OR
        ("completed_at" IS NOT NULL AND "status_code" IN ('completed','incomplete','canceled'))
    ),
    CONSTRAINT fk_idc_product_delivery_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_delivery_order_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item("order_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_delivery_product FOREIGN KEY ("product_no")
        REFERENCES plugin_idc_core_catalog_product("product_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_delivery_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_product_delivery_customer_status
    ON plugin_idc_core_product_delivery_record("tenant_id", "customer_no", "status_code", "id" DESC);

CREATE INDEX IF NOT EXISTS idx_idc_product_delivery_order
    ON plugin_idc_core_product_delivery_record("tenant_id", "order_no", "order_item_no");

-- Purpose: Stores Service Entitlement records.
-- 用途：保存服务权益相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_entitlement (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "id" BIGSERIAL UNIQUE,
    "entitlement_no" VARCHAR(40) PRIMARY KEY,
    "delivery_no" VARCHAR(40) NOT NULL,
    "source_intent_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "unit_sequence" INT NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL DEFAULT '',
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "plugin_id" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "contract_version" VARCHAR(32) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'active',
    "started_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_service_entitlement_intent UNIQUE ("source_intent_no"),
    CONSTRAINT ck_idc_service_entitlement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_entitlement_unit CHECK ("unit_sequence" BETWEEN 1 AND 1000),
    CONSTRAINT ck_idc_service_entitlement_evidence CHECK (
        "interface_instance_no" <> '' AND "plugin_id" <> '' AND "plugin_release_version" <> '' AND
        "contract_version" <> '' AND "declaration_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_service_entitlement_status CHECK (
        "status_code" IN ('pending_activation','active','suspended','expired','terminated','cancelled')
    ),
    CONSTRAINT ck_idc_service_entitlement_time CHECK ("expires_at" IS NULL OR "expires_at" > "started_at"),
    CONSTRAINT fk_idc_service_entitlement_delivery FOREIGN KEY ("delivery_no")
        REFERENCES plugin_idc_core_product_delivery_record("delivery_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_entitlement_intent FOREIGN KEY ("source_intent_no")
        REFERENCES plugin_idc_core_product_fulfillment_intent("intent_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_entitlement_customer_status
    ON plugin_idc_core_service_entitlement("tenant_id", "customer_no", "status_code", "id" DESC);

CREATE INDEX IF NOT EXISTS idx_idc_service_entitlement_order
    ON plugin_idc_core_service_entitlement("tenant_id", "order_no", "order_item_no");

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_order_item_delivery_identity
    ON plugin_idc_core_commerce_order_item("tenant_id","order_item_no","order_no","product_no","version_no");
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_order_delivery_customer
    ON plugin_idc_core_commerce_order("tenant_id","order_no","customer_no");
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_delivery_entitlement_identity
    ON plugin_idc_core_product_delivery_record(
        "tenant_id","delivery_no","order_no","order_item_no","customer_no","product_no","version_no"
    );
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_intent_entitlement_identity
    ON plugin_idc_core_product_fulfillment_intent(
        "tenant_id","intent_no","order_no","order_item_no","customer_no","product_no","version_no","unit_sequence"
    );

DO $constraint$ BEGIN
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_delivery_record'::regclass AND conname='fk_idc_product_delivery_order_item_identity') THEN
    ALTER TABLE plugin_idc_core_product_delivery_record
      ADD CONSTRAINT fk_idc_product_delivery_order_item_identity
      FOREIGN KEY("tenant_id","order_item_no","order_no","product_no","version_no")
      REFERENCES plugin_idc_core_commerce_order_item("tenant_id","order_item_no","order_no","product_no","version_no")
      ON DELETE RESTRICT;
END IF;
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_product_delivery_record'::regclass AND conname='fk_idc_product_delivery_customer_identity') THEN
    ALTER TABLE plugin_idc_core_product_delivery_record
      ADD CONSTRAINT fk_idc_product_delivery_customer_identity
      FOREIGN KEY("tenant_id","order_no","customer_no")
      REFERENCES plugin_idc_core_commerce_order("tenant_id","order_no","customer_no")
      ON DELETE RESTRICT;
END IF;
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_service_entitlement'::regclass AND conname='fk_idc_service_entitlement_delivery_identity') THEN
    ALTER TABLE plugin_idc_core_service_entitlement
      ADD CONSTRAINT fk_idc_service_entitlement_delivery_identity
      FOREIGN KEY("tenant_id","delivery_no","order_no","order_item_no","customer_no","product_no","version_no")
      REFERENCES plugin_idc_core_product_delivery_record("tenant_id","delivery_no","order_no","order_item_no","customer_no","product_no","version_no")
      ON DELETE RESTRICT;
END IF;
IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='plugin_idc_core_service_entitlement'::regclass AND conname='fk_idc_service_entitlement_intent_identity') THEN
    ALTER TABLE plugin_idc_core_service_entitlement
      ADD CONSTRAINT fk_idc_service_entitlement_intent_identity
      FOREIGN KEY("tenant_id","source_intent_no","order_no","order_item_no","customer_no","product_no","version_no","unit_sequence")
      REFERENCES plugin_idc_core_product_fulfillment_intent("tenant_id","intent_no","order_no","order_item_no","customer_no","product_no","version_no","unit_sequence")
      ON DELETE RESTRICT;
END IF;
END $constraint$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reconcile_product_delivery()
RETURNS TRIGGER AS $$
DECLARE
    target_order_item_no VARCHAR(40);
    target_delivery_no VARCHAR(40);
    target_status VARCHAR(32);
    intent_count INT;
    terminal_count INT;
    completed_count INT;
    incomplete_count INT;
    canceled_count INT;
BEGIN
    target_order_item_no := COALESCE(NEW."order_item_no", OLD."order_item_no");
    SELECT "delivery_no" INTO target_delivery_no
    FROM plugin_idc_core_product_delivery_record
    WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no
    FOR UPDATE;
    IF target_delivery_no IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT count(*),
           count(*) FILTER (WHERE "status_code" IN ('completed','incomplete','canceled')),
           count(*) FILTER (WHERE "status_code"='completed'),
           count(*) FILTER (WHERE "status_code"='incomplete'),
           count(*) FILTER (WHERE "status_code"='canceled')
      INTO intent_count,terminal_count,completed_count,incomplete_count,canceled_count
    FROM plugin_idc_core_product_fulfillment_intent
    WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no;

    IF intent_count > 0 AND completed_count = intent_count THEN
        target_status := 'completed';
    ELSIF intent_count > 0 AND canceled_count = intent_count THEN
        target_status := 'canceled';
    ELSIF intent_count > 0 AND terminal_count = intent_count THEN
        target_status := 'incomplete';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='manual_pending') THEN
        target_status := 'manual_pending';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='processing') THEN
        target_status := 'processing';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='review_pending') THEN
        target_status := 'review_pending';
    ELSE
        target_status := 'queued';
    END IF;

    UPDATE plugin_idc_core_product_delivery_record
    SET "status_code"=target_status,
        "completed_at"=CASE WHEN target_status IN ('completed','incomplete','canceled') THEN COALESCE("completed_at",CURRENT_TIMESTAMP) ELSE NULL END,
        "version"=CASE WHEN "status_code" IS DISTINCT FROM target_status THEN "version"+1 ELSE "version" END,
        "updated_at"=CASE WHEN "status_code" IS DISTINCT FROM target_status THEN CURRENT_TIMESTAMP ELSE "updated_at" END
    WHERE "delivery_no"=target_delivery_no;

    INSERT INTO plugin_idc_core_service_entitlement(
            "tenant_id","entitlement_no","delivery_no","source_intent_no","customer_no","order_no","order_item_no",
            "product_no","version_no","unit_sequence","configuration_snapshot_no","interface_instance_no",
            "plugin_id","plugin_release_version","contract_version","declaration_digest","status_code","started_at","version"
        )
        SELECT 0,'ent_'||substr(md5('entitlement:'||intent."intent_no"),1,32),target_delivery_no,intent."intent_no",
               intent."customer_no",intent."order_no",intent."order_item_no",intent."product_no",intent."version_no",
               intent."unit_sequence",intent."configuration_snapshot_no",intent."interface_instance_no",
               intent."plugin_id",intent."plugin_release_version",intent."contract_version",intent."declaration_digest",
               CASE intent."status_code" WHEN 'completed' THEN 'active' WHEN 'canceled' THEN 'cancelled' ELSE 'pending_activation' END,
               CURRENT_TIMESTAMP,1
        FROM plugin_idc_core_product_fulfillment_intent intent
        WHERE intent."tenant_id"=0 AND intent."order_item_no"=target_order_item_no
          AND intent."status_code" IN ('processing','completed','canceled') AND intent."ongoing_management"
        ON CONFLICT("source_intent_no") DO UPDATE
        SET "status_code"=CASE EXCLUDED."status_code"
                WHEN 'active' THEN 'active'
                WHEN 'cancelled' THEN 'cancelled'
                ELSE plugin_idc_core_service_entitlement."status_code"
            END,
            "version"=CASE WHEN plugin_idc_core_service_entitlement."status_code" IS DISTINCT FROM EXCLUDED."status_code"
                           AND EXCLUDED."status_code" IN ('active','cancelled')
                           THEN plugin_idc_core_service_entitlement."version"+1 ELSE plugin_idc_core_service_entitlement."version" END,
            "updated_at"=CASE WHEN plugin_idc_core_service_entitlement."status_code" IS DISTINCT FROM EXCLUDED."status_code"
                              AND EXCLUDED."status_code" IN ('active','cancelled')
                              THEN CURRENT_TIMESTAMP ELSE plugin_idc_core_service_entitlement."updated_at" END;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_reconcile_product_delivery
    ON plugin_idc_core_product_fulfillment_intent;
CREATE TRIGGER trg_idc_reconcile_product_delivery
AFTER INSERT OR UPDATE OF "status_code" ON plugin_idc_core_product_fulfillment_intent
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reconcile_product_delivery();
