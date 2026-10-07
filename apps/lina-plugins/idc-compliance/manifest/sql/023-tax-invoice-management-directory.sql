-- Bounded staff directories introduced by task 10.12.g.

CREATE INDEX IF NOT EXISTS idx_idc_compliance_tax_invoice_management_directory
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "submitted_at" DESC, "id" DESC
    ) WHERE "submitted_at" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_compliance_tax_invoice_management_status
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "status_code", "submitted_at" DESC, "id" DESC
    ) WHERE "submitted_at" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_compliance_red_letter_management_directory
    ON plugin_idc_compliance_red_letter_task (
        "tenant_id", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_compliance_red_letter_management_status
    ON plugin_idc_compliance_red_letter_task (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );
