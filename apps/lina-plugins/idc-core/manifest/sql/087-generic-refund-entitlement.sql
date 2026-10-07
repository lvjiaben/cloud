-- 087: Move online refunds from server instances to generic order-item
-- quantities and optional long-lived service entitlements.

DO $legacy_refund_guard$
DECLARE legacy_exists BOOLEAN:=FALSE;
BEGIN
    IF EXISTS(SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_core_refund_request_item' AND column_name='service_no') THEN
        EXECUTE 'SELECT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item WHERE target_type_code=''service'' OR service_no IS NOT NULL)'
          INTO legacy_exists;
    END IF;
    IF to_regclass('plugin_idc_core_refund_service_impact') IS NOT NULL THEN
        EXECUTE 'SELECT $1 OR EXISTS(SELECT 1 FROM plugin_idc_core_refund_service_impact)'
          INTO legacy_exists USING legacy_exists;
    END IF;
    IF to_regclass('plugin_idc_core_refund_service_lock') IS NOT NULL THEN
        EXECUTE 'SELECT $1 OR EXISTS(SELECT 1 FROM plugin_idc_core_refund_service_lock)'
          INTO legacy_exists USING legacy_exists;
    END IF;
    IF legacy_exists THEN
        RAISE EXCEPTION
            'legacy server-shaped refund evidence exists; preserve it and rebuild the development database before applying migration 087'
            USING ERRCODE='23514';
    END IF;
END;
$legacy_refund_guard$;

-- During a full-manifest replay, migration 052 recreates the legacy-shaped
-- tables next to the already migrated entitlement tables. The guard above has
-- proved that these compatibility tables contain no evidence, so discard only
-- those replay artifacts. On a fresh database there is no entitlement table
-- yet and the original tables continue through the rename path below.
DO $drop_replayed_refund_compatibility_tables$
BEGIN
    IF to_regclass('plugin_idc_core_refund_service_lock') IS NOT NULL
       AND to_regclass('plugin_idc_core_refund_entitlement_lock') IS NOT NULL THEN
        DROP TABLE plugin_idc_core_refund_service_lock;
    END IF;
    IF to_regclass('plugin_idc_core_refund_service_impact') IS NOT NULL
       AND to_regclass('plugin_idc_core_refund_entitlement_impact') IS NOT NULL THEN
        DROP TABLE plugin_idc_core_refund_service_impact;
    END IF;
END;
$drop_replayed_refund_compatibility_tables$;

DROP TRIGGER IF EXISTS trg_idc_refund_conflict_operation
    ON plugin_idc_core_business_operation;
DROP TRIGGER IF EXISTS trg_idc_refund_conflict_lifecycle_task
    ON plugin_idc_core_service_lifecycle_task;
DROP FUNCTION IF EXISTS plugin_idc_core_reject_refund_conflict_operation();
DROP FUNCTION IF EXISTS plugin_idc_core_reject_refund_conflict_lifecycle_task();

DO $rename_refund_request_termination$
BEGIN
    IF EXISTS(SELECT 1 FROM information_schema.columns
              WHERE table_schema=current_schema()
                AND table_name='plugin_idc_core_refund_request'
                AND column_name='requires_service_termination') THEN
        IF EXISTS(SELECT 1 FROM information_schema.columns
                  WHERE table_schema=current_schema()
                    AND table_name='plugin_idc_core_refund_request'
                    AND column_name='requires_entitlement_termination') THEN
            ALTER TABLE plugin_idc_core_refund_request
                DROP COLUMN "requires_service_termination";
        ELSE
            ALTER TABLE plugin_idc_core_refund_request
                RENAME COLUMN "requires_service_termination" TO "requires_entitlement_termination";
        END IF;
    END IF;
END;
$rename_refund_request_termination$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_refund_request_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'refund requests cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."refund_no", OLD."customer_no", OLD."order_no",
        OLD."invoice_no", OLD."currency_code", OLD."reason_code", OLD."reason_detail",
        OLD."requested_amount_minor", OLD."approval_mode_code",
        OLD."requires_entitlement_termination", OLD."has_resource_impact",
        OLD."has_dangerous_operation", OLD."has_fund_anomaly", OLD."high_amount",
        OLD."policy_no", OLD."policy_version", OLD."impact_digest",
        OLD."request_digest", OLD."idempotency_digest", OLD."submitted_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."refund_no", NEW."customer_no", NEW."order_no",
        NEW."invoice_no", NEW."currency_code", NEW."reason_code", NEW."reason_detail",
        NEW."requested_amount_minor", NEW."approval_mode_code",
        NEW."requires_entitlement_termination", NEW."has_resource_impact",
        NEW."has_dangerous_operation", NEW."has_fund_anomaly", NEW."high_amount",
        NEW."policy_no", NEW."policy_version", NEW."impact_digest",
        NEW."request_digest", NEW."idempotency_digest", NEW."submitted_at"
    ) THEN
        RAISE EXCEPTION 'refund request intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_refund_request_composition
    ON plugin_idc_core_refund_request;
