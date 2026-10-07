-- 084: General product-version cost settings and order-time cost snapshots.
-- Product costs belong to a published product version and never require a
-- server resource-pool binding. The legacy binding source remains a temporary
-- read fallback until the server contract migration is complete.

-- Purpose: Effective unit-cost settings for any product version, independent of product kind or provider binding.
-- 用途：保存目录商品版本成本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_cost (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cost_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "unit_cost_minor" BIGINT NOT NULL,
    "effective_from" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "effective_until" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_version_cost_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_version_cost_dimensions CHECK (
        "currency_code" ~ '^[A-Z]{3}$' AND
        "period_unit_code" IN ('once', 'hour', 'day', 'month', 'year') AND
        "period_count" BETWEEN 1 AND 87600 AND "unit_cost_minor" >= 0
    ),
    CONSTRAINT ck_idc_product_version_cost_window CHECK (
        "effective_until" IS NULL OR "effective_until" > "effective_from"
    ),
    CONSTRAINT ck_idc_product_version_cost_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_product_version_cost_no UNIQUE ("cost_no"),
    CONSTRAINT uk_idc_product_version_cost_effective UNIQUE (
        "version_no", "currency_code", "period_unit_code", "period_count", "effective_from"
    ),
    CONSTRAINT fk_idc_product_version_cost_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_catalog_product_version_cost
    ADD COLUMN IF NOT EXISTS "cost_no" VARCHAR(40);
UPDATE plugin_idc_core_catalog_product_version_cost
SET "cost_no" = 'cst_' || md5('product-version-cost:' || "id"::TEXT)
WHERE "cost_no" IS NULL;
ALTER TABLE plugin_idc_core_catalog_product_version_cost
    ALTER COLUMN "cost_no" SET NOT NULL;
DO $$ BEGIN
IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid='plugin_idc_core_catalog_product_version_cost'::regclass
      AND conname='uk_idc_product_version_cost_no'
) THEN
    ALTER TABLE plugin_idc_core_catalog_product_version_cost
        ADD CONSTRAINT uk_idc_product_version_cost_no UNIQUE("cost_no");
END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_idc_product_version_cost_lookup
    ON plugin_idc_core_catalog_product_version_cost (
        "tenant_id", "version_no", "currency_code", "period_unit_code", "period_count",
        "effective_from" DESC, "id" DESC
    );

DROP TRIGGER IF EXISTS trg_idc_core_capture_order_cost
    ON plugin_idc_core_commerce_order_item;

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    ADD COLUMN IF NOT EXISTS "version_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "quantity" INT,
    ADD COLUMN IF NOT EXISTS "product_version_cost_id" BIGINT,
    ADD COLUMN IF NOT EXISTS "product_version_cost_version" BIGINT;

UPDATE plugin_idc_core_commerce_order_cost_snapshot AS cost
SET "version_no" = item."version_no",
    "quantity" = COALESCE(quantity_snapshot."quantity", cost."server_quantity")
FROM plugin_idc_core_commerce_order_item AS item
LEFT JOIN plugin_idc_core_commerce_order_item_quantity_snapshot AS quantity_snapshot
  ON quantity_snapshot."tenant_id" = item."tenant_id"
 AND quantity_snapshot."order_item_no" = item."order_item_no"
