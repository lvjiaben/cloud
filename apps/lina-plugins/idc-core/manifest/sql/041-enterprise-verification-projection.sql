-- Enterprise verification adds a purpose-specific consent category and a
-- database uniqueness fence for the normalized registration identifier.

ALTER TABLE plugin_idc_core_customer_consent
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_consent_type;
ALTER TABLE plugin_idc_core_customer_consent
    ADD CONSTRAINT ck_plugin_idc_core_consent_type CHECK (
        "consent_type_code" IN (
            'terms', 'privacy', 'sensitive_personal_information', 'enterprise_verification',
            'marketing', 'data_export', 'account_closure'
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_ent_profile_registration_digest
    ON plugin_idc_core_customer_enterprise_profile (
        "tenant_id", "registration_lookup_digest"
    ) WHERE "registration_lookup_digest" <> '';

COMMENT ON INDEX uk_plugin_idc_core_ent_profile_registration_digest IS
    'Concurrency fence preventing two customers from owning the same keyed enterprise registration digest';

ALTER TABLE plugin_idc_core_customer_legal_change_request
    ADD COLUMN IF NOT EXISTS "review_verification_case_no" VARCHAR(40) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_customer_legal_change_request
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_legal_change_review_case;
ALTER TABLE plugin_idc_core_customer_legal_change_request
    ADD CONSTRAINT ck_plugin_idc_core_legal_change_review_case CHECK (
        ("status_code" = 'submitted' AND "review_verification_case_no" = '') OR
        ("status_code" IN ('under_review', 'approved', 'rejected') AND
            "review_verification_case_no" <> '') OR
        "status_code" = 'cancelled'
    );

COMMENT ON COLUMN plugin_idc_core_customer_legal_change_request."review_verification_case_no" IS
    'Opaque compliance case performing this legal-data review; proposed values and materials remain compliance-owned';