DROP TRIGGER IF EXISTS trg_idc_refund_item_composition
    ON plugin_idc_core_refund_request_item;
DROP TRIGGER IF EXISTS trg_idc_refund_source_composition
    ON plugin_idc_core_refund_source_allocation;
DO $drop_old_refund_triggers$
BEGIN
    IF to_regclass('plugin_idc_core_refund_service_impact') IS NOT NULL THEN
        EXECUTE 'DROP TRIGGER IF EXISTS trg_idc_refund_impact_composition ON plugin_idc_core_refund_service_impact';
    END IF;
    IF to_regclass('plugin_idc_core_refund_entitlement_impact') IS NOT NULL THEN
        EXECUTE 'DROP TRIGGER IF EXISTS trg_idc_refund_impact_composition ON plugin_idc_core_refund_entitlement_impact';
    END IF;
    IF to_regclass('plugin_idc_core_refund_service_lock') IS NOT NULL THEN
        EXECUTE 'DROP TRIGGER IF EXISTS trg_idc_refund_lock_composition ON plugin_idc_core_refund_service_lock';
    END IF;
    IF to_regclass('plugin_idc_core_refund_entitlement_lock') IS NOT NULL THEN
        EXECUTE 'DROP TRIGGER IF EXISTS trg_idc_refund_lock_composition ON plugin_idc_core_refund_entitlement_lock';
    END IF;
END;
$drop_old_refund_triggers$;

DROP INDEX IF EXISTS uk_idc_refund_item_target;
DROP INDEX IF EXISTS uk_idc_refund_item_order_item;
ALTER TABLE plugin_idc_core_refund_request_item
    DROP CONSTRAINT IF EXISTS ck_idc_refund_item_target,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_item_service,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_item_service_identity,
    DROP CONSTRAINT IF EXISTS ck_idc_refund_item_quantity,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_item_entitlement;
DO $rename_refund_item_target$
BEGIN
    IF EXISTS(SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_core_refund_request_item' AND column_name='service_no') THEN
        ALTER TABLE plugin_idc_core_refund_request_item RENAME COLUMN "service_no" TO "entitlement_no";
    END IF;
END;
$rename_refund_item_target$;
ALTER TABLE plugin_idc_core_refund_request_item
    ADD COLUMN IF NOT EXISTS "quantity" INT NOT NULL DEFAULT 1;
ALTER TABLE plugin_idc_core_refund_request_item
	DROP COLUMN IF EXISTS "target_type_code",
	DROP COLUMN IF EXISTS "entitlement_no",
    ALTER COLUMN "quantity" DROP DEFAULT,
    ADD CONSTRAINT ck_idc_refund_item_quantity CHECK ("quantity" BETWEEN 1 AND 1000);
DO $refund_item_identity$
BEGIN
    IF NOT EXISTS(SELECT 1 FROM pg_constraint
        WHERE conrelid='plugin_idc_core_refund_request_item'::regclass
          AND conname='uk_idc_refund_item_refund_item') THEN
        ALTER TABLE plugin_idc_core_refund_request_item
            ADD CONSTRAINT uk_idc_refund_item_refund_item UNIQUE("refund_no","refund_item_no");
    END IF;
END;
$refund_item_identity$;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_item_order_item
	ON plugin_idc_core_refund_request_item("tenant_id","refund_no","invoice_item_no");

DROP INDEX IF EXISTS idx_idc_refund_impact_request;
DO $rename_refund_impact$
BEGIN
    IF to_regclass('plugin_idc_core_refund_service_impact') IS NOT NULL THEN
        ALTER TABLE plugin_idc_core_refund_service_impact
            DROP CONSTRAINT IF EXISTS ck_idc_refund_impact_version,
            DROP CONSTRAINT IF EXISTS ck_idc_refund_impact_termination,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_service,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_service_identity,
            DROP CONSTRAINT IF EXISTS uk_idc_refund_impact_service,
            DROP CONSTRAINT IF EXISTS ck_idc_refund_impact_version_legacy_v1,
            DROP CONSTRAINT IF EXISTS ck_idc_refund_impact_termination_legacy_v1,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_service_legacy_v1,
            DROP CONSTRAINT IF EXISTS uk_idc_refund_impact_service_legacy_v1;
        ALTER TABLE plugin_idc_core_refund_service_impact
            RENAME TO plugin_idc_core_refund_entitlement_impact;
    END IF;
    IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='plugin_idc_core_refund_entitlement_impact' AND column_name='service_no') THEN
        ALTER TABLE plugin_idc_core_refund_entitlement_impact RENAME COLUMN "service_no" TO "entitlement_no";
        ALTER TABLE plugin_idc_core_refund_entitlement_impact RENAME COLUMN "service_version_snapshot" TO "entitlement_version_snapshot";
        ALTER TABLE plugin_idc_core_refund_entitlement_impact RENAME COLUMN "entitlement_status_snapshot" TO "status_snapshot";
        ALTER TABLE plugin_idc_core_refund_entitlement_impact RENAME COLUMN "requires_service_termination" TO "requires_termination";
    END IF;
END;
$rename_refund_impact$;
ALTER TABLE plugin_idc_core_refund_entitlement_impact
    DROP CONSTRAINT IF EXISTS ck_idc_refund_impact_version,
    DROP CONSTRAINT IF EXISTS uk_idc_refund_impact_entitlement,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_entitlement,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_item,
    DROP COLUMN IF EXISTS "runtime_status_snapshot",
    DROP COLUMN IF EXISTS "provider_task_status_snapshot",
    DROP COLUMN IF EXISTS "causes_downtime",
    DROP COLUMN IF EXISTS "causes_data_deletion",
    ADD CONSTRAINT ck_idc_refund_impact_version CHECK ("entitlement_version_snapshot">0),
    ADD CONSTRAINT uk_idc_refund_impact_entitlement UNIQUE(
        "refund_no","refund_item_no","entitlement_no"
    ),
    ADD CONSTRAINT fk_idc_refund_impact_item FOREIGN KEY("refund_no","refund_item_no")
        REFERENCES plugin_idc_core_refund_request_item("refund_no","refund_item_no")
        ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_refund_impact_entitlement FOREIGN KEY("entitlement_no")
        REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT;
DO $refund_impact_identity$
BEGIN
    IF NOT EXISTS(SELECT 1 FROM pg_constraint
        WHERE conrelid='plugin_idc_core_refund_entitlement_impact'::regclass
          AND conname='uk_idc_refund_impact_refund_item') THEN
        ALTER TABLE plugin_idc_core_refund_entitlement_impact
            ADD CONSTRAINT uk_idc_refund_impact_refund_item UNIQUE(
                "refund_no","refund_item_no","impact_no"
            );
    END IF;
END;
$refund_impact_identity$;
CREATE INDEX IF NOT EXISTS idx_idc_refund_impact_request
    ON plugin_idc_core_refund_entitlement_impact(
        "tenant_id","refund_no","refund_item_no","entitlement_no","id"
    );

ALTER TABLE plugin_idc_core_refund_source_allocation
    DROP CONSTRAINT IF EXISTS fk_idc_refund_source_item,
    ADD CONSTRAINT fk_idc_refund_source_item FOREIGN KEY("refund_no","refund_item_no")
        REFERENCES plugin_idc_core_refund_request_item("refund_no","refund_item_no")
        ON DELETE RESTRICT;

