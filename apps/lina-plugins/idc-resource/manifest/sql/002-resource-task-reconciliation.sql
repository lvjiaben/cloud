-- Purpose: Persists business records for plugin_idc_resource_task_evidence.
-- 用途：持久化 plugin_idc_resource_task_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_task_evidence (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "evidence_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "source_kind" VARCHAR(24) NOT NULL,
    "evidence_kind" VARCHAR(128) NOT NULL,
    "reference" VARCHAR(512) NOT NULL DEFAULT '',
    "summary" VARCHAR(4096) NOT NULL DEFAULT '',
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_task_evidence_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_task_evidence_source CHECK (
        "source_kind" IN ('provider_task', 'provider_resource', 'owner_resource_state')
    ),
    CONSTRAINT ck_plugin_idc_resource_task_evidence_kind CHECK (
        "evidence_kind" = btrim("evidence_kind") AND length("evidence_kind") > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_task_evidence_content CHECK (
        "reference" = btrim("reference") AND
        "summary" = btrim("summary") AND
        (length("reference") > 0 OR length("summary") > 0)
    ),
    CONSTRAINT uk_plugin_idc_resource_task_evidence_no UNIQUE ("evidence_no"),
    CONSTRAINT fk_plugin_idc_resource_task_evidence_task FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_resource_task ("task_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_evidence_task
    ON plugin_idc_resource_task_evidence (
        "tenant_id", "task_no", "observed_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_resource_task_evidence IS 'Immutable redacted evidence supporting Provider task reconciliation';
COMMENT ON COLUMN plugin_idc_resource_task_evidence."reference" IS 'Bounded upstream task or resource reference; never a secret handle or raw payload';
COMMENT ON COLUMN plugin_idc_resource_task_evidence."summary" IS 'Bounded credential-scanned Provider or owner summary; never a raw response';
