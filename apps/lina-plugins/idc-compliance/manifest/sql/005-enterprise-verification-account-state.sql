-- Bounded latest-case lookup for the authenticated enterprise Portal.
-- The projection never reads legal plaintext or private material metadata.

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ent_customer_current
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "customer_no", "subject_type_code", "created_at" DESC, "id" DESC
    ) WHERE "subject_type_code" = 'enterprise';
