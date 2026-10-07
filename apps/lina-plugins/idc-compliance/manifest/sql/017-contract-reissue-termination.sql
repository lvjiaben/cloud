-- Immutable contract replacement lineage and independent reissue/termination operations.

DROP INDEX IF EXISTS uk_plugin_idc_compliance_contract_order;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_contract_order_history
    ON plugin_idc_compliance_contract (
        "tenant_id", "order_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_compliance_contract_replacement.
-- 用途：持久化 plugin_idc_compliance_contract_replacement 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_replacement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "replacement_no" VARCHAR(64) NOT NULL,
    "old_contract_no" VARCHAR(64) NOT NULL,
    "new_contract_no" VARCHAR(64) NOT NULL,
    "idempotency_sha256" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_contract_replacement_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_contract_replacement_distinct
        CHECK ("old_contract_no" <> "new_contract_no"),
    CONSTRAINT ck_idc_compliance_contract_replacement_idem
        CHECK ("idempotency_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT uk_plugin_idc_compliance_contract_replacement_no
        UNIQUE ("replacement_no"),
    CONSTRAINT uk_plugin_idc_compliance_contract_replacement_old
        UNIQUE ("tenant_id", "old_contract_no"),
    CONSTRAINT uk_plugin_idc_compliance_contract_replacement_new
        UNIQUE ("tenant_id", "new_contract_no"),
    CONSTRAINT uk_idc_compliance_contract_replacement_idem
        UNIQUE ("tenant_id", "idempotency_sha256"),
    CONSTRAINT fk_plugin_idc_compliance_contract_replacement_old
        FOREIGN KEY ("old_contract_no")
        REFERENCES plugin_idc_compliance_contract ("contract_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_contract_replacement_new
        FOREIGN KEY ("new_contract_no")
        REFERENCES plugin_idc_compliance_contract ("contract_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_contract_replacement_chain
    ON plugin_idc_compliance_contract_replacement (
        "tenant_id", "old_contract_no", "new_contract_no", "id"
    );

DROP TRIGGER IF EXISTS trg_idc_compliance_contract_replacement_append_only
    ON plugin_idc_compliance_contract_replacement;
CREATE TRIGGER trg_idc_compliance_contract_replacement_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_contract_replacement
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

COMMENT ON TABLE plugin_idc_compliance_contract_replacement IS
    'Append-only lineage between an immutable failed contract snapshot and its replacement preview';
