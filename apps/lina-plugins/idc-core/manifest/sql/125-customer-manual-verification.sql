-- Purpose: Record explicit staff verification without fabricated legal identity or compliance cases.
-- Manual approval preserves current legal data; clearing removes the current projection only.
ALTER TABLE plugin_idc_core_customer_individual_profile ADD COLUMN IF NOT EXISTS manual_verification BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE plugin_idc_core_customer_enterprise_profile ADD COLUMN IF NOT EXISTS manual_verification BOOLEAN NOT NULL DEFAULT FALSE;
COMMENT ON COLUMN plugin_idc_core_customer_individual_profile.manual_verification IS 'Current verification status set by staff; not supplier identity evidence';
COMMENT ON COLUMN plugin_idc_core_customer_enterprise_profile.manual_verification IS 'Current verification status set by staff; not supplier identity evidence';
ALTER TABLE plugin_idc_core_customer_individual_profile DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_ind_profile_verified;
ALTER TABLE plugin_idc_core_customer_individual_profile ADD CONSTRAINT ck_plugin_idc_core_ind_profile_verified CHECK (
 identity_status_code <> 'verified' OR (identity_verified_at IS NOT NULL AND
 (manual_verification OR (legal_name <> '' AND legal_name_masked <> '' AND identity_identifier_masked <> '' AND identity_lookup_digest ~ '^[0-9a-f]{64}$' AND identity_case_no <> '')))
);
ALTER TABLE plugin_idc_core_customer_enterprise_profile DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_ent_profile_verified;
ALTER TABLE plugin_idc_core_customer_enterprise_profile ADD CONSTRAINT ck_plugin_idc_core_ent_profile_verified CHECK (
 verification_status_code <> 'verified' OR (verified_at IS NOT NULL AND
 (manual_verification OR (legal_name <> '' AND registration_identifier <> '' AND registration_identifier_masked <> '' AND registration_lookup_digest ~ '^[0-9a-f]{64}$' AND verification_case_no <> '')))
);