DROP INDEX IF EXISTS uk_idc_refund_lock_active_service;
DO $rename_refund_lock$
BEGIN
    IF to_regclass('plugin_idc_core_refund_service_lock') IS NOT NULL THEN
        ALTER TABLE plugin_idc_core_refund_service_lock
            DROP CONSTRAINT IF EXISTS ck_idc_refund_lock_version,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_lock_service,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_lock_service_identity,
            DROP CONSTRAINT IF EXISTS ck_idc_refund_lock_version_legacy_v1,
            DROP CONSTRAINT IF EXISTS fk_idc_refund_lock_service_legacy_v1;
        ALTER TABLE plugin_idc_core_refund_service_lock
            RENAME TO plugin_idc_core_refund_entitlement_lock;
    END IF;
    IF EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='plugin_idc_core_refund_entitlement_lock' AND column_name='service_no') THEN
        ALTER TABLE plugin_idc_core_refund_entitlement_lock RENAME COLUMN "service_no" TO "entitlement_no";
        ALTER TABLE plugin_idc_core_refund_entitlement_lock RENAME COLUMN "expected_service_version" TO "expected_entitlement_version";
    END IF;
END;
$rename_refund_lock$;
ALTER TABLE plugin_idc_core_refund_entitlement_lock
    DROP CONSTRAINT IF EXISTS ck_idc_refund_lock_version,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_lock_entitlement,
    DROP CONSTRAINT IF EXISTS fk_idc_refund_lock_impact,
    ADD CONSTRAINT ck_idc_refund_lock_version CHECK("expected_entitlement_version">0),
    ADD CONSTRAINT fk_idc_refund_lock_impact FOREIGN KEY(
        "refund_no","refund_item_no","impact_no"
    ) REFERENCES plugin_idc_core_refund_entitlement_impact(
        "refund_no","refund_item_no","impact_no"
    ) ON DELETE RESTRICT,
    ADD CONSTRAINT fk_idc_refund_lock_entitlement FOREIGN KEY("entitlement_no")
        REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_lock_active_entitlement
    ON plugin_idc_core_refund_entitlement_lock("tenant_id","entitlement_no")
    WHERE "released_at" IS NULL;

DROP FUNCTION IF EXISTS plugin_idc_core_validate_refund_composition();
CREATE OR REPLACE FUNCTION plugin_idc_core_validate_refund_composition(target_refund_no VARCHAR(40))
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    expected_amount BIGINT;
    item_amount BIGINT;
    invalid_items BIGINT;
    invalid_impacts BIGINT;
    invalid_sources BIGINT;
    missing_locks BIGINT;
    expected_resource_impact BOOLEAN;
    expected_termination BOOLEAN;
