-- Purpose: Persists business records for idc_gateway_business_request.
-- 用途：持久化 idc_gateway_business_request 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_business_request (
    "request_no" VARCHAR(40) PRIMARY KEY,
    "instance_no" VARCHAR(40) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "idempotency_hmac" BYTEA NOT NULL,
    "business_request_sha256" BYTEA NOT NULL,
    "status" VARCHAR(24) NOT NULL DEFAULT 'accepted',
    "result_ref" VARCHAR(128),
    "result_code" VARCHAR(64),
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    CONSTRAINT fk_gateway_business_request_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT uq_gateway_business_request_idempotency UNIQUE ("instance_no", "idempotency_hmac"),
    CONSTRAINT ck_gateway_business_request_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_business_request_idempotency CHECK (octet_length("idempotency_hmac") = 32),
    CONSTRAINT ck_gateway_business_request_digest CHECK (octet_length("business_request_sha256") = 32),
    CONSTRAINT ck_gateway_business_request_status CHECK (
        "status" IN ('accepted', 'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover')
    ),
    CONSTRAINT ck_gateway_business_request_lifecycle CHECK (
        ("status" IN ('accepted', 'running', 'result_unknown') AND completed_at IS NULL
            AND "result_ref" IS NULL AND "result_code" IS NULL)
        OR ("status" = 'succeeded' AND completed_at IS NOT NULL
            AND "result_ref" IS NOT NULL AND "result_code" IS NULL)
        OR ("status" = 'failed' AND completed_at IS NOT NULL AND "result_code" IS NOT NULL)
        OR ("status" = 'manual_takeover' AND completed_at IS NOT NULL
            AND "result_code" = 'manual_takeover')
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_business_request_status
    ON idc_gateway_business_request ("status", accepted_at, "request_no");

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_business_request_owner
    ON idc_gateway_business_request ("request_no", "instance_no", "operation_scope");

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_instance_key_owner
    ON idc_gateway_instance_key ("instance_no", "key_no");

-- Purpose: Persists business records for idc_gateway_request_nonce.
-- 用途：持久化 idc_gateway_request_nonce 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_request_nonce (
    "instance_no" VARCHAR(40) NOT NULL,
    "nonce_sha256" BYTEA NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "key_no" VARCHAR(40) NOT NULL,
    "signed_request_sha256" BYTEA NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NOT NULL,
    purge_after TIMESTAMPTZ NOT NULL,
    PRIMARY KEY ("instance_no", "nonce_sha256"),
    CONSTRAINT fk_gateway_request_nonce_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT fk_gateway_request_nonce_request FOREIGN KEY ("request_no")
        REFERENCES idc_gateway_business_request ("request_no"),
    CONSTRAINT fk_gateway_request_nonce_key FOREIGN KEY ("instance_no", "key_no")
        REFERENCES idc_gateway_instance_key ("instance_no", "key_no"),
    CONSTRAINT ck_gateway_request_nonce_digest CHECK (octet_length("nonce_sha256") = 32),
    CONSTRAINT ck_gateway_request_nonce_request_digest CHECK (
        octet_length("signed_request_sha256") = 32
    ),
    CONSTRAINT ck_gateway_request_nonce_lifecycle CHECK (
        expires_at > observed_at - INTERVAL '30 seconds'
        AND purge_after >= expires_at + INTERVAL '24 hours'
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_request_nonce_purge
    ON idc_gateway_request_nonce (purge_after, "instance_no", "nonce_sha256");

-- Purpose: Persists business records for idc_gateway_rate_limit_window.
-- 用途：持久化 idc_gateway_rate_limit_window 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_rate_limit_window (
    "dimension_kind" VARCHAR(16) NOT NULL,
    "instance_no" VARCHAR(40) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    window_seconds INTEGER NOT NULL,
    limit_count INTEGER NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    "request_count" INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("dimension_kind", "instance_no", "operation_scope"),
    CONSTRAINT fk_gateway_rate_limit_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT ck_gateway_rate_limit_dimension CHECK (
        ("dimension_kind" = 'instance' AND "operation_scope" = '')
        OR ("dimension_kind" = 'operation'
            AND "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$')
    ),
    CONSTRAINT ck_gateway_rate_limit_policy CHECK (
        "policy_version" > 0 AND window_seconds BETWEEN 1 AND 3600
        AND limit_count BETWEEN 1 AND 100000
        AND "request_count" BETWEEN 0 AND limit_count
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_circuit.
-- 用途：持久化 idc_gateway_supplier_circuit 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_circuit (
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "failure_threshold" INTEGER NOT NULL,
    open_seconds INTEGER NOT NULL,
    probe_timeout_seconds INTEGER NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'closed',
    "consecutive_failures" INTEGER NOT NULL DEFAULT 0,
    open_until TIMESTAMPTZ,
    "probe_attempt_no" VARCHAR(40),
    "revision" BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("provider_code", "operation_scope"),
    CONSTRAINT ck_gateway_supplier_circuit_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_supplier_circuit_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_supplier_circuit_policy CHECK (
        "policy_version" > 0 AND "failure_threshold" BETWEEN 1 AND 100
        AND open_seconds BETWEEN 1 AND 3600
        AND probe_timeout_seconds BETWEEN 1 AND 600
        AND "consecutive_failures" >= 0 AND "revision" > 0
    ),
    CONSTRAINT ck_gateway_supplier_circuit_lifecycle CHECK (
        ("status" = 'closed' AND open_until IS NULL AND "probe_attempt_no" IS NULL)
        OR ("status" = 'open' AND open_until IS NOT NULL AND "probe_attempt_no" IS NULL)
        OR ("status" = 'half_open' AND open_until IS NOT NULL AND "probe_attempt_no" IS NOT NULL)
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_write_attempt.
-- 用途：持久化 idc_gateway_supplier_write_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_write_attempt (
    "attempt_no" VARCHAR(40) PRIMARY KEY,
    "request_no" VARCHAR(40) NOT NULL,
    "instance_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "circuit_revision" BIGINT NOT NULL,
    probe BOOLEAN NOT NULL DEFAULT FALSE,
    "status" VARCHAR(16) NOT NULL DEFAULT 'admitted',
    "result_class" VARCHAR(32),
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    observed_at TIMESTAMPTZ,
    CONSTRAINT fk_gateway_supplier_attempt_request FOREIGN KEY (
        "request_no", "instance_no", "operation_scope"
    ) REFERENCES idc_gateway_business_request (
        "request_no", "instance_no", "operation_scope"
    ),
    CONSTRAINT uq_gateway_supplier_attempt_request UNIQUE ("request_no"),
    CONSTRAINT uq_gateway_supplier_attempt_route_owner UNIQUE (
        "attempt_no", "request_no", "instance_no", "provider_code", "operation_scope"
    ),
    CONSTRAINT ck_gateway_supplier_attempt_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_supplier_attempt_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_supplier_attempt_revision CHECK ("circuit_revision" > 0),
    CONSTRAINT ck_gateway_supplier_attempt_lifecycle CHECK (
        ("status" = 'admitted' AND "result_class" IS NULL AND observed_at IS NULL)
        OR ("status" = 'observed' AND "result_class" IN (
            'succeeded', 'definitive_failure', 'result_unknown'
        ) AND observed_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_attempt_pending
    ON idc_gateway_supplier_write_attempt (
        "provider_code", "operation_scope", "status", admitted_at, "attempt_no"
    );

CREATE OR REPLACE FUNCTION idc_gateway_reject_request_nonce_update()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway request nonces are immutable';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_business_request_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."idempotency_hmac" IS DISTINCT FROM NEW."idempotency_hmac"
        OR OLD."business_request_sha256" IS DISTINCT FROM NEW."business_request_sha256"
        OR OLD.accepted_at IS DISTINCT FROM NEW.accepted_at
        OR NOT (
            (OLD."status" = 'accepted' AND NEW."status" IN ('running', 'failed', 'result_unknown'))
            OR (OLD."status" = 'running'
                AND NEW."status" IN ('succeeded', 'failed', 'result_unknown', 'manual_takeover'))
            OR (OLD."status" = 'result_unknown'
                AND NEW."status" IN ('running', 'succeeded', 'failed', 'manual_takeover'))
        )
    THEN
        RAISE EXCEPTION 'gateway business request transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_supplier_attempt_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."circuit_revision" IS DISTINCT FROM NEW."circuit_revision"
        OR OLD.probe IS DISTINCT FROM NEW.probe
        OR OLD.admitted_at IS DISTINCT FROM NEW.admitted_at
        OR OLD."status" <> 'admitted'
        OR NEW."status" <> 'observed'
    THEN
        RAISE EXCEPTION 'gateway supplier attempt transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_supplier_circuit_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR NOT (
            (OLD."status" = 'closed' AND NEW."status" IN ('closed', 'open'))
            OR (OLD."status" = 'open' AND NEW."status" IN ('open', 'half_open'))
            OR (OLD."status" = 'half_open' AND NEW."status" IN ('closed', 'open'))
        )
    THEN
        RAISE EXCEPTION 'gateway supplier circuit transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_request_nonce_immutable ON idc_gateway_request_nonce;
CREATE TRIGGER trg_gateway_request_nonce_immutable
BEFORE UPDATE ON idc_gateway_request_nonce
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_request_nonce_update();

DROP TRIGGER IF EXISTS trg_gateway_business_request_guard_update ON idc_gateway_business_request;
CREATE TRIGGER trg_gateway_business_request_guard_update
BEFORE UPDATE ON idc_gateway_business_request
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_business_request_update();

DROP TRIGGER IF EXISTS trg_gateway_business_request_no_delete ON idc_gateway_business_request;
CREATE TRIGGER trg_gateway_business_request_no_delete
BEFORE DELETE ON idc_gateway_business_request
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_attempt_no_delete ON idc_gateway_supplier_write_attempt;
CREATE TRIGGER trg_gateway_supplier_attempt_no_delete
BEFORE DELETE ON idc_gateway_supplier_write_attempt
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_attempt_guard_update ON idc_gateway_supplier_write_attempt;
CREATE TRIGGER trg_gateway_supplier_attempt_guard_update
BEFORE UPDATE ON idc_gateway_supplier_write_attempt
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_supplier_attempt_update();

DROP TRIGGER IF EXISTS trg_gateway_supplier_circuit_no_delete ON idc_gateway_supplier_circuit;
CREATE TRIGGER trg_gateway_supplier_circuit_no_delete
BEFORE DELETE ON idc_gateway_supplier_circuit
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_circuit_guard_update ON idc_gateway_supplier_circuit;
CREATE TRIGGER trg_gateway_supplier_circuit_guard_update
BEFORE UPDATE ON idc_gateway_supplier_circuit
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_supplier_circuit_update();
