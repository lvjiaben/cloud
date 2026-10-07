-- 106: Every customer starts as an individual and keeps that verified personal
-- identity when the same subject is later approved as an enterprise.

ALTER TABLE IF EXISTS plugin_idc_core_customer_individual_profile
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_ind_profile_customer;
ALTER TABLE IF EXISTS plugin_idc_core_customer_individual_profile
    ADD CONSTRAINT fk_plugin_idc_core_ind_profile_customer
        FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT;

ALTER TABLE IF EXISTS plugin_idc_core_customer_enterprise_profile
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_ent_profile_customer;
ALTER TABLE IF EXISTS plugin_idc_core_customer_enterprise_profile
    ADD CONSTRAINT fk_plugin_idc_core_ent_profile_customer
        FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT;

-- Payment attempts are historical snapshots. Their recorded customer type must
-- not prevent a later personal-to-enterprise upgrade of the owning subject.
ALTER TABLE IF EXISTS plugin_idc_core_commerce_payment_context
    DROP CONSTRAINT IF EXISTS fk_idc_com_payment_context_customer;
ALTER TABLE IF EXISTS plugin_idc_core_commerce_payment_context
    ADD CONSTRAINT fk_idc_com_payment_context_customer
        FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT;

INSERT INTO plugin_idc_core_customer_individual_profile (
    "tenant_id", "customer_no", "customer_type_code"
)
SELECT "subject"."tenant_id", "subject"."customer_no", 'individual'
FROM plugin_idc_core_customer_subject "subject"
WHERE NOT EXISTS (
    SELECT 1
    FROM plugin_idc_core_customer_individual_profile profile
    WHERE profile."customer_no" = "subject"."customer_no"
);

INSERT INTO plugin_idc_core_customer_enterprise_profile (
    "tenant_id", "customer_no", "customer_type_code"
)
SELECT "subject"."tenant_id", "subject"."customer_no", 'enterprise'
FROM plugin_idc_core_customer_subject "subject"
WHERE NOT EXISTS (
    SELECT 1
    FROM plugin_idc_core_customer_enterprise_profile profile
    WHERE profile."customer_no" = "subject"."customer_no"
);

COMMENT ON TABLE plugin_idc_core_customer_individual_profile IS
    'Persistent personal identity projection retained after an approved enterprise upgrade';
COMMENT ON TABLE plugin_idc_core_customer_enterprise_profile IS
    'Enterprise application and verified profile for the same personal-first customer subject';