BEGIN
    SELECT "requested_amount_minor" INTO expected_amount
    FROM plugin_idc_core_refund_request WHERE "refund_no"=target_refund_no;
    IF NOT FOUND THEN RETURN; END IF;

    SELECT COALESCE(SUM("requested_amount_minor"),0) INTO item_amount
    FROM plugin_idc_core_refund_request_item WHERE "refund_no"=target_refund_no;

    SELECT COUNT(*) INTO invalid_items
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN plugin_idc_core_billing_invoice_item invoice_item
      ON invoice_item."invoice_no"=item."invoice_no"
     AND invoice_item."invoice_item_no"=item."invoice_item_no"
     AND invoice_item."order_item_no"=item."order_item_no"
     AND invoice_item."customer_no"=item."customer_no"
     AND invoice_item."currency_code"=item."currency_code"
    LEFT JOIN plugin_idc_core_commerce_order_item_quantity_snapshot "quantity"
      ON "quantity"."order_item_no"=item."order_item_no"
    WHERE item."refund_no"=target_refund_no AND (
        invoice_item."id" IS NULL OR "quantity"."order_item_no" IS NULL
		OR item."quantity"<1 OR item."quantity">"quantity"."quantity"
    );

    SELECT COUNT(*) INTO invalid_impacts
    FROM plugin_idc_core_refund_entitlement_impact "impact"
    JOIN plugin_idc_core_refund_request_item item
      ON item."refund_no"="impact"."refund_no" AND item."refund_item_no"="impact"."refund_item_no"
    JOIN plugin_idc_core_refund_request request
      ON request."refund_no"="impact"."refund_no"
    LEFT JOIN plugin_idc_core_service_entitlement entitlement
      ON entitlement."entitlement_no"="impact"."entitlement_no"
     AND entitlement."customer_no"="impact"."customer_no"
     AND entitlement."order_item_no"="impact"."order_item_no"
    LEFT JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
      ON entitlement_lock."refund_no"="impact"."refund_no"
     AND entitlement_lock."refund_item_no"="impact"."refund_item_no"
     AND entitlement_lock."impact_no"="impact"."impact_no"
     AND entitlement_lock."entitlement_no"="impact"."entitlement_no"
     AND entitlement_lock."expected_entitlement_version"="impact"."entitlement_version_snapshot"
    WHERE "impact"."refund_no"=target_refund_no AND (
        entitlement."id" IS NULL OR "impact"."order_item_no"<>item."order_item_no"
        OR "impact"."customer_no"<>item."customer_no"
        OR "impact"."currency_code"<>item."currency_code"
        OR entitlement_lock."id" IS NULL
        OR (
            request."status_code"='succeeded' AND "impact"."requires_termination" AND (
                (entitlement_lock."released_at" IS NULL AND (
                    "impact"."entitlement_version_snapshot"<>entitlement."version"
                    OR "impact"."status_snapshot"<>entitlement."status_code"
                ))
                OR (entitlement_lock."released_at" IS NOT NULL AND (
                    entitlement."status_code"<>'terminated'
                    OR entitlement."version"<>"impact"."entitlement_version_snapshot"+1
                ))
            )
        )
        OR (
            NOT (request."status_code"='succeeded' AND "impact"."requires_termination")
            AND (
                "impact"."entitlement_version_snapshot"<>entitlement."version"
                OR "impact"."status_snapshot"<>entitlement."status_code"
            )
        )
    );

    -- Entitlement impacts are optional and independent from the refunded item
    -- quantity. A per-order-item entitlement may represent multiple purchased
    -- units, so only the upper bound is enforced here.
    IF EXISTS(
        SELECT 1
        FROM plugin_idc_core_refund_request_item item
        LEFT JOIN plugin_idc_core_refund_entitlement_impact "impact"
          ON "impact"."refund_no"=item."refund_no"
         AND "impact"."refund_item_no"=item."refund_item_no"
        WHERE item."refund_no"=target_refund_no
        GROUP BY item."refund_item_no",item."quantity"
        HAVING COUNT("impact"."impact_no")>item."quantity"
    ) THEN invalid_impacts:=invalid_impacts+1; END IF;

    SELECT EXISTS(
        SELECT 1 FROM plugin_idc_core_refund_entitlement_impact
        WHERE "refund_no"=target_refund_no
    ),COALESCE(BOOL_OR("requires_termination"),FALSE)
      INTO expected_resource_impact,expected_termination
    FROM plugin_idc_core_refund_entitlement_impact
    WHERE "refund_no"=target_refund_no;

    SELECT COUNT(*) INTO invalid_sources
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN LATERAL(
        SELECT COALESCE(SUM(source."amount_minor"),0) AS "source_amount"
        FROM plugin_idc_core_refund_source_allocation source
        LEFT JOIN plugin_idc_core_billing_payment_allocation payment
          ON payment."allocation_no"=source."payment_allocation_no"
         AND payment."invoice_no"=item."invoice_no"
         AND payment."customer_no"=item."customer_no"
         AND payment."currency_code"=item."currency_code"
         AND payment."payment_source_type_code"=source."payment_source_type_code"
         AND payment."payment_source_reference"=source."payment_source_reference"
         AND (payment."invoice_item_no" IS NULL OR payment."invoice_item_no"=item."invoice_item_no")
        LEFT JOIN plugin_idc_core_credit_exposure exposure
          ON exposure."exposure_no"=source."credit_exposure_no"
         AND exposure."invoice_no"=item."invoice_no"
         AND exposure."order_no"=(SELECT invoice."order_no" FROM plugin_idc_core_billing_invoice invoice
                                WHERE invoice."invoice_no"=item."invoice_no")
         AND exposure."customer_no"=item."customer_no"
         AND exposure."currency_code"=item."currency_code"
         AND exposure."source_type_code"='order'
        LEFT JOIN plugin_idc_core_credit_repayment_allocation credit_allocation
          ON credit_allocation."allocation_no"=source."credit_repayment_allocation_no"
         AND credit_allocation."direction_code"='apply'
         AND credit_allocation."exposure_no"=source."credit_exposure_no"
        LEFT JOIN plugin_idc_core_credit_repayment repayment
          ON repayment."repayment_no"=credit_allocation."repayment_no"
         AND repayment."credit_account_no"=exposure."credit_account_no"
         AND repayment."customer_no"=item."customer_no"
         AND repayment."currency_code"=item."currency_code"
        WHERE source."refund_no"=item."refund_no"
          AND source."refund_item_no"=item."refund_item_no" AND (
            (source."payment_allocation_no" IS NOT NULL AND payment."id" IS NOT NULL)
            OR (source."credit_exposure_no" IS NOT NULL AND exposure."id" IS NOT NULL AND (
                (source."credit_component_code" IN('unbilled','billed')
                    AND source."payment_source_reference"=exposure."exposure_no")
                OR (source."credit_component_code"='repaid'
                    AND credit_allocation."id" IS NOT NULL AND repayment."id" IS NOT NULL
                    AND source."payment_source_reference"=repayment."funding_source_reference"
                    AND (
                        (repayment."funding_source_type_code"='online_payment'
                            AND source."payment_source_type_code"='channel_payment'
                            AND source."execution_owner_code"='payment')
                        OR (repayment."funding_source_type_code"='customer_balance'
                            AND source."payment_source_type_code"='balance'
                            AND source."execution_owner_code"='core')
                        OR (repayment."funding_source_type_code"='manual_receipt'
                            AND source."payment_source_type_code"='manual_receipt'
                            AND source."execution_owner_code"='manual')
                        OR (repayment."funding_source_type_code"='migration'
                            AND source."payment_source_type_code"='migration'
                            AND source."execution_owner_code"='manual')
                    ))
            ))
        )
    ) source_total ON TRUE
    WHERE item."refund_no"=target_refund_no
      AND source_total."source_amount"<>item."requested_amount_minor";

    IF EXISTS(
        SELECT 1 FROM plugin_idc_core_refund_source_allocation source
        WHERE source."refund_no"=target_refund_no AND source."credit_exposure_no" IS NOT NULL
        GROUP BY source."refund_no"
        HAVING COUNT(DISTINCT source."credit_exposure_no")<>1
            OR COUNT(DISTINCT source."credit_plan_digest")<>1
    ) THEN invalid_sources:=invalid_sources+1; END IF;

    SELECT COUNT(*) INTO missing_locks
    FROM plugin_idc_core_refund_entitlement_impact "impact"
    JOIN plugin_idc_core_refund_request request ON request."refund_no"="impact"."refund_no"
    LEFT JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
      ON entitlement_lock."refund_no"="impact"."refund_no"
     AND entitlement_lock."refund_item_no"="impact"."refund_item_no"
     AND entitlement_lock."impact_no"="impact"."impact_no"
     AND entitlement_lock."entitlement_no"="impact"."entitlement_no"
     AND entitlement_lock."expected_entitlement_version"="impact"."entitlement_version_snapshot"
     AND entitlement_lock."released_at" IS NULL
    WHERE "impact"."refund_no"=target_refund_no
      AND request."status_code" IN('pending_review','approved','awaiting_balance_consent',
                                 'executing','result_unknown','manual_confirmation')
      AND entitlement_lock."id" IS NULL;

    IF item_amount<>expected_amount OR invalid_items<>0 OR invalid_impacts<>0
       OR invalid_sources<>0 OR missing_locks<>0 THEN
        RAISE EXCEPTION 'refund composition does not match request %',target_refund_no;
    END IF;
    IF EXISTS(
        SELECT 1 FROM plugin_idc_core_refund_request request
        WHERE request."refund_no"=target_refund_no AND (
            request."has_resource_impact"<>expected_resource_impact
            OR request."requires_entitlement_termination"<>expected_termination
        )
    ) THEN
        RAISE EXCEPTION 'refund entitlement flags do not match request %',target_refund_no;
    END IF;
    RETURN;
