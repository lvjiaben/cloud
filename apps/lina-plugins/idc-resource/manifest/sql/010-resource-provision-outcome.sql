-- 010: Persist the resource owner's idempotent receipt only after core has
-- durably applied a provisioning outcome. Credential lifecycle remains local.

-- Purpose: Persists business records for plugin_idc_resource_provision_outcome_receipt.
-- 用途：持久化 plugin_idc_resource_provision_outcome_receipt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provision_outcome_receipt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "receipt_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "service_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "task_evidence_digest" CHAR(64) NOT NULL,
    "core_evidence_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_provision_outcome_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_provision_outcome_status CHECK (
        "status_code" IN ('succeeded', 'failed', 'manual_review')
    ),
    CONSTRAINT ck_plugin_idc_resource_provision_outcome_digest CHECK (
        "request_digest" ~ '^[0-9a-f]{64}$'
        AND "task_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "core_evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_resource_provision_outcome_no
        UNIQUE ("receipt_no"),
    CONSTRAINT uk_plugin_idc_resource_provision_outcome_task
        UNIQUE ("task_no"),
    CONSTRAINT fk_plugin_idc_resource_provision_outcome_task
        FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_resource_task ("task_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_provision_outcome_service
    ON plugin_idc_resource_provision_outcome_receipt (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

CREATE OR REPLACE FUNCTION plugin_idc_resource_reject_provision_outcome_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'resource provisioning outcome receipts are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_resource_provision_outcome_immutable
    ON plugin_idc_resource_provision_outcome_receipt;
CREATE TRIGGER trg_plugin_idc_resource_provision_outcome_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_resource_provision_outcome_receipt
FOR EACH ROW EXECUTE FUNCTION
    plugin_idc_resource_reject_provision_outcome_mutation();

COMMENT ON TABLE plugin_idc_resource_provision_outcome_receipt
    IS 'Append-only resource-owner receipt of one core-persisted provisioning outcome';
COMMENT ON COLUMN plugin_idc_resource_provision_outcome_receipt."core_evidence_digest"
    IS 'Secret-free evidence produced after core commits its service and fulfillment state';
