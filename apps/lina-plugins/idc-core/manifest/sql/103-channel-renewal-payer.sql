-- Channel renewal keeps the service owner as the business-operation subject
-- while charging the approved channel enterprise recorded as payer.
ALTER TABLE plugin_idc_core_business_operation
    ADD COLUMN IF NOT EXISTS "payer_customer_no" VARCHAR(40);

UPDATE plugin_idc_core_business_operation
SET "payer_customer_no" = "customer_no"
WHERE "payer_customer_no" IS NULL;

ALTER TABLE plugin_idc_core_business_operation
    ALTER COLUMN "payer_customer_no" SET NOT NULL,
    DROP CONSTRAINT IF EXISTS fk_idc_business_operation_invoice;

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_business_operation'::regclass
          AND conname = 'fk_idc_business_operation_payer'
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT fk_idc_business_operation_payer
            FOREIGN KEY ("payer_customer_no")
            REFERENCES plugin_idc_core_customer_subject("customer_no")
            ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_business_operation'::regclass
          AND conname = 'fk_idc_business_operation_invoice_payer'
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT fk_idc_business_operation_invoice_payer FOREIGN KEY (
                "invoice_no", "payer_customer_no", "currency_code"
            ) REFERENCES plugin_idc_core_billing_invoice (
                "invoice_no", "customer_no", "currency_code"
            ) ON DELETE RESTRICT;
    END IF;
END
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_payer
    ON plugin_idc_core_business_operation(
        "tenant_id", "payer_customer_no", "created_at" DESC, "id" DESC
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_normalize_business_operation_subject()
RETURNS TRIGGER AS $$ BEGIN
    IF NEW."subject_type_code"='service' AND NEW."subject_no" IS NULL AND NEW."service_no" IS NOT NULL THEN
        NEW."subject_no" := NEW."service_no";
    END IF;
    IF NEW."subject_type_code"='service' AND NEW."expected_subject_version" IS NULL THEN
        NEW."expected_subject_version" := NEW."expected_service_version";
    END IF;
    IF NEW."payer_customer_no" IS NULL OR btrim(NEW."payer_customer_no") = '' THEN
        NEW."payer_customer_no" := NEW."customer_no";
    END IF;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF ROW(OLD."tenant_id",OLD."operation_no",OLD."operation_type_code",OLD."customer_no",
           OLD."payer_customer_no",OLD."service_no",OLD."subject_type_code",OLD."subject_no",OLD."currency_code",
           OLD."price_snapshot_no",OLD."quote_no",OLD."order_no",OLD."invoice_no",
           OLD."financial_settlement_required",OLD."execution_mode_code",OLD."resource_owner_code",
           OLD."channel_no",OLD."pool_no",OLD."capability_evidence_no",OLD."capability_evidence_digest",
           OLD."idempotency_digest",OLD."request_digest",OLD."content_digest",
           OLD."expected_service_version",OLD."expected_subject_version",OLD."created_at")
       IS DISTINCT FROM
       ROW(NEW."tenant_id",NEW."operation_no",NEW."operation_type_code",NEW."customer_no",
           NEW."payer_customer_no",NEW."service_no",NEW."subject_type_code",NEW."subject_no",NEW."currency_code",
           NEW."price_snapshot_no",NEW."quote_no",NEW."order_no",NEW."invoice_no",
           NEW."financial_settlement_required",NEW."execution_mode_code",NEW."resource_owner_code",
           NEW."channel_no",NEW."pool_no",NEW."capability_evidence_no",NEW."capability_evidence_digest",
           NEW."idempotency_digest",NEW."request_digest",NEW."content_digest",
           NEW."expected_service_version",NEW."expected_subject_version",NEW."created_at") THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    RETURN NEW;
END; $$;