WHERE cost."tenant_id" = item."tenant_id"
  AND cost."order_no" = item."order_no"
  AND cost."order_item_no" = item."order_item_no"
  AND (cost."version_no" IS NULL OR cost."quantity" IS NULL);

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    ALTER COLUMN "version_no" SET NOT NULL,
    ALTER COLUMN "quantity" SET NOT NULL,
    ALTER COLUMN "binding_no" DROP NOT NULL;

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_order_cost_dimensions,
    DROP CONSTRAINT IF EXISTS ck_idc_order_cost_state,
    DROP CONSTRAINT IF EXISTS fk_idc_order_cost_source,
    DROP CONSTRAINT IF EXISTS fk_idc_order_cost_version,
    DROP CONSTRAINT IF EXISTS fk_idc_order_cost_product_version_source;

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    ADD CONSTRAINT ck_idc_order_cost_dimensions CHECK (
        "currency_code" ~ '^[A-Z]{3}$' AND
        "period_unit_code" IN ('once', 'hour', 'day', 'month', 'year') AND
        "period_count" BETWEEN 1 AND 87600 AND "quantity" BETWEEN 1 AND 1000 AND
        "purchase_cycles" BETWEEN 1 AND 120 AND "server_quantity" = "quantity"
    ),
    ADD CONSTRAINT ck_idc_order_cost_state CHECK (
        ("status_code" = 'missing' AND "cost_snapshot_no" IS NULL AND "product_version_cost_id" IS NULL AND
            "product_version_cost_version" IS NULL AND "unit_cost_minor" IS NULL AND "total_cost_minor" IS NULL AND
            "source_code" IS NULL AND "source_observed_at" IS NULL AND "correction_request_no" IS NULL AND
            "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND "corrected_by" IS NULL AND
            "corrected_at" IS NULL) OR
        ("status_code" = 'frozen' AND
            (("product_version_cost_id" IS NOT NULL AND "product_version_cost_version" > 0 AND "cost_snapshot_no" IS NULL AND
                "binding_no" IS NULL AND "source_code" = 'product_version_setting') OR
             ("product_version_cost_id" IS NULL AND "product_version_cost_version" IS NULL AND "cost_snapshot_no" IS NOT NULL AND
                "binding_no" IS NOT NULL AND "source_code" IN ('manual', 'provider'))) AND
            "unit_cost_minor" >= 0 AND
            "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "quantity" * "purchase_cycles" AND
            "source_observed_at" IS NOT NULL AND "correction_request_no" IS NULL AND
            "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND "corrected_by" IS NULL AND
            "corrected_at" IS NULL) OR
        ("status_code" = 'corrected' AND "cost_snapshot_no" IS NULL AND "product_version_cost_id" IS NULL AND
            "product_version_cost_version" IS NULL AND "unit_cost_minor" >= 0 AND
            "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "quantity" * "purchase_cycles" AND
            "source_code" = 'historical_correction' AND "source_observed_at" IS NOT NULL AND
            length(btrim("correction_request_no")) BETWEEN 8 AND 64 AND
            "correction_request_digest" ~ '^[0-9a-f]{64}$' AND
            length(btrim("correction_reason")) BETWEEN 2 AND 500 AND "corrected_by" > 0 AND "corrected_at" IS NOT NULL)
    ),
    ADD CONSTRAINT fk_idc_order_cost_source FOREIGN KEY ("cost_snapshot_no")
        REFERENCES plugin_idc_core_catalog_cost_snapshot("cost_snapshot_no") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_order_cost_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version("version_no") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_order_cost_product_version_source FOREIGN KEY ("product_version_cost_id")
        REFERENCES plugin_idc_core_catalog_product_version_cost("id") ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_idc_order_cost_version
    ON plugin_idc_core_commerce_order_cost_snapshot (
        "tenant_id", "version_no", "currency_code", "status_code", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_order_cost_dimensions()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'order cost evidence cannot be deleted';
    END IF;
    IF ROW(NEW."tenant_id", NEW."order_item_no", NEW."order_no", NEW."version_no",
           NEW."binding_no", NEW."currency_code", NEW."period_unit_code", NEW."period_count",
           NEW."quantity", NEW."server_quantity", NEW."purchase_cycles", NEW."cost_snapshot_no",
           NEW."product_version_cost_id", NEW."product_version_cost_version", NEW."created_at")
       IS DISTINCT FROM
       ROW(OLD."tenant_id", OLD."order_item_no", OLD."order_no", OLD."version_no",
           OLD."binding_no", OLD."currency_code", OLD."period_unit_code", OLD."period_count",
           OLD."quantity", OLD."server_quantity", OLD."purchase_cycles", OLD."cost_snapshot_no",
           OLD."product_version_cost_id", OLD."product_version_cost_version", OLD."created_at") THEN
        RAISE EXCEPTION 'order cost dimensions and source identity are immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_guard_order_cost_dimensions
    ON plugin_idc_core_commerce_order_cost_snapshot;
CREATE TRIGGER trg_idc_guard_order_cost_dimensions
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_order_cost_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_order_cost_dimensions();

CREATE OR REPLACE FUNCTION plugin_idc_core_capture_order_cost()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    order_item plugin_idc_core_commerce_order_item%ROWTYPE;
    product_cost plugin_idc_core_catalog_product_version_cost%ROWTYPE;
    legacy_cost plugin_idc_core_catalog_cost_snapshot%ROWTYPE;
    computed_total NUMERIC;
BEGIN
    SELECT * INTO order_item
    FROM plugin_idc_core_commerce_order_item item
    WHERE item."tenant_id" = NEW."tenant_id"
      AND item."order_no" = NEW."order_no"
      AND item."order_item_no" = NEW."order_item_no";

    IF order_item."id" IS NULL THEN
        RAISE EXCEPTION 'order item is missing for generic cost capture';
    END IF;

    SELECT * INTO product_cost
    FROM plugin_idc_core_catalog_product_version_cost cost
    WHERE cost."tenant_id" = NEW."tenant_id"
      AND cost."version_no" = order_item."version_no"
      AND cost."currency_code" = order_item."currency_code"
      AND cost."period_unit_code" = order_item."period_unit_code"
      AND cost."period_count" = order_item."period_count"
      AND cost."effective_from" <= order_item."created_at"
      AND (cost."effective_until" IS NULL OR cost."effective_until" > order_item."created_at")
    ORDER BY cost."effective_from" DESC, cost."id" DESC
    LIMIT 1;

    IF product_cost."id" IS NULL AND NULLIF(order_item."initial_binding_no", '') IS NOT NULL THEN
        SELECT * INTO legacy_cost
        FROM plugin_idc_core_catalog_cost_snapshot cost
        WHERE cost."tenant_id" = NEW."tenant_id"
          AND cost."binding_no" = order_item."initial_binding_no"
          AND cost."currency_code" = order_item."currency_code"
          AND cost."period_unit_code" = order_item."period_unit_code"
          AND cost."period_count" = order_item."period_count"
          AND cost."observed_at" <= order_item."created_at"
          AND (cost."valid_until" IS NULL OR cost."valid_until" > order_item."created_at")
        ORDER BY cost."observed_at" DESC, cost."id" DESC
        LIMIT 1;
    END IF;

    IF product_cost."id" IS NULL AND legacy_cost."id" IS NULL THEN
        INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (
            "tenant_id", "order_item_no", "order_no", "version_no", "binding_no", "currency_code",
            "period_unit_code", "period_count", "quantity", "server_quantity", "purchase_cycles",
            "status_code", "created_at", "updated_at"
        ) VALUES (
            NEW."tenant_id", NEW."order_item_no", NEW."order_no", order_item."version_no", NULL,
            order_item."currency_code", order_item."period_unit_code", order_item."period_count",
            NEW."quantity", NEW."quantity", order_item."purchase_cycles", 'missing',
            order_item."created_at", order_item."created_at"
        ) ON CONFLICT ("order_item_no") DO NOTHING;
        RETURN NEW;
    END IF;

    IF product_cost."id" IS NOT NULL THEN
        computed_total := product_cost."unit_cost_minor"::NUMERIC * NEW."quantity" * order_item."purchase_cycles";
    ELSE
        computed_total := legacy_cost."amount_minor"::NUMERIC * NEW."quantity" * order_item."purchase_cycles";
    END IF;
    IF computed_total > 9223372036854775807 THEN
        RAISE EXCEPTION 'order cost snapshot exceeds BIGINT range';
    END IF;

    IF product_cost."id" IS NOT NULL THEN
        INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (
            "tenant_id", "order_item_no", "order_no", "version_no", "binding_no", "currency_code",
            "period_unit_code", "period_count", "quantity", "server_quantity", "purchase_cycles",
            "status_code", "product_version_cost_id", "product_version_cost_version",
            "unit_cost_minor", "total_cost_minor", "source_code", "source_observed_at",
            "created_at", "updated_at"
        ) VALUES (
            NEW."tenant_id", NEW."order_item_no", NEW."order_no", order_item."version_no", NULL,
            order_item."currency_code", order_item."period_unit_code", order_item."period_count",
            NEW."quantity", NEW."quantity", order_item."purchase_cycles", 'frozen',
            product_cost."id", product_cost."version", product_cost."unit_cost_minor",
            computed_total::BIGINT, 'product_version_setting', product_cost."effective_from",
            order_item."created_at", order_item."created_at"
        ) ON CONFLICT ("order_item_no") DO NOTHING;
    ELSE
        INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (
            "tenant_id", "order_item_no", "order_no", "version_no", "binding_no", "currency_code",
            "period_unit_code", "period_count", "quantity", "server_quantity", "purchase_cycles",
            "status_code", "cost_snapshot_no", "unit_cost_minor", "total_cost_minor",
            "source_code", "source_observed_at", "created_at", "updated_at"
        ) VALUES (
            NEW."tenant_id", NEW."order_item_no", NEW."order_no", order_item."version_no",
            order_item."initial_binding_no", order_item."currency_code",
            order_item."period_unit_code", order_item."period_count", NEW."quantity", NEW."quantity",
            order_item."purchase_cycles", 'frozen', legacy_cost."cost_snapshot_no",
            legacy_cost."amount_minor", computed_total::BIGINT, legacy_cost."source_code",
            legacy_cost."observed_at", order_item."created_at", order_item."created_at"
        ) ON CONFLICT ("order_item_no") DO NOTHING;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_core_capture_order_cost
    ON plugin_idc_core_commerce_order_item_quantity_snapshot;
CREATE TRIGGER trg_idc_core_capture_order_cost
AFTER INSERT ON plugin_idc_core_commerce_order_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_capture_order_cost();

COMMENT ON TABLE plugin_idc_core_catalog_product_version_cost
    IS 'Effective unit-cost settings for any product version, independent of product kind or provider binding.';
COMMENT ON TABLE plugin_idc_core_commerce_order_cost_snapshot
    IS 'Immutable order-time cost evidence using the generic product version, quantity and purchase cycles.';
