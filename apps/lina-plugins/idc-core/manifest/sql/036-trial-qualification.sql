-- 036: Customer-and-product trial qualification scopes, one held entitlement
-- per server fulfillment identity, and append-only projection movements.

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD COLUMN IF NOT EXISTS "trial_limit_quantity" SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "prior_trial_uses" SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_trial;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD CONSTRAINT ck_idc_com_quote_item_trial CHECK (
        (
            "charge_type_code" = 'trial'
            AND "trial_limit_quantity" BETWEEN 1 AND 100
            AND "prior_trial_uses" BETWEEN 0 AND "trial_limit_quantity" - 1
            AND "purchase_cycles" = 1
            AND "setup_subtotal_minor" = 0
            AND "period_subtotal_minor" = 0
            AND "subtotal_minor" = 0
            AND "discount_minor" = 0
            AND "net_amount_minor" = 0
            AND "tax_amount_minor" = 0
            AND "total_amount_minor" = 0
        ) OR (
            "charge_type_code" <> 'trial'
            AND "trial_limit_quantity" = 0
            AND "prior_trial_uses" = 0
        )
    );

ALTER TABLE plugin_idc_core_commerce_order_item
    ADD COLUMN IF NOT EXISTS "trial_limit_quantity" SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "prior_trial_uses" SMALLINT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_core_commerce_order_item
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_item_trial;
ALTER TABLE plugin_idc_core_commerce_order_item
    ADD CONSTRAINT ck_idc_com_order_item_trial CHECK (
        (
            "charge_type_code" = 'trial'
            AND "trial_limit_quantity" BETWEEN 1 AND 100
            AND "prior_trial_uses" BETWEEN 0 AND "trial_limit_quantity" - 1
            AND "purchase_cycles" = 1
            AND "setup_subtotal_minor" = 0
            AND "period_subtotal_minor" = 0
            AND "subtotal_minor" = 0
            AND "discount_minor" = 0
            AND "net_amount_minor" = 0
            AND "tax_amount_minor" = 0
            AND "total_amount_minor" = 0
        ) OR (
            "charge_type_code" <> 'trial'
            AND "trial_limit_quantity" = 0
            AND "prior_trial_uses" = 0
        )
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_com_fulfillment_service_snapshot'
          AND conrelid = 'plugin_idc_core_commerce_fulfillment_unit'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
            ADD CONSTRAINT uk_idc_com_fulfillment_service_snapshot UNIQUE (
                "fulfillment_no", "order_no", "order_item_no"
            );
    END IF;
END;
$constraint$;

-- Purpose: Current customer-and-product trial usage projection serialized by checkout.
-- 用途：保存试用资格范围相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_trial_qualification_scope (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "scope_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "held_quantity" INT NOT NULL DEFAULT 0,
    "consumed_quantity" INT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_trial_scope_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_trial_scope_counts CHECK (
        "held_quantity" BETWEEN 0 AND 100
        AND "consumed_quantity" BETWEEN 0 AND 100
        AND "held_quantity" + "consumed_quantity" BETWEEN 0 AND 100
        AND "version" > 0
    ),
    CONSTRAINT uk_idc_trial_scope_no UNIQUE ("scope_no"),
    CONSTRAINT uk_idc_trial_scope_owner UNIQUE (
        "tenant_id", "customer_no", "product_no"
    ),
    CONSTRAINT uk_idc_trial_scope_reference UNIQUE (
        "scope_no", "customer_no", "product_no"
    ),
    CONSTRAINT fk_idc_trial_scope_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_idc_trial_scope_product FOREIGN KEY ("product_no")
        REFERENCES plugin_idc_core_catalog_product ("product_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_trial_scope_customer
    ON plugin_idc_core_trial_qualification_scope (
        "tenant_id", "customer_no", "product_no", "id"
    );

-- Purpose: One held or consumed trial entitlement per server fulfillment identity.
-- 用途：保存试用权益相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_trial_entitlement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "scope_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "fulfillment_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "trial_limit_quantity" SMALLINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'held',
    "hold_expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "identity_evidence_digest" CHAR(64),
    "content_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_trial_entitlement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_trial_entitlement_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "trial_limit_quantity" BETWEEN 1 AND 100
        AND "hold_expires_at" > "created_at"
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_trial_entitlement_status CHECK (
        "status_code" IN ('held', 'consumed', 'released', 'expired')
        AND (
            ("status_code" = 'held' AND "consumed_at" IS NULL
                AND "released_at" IS NULL
                AND "identity_evidence_digest" IS NULL)
            OR ("status_code" = 'consumed' AND "consumed_at" IS NOT NULL
                AND "released_at" IS NULL
                AND "identity_evidence_digest" ~ '^[0-9a-f]{64}$')
            OR ("status_code" IN ('released', 'expired')
                AND "consumed_at" IS NULL AND "released_at" IS NOT NULL
                AND "identity_evidence_digest" IS NULL)
        )
    ),
    CONSTRAINT uk_idc_trial_entitlement_no UNIQUE ("entitlement_no"),
    CONSTRAINT uk_idc_trial_entitlement_server UNIQUE ("fulfillment_no"),
    CONSTRAINT uk_idc_trial_entitlement_owner_server UNIQUE (
        "customer_no", "product_no", "fulfillment_no"
    ),
    CONSTRAINT fk_idc_trial_entitlement_scope FOREIGN KEY (
        "scope_no", "customer_no", "product_no"
    ) REFERENCES plugin_idc_core_trial_qualification_scope (
        "scope_no", "customer_no", "product_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_trial_entitlement_order_item FOREIGN KEY (
        "order_no", "order_item_no"
    ) REFERENCES plugin_idc_core_commerce_order_item (
        "order_no", "order_item_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_trial_entitlement_fulfillment FOREIGN KEY (
        "fulfillment_no", "order_no", "order_item_no"
    ) REFERENCES plugin_idc_core_commerce_fulfillment_unit (
        "fulfillment_no", "order_no", "order_item_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_trial_entitlement_invoice FOREIGN KEY (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_trial_entitlement_order
    ON plugin_idc_core_trial_entitlement (
        "tenant_id", "order_no", "status_code", "fulfillment_no", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_trial_entitlement_scope
    ON plugin_idc_core_trial_entitlement (
        "tenant_id", "scope_no", "status_code", "id"
    );

-- Purpose: Append-only trial qualification projection movement chain.
-- 用途：保存试用资格变动相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_trial_qualification_movement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "movement_no" VARCHAR(40) NOT NULL,
    "scope_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "movement_type_code" VARCHAR(16) NOT NULL,
    "quantity" INT NOT NULL,
    "held_before" INT NOT NULL,
    "held_after" INT NOT NULL,
    "consumed_before" INT NOT NULL,
    "consumed_after" INT NOT NULL,
    "scope_version_before" BIGINT NOT NULL,
    "scope_version_after" BIGINT NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_trial_movement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_trial_movement_values CHECK (
        "movement_type_code" IN ('hold', 'consume', 'release', 'expire')
        AND "quantity" BETWEEN 1 AND 100
        AND "held_before" BETWEEN 0 AND 100
        AND "held_after" BETWEEN 0 AND 100
        AND "consumed_before" BETWEEN 0 AND 100
        AND "consumed_after" BETWEEN 0 AND 100
        AND "held_before" + "consumed_before" BETWEEN 0 AND 100
        AND "held_after" + "consumed_after" BETWEEN 0 AND 100
        AND "scope_version_before" > 0
        AND "scope_version_after" = "scope_version_before" + 1
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (
            ("movement_type_code" = 'hold'
                AND "held_after" = "held_before" + "quantity"
                AND "consumed_after" = "consumed_before")
            OR ("movement_type_code" = 'consume'
                AND "held_after" = "held_before" - "quantity"
                AND "consumed_after" = "consumed_before" + "quantity")
            OR ("movement_type_code" IN ('release', 'expire')
                AND "held_after" = "held_before" - "quantity"
                AND "consumed_after" = "consumed_before")
        )
    ),
    CONSTRAINT uk_idc_trial_movement_no UNIQUE ("movement_no"),
    CONSTRAINT uk_idc_trial_movement_before UNIQUE (
        "scope_no", "scope_version_before"
    ),
    CONSTRAINT uk_idc_trial_movement_after UNIQUE (
        "scope_no", "scope_version_after"
    ),
    CONSTRAINT uk_idc_trial_movement_order_type UNIQUE (
        "scope_no", "order_no", "movement_type_code"
    ),
    CONSTRAINT fk_idc_trial_movement_scope FOREIGN KEY (
        "scope_no", "customer_no", "product_no"
    ) REFERENCES plugin_idc_core_trial_qualification_scope (
        "scope_no", "customer_no", "product_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_trial_movement_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_trial_movement_scope
    ON plugin_idc_core_trial_qualification_movement (
        "tenant_id", "scope_no", "scope_version_after", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_trial_scope(
    checked_scope_no VARCHAR
) RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    scope_row plugin_idc_core_trial_qualification_scope%ROWTYPE;
    projected_held BIGINT;
    projected_consumed BIGINT;
    latest_movement plugin_idc_core_trial_qualification_movement%ROWTYPE;
BEGIN
    SELECT * INTO scope_row
    FROM plugin_idc_core_trial_qualification_scope
    WHERE "scope_no" = checked_scope_no;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'trial qualification scope is missing';
    END IF;

    SELECT COUNT(*) FILTER (WHERE "status_code" = 'held'),
           COUNT(*) FILTER (WHERE "status_code" = 'consumed')
      INTO projected_held, projected_consumed
    FROM plugin_idc_core_trial_entitlement
    WHERE "scope_no" = checked_scope_no;
    IF projected_held <> scope_row."held_quantity"
       OR projected_consumed <> scope_row."consumed_quantity" THEN
        RAISE EXCEPTION 'trial qualification projection is inconsistent';
    END IF;

    SELECT * INTO latest_movement
    FROM plugin_idc_core_trial_qualification_movement
    WHERE "scope_no" = checked_scope_no
    ORDER BY "scope_version_after" DESC
    LIMIT 1;
    IF NOT FOUND
       OR latest_movement."scope_version_after" <> scope_row."version"
       OR latest_movement."held_after" <> scope_row."held_quantity"
       OR latest_movement."consumed_after" <> scope_row."consumed_quantity" THEN
        RAISE EXCEPTION 'trial qualification movement chain is inconsistent';
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_check_trial_scope_deferred()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM plugin_idc_core_validate_trial_scope(
        COALESCE(NEW."scope_no", OLD."scope_no")
    );
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_trial_scope_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'trial qualification scope cannot be deleted';
    END IF;
    IF NEW."tenant_id" <> OLD."tenant_id"
       OR NEW."scope_no" <> OLD."scope_no"
       OR NEW."customer_no" <> OLD."customer_no"
       OR NEW."product_no" <> OLD."product_no"
       OR NEW."created_at" <> OLD."created_at"
       OR NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'trial qualification scope mutation is invalid';
    END IF;
    RETURN NEW;
END;
$$;

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
       OR NEW."fulfillment_no" <> OLD."fulfillment_no"
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

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_trial_movement_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'trial qualification movements are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_trial_scope_guard
    ON plugin_idc_core_trial_qualification_scope;
CREATE TRIGGER trg_idc_trial_scope_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_trial_qualification_scope
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_trial_scope_mutation();

DROP TRIGGER IF EXISTS trg_idc_trial_entitlement_guard
    ON plugin_idc_core_trial_entitlement;
CREATE TRIGGER trg_idc_trial_entitlement_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_trial_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_trial_entitlement_mutation();

DROP TRIGGER IF EXISTS trg_idc_trial_movement_immutable
    ON plugin_idc_core_trial_qualification_movement;
CREATE TRIGGER trg_idc_trial_movement_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_trial_qualification_movement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_trial_movement_mutation();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_trial_qualification_scope'::regclass
           AND conname = 'trg_idc_trial_scope_projection'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_trial_qualification_scope'::regclass
               AND constraint_row.conname = 'trg_idc_trial_scope_projection'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_trial_scope_projection'
               AND trigger_row.tgfoid = 'plugin_idc_core_check_trial_scope_deferred()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_trial_scope_projection catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_trial_scope_projection
        AFTER INSERT OR UPDATE ON plugin_idc_core_trial_qualification_scope
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_trial_scope_deferred();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_trial_entitlement'::regclass
           AND conname = 'trg_idc_trial_entitlement_projection'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_trial_entitlement'::regclass
               AND constraint_row.conname = 'trg_idc_trial_entitlement_projection'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_trial_entitlement_projection'
               AND trigger_row.tgfoid = 'plugin_idc_core_check_trial_scope_deferred()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_trial_entitlement_projection catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_trial_entitlement_projection
        AFTER INSERT OR UPDATE ON plugin_idc_core_trial_entitlement
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_trial_scope_deferred();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_trial_qualification_movement'::regclass
           AND conname = 'trg_idc_trial_movement_projection'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_trial_qualification_movement'::regclass
               AND constraint_row.conname = 'trg_idc_trial_movement_projection'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_trial_movement_projection'
               AND trigger_row.tgfoid = 'plugin_idc_core_check_trial_scope_deferred()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_trial_movement_projection catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_trial_movement_projection
        AFTER INSERT ON plugin_idc_core_trial_qualification_movement
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_check_trial_scope_deferred();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_trial_qualification_scope
    IS 'Current customer-and-product trial usage projection serialized by checkout';
COMMENT ON TABLE plugin_idc_core_trial_entitlement
    IS 'One held or consumed trial entitlement per server fulfillment identity';
COMMENT ON TABLE plugin_idc_core_trial_qualification_movement
    IS 'Append-only trial qualification projection movement chain';
