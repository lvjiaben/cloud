-- Customer-maintained display metadata for one server service.
-- These fields are intentionally separate from staff-only internal remarks.

ALTER TABLE plugin_idc_core_service_instance
    ADD COLUMN IF NOT EXISTS "customer_alias" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "customer_remark" VARCHAR(1000) NOT NULL DEFAULT '';

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_service_customer_metadata'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT ck_idc_service_customer_metadata CHECK (
                "customer_alias" = btrim("customer_alias")
                AND "customer_remark" = btrim("customer_remark")
                AND "customer_alias" !~ '[[:cntrl:]]'
                AND "customer_remark" !~ '[[:cntrl:]]'
            );
    END IF;
END;
$constraint$;
