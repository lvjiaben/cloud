-- Purpose: Persists business records for idc_gateway_local_business_request.
-- 用途：持久化 idc_gateway_local_business_request 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_local_business_request (
    "request_no" VARCHAR(40) PRIMARY KEY,
    "installation_id" VARCHAR(70) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "method" VARCHAR(8) NOT NULL,
    "request_target_sha256" BYTEA NOT NULL,
    "idempotency_hmac" BYTEA NOT NULL,
    "business_content_sha256" BYTEA NOT NULL,
    "body_sha256" BYTEA NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'registered',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_local_business_request_idempotency UNIQUE (
        "installation_id", "idempotency_hmac"
    ),
    CONSTRAINT uq_gateway_local_business_request_owner UNIQUE (
        "request_no", "installation_id"
    ),
    CONSTRAINT ck_gateway_local_business_request_no CHECK (
        "request_no" ~ '^lbr_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_business_request_installation CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_local_business_request_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9.:-]{1,63}$'
    ),
    CONSTRAINT ck_gateway_local_business_request_method CHECK (
        "method" IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')
    ),
    CONSTRAINT ck_gateway_local_business_request_digest CHECK (
        octet_length("request_target_sha256") = 32
        AND octet_length("idempotency_hmac") = 32
        AND octet_length("business_content_sha256") = 32
        AND octet_length("body_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_business_request_status CHECK (
        "status_code" IN ('registered', 'accepted', 'succeeded', 'failed',
                        'result_unknown', 'manual_takeover')
    )
);

-- Purpose: Persists business records for idc_gateway_process_nonce.
-- 用途：持久化 idc_gateway_process_nonce 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_process_nonce (
    "installation_id" VARCHAR(70) NOT NULL,
    "nonce_sha256" BYTEA NOT NULL,
    "frame_sha256" BYTEA NOT NULL,
    "request_no" VARCHAR(40),
    "key_id" VARCHAR(20) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("installation_id", "nonce_sha256"),
    CONSTRAINT fk_gateway_process_nonce_request FOREIGN KEY (
        "request_no", "installation_id"
    ) REFERENCES idc_gateway_local_business_request (
        "request_no", "installation_id"
    ),
    CONSTRAINT ck_gateway_process_nonce_installation CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_process_nonce_key CHECK ("key_id" ~ '^lgk_[0-9a-f]{16}$'),
    CONSTRAINT ck_gateway_process_nonce_digest CHECK (
        octet_length("nonce_sha256") = 32 AND octet_length("frame_sha256") = 32
    ),
    CONSTRAINT ck_gateway_process_nonce_expiry CHECK (expires_at > consumed_at)
);

CREATE INDEX IF NOT EXISTS idx_gateway_process_nonce_expiry
    ON idc_gateway_process_nonce (expires_at);

-- Purpose: Persists business records for idc_gateway_supplier_operation.
-- 用途：持久化 idc_gateway_supplier_operation 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_operation (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_kind" VARCHAR(48) NOT NULL,
    "idempotency_hmac" BYTEA NOT NULL UNIQUE,
    "request_sha256" BYTEA NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "external_ref_hmac" BYTEA,
    "attempt_count" INTEGER NOT NULL DEFAULT 0,
    "last_stable_code" VARCHAR(64),
    observed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_local_operation_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_operation_kind CHECK ("operation_kind" ~ '^[a-z][a-z0-9_.-]{1,47}$'),
    CONSTRAINT ck_gateway_local_operation_digest CHECK (
        octet_length("idempotency_hmac") = 32 AND octet_length("request_sha256") = 32 AND
        ("external_ref_hmac" IS NULL OR octet_length("external_ref_hmac") = 32)
    ),
    CONSTRAINT ck_gateway_local_operation_status CHECK (
        "status_code" IN ('pending', 'accepted', 'succeeded', 'failed', 'result_unknown', 'manual_takeover')
    ),
    CONSTRAINT ck_gateway_local_operation_attempt CHECK ("attempt_count" >= 0 AND "attempt_count" <= 32),
    CONSTRAINT ck_gateway_local_operation_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_local_operation_time CHECK (updated_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_gateway_local_operation_pending
    ON idc_gateway_supplier_operation ("provider_code", "status_code", updated_at, "operation_no");

-- Purpose: Persists business records for idc_gateway_supplier_evidence.
-- 用途：持久化 idc_gateway_supplier_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_evidence (
    "evidence_no" VARCHAR(40) PRIMARY KEY,
    "operation_no" VARCHAR(40) NOT NULL REFERENCES idc_gateway_supplier_operation ("operation_no"),
    "evidence_kind" VARCHAR(48) NOT NULL,
    "event_hmac" BYTEA NOT NULL UNIQUE,
    "payload_sha256" BYTEA NOT NULL,
    "payload_ciphertext" BYTEA NOT NULL,
    "key_version" VARCHAR(32) NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_local_evidence_kind CHECK ("evidence_kind" ~ '^[a-z][a-z0-9_.-]{1,47}$'),
    CONSTRAINT ck_gateway_local_evidence_digest CHECK (
        octet_length("event_hmac") = 32 AND octet_length("payload_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_evidence_payload CHECK (
        octet_length("payload_ciphertext") BETWEEN 17 AND 1048576
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_local_evidence_operation
    ON idc_gateway_supplier_evidence ("operation_no", observed_at, "evidence_no");

-- Purpose: Persists business records for idc_gateway_supplier_callback.
-- 用途：持久化 idc_gateway_supplier_callback 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_callback (
    "callback_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "callback_event_hmac" BYTEA NOT NULL UNIQUE,
    "operation_no" VARCHAR(40) REFERENCES idc_gateway_supplier_operation ("operation_no"),
    "signature_status" VARCHAR(16) NOT NULL,
    "reconciliation_status" VARCHAR(24) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reconciled_at TIMESTAMPTZ,
    CONSTRAINT ck_gateway_local_callback_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_callback_digest CHECK (octet_length("callback_event_hmac") = 32),
    CONSTRAINT ck_gateway_local_callback_signature CHECK ("signature_status" IN ('verified', 'rejected')),
    CONSTRAINT ck_gateway_local_callback_reconciliation CHECK (
        "reconciliation_status" IN ('pending', 'applied', 'duplicate', 'rejected', 'manual_review')
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_circuit.
-- 用途：持久化 idc_gateway_supplier_circuit 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_circuit (
    "provider_code" VARCHAR(32) PRIMARY KEY,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'closed',
    "consecutive_failures" INTEGER NOT NULL DEFAULT 0,
    opened_until TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_local_circuit_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_circuit_status CHECK ("status_code" IN ('closed', 'open', 'half_open')),
    CONSTRAINT ck_gateway_local_circuit_failures CHECK ("consecutive_failures" >= 0),
    CONSTRAINT ck_gateway_local_circuit_version CHECK ("version" > 0)
);

-- Purpose: Persists business records for idc_gateway_supplier_takeover.
-- 用途：持久化 idc_gateway_supplier_takeover 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_takeover (
    "takeover_no" VARCHAR(40) PRIMARY KEY,
    "operation_no" VARCHAR(40) NOT NULL UNIQUE REFERENCES idc_gateway_supplier_operation ("operation_no"),
    "employee_principal_no" VARCHAR(64) NOT NULL,
    "reason_code" VARCHAR(48) NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    full_auth_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_local_takeover_employee CHECK (
        "employee_principal_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_local_takeover_reason CHECK ("reason_code" ~ '^[a-z][a-z0-9_.-]{1,47}$'),
    CONSTRAINT ck_gateway_local_takeover_digest CHECK (octet_length("evidence_set_sha256") = 32),
    CONSTRAINT ck_gateway_local_takeover_auth CHECK (full_auth_at <= created_at)
);

CREATE OR REPLACE FUNCTION idc_gateway_reject_local_immutable_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'local gateway immutable history cannot be changed';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_local_evidence_immutable
    ON idc_gateway_supplier_evidence;
CREATE TRIGGER trg_gateway_local_evidence_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_evidence
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_local_callback_no_delete
    ON idc_gateway_supplier_callback;
CREATE TRIGGER trg_gateway_local_callback_no_delete
    BEFORE DELETE ON idc_gateway_supplier_callback
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_local_takeover_immutable
    ON idc_gateway_supplier_takeover;
CREATE TRIGGER trg_gateway_local_takeover_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_takeover
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
