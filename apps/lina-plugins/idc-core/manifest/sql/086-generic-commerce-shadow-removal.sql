-- 086: Remove the temporary server-shaped commerce shadows after generic
-- quantity, inventory, delivery, entitlement and product-cost evidence became
-- authoritative. Historical server records remain in their owner tables for
-- the server-plugin migration; new commerce writes no longer depend on them.

DROP TRIGGER IF EXISTS trg_idc_validate_cart_item_quantity
    ON plugin_idc_core_commerce_cart_item_quantity;
DROP TRIGGER IF EXISTS trg_idc_validate_assisted_item_quantity
    ON plugin_idc_core_commerce_assisted_quote_item_quantity;
DROP TRIGGER IF EXISTS trg_idc_validate_quote_item_quantity
    ON plugin_idc_core_commerce_quote_item_quantity_snapshot;
DROP TRIGGER IF EXISTS trg_idc_validate_order_item_quantity
    ON plugin_idc_core_commerce_order_item_quantity_snapshot;
DROP TRIGGER IF EXISTS trg_idc_check_cart_item_legacy_quantity
    ON plugin_idc_core_commerce_cart_item;
DROP TRIGGER IF EXISTS trg_idc_check_assisted_item_legacy_quantity
    ON plugin_idc_core_commerce_assisted_quote_request_item;
DROP TRIGGER IF EXISTS trg_idc_check_quote_item_legacy_quantity
    ON plugin_idc_core_commerce_quote_item_snapshot;
DROP TRIGGER IF EXISTS trg_idc_check_order_item_legacy_quantity
    ON plugin_idc_core_commerce_order_item;

DROP FUNCTION IF EXISTS plugin_idc_core_validate_generic_quantity_shadow();
DROP FUNCTION IF EXISTS plugin_idc_core_validate_legacy_quantity_shadow();

ALTER TABLE plugin_idc_core_commerce_cart_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_cart_item_quantity;
ALTER TABLE plugin_idc_core_commerce_cart_item
    DROP COLUMN IF EXISTS "server_quantity";
ALTER TABLE plugin_idc_core_commerce_cart_item
    ADD CONSTRAINT ck_idc_com_cart_item_quantity CHECK ("purchase_cycles" BETWEEN 1 AND 120);

