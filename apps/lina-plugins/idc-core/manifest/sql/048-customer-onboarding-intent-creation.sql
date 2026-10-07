ALTER TABLE plugin_idc_core_customer_onboarding_intent
    ADD COLUMN IF NOT EXISTS "operation_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "request_digest" CHAR(64);

ALTER TABLE plugin_idc_core_customer_onboarding_intent
    DROP CONSTRAINT IF EXISTS ck_idc_customer_onboarding_operation_digest,
    DROP CONSTRAINT IF EXISTS ck_idc_customer_onboarding_request_digest;

ALTER TABLE plugin_idc_core_customer_onboarding_intent
    ADD CONSTRAINT ck_idc_customer_onboarding_operation_digest
        CHECK ("operation_digest" IS NULL OR "operation_digest" ~ '^[0-9a-f]{64}$'),
    ADD CONSTRAINT ck_idc_customer_onboarding_request_digest
        CHECK ("request_digest" IS NULL OR "request_digest" ~ '^[0-9a-f]{64}$');

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_customer_onboarding_operation
    ON plugin_idc_core_customer_onboarding_intent ("tenant_id", "operation_digest")
    WHERE "operation_digest" IS NOT NULL;