END;
$$;

-- Source tables enqueue one immutable transaction/refund key. PostgreSQL only
-- creates the deferred validation event for the first successful insert, so a
-- refund with many item/source/impact/lock rows is still validated exactly once
-- at commit.
-- Purpose: Stores Refund Composition Validation Queue records.
-- 用途：保存退款组成校验队列相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_refund_composition_validation_queue (
    "transaction_id" BIGINT NOT NULL,
    "refund_no" VARCHAR(40) NOT NULL,
    "queued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("transaction_id","refund_no"),
    CONSTRAINT fk_idc_refund_validation_queue_request FOREIGN KEY("refund_no")
        REFERENCES plugin_idc_core_refund_request("refund_no") ON DELETE CASCADE
);

CREATE OR REPLACE FUNCTION plugin_idc_core_enqueue_refund_composition_validation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_refund_no VARCHAR(40);
BEGIN
    target_refund_no:=COALESCE(NEW."refund_no",OLD."refund_no");
    INSERT INTO plugin_idc_core_refund_composition_validation_queue("transaction_id","refund_no")
    VALUES(txid_current(),target_refund_no)
    ON CONFLICT("transaction_id","refund_no") DO NOTHING;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_run_queued_refund_composition_validation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM plugin_idc_core_validate_refund_composition(NEW."refund_no");
    DELETE FROM plugin_idc_core_refund_composition_validation_queue
    WHERE "transaction_id"=NEW."transaction_id" AND "refund_no"=NEW."refund_no";
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_refund_request_composition ON plugin_idc_core_refund_request;
CREATE TRIGGER trg_idc_refund_request_composition
AFTER INSERT OR UPDATE ON plugin_idc_core_refund_request
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation();

