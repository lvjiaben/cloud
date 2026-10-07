-- 085: Bind trial qualification to a generic order-item quantity unit instead
-- of the retired server fulfillment unit. Existing development data is
-- migrated from the immutable fulfillment sequence before the old reference
-- is removed.

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

ALTER TABLE plugin_idc_core_trial_entitlement
    ADD COLUMN IF NOT EXISTS "unit_sequence" INT;

DO $backfill$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_core_trial_entitlement'
          AND column_name = 'fulfillment_no'
    ) THEN
        EXECUTE $sql$
            UPDATE plugin_idc_core_trial_entitlement entitlement
            SET "unit_sequence" = fulfillment."unit_sequence"
            FROM plugin_idc_core_commerce_fulfillment_unit fulfillment
            WHERE entitlement."unit_sequence" IS NULL
              AND fulfillment."tenant_id" = entitlement."tenant_id"
              AND fulfillment."fulfillment_no" = entitlement."fulfillment_no"
              AND fulfillment."order_no" = entitlement."order_no"
              AND fulfillment."order_item_no" = entitlement."order_item_no"
        $sql$;
    END IF;
END;
$backfill$;

DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1 FROM plugin_idc_core_trial_entitlement
        WHERE "unit_sequence" IS NULL
    ) THEN
        RAISE EXCEPTION 'trial entitlement unit migration is incomplete';
    END IF;
END;
$migration$;

ALTER TABLE plugin_idc_core_trial_entitlement
    ALTER COLUMN "unit_sequence" SET NOT NULL,
    DROP CONSTRAINT IF EXISTS fk_idc_trial_entitlement_fulfillment,
    DROP CONSTRAINT IF EXISTS uk_idc_trial_entitlement_server,
    DROP CONSTRAINT IF EXISTS uk_idc_trial_entitlement_owner_server,
    DROP CONSTRAINT IF EXISTS uk_idc_trial_entitlement_order_unit,
    DROP CONSTRAINT IF EXISTS uk_idc_trial_entitlement_owner_unit,
    DROP CONSTRAINT IF EXISTS ck_idc_trial_entitlement_unit;

DROP INDEX IF EXISTS idx_idc_trial_entitlement_order;

ALTER TABLE plugin_idc_core_trial_entitlement
    ADD CONSTRAINT ck_idc_trial_entitlement_unit CHECK ("unit_sequence" BETWEEN 1 AND 1000),
    ADD CONSTRAINT uk_idc_trial_entitlement_order_unit UNIQUE ("order_item_no", "unit_sequence"),
    ADD CONSTRAINT uk_idc_trial_entitlement_owner_unit UNIQUE (
        "customer_no", "product_no", "order_item_no", "unit_sequence"
    );

CREATE INDEX IF NOT EXISTS idx_idc_trial_entitlement_order
    ON plugin_idc_core_trial_entitlement (
        "tenant_id", "order_no", "status_code", "order_item_no", "unit_sequence", "id"
    );

-- The generic unit is still bounded by the immutable order-item quantity.
-- Use a statement-level transition table so a bulk hold performs one indexed
-- validation query rather than one query per unit.
DO $existing_unit_guard$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_trial_entitlement entitlement
        LEFT JOIN plugin_idc_core_commerce_order_item_quantity_snapshot "quantity"
          ON "quantity"."order_item_no" = entitlement."order_item_no"
        WHERE "quantity"."order_item_no" IS NULL
           OR entitlement."unit_sequence" > "quantity"."quantity"
    ) THEN
        RAISE EXCEPTION 'trial entitlement unit exceeds the frozen order item quantity';
    END IF;
END;
$existing_unit_guard$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_trial_entitlement_units()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM changed_trial_entitlements entitlement
        LEFT JOIN plugin_idc_core_commerce_order_item_quantity_snapshot "quantity"
          ON "quantity"."order_item_no" = entitlement."order_item_no"
        WHERE "quantity"."order_item_no" IS NULL
           OR entitlement."unit_sequence" > "quantity"."quantity"
    ) THEN
        RAISE EXCEPTION 'trial entitlement unit exceeds the frozen order item quantity';
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_validate_trial_entitlement_units_insert
    ON plugin_idc_core_trial_entitlement;
CREATE TRIGGER trg_idc_validate_trial_entitlement_units_insert
AFTER INSERT ON plugin_idc_core_trial_entitlement
REFERENCING NEW TABLE AS changed_trial_entitlements
FOR EACH STATEMENT EXECUTE FUNCTION plugin_idc_core_validate_trial_entitlement_units();

DROP TRIGGER IF EXISTS trg_idc_validate_trial_entitlement_units_update
    ON plugin_idc_core_trial_entitlement;
CREATE TRIGGER trg_idc_validate_trial_entitlement_units_update
AFTER UPDATE ON plugin_idc_core_trial_entitlement
REFERENCING NEW TABLE AS changed_trial_entitlements
FOR EACH STATEMENT EXECUTE FUNCTION plugin_idc_core_validate_trial_entitlement_units();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_trial_entitlement_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'trial entitlement cannot be deleted';
    END IF;
    IF OLD."status_code" <> 'held'
       OR NEW."status_code" NOT IN ('consumed', 'released', 'expired')
       OR NEW."tenant_id" <> OLD."tenant_id"
       OR NEW."entitlement_no" <> OLD."entitlement_no"
       OR NEW."scope_no" <> OLD."scope_no"
       OR NEW."customer_no" <> OLD."customer_no"
       OR NEW."product_no" <> OLD."product_no"
       OR NEW."version_no" <> OLD."version_no"
       OR NEW."billing_rule_no" <> OLD."billing_rule_no"
       OR NEW."order_no" <> OLD."order_no"
       OR NEW."invoice_no" <> OLD."invoice_no"
       OR NEW."order_item_no" <> OLD."order_item_no"
       OR NEW."unit_sequence" <> OLD."unit_sequence"
       OR NEW."currency_code" <> OLD."currency_code"
       OR NEW."trial_limit_quantity" <> OLD."trial_limit_quantity"
       OR NEW."hold_expires_at" <> OLD."hold_expires_at"
       OR NEW."content_digest" <> OLD."content_digest"
       OR NEW."created_at" <> OLD."created_at"
       OR NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'trial entitlement mutation is invalid';
    END IF;
    RETURN NEW;
END;
$$;

ALTER TABLE plugin_idc_core_trial_entitlement
    DROP COLUMN IF EXISTS "fulfillment_no";

COMMENT ON COLUMN plugin_idc_core_trial_entitlement."unit_sequence"
    IS 'Generic one-based quantity unit inside the immutable order item; it has no server meaning.';
