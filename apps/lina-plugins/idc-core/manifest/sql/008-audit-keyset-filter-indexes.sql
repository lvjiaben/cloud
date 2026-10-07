-- 008: Exact keyset ordering for every supported audit timeline filter
--
-- Audit search always orders by occurred_at DESC, id DESC. Each selective
-- filter index carries the same final tie-breaker so equal timestamps can
-- satisfy bounded pages without an incremental sort or an unbounded scan.

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_actor;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_actor
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "actor_user_id", "occurred_at" DESC, "id" DESC)
    WHERE "actor_user_id" IS NOT NULL;

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_acting;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_acting
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "acting_user_id", "occurred_at" DESC, "id" DESC)
    WHERE "acting_user_id" IS NOT NULL;

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_correlation;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_correlation
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "correlation_no", "occurred_at" DESC, "id" DESC)
    WHERE "correlation_no" <> '';

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_operation;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_operation
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "operation_code", "occurred_at" DESC, "id" DESC);

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_result;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_result
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "result_code", "occurred_at" DESC, "id" DESC);

DROP INDEX IF EXISTS idx_plugin_idc_core_audit_risk;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_risk
    ON plugin_idc_core_business_audit_event
    ("tenant_id", "risk_code", "occurred_at" DESC, "id" DESC);
