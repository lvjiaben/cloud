-- Purpose: Persists business records for idc_gateway_supplier_processing_state.
-- 用途：持久化 idc_gateway_supplier_processing_state 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_processing_state (
    "provider_code" VARCHAR(32) PRIMARY KEY,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'allowed',
    "operation_sha256" BYTEA,
    "updated_by_employee_no" VARCHAR(64) NOT NULL DEFAULT 'system:installation',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_processing_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_processing_status CHECK (
        "status_code" IN ('allowed', 'stopped')
    ),
    CONSTRAINT ck_gateway_processing_operation CHECK (
        ("status_code" = 'allowed' AND "operation_sha256" IS NULL)
        OR ("status_code" = 'stopped' AND octet_length("operation_sha256") = 32)
    ),
    CONSTRAINT ck_gateway_processing_employee CHECK (
        "updated_by_employee_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_processing_version CHECK ("version" > 0)
);

INSERT INTO idc_gateway_supplier_processing_state ("provider_code")
VALUES ('tencent_essbasic'), ('nuonuo_invoice')
ON CONFLICT ("provider_code") DO NOTHING;

ALTER TABLE idc_gateway_supplier_authorization_event
    ADD COLUMN IF NOT EXISTS "operation_sha256" BYTEA;

ALTER TABLE idc_gateway_supplier_authorization_event
    ALTER COLUMN "action_code" TYPE VARCHAR(24);

ALTER TABLE idc_gateway_supplier_authorization_event
    DROP CONSTRAINT IF EXISTS ck_gateway_supplier_auth_action;
ALTER TABLE idc_gateway_supplier_authorization_event
    ADD CONSTRAINT ck_gateway_supplier_auth_action CHECK (
        "action_code" IN (
            'configured', 'revoked', 'processing_stopped', 'processing_resumed'
        )
    );

ALTER TABLE idc_gateway_supplier_authorization_event
    DROP CONSTRAINT IF EXISTS ck_gateway_supplier_auth_operation;
ALTER TABLE idc_gateway_supplier_authorization_event
    ADD CONSTRAINT ck_gateway_supplier_auth_operation CHECK (
        "operation_sha256" IS NULL OR octet_length("operation_sha256") = 32
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_supplier_auth_operation
    ON idc_gateway_supplier_authorization_event (
        "provider_code", "action_code", "operation_sha256"
    ) WHERE "operation_sha256" IS NOT NULL;

-- Purpose: Persists business records for idc_gateway_retention_disposition.
-- 用途：持久化 idc_gateway_retention_disposition 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_retention_disposition (
    "disposition_no" VARCHAR(40) PRIMARY KEY,
    "data_class_code" VARCHAR(48) NOT NULL REFERENCES idc_gateway_data_lifecycle_policy ("data_class_code"),
    "deletion_mode" VARCHAR(24) NOT NULL,
    "outcome_code" VARCHAR(32) NOT NULL,
    selected_rows INTEGER NOT NULL,
    "selected_set_sha256" BYTEA NOT NULL,
    cutoff_at TIMESTAMPTZ NOT NULL,
    "employee_principal_no" VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_retention_disposition_no CHECK (
        "disposition_no" ~ '^lrd_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_retention_mode CHECK (
        "deletion_mode" IN ('delete', 'crypto_erase', 'retain_digest')
    ),
    CONSTRAINT ck_gateway_retention_outcome CHECK (
        "outcome_code" IN (
            'deleted', 'no_due_records', 'legal_hold',
            'incomplete_operation', 'supplier_active',
            'statutory_retained', 'crypto_erase_required'
        )
    ),
    CONSTRAINT ck_gateway_retention_count CHECK (
        selected_rows BETWEEN 0 AND 1000
    ),
    CONSTRAINT ck_gateway_retention_digest CHECK (
        octet_length("selected_set_sha256") = 32
    ),
    CONSTRAINT ck_gateway_retention_employee CHECK (
        "employee_principal_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_retention_history
    ON idc_gateway_retention_disposition (
        "data_class_code", occurred_at DESC, "disposition_no" DESC
    );

ALTER TABLE idc_gateway_data_lifecycle_policy
    DROP CONSTRAINT IF EXISTS ck_gateway_lifecycle_statutory;
ALTER TABLE idc_gateway_data_lifecycle_policy
    ADD CONSTRAINT ck_gateway_lifecycle_statutory CHECK (
        "data_class_code" NOT IN ('supplier_callback', 'supplier_evidence', 'audit')
        OR "deletion_mode" = 'retain_digest'
    );

CREATE OR REPLACE FUNCTION idc_gateway_guard_processing_state_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD.updated_at > NEW.updated_at
        OR NEW."version" <> OLD."version" + 1
        OR NOT (
            (OLD."status_code" = 'allowed' AND NEW."status_code" = 'stopped')
            OR (OLD."status_code" = 'stopped' AND NEW."status_code" = 'allowed')
        )
    THEN
        RAISE EXCEPTION 'supplier processing state transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_retention_projection_delete()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP <> 'DELETE'
        OR current_setting('linapro.lifecycle_executor', TRUE)
            IS DISTINCT FROM 'retention-v1'
    THEN
        RAISE EXCEPTION 'local lifecycle projection mutation is forbidden';
    END IF;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_processing_state_guard
    ON idc_gateway_supplier_processing_state;
CREATE TRIGGER trg_gateway_processing_state_guard
BEFORE UPDATE ON idc_gateway_supplier_processing_state
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_processing_state_update();

DROP TRIGGER IF EXISTS trg_gateway_processing_state_no_delete
    ON idc_gateway_supplier_processing_state;
CREATE TRIGGER trg_gateway_processing_state_no_delete
BEFORE DELETE ON idc_gateway_supplier_processing_state
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_retention_disposition_immutable
    ON idc_gateway_retention_disposition;
CREATE TRIGGER trg_gateway_retention_disposition_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_retention_disposition
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_supplier_usage_immutable
    ON idc_gateway_supplier_usage_snapshot;
CREATE TRIGGER trg_gateway_supplier_usage_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_usage_snapshot
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_retention_projection_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_alert_immutable
    ON idc_gateway_supplier_budget_alert;
CREATE TRIGGER trg_gateway_supplier_alert_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_budget_alert
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_retention_projection_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_recon_immutable
    ON idc_gateway_supplier_reconciliation;
CREATE TRIGGER trg_gateway_supplier_recon_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_reconciliation
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_retention_projection_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_sla_immutable
    ON idc_gateway_supplier_sla_snapshot;
CREATE TRIGGER trg_gateway_supplier_sla_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_sla_snapshot
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_retention_projection_delete();
