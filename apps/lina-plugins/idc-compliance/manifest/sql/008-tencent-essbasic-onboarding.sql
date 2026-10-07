-- Enable only the Tencent adapter implemented from the reviewed public API contract.
-- NuoNuo remains database-enforced closed behind its separate evidence gate.

ALTER TABLE plugin_idc_compliance_supplier_configuration
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_evidence,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_adapter,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_credential_ready;

ALTER TABLE plugin_idc_compliance_supplier_configuration
    ADD CONSTRAINT ck_plugin_idc_compliance_scfg_evidence
        CHECK (
            (
                "provider_code" = 'tencent_essbasic' AND
                "capability_evidence_code" IN (
                    'external_confirmation_required',
                    'official_public_contract'
                )
            ) OR
            (
                "provider_code" = 'nuonuo_invoice' AND
                "capability_evidence_code" = 'external_confirmation_required'
            )
        ),
    ADD CONSTRAINT ck_plugin_idc_compliance_scfg_adapter
        CHECK (
            "adapter_enabled" = FALSE OR
            (
                "provider_code" = 'tencent_essbasic' AND
                "capability_evidence_code" = 'official_public_contract' AND
                "subject_status_code" = 'active'
            )
        ),
    ADD CONSTRAINT ck_plugin_idc_compliance_scfg_credential_ready
        CHECK (
            "credential_status_code" <> 'configured' OR
            (
                "credential_secret_ref" IS NOT NULL AND
                "credential_generation" > 0
            )
        );

COMMENT ON TABLE plugin_idc_compliance_supplier_configuration IS
    'Installation-local merchant-owned supplier configuration; Tencent may activate only after official onboarding while NuoNuo remains evidence-gated';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."capability_evidence_code" IS
    'Reviewed provider-contract gate; official_public_contract is valid only for Tencent ESS Basic';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."adapter_enabled" IS
    'True only for an active Tencent local subject backed by the reviewed public API contract';
