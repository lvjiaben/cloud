-- Add a short, monotonic customer number for management and customer-facing display.
CREATE SEQUENCE IF NOT EXISTS plugin_idc_core_customer_code_seq
    START WITH 10000 INCREMENT BY 1 NO CYCLE;

ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "customer_code" BIGINT;
ALTER TABLE plugin_idc_core_customer_subject
    ALTER COLUMN "customer_code" SET DEFAULT nextval('plugin_idc_core_customer_code_seq');

UPDATE plugin_idc_core_customer_subject
SET "customer_code" = nextval('plugin_idc_core_customer_code_seq')
WHERE "customer_code" IS NULL;

SELECT setval(
    'plugin_idc_core_customer_code_seq',
    GREATEST(COALESCE((SELECT MAX("customer_code") FROM plugin_idc_core_customer_subject), 9999), 9999),
    TRUE
);

ALTER TABLE plugin_idc_core_customer_subject
    ALTER COLUMN "customer_code" SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_customer_subject_code
    ON plugin_idc_core_customer_subject ("tenant_id", "customer_code");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_contact_phone_search
    ON plugin_idc_core_customer_contact ("tenant_id", "phone", "status_code");

COMMENT ON COLUMN plugin_idc_core_customer_subject."customer_code" IS
    'Short monotonic customer number shown to staff and customers; starts at 10000 and is never reused';