DROP TRIGGER IF EXISTS trg_idc_refund_item_composition ON plugin_idc_core_refund_request_item;
CREATE TRIGGER trg_idc_refund_item_composition
AFTER INSERT ON plugin_idc_core_refund_request_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation();

DROP TRIGGER IF EXISTS trg_idc_refund_source_composition ON plugin_idc_core_refund_source_allocation;
CREATE TRIGGER trg_idc_refund_source_composition
AFTER INSERT ON plugin_idc_core_refund_source_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation();

DROP TRIGGER IF EXISTS trg_idc_refund_impact_composition ON plugin_idc_core_refund_entitlement_impact;
CREATE TRIGGER trg_idc_refund_impact_composition
AFTER INSERT ON plugin_idc_core_refund_entitlement_impact
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation();

DROP TRIGGER IF EXISTS trg_idc_refund_lock_composition ON plugin_idc_core_refund_entitlement_lock;
CREATE TRIGGER trg_idc_refund_lock_composition
AFTER INSERT OR UPDATE ON plugin_idc_core_refund_entitlement_lock
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_refund_composition_validation();

DO $constraint_trigger$
BEGIN
    -- The catalog check and CREATE must be one transaction-safe unit. The
    -- advisory lock also serializes concurrent install/replay attempts that
    -- observe the same initially absent catalog row.
    PERFORM pg_advisory_xact_lock(
        hashtextextended('idc-core:trg_idc_refund_validation_queue_composition', 0)
    );
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_refund_composition_validation_queue'::regclass
          AND conname = 'trg_idc_refund_validation_queue_composition'
          AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
            FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row
              ON trigger_row.tgconstraint = constraint_row.oid
            WHERE constraint_row.conrelid = 'plugin_idc_core_refund_composition_validation_queue'::regclass
              AND constraint_row.conname = 'trg_idc_refund_validation_queue_composition'
              AND constraint_row.contype = 't'
              AND constraint_row.condeferrable
              AND constraint_row.condeferred
              AND trigger_row.tgrelid = constraint_row.conrelid
              AND NOT trigger_row.tgisinternal
              AND trigger_row.tgname = 'trg_idc_refund_validation_queue_composition'
              AND trigger_row.tgfoid = 'plugin_idc_core_run_queued_refund_composition_validation()'::regprocedure
              AND trigger_row.tgtype = 5
              AND trigger_row.tgenabled = 'O'
              AND trigger_row.tgdeferrable
              AND trigger_row.tginitdeferred
        ) THEN
            RAISE EXCEPTION
                'constraint trigger trg_idc_refund_validation_queue_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSIF EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgrelid = 'plugin_idc_core_refund_composition_validation_queue'::regclass
          AND tgname = 'trg_idc_refund_validation_queue_composition'
          AND NOT tgisinternal
    ) THEN
        RAISE EXCEPTION
            'constraint trigger trg_idc_refund_validation_queue_composition catalog contract is invalid'
            USING ERRCODE = '23514';
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_refund_validation_queue_composition
        AFTER INSERT ON plugin_idc_core_refund_composition_validation_queue
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_run_queued_refund_composition_validation();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_refund_request_item
    IS 'Immutable generic order-item quantity and amount refund selection.';
COMMENT ON TABLE plugin_idc_core_refund_entitlement_impact
    IS 'Immutable selected long-lived entitlement status and version snapshot.';
COMMENT ON TABLE plugin_idc_core_refund_entitlement_lock
    IS 'Active refund fence preventing duplicate handling of one entitlement.';
