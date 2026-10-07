-- Phase 6.13: allow the normal order-refund workflow to carry a credit
-- exposure and its original repayment allocations without inventing a cash
-- payment allocation for a credit-settled invoice.

ALTER TABLE plugin_idc_core_refund_source_allocation
    ALTER COLUMN "payment_allocation_no" DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS "credit_exposure_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "credit_component_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "credit_repayment_allocation_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "credit_plan_digest" CHAR(64);

ALTER TABLE plugin_idc_core_refund_source_allocation
    DROP CONSTRAINT IF EXISTS fk_idc_refund_source_payment;
ALTER TABLE plugin_idc_core_refund_source_allocation
    ADD CONSTRAINT fk_idc_refund_source_payment FOREIGN KEY ("payment_allocation_no")
        REFERENCES plugin_idc_core_billing_payment_allocation ("allocation_no")
        ON DELETE RESTRICT;

ALTER TABLE plugin_idc_core_refund_source_allocation
    DROP CONSTRAINT IF EXISTS ck_idc_refund_source_credit_binding;
ALTER TABLE plugin_idc_core_refund_source_allocation
    ADD CONSTRAINT ck_idc_refund_source_credit_binding CHECK (
        (
            "payment_allocation_no" IS NOT NULL
            AND "credit_exposure_no" IS NULL
            AND "credit_component_code" IS NULL
            AND "credit_repayment_allocation_no" IS NULL
            AND "credit_plan_digest" IS NULL
        ) OR (
            "payment_allocation_no" IS NULL
            AND "credit_exposure_no" IS NOT NULL
            AND "credit_component_code" IN ('unbilled', 'billed', 'repaid')
            AND "credit_plan_digest" ~ '^[0-9a-f]{64}$'
            AND (
                ("credit_component_code" IN ('unbilled', 'billed')
                    AND "credit_repayment_allocation_no" IS NULL
                    AND "payment_source_type_code" = 'credit'
                    AND "execution_owner_code" = 'core')
                OR ("credit_component_code" = 'repaid'
                    AND "credit_repayment_allocation_no" IS NOT NULL)
            )
        )
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_refund_source_credit_exposure'
          AND conrelid = 'plugin_idc_core_refund_source_allocation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_refund_source_allocation
            ADD CONSTRAINT fk_idc_refund_source_credit_exposure
            FOREIGN KEY ("credit_exposure_no")
            REFERENCES plugin_idc_core_credit_exposure ("exposure_no")
            ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_refund_source_credit_repayment'
          AND conrelid = 'plugin_idc_core_refund_source_allocation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_refund_source_allocation
            ADD CONSTRAINT fk_idc_refund_source_credit_repayment
            FOREIGN KEY ("credit_repayment_allocation_no")
            REFERENCES plugin_idc_core_credit_repayment_allocation ("allocation_no")
            ON DELETE RESTRICT;
    END IF;
END;
$constraint$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_refund_source_credit
    ON plugin_idc_core_refund_source_allocation (
        "refund_no", "refund_item_no", "credit_exposure_no",
        "credit_component_code", COALESCE("credit_repayment_allocation_no", '')
    ) WHERE "credit_exposure_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_refund_source_credit_request
    ON plugin_idc_core_refund_source_allocation (
        "tenant_id", "credit_exposure_no", "refund_no", "id"
    ) WHERE "credit_exposure_no" IS NOT NULL;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_refund_composition()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_refund_no VARCHAR(40);
    expected_amount BIGINT;
    item_amount BIGINT;
    invalid_items BIGINT;
    invalid_impacts BIGINT;
    invalid_sources BIGINT;
    missing_locks BIGINT;
    expected_resource_impact BOOLEAN;
    expected_termination BOOLEAN;
BEGIN
    target_refund_no := COALESCE(NEW."refund_no", OLD."refund_no");

    SELECT "requested_amount_minor" INTO expected_amount
    FROM plugin_idc_core_refund_request
    WHERE "refund_no" = target_refund_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COALESCE(SUM("requested_amount_minor"), 0)
    INTO item_amount
    FROM plugin_idc_core_refund_request_item
    WHERE "refund_no" = target_refund_no;

    SELECT COUNT(*) INTO invalid_items
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN plugin_idc_core_billing_invoice_item invoice_item
      ON invoice_item."invoice_no" = item."invoice_no"
     AND invoice_item."invoice_item_no" = item."invoice_item_no"
     AND invoice_item."order_item_no" = item."order_item_no"
     AND invoice_item."customer_no" = item."customer_no"
     AND invoice_item."currency_code" = item."currency_code"
    WHERE item."refund_no" = target_refund_no
      AND (
          invoice_item."id" IS NULL
          OR (item."target_type_code" = 'service' AND NOT EXISTS (
              SELECT 1
              FROM plugin_idc_core_service_instance service
              WHERE service."service_no" = item."service_no"
                AND service."customer_no" = item."customer_no"
                AND service."currency_code" = item."currency_code"
                AND service."order_item_no" = item."order_item_no"
          ))
      );

    SELECT COUNT(*) INTO invalid_impacts
    FROM plugin_idc_core_refund_service_impact "impact"
    JOIN plugin_idc_core_refund_request_item item
      ON item."refund_no" = "impact"."refund_no"
     AND item."refund_item_no" = "impact"."refund_item_no"
    LEFT JOIN plugin_idc_core_service_instance service
      ON service."service_no" = "impact"."service_no"
     AND service."customer_no" = "impact"."customer_no"
     AND service."currency_code" = "impact"."currency_code"
     AND service."order_item_no" = "impact"."order_item_no"
    WHERE "impact"."refund_no" = target_refund_no
      AND (
          service."id" IS NULL
          OR "impact"."order_item_no" <> item."order_item_no"
          OR "impact"."customer_no" <> item."customer_no"
          OR "impact"."currency_code" <> item."currency_code"
          OR (item."target_type_code" = 'service' AND "impact"."service_no" <> item."service_no")
          OR "impact"."service_version_snapshot" <> service."version"
          OR "impact"."entitlement_status_snapshot" <> service."entitlement_status_code"
          OR "impact"."runtime_status_snapshot" <> service."runtime_status_code"
          OR "impact"."provider_task_status_snapshot" <> service."provider_task_status_code"
      );

    SELECT COALESCE(BOOL_OR(
        "requires_service_termination" OR "causes_downtime" OR "causes_data_deletion"
    ), FALSE), COALESCE(BOOL_OR("requires_service_termination"), FALSE)
    INTO expected_resource_impact, expected_termination
    FROM plugin_idc_core_refund_service_impact
    WHERE "refund_no" = target_refund_no;

    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_request_item item
        WHERE item."refund_no" = target_refund_no
          AND (
              (item."target_type_code" = 'service' AND (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_refund_service_impact "impact"
                  WHERE "impact"."refund_item_no" = item."refund_item_no"
                    AND "impact"."service_no" = item."service_no"
              ) <> 1)
              OR (item."target_type_code" = 'order_item' AND (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_refund_service_impact "impact"
                  WHERE "impact"."refund_item_no" = item."refund_item_no"
              ) <> (
                  SELECT COUNT(*)
                  FROM plugin_idc_core_service_instance service
                  WHERE service."customer_no" = item."customer_no"
                    AND service."currency_code" = item."currency_code"
                    AND service."order_item_no" = item."order_item_no"
              ))
          )
    ) THEN
        invalid_impacts := invalid_impacts + 1;
    END IF;

    SELECT COUNT(*) INTO invalid_sources
    FROM plugin_idc_core_refund_request_item item
    LEFT JOIN LATERAL (
        SELECT COALESCE(SUM(source."amount_minor"), 0) AS "source_amount"
        FROM plugin_idc_core_refund_source_allocation source
        LEFT JOIN plugin_idc_core_billing_payment_allocation payment
          ON payment."allocation_no" = source."payment_allocation_no"
         AND payment."invoice_no" = item."invoice_no"
         AND payment."customer_no" = item."customer_no"
         AND payment."currency_code" = item."currency_code"
         AND payment."payment_source_type_code" = source."payment_source_type_code"
         AND payment."payment_source_reference" = source."payment_source_reference"
         AND (payment."invoice_item_no" IS NULL
              OR payment."invoice_item_no" = item."invoice_item_no")
        LEFT JOIN plugin_idc_core_credit_exposure exposure
          ON exposure."exposure_no" = source."credit_exposure_no"
         AND exposure."invoice_no" = item."invoice_no"
         AND exposure."order_no" = (
             SELECT invoice."order_no"
             FROM plugin_idc_core_billing_invoice invoice
             WHERE invoice."invoice_no" = item."invoice_no"
         )
         AND exposure."customer_no" = item."customer_no"
         AND exposure."currency_code" = item."currency_code"
         AND exposure."source_type_code" = 'order'
        LEFT JOIN plugin_idc_core_credit_repayment_allocation credit_allocation
          ON credit_allocation."allocation_no" = source."credit_repayment_allocation_no"
         AND credit_allocation."direction_code" = 'apply'
         AND credit_allocation."exposure_no" = source."credit_exposure_no"
        LEFT JOIN plugin_idc_core_credit_repayment repayment
          ON repayment."repayment_no" = credit_allocation."repayment_no"
         AND repayment."credit_account_no" = exposure."credit_account_no"
         AND repayment."customer_no" = item."customer_no"
         AND repayment."currency_code" = item."currency_code"
        WHERE source."refund_item_no" = item."refund_item_no"
          AND (
              (source."payment_allocation_no" IS NOT NULL AND payment."id" IS NOT NULL)
              OR (
                  source."credit_exposure_no" IS NOT NULL
                  AND exposure."id" IS NOT NULL
                  AND (
                      (source."credit_component_code" IN ('unbilled', 'billed')
                          AND source."payment_source_reference" = exposure."exposure_no")
                      OR (source."credit_component_code" = 'repaid'
                          AND credit_allocation."id" IS NOT NULL
                          AND repayment."id" IS NOT NULL
                          AND source."payment_source_reference" = repayment."funding_source_reference"
                          AND (
                              (repayment."funding_source_type_code" = 'online_payment'
                                  AND source."payment_source_type_code" = 'channel_payment'
                                  AND source."execution_owner_code" = 'payment')
                              OR (repayment."funding_source_type_code" = 'customer_balance'
                                  AND source."payment_source_type_code" = 'balance'
                                  AND source."execution_owner_code" = 'core')
                              OR (repayment."funding_source_type_code" = 'manual_receipt'
                                  AND source."payment_source_type_code" = 'manual_receipt'
                                  AND source."execution_owner_code" = 'manual')
                              OR (repayment."funding_source_type_code" = 'migration'
                                  AND source."payment_source_type_code" = 'migration'
                                  AND source."execution_owner_code" = 'manual')
                          ))
                  )
              )
          )
    ) source_total ON TRUE
    WHERE item."refund_no" = target_refund_no
      AND source_total."source_amount" <> item."requested_amount_minor";

    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_source_allocation source
        WHERE source."refund_no" = target_refund_no
          AND source."credit_exposure_no" IS NOT NULL
        GROUP BY source."refund_no"
        HAVING COUNT(DISTINCT source."credit_exposure_no") <> 1
            OR COUNT(DISTINCT source."credit_plan_digest") <> 1
    ) THEN
        invalid_sources := invalid_sources + 1;
    END IF;

    SELECT COUNT(*) INTO missing_locks
    FROM plugin_idc_core_refund_service_impact "impact"
    JOIN plugin_idc_core_refund_request_item item
      ON item."refund_item_no" = "impact"."refund_item_no"
     AND item."refund_no" = "impact"."refund_no"
    JOIN plugin_idc_core_refund_request request
      ON request."refund_no" = "impact"."refund_no"
    LEFT JOIN plugin_idc_core_refund_service_lock service_lock
      ON service_lock."refund_no" = "impact"."refund_no"
     AND service_lock."refund_item_no" = "impact"."refund_item_no"
     AND service_lock."impact_no" = "impact"."impact_no"
     AND service_lock."service_no" = "impact"."service_no"
     AND service_lock."expected_service_version" = "impact"."service_version_snapshot"
     AND service_lock."released_at" IS NULL
    WHERE "impact"."refund_no" = target_refund_no
      AND request."status_code" IN (
          'pending_review', 'approved', 'awaiting_balance_consent',
          'executing', 'result_unknown', 'manual_confirmation'
      )
      AND service_lock."id" IS NULL;

    IF item_amount <> expected_amount OR invalid_items <> 0 OR invalid_impacts <> 0
        OR invalid_sources <> 0 OR missing_locks <> 0 THEN
        RAISE EXCEPTION 'refund composition does not match request %', target_refund_no;
    END IF;
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_refund_request request
        WHERE request."refund_no" = target_refund_no
          AND (
              request."has_resource_impact" <> expected_resource_impact
              OR request."requires_service_termination" <> expected_termination
          )
    ) THEN
        RAISE EXCEPTION 'refund impact flags do not match request %', target_refund_no;
    END IF;
    RETURN NULL;
END;
$$;

COMMENT ON COLUMN plugin_idc_core_refund_source_allocation."credit_exposure_no"
    IS 'Credit purchase exposure; mutually exclusive with a cash payment allocation';
COMMENT ON COLUMN plugin_idc_core_refund_source_allocation."credit_repayment_allocation_no"
    IS 'Original credit repayment allocation for an already-repaid component';
