-- Phase 10 customer tax-invoice history directory.
-- This index matches the customer-owned keyset order and avoids filtering all
-- statuses through the work-queue index.

CREATE INDEX IF NOT EXISTS idx_idc_compliance_tax_invoice_customer_submitted
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "customer_no", "submitted_at" DESC, "id" DESC
    ) WHERE "submitted_at" IS NOT NULL;
