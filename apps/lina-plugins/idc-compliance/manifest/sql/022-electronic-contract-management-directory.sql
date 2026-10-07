-- Adds bounded management-directory access paths introduced by task 10.12.f.

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_management_directory
    ON plugin_idc_compliance_contract (
        "tenant_id", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_management_status
    ON plugin_idc_compliance_contract (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_archive_job_contract
    ON plugin_idc_compliance_contract_archive_job (
        "tenant_id", "contract_no", "created_at" DESC, "id" DESC
    );
