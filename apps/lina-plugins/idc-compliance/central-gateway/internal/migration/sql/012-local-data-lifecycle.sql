-- Purpose: Persists business records for idc_gateway_data_lifecycle_policy.
-- 用途：持久化 idc_gateway_data_lifecycle_policy 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_data_lifecycle_policy (
    "data_class_code" VARCHAR(48) PRIMARY KEY,
    "retention_days" INTEGER NOT NULL,
    "deletion_mode" VARCHAR(24) NOT NULL,
    "legal_basis_code" VARCHAR(48) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    "updated_by_employee_no" VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_lifecycle_class CHECK (
        "data_class_code" IN (
            'supplier_callback', 'supplier_evidence', 'supplier_usage',
            'supplier_cost', 'supplier_sla', 'process_nonce', 'audit'
        )
    ),
    CONSTRAINT ck_gateway_lifecycle_retention CHECK ("retention_days" BETWEEN 1 AND 36500),
    CONSTRAINT ck_gateway_lifecycle_deletion CHECK (
        "deletion_mode" IN ('delete', 'crypto_erase', 'retain_digest')
    ),
    CONSTRAINT ck_gateway_lifecycle_basis CHECK (
        "legal_basis_code" ~ '^[a-z][a-z0-9_.-]{1,47}$'
    ),
    CONSTRAINT ck_gateway_lifecycle_employee CHECK (
        "updated_by_employee_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_lifecycle_version CHECK ("version" > 0)
);

-- Purpose: Persists business records for idc_gateway_legal_hold_event.
-- 用途：持久化 idc_gateway_legal_hold_event 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_legal_hold_event (
    "hold_event_no" VARCHAR(40) PRIMARY KEY,
    "hold_no" VARCHAR(40) NOT NULL,
    "data_class_code" VARCHAR(48) NOT NULL REFERENCES idc_gateway_data_lifecycle_policy ("data_class_code"),
    "aggregate_ref_hmac" BYTEA,
    "action_code" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(48) NOT NULL,
    "employee_principal_no" VARCHAR(64) NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_legal_hold_ref CHECK (
        "aggregate_ref_hmac" IS NULL OR octet_length("aggregate_ref_hmac") = 32
    ),
    CONSTRAINT ck_gateway_legal_hold_action CHECK ("action_code" IN ('placed', 'released')),
    CONSTRAINT ck_gateway_legal_hold_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9_.-]{1,47}$'
    ),
    CONSTRAINT ck_gateway_legal_hold_employee CHECK (
        "employee_principal_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT uq_gateway_legal_hold_action UNIQUE ("hold_no", "action_code")
);

CREATE INDEX IF NOT EXISTS idx_gateway_legal_hold_scope
    ON idc_gateway_legal_hold_event (
        "data_class_code", "aggregate_ref_hmac", occurred_at DESC, "hold_event_no"
    );

-- Purpose: Persists business records for idc_gateway_evidence_export.
-- 用途：持久化 idc_gateway_evidence_export 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_evidence_export (
    "export_no" VARCHAR(40) PRIMARY KEY,
    "requested_by_employee_no" VARCHAR(64) NOT NULL,
    "scope_sha256" BYTEA NOT NULL,
    "manifest_sha256" BYTEA,
    "status_code" VARCHAR(24) NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_evidence_export_employee CHECK (
        "requested_by_employee_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_evidence_export_digest CHECK (
        octet_length("scope_sha256") = 32 AND
        ("manifest_sha256" IS NULL OR octet_length("manifest_sha256") = 32)
    ),
    CONSTRAINT ck_gateway_evidence_export_status CHECK (
        "status_code" IN ('completed', 'failed')
    ),
    CONSTRAINT ck_gateway_evidence_export_time CHECK (
        completed_at >= requested_at
    ),
    CONSTRAINT ck_gateway_evidence_export_expiry CHECK (
        expires_at IS NULL OR expires_at > completed_at
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_authorization_event.
-- 用途：持久化 idc_gateway_supplier_authorization_event 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_authorization_event (
    "authorization_event_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "action_code" VARCHAR(16) NOT NULL,
    "employee_principal_no" VARCHAR(64) NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_auth_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_auth_action CHECK (
        "action_code" IN ('configured', 'revoked', 'processing_stopped')
    ),
    CONSTRAINT ck_gateway_supplier_auth_employee CHECK (
        "employee_principal_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_supplier_auth_digest CHECK (octet_length("evidence_set_sha256") = 32)
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_auth_current
    ON idc_gateway_supplier_authorization_event (
        "provider_code", occurred_at DESC, "authorization_event_no"
    );

-- Purpose: Persists business records for idc_gateway_restore_verification.
-- 用途：持久化 idc_gateway_restore_verification 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_restore_verification (
    "restore_verification_no" VARCHAR(40) PRIMARY KEY,
    "backup_manifest_sha256" BYTEA NOT NULL,
    database_verified BOOLEAN NOT NULL,
    keyring_verified BOOLEAN NOT NULL,
    evidence_integrity_verified BOOLEAN NOT NULL,
    supplier_writes_blocked BOOLEAN NOT NULL DEFAULT TRUE,
    "verified_by_employee_no" VARCHAR(64) NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_restore_manifest CHECK (octet_length("backup_manifest_sha256") = 32),
    CONSTRAINT ck_gateway_restore_fail_closed CHECK (supplier_writes_blocked = TRUE),
    CONSTRAINT ck_gateway_restore_employee CHECK (
        "verified_by_employee_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    )
);

CREATE TRIGGER trg_gateway_legal_hold_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_legal_hold_event
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_evidence_export_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_evidence_export
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_supplier_auth_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_authorization_event
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_restore_verification_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_restore_verification
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