ALTER TABLE plugin_idc_core_commerce_assisted_quote_request_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_assisted_item_line;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request_item
    DROP COLUMN IF EXISTS "server_quantity";
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request_item
    ADD CONSTRAINT ck_idc_com_assisted_item_line CHECK (
        "line_number" BETWEEN 1 AND 8 AND "purchase_cycles" BETWEEN 1 AND 120
    );

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_line,
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_product,
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_inventory,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_item_binding,
    DROP CONSTRAINT IF EXISTS fk_idc_com_quote_item_inventory;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP COLUMN IF EXISTS "product_type_code",
    DROP COLUMN IF EXISTS "server_quantity",
    DROP COLUMN IF EXISTS "binding_no",
    DROP COLUMN IF EXISTS "inventory_snapshot_no";
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD CONSTRAINT ck_idc_com_quote_item_line CHECK (
        "line_number" BETWEEN 1 AND 1000 AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    ADD CONSTRAINT ck_idc_com_quote_item_inventory CHECK (
        "inventory_decision_code" IN ('available','manual_confirmation')
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );

ALTER TABLE plugin_idc_core_service_instance
    DROP CONSTRAINT IF EXISTS fk_idc_service_order_item;
ALTER TABLE plugin_idc_core_commerce_order_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_line,
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_product,
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_policy,
    DROP CONSTRAINT IF EXISTS fk_idc_com_order_item_binding,
    DROP CONSTRAINT IF EXISTS fk_idc_com_order_item_inventory,
    DROP CONSTRAINT IF EXISTS uk_idc_com_order_item_service_snapshot;
ALTER TABLE plugin_idc_core_commerce_order_item
    DROP COLUMN IF EXISTS "product_type_code",
    DROP COLUMN IF EXISTS "server_quantity",
    DROP COLUMN IF EXISTS "initial_binding_no",
    DROP COLUMN IF EXISTS "inventory_snapshot_no";
ALTER TABLE plugin_idc_core_commerce_order_item
    ADD CONSTRAINT ck_idc_com_order_item_line CHECK (
        "line_number" BETWEEN 1 AND 1000 AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    ADD CONSTRAINT ck_idc_com_order_item_policy CHECK (
        "fulfillment_mode_code" IN ('automatic','manual_review')
        AND "risk_decision_code" IN ('approved','manual_review')
        AND "inventory_decision_code" IN ('available','manual_confirmation')
        AND "route_decision_digest" ~ '^[0-9a-f]{64}$'
    );
ALTER TABLE plugin_idc_core_service_instance
    ADD CONSTRAINT fk_idc_service_order_item FOREIGN KEY ("order_no","order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item("order_no","order_item_no")
        ON DELETE RESTRICT;

DROP TRIGGER IF EXISTS trg_idc_guard_order_cost_dimensions
    ON plugin_idc_core_commerce_order_cost_snapshot;
DROP TRIGGER IF EXISTS trg_idc_core_capture_order_cost
    ON plugin_idc_core_commerce_order_item_quantity_snapshot;
DROP FUNCTION IF EXISTS plugin_idc_core_guard_order_cost_dimensions();
DROP FUNCTION IF EXISTS plugin_idc_core_capture_order_cost();

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_order_cost_dimensions,
    DROP CONSTRAINT IF EXISTS ck_idc_order_cost_state;
ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    DROP COLUMN IF EXISTS "binding_no",
    DROP COLUMN IF EXISTS "server_quantity";
ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    ADD CONSTRAINT ck_idc_order_cost_dimensions CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "period_unit_code" IN ('once','hour','day','month','year')
        AND "period_count" BETWEEN 1 AND 87600
        AND "quantity" BETWEEN 1 AND 1000
        AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    ADD CONSTRAINT ck_idc_order_cost_state CHECK (
        ("status_code"='missing' AND "cost_snapshot_no" IS NULL AND "product_version_cost_id" IS NULL
            AND "product_version_cost_version" IS NULL AND "unit_cost_minor" IS NULL AND "total_cost_minor" IS NULL
            AND "source_code" IS NULL AND "source_observed_at" IS NULL AND "correction_request_no" IS NULL
            AND "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND "corrected_by" IS NULL
            AND "corrected_at" IS NULL)
        OR ("status_code"='frozen' AND (
                ("product_version_cost_id" IS NOT NULL AND "product_version_cost_version" > 0
                    AND "cost_snapshot_no" IS NULL AND "source_code"='product_version_setting')
                OR ("product_version_cost_id" IS NULL AND "product_version_cost_version" IS NULL
                    AND "cost_snapshot_no" IS NOT NULL AND "source_code" IN ('manual','provider'))
            ) AND "unit_cost_minor" >= 0
            AND "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "quantity" * "purchase_cycles"
            AND "source_observed_at" IS NOT NULL AND "correction_request_no" IS NULL
            AND "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND "corrected_by" IS NULL
            AND "corrected_at" IS NULL)
        OR ("status_code"='corrected' AND "cost_snapshot_no" IS NULL AND "product_version_cost_id" IS NULL
            AND "product_version_cost_version" IS NULL AND "unit_cost_minor" >= 0
            AND "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "quantity" * "purchase_cycles"
            AND "source_code"='historical_correction' AND "source_observed_at" IS NOT NULL
            AND length(btrim("correction_request_no")) BETWEEN 8 AND 64
            AND "correction_request_digest" ~ '^[0-9a-f]{64}$'
            AND length(btrim("correction_reason")) BETWEEN 2 AND 500
            AND "corrected_by" > 0 AND "corrected_at" IS NOT NULL)
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_order_cost_dimensions()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'order cost evidence cannot be deleted';
    END IF;
    IF ROW(NEW."tenant_id",NEW."order_item_no",NEW."order_no",NEW."version_no",
           NEW."currency_code",NEW."period_unit_code",NEW."period_count",NEW."quantity",
           NEW."purchase_cycles",NEW."cost_snapshot_no",NEW."product_version_cost_id",
           NEW."product_version_cost_version",NEW."created_at")
       IS DISTINCT FROM
       ROW(OLD."tenant_id",OLD."order_item_no",OLD."order_no",OLD."version_no",
           OLD."currency_code",OLD."period_unit_code",OLD."period_count",OLD."quantity",
           OLD."purchase_cycles",OLD."cost_snapshot_no",OLD."product_version_cost_id",
           OLD."product_version_cost_version",OLD."created_at") THEN
        RAISE EXCEPTION 'order cost dimensions and source identity are immutable';
    END IF;
    RETURN NEW;
END;
$$;

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
    computed_total NUMERIC;
BEGIN
    SELECT * INTO order_item FROM plugin_idc_core_commerce_order_item item
    WHERE item."tenant_id"=NEW."tenant_id" AND item."order_no"=NEW."order_no"
      AND item."order_item_no"=NEW."order_item_no";
    IF order_item."id" IS NULL THEN
        RAISE EXCEPTION 'order item is missing for generic cost capture';
    END IF;
    SELECT * INTO product_cost FROM plugin_idc_core_catalog_product_version_cost cost
    WHERE cost."tenant_id"=NEW."tenant_id" AND cost."version_no"=order_item."version_no"
      AND cost."currency_code"=order_item."currency_code"
      AND cost."period_unit_code"=order_item."period_unit_code"
      AND cost."period_count"=order_item."period_count"
      AND cost."effective_from"<=order_item."created_at"
      AND (cost."effective_until" IS NULL OR cost."effective_until">order_item."created_at")
    ORDER BY cost."effective_from" DESC,cost."id" DESC LIMIT 1;
    IF product_cost."id" IS NULL THEN
        INSERT INTO plugin_idc_core_commerce_order_cost_snapshot(
            "tenant_id","order_item_no","order_no","version_no","currency_code","period_unit_code",
            "period_count","quantity","purchase_cycles","status_code","created_at","updated_at")
        VALUES(NEW."tenant_id",NEW."order_item_no",NEW."order_no",order_item."version_no",
            order_item."currency_code",order_item."period_unit_code",order_item."period_count",
            NEW."quantity",order_item."purchase_cycles",'missing',order_item."created_at",order_item."created_at")
        ON CONFLICT("order_item_no") DO NOTHING;
        RETURN NEW;
    END IF;
    computed_total:=product_cost."unit_cost_minor"::NUMERIC*NEW."quantity"*order_item."purchase_cycles";
    IF computed_total>9223372036854775807 THEN
        RAISE EXCEPTION 'order cost snapshot exceeds BIGINT range';
    END IF;
    INSERT INTO plugin_idc_core_commerce_order_cost_snapshot(
        "tenant_id","order_item_no","order_no","version_no","currency_code","period_unit_code",
        "period_count","quantity","purchase_cycles","status_code","product_version_cost_id",
        "product_version_cost_version","unit_cost_minor","total_cost_minor","source_code",
        "source_observed_at","created_at","updated_at")
    VALUES(NEW."tenant_id",NEW."order_item_no",NEW."order_no",order_item."version_no",
        order_item."currency_code",order_item."period_unit_code",order_item."period_count",
        NEW."quantity",order_item."purchase_cycles",'frozen',product_cost."id",product_cost."version",
        product_cost."unit_cost_minor",computed_total::BIGINT,'product_version_setting',
        product_cost."effective_from",order_item."created_at",order_item."created_at")
    ON CONFLICT("order_item_no") DO NOTHING;
    RETURN NEW;
END;
$$;

CREATE TRIGGER trg_idc_core_capture_order_cost
AFTER INSERT ON plugin_idc_core_commerce_order_item_quantity_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_capture_order_cost();

COMMENT ON TABLE plugin_idc_core_commerce_order_cost_snapshot
    IS 'Immutable generic order-item cost evidence without server quantity or resource-pool fields.';
