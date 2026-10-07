-- 015: Persist one evidence-backed operator takeover without creating any
-- Provider write, status verdict, retry, or secret-bearing free-text path.

-- Purpose: Persists business records for plugin_idc_resource_task_manual_takeover.
-- 用途：持久化 plugin_idc_resource_task_manual_takeover 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_task_manual_takeover (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "takeover_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "from_status_code" VARCHAR(24) NOT NULL,
    "task_version" BIGINT NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "evidence_count" SMALLINT NOT NULL,
    "assigned_user_id" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_task_takeover_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_from_status CHECK (
        "from_status_code" IN ('accepted', 'running', 'result_unknown')
    ),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_version
        CHECK ("task_version" > 0),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_reason CHECK (
        "reason_code" IN (
            'provider_result_unresolved',
            'provider_task_stalled',
            'resource_state_conflict',
            'external_coordination_required',
            'compensation_required'
        )
    ),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_digest CHECK (
        "request_digest" ~ '^[0-9a-f]{64}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_evidence
        CHECK ("evidence_count" BETWEEN 1 AND 16),
    CONSTRAINT ck_plugin_idc_resource_task_takeover_actor
        CHECK ("assigned_user_id" > 0),
    CONSTRAINT uk_plugin_idc_resource_task_takeover_no
        UNIQUE ("takeover_no"),
    CONSTRAINT uk_plugin_idc_resource_task_takeover_task
        UNIQUE ("task_no"),
    CONSTRAINT fk_plugin_idc_resource_task_takeover_task
        FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_resource_task ("task_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_task_takeover_actor
    ON plugin_idc_resource_task_manual_takeover (
        "tenant_id", "assigned_user_id", "created_at" DESC, "id" DESC
    );

CREATE OR REPLACE FUNCTION plugin_idc_resource_reject_task_evidence_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'resource task evidence is append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_resource_task_evidence_immutable
    ON plugin_idc_resource_task_evidence;
CREATE TRIGGER trg_plugin_idc_resource_task_evidence_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_resource_task_evidence
FOR EACH ROW EXECUTE FUNCTION
    plugin_idc_resource_reject_task_evidence_mutation();

CREATE OR REPLACE FUNCTION plugin_idc_resource_reject_task_takeover_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'resource task manual takeover records are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_resource_task_takeover_immutable
    ON plugin_idc_resource_task_manual_takeover;
CREATE TRIGGER trg_plugin_idc_resource_task_takeover_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_resource_task_manual_takeover
FOR EACH ROW EXECUTE FUNCTION
    plugin_idc_resource_reject_task_takeover_mutation();

COMMENT ON TABLE plugin_idc_resource_task_manual_takeover
    IS 'Append-only evidence-backed assignment that terminates automatic Provider task handling without deciding upstream success or failure';
COMMENT ON COLUMN plugin_idc_resource_task_manual_takeover."request_digest"
    IS 'Canonical digest of actor, task version, reason, and sorted evidence numbers for exact safe replay';
COMMENT ON COLUMN plugin_idc_resource_task_manual_takeover."evidence_digest"
    IS 'Canonical digest of immutable task evidence numbers and their redacted contents';
