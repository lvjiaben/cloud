-- The former central-owner guard tables were retired by migration 008. This
-- migration creates the installation-local write guard without restoring any
-- sales-platform identity, remote instance registry, or cross-customer route.

DROP TABLE IF EXISTS idc_gateway_supplier_circuit CASCADE;

-- Purpose: Persists business records for idc_gateway_local_rate_limit_window.
-- 用途：持久化 idc_gateway_local_rate_limit_window 的业务记录。
CREATE TABLE idc_gateway_local_rate_limit_window (
    "dimension_kind" VARCHAR(16) NOT NULL,
    "installation_id" VARCHAR(70) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    window_seconds INTEGER NOT NULL,
    limit_count INTEGER NOT NULL,
    window_started_at TIMESTAMPTZ NOT NULL,
    "request_count" INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("dimension_kind", "installation_id", "operation_scope"),
    CONSTRAINT fk_gateway_local_rate_instance FOREIGN KEY ("installation_id")
        REFERENCES idc_gateway_local_supplier_instance ("installation_id"),
    CONSTRAINT ck_gateway_local_rate_dimension CHECK (
        ("dimension_kind" = 'installation' AND "operation_scope" = '')
        OR ("dimension_kind" = 'operation'
            AND "operation_scope" ~ '^[a-z][a-z0-9.:-]{1,63}$')
    ),
    CONSTRAINT ck_gateway_local_rate_policy CHECK (
        "policy_version" > 0
        AND window_seconds BETWEEN 1 AND 3600
        AND limit_count BETWEEN 1 AND 100000
        AND "request_count" BETWEEN 0 AND limit_count
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_circuit.
-- 用途：持久化 idc_gateway_supplier_circuit 的业务记录。
CREATE TABLE idc_gateway_supplier_circuit (
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "sample_window" SMALLINT NOT NULL,
    "minimum_samples" SMALLINT NOT NULL,
    "failure_threshold" SMALLINT NOT NULL,
    "unknown_threshold" SMALLINT NOT NULL,
    "consecutive_failure_threshold" SMALLINT NOT NULL,
    open_seconds INTEGER NOT NULL,
    probe_timeout_seconds INTEGER NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'closed',
    "total_samples" SMALLINT NOT NULL DEFAULT 0,
    "failure_samples" SMALLINT NOT NULL DEFAULT 0,
    "unknown_samples" SMALLINT NOT NULL DEFAULT 0,
    "consecutive_failures" SMALLINT NOT NULL DEFAULT 0,
    open_until TIMESTAMPTZ,
    "probe_attempt_no" VARCHAR(40),
    probe_expires_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY ("provider_code", "operation_scope"),
    CONSTRAINT ck_gateway_local_circuit_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_circuit_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9.:-]{1,63}$'
    ),
    CONSTRAINT ck_gateway_local_circuit_policy CHECK (
        "policy_version" > 0
        AND "sample_window" BETWEEN 5 AND 100
        AND "minimum_samples" BETWEEN 1 AND "sample_window"
        AND "failure_threshold" BETWEEN 1 AND "sample_window"
        AND "unknown_threshold" BETWEEN 1 AND "sample_window"
        AND "consecutive_failure_threshold" BETWEEN 1 AND "sample_window"
        AND open_seconds BETWEEN 60 AND 86400
        AND probe_timeout_seconds BETWEEN 1 AND 600
        AND "version" > 0
    ),
    CONSTRAINT ck_gateway_local_circuit_counters CHECK (
        "total_samples" BETWEEN 0 AND "sample_window"
        AND "failure_samples" BETWEEN 0 AND "total_samples"
        AND "unknown_samples" BETWEEN 0 AND "total_samples"
        AND "consecutive_failures" BETWEEN 0 AND "total_samples"
        AND "failure_samples" + "unknown_samples" <= "total_samples"
    ),
    CONSTRAINT ck_gateway_local_circuit_lifecycle CHECK (
        ("status_code" = 'closed' AND open_until IS NULL
            AND "probe_attempt_no" IS NULL AND probe_expires_at IS NULL)
        OR ("status_code" = 'open' AND open_until IS NOT NULL
            AND "probe_attempt_no" IS NULL AND probe_expires_at IS NULL)
        OR ("status_code" = 'half_open' AND open_until IS NULL
            AND "probe_attempt_no" IS NOT NULL AND probe_expires_at IS NOT NULL)
    )
);

-- Purpose: Persists business records for idc_gateway_local_supplier_write_attempt.
-- 用途：持久化 idc_gateway_local_supplier_write_attempt 的业务记录。
CREATE TABLE idc_gateway_local_supplier_write_attempt (
    "attempt_no" VARCHAR(40) PRIMARY KEY,
    "request_no" VARCHAR(40) NOT NULL,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "circuit_version" BIGINT NOT NULL,
    probe BOOLEAN NOT NULL DEFAULT FALSE,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'admitted',
    "result_class" VARCHAR(32),
    "stable_code_sha256" BYTEA,
    elapsed_milliseconds INTEGER,
    admitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    observed_at TIMESTAMPTZ,
    CONSTRAINT fk_gateway_local_attempt_request FOREIGN KEY (
        "request_no", "installation_id"
    ) REFERENCES idc_gateway_local_business_request (
        "request_no", "installation_id"
    ),
    CONSTRAINT uq_gateway_local_attempt_request UNIQUE ("request_no"),
    CONSTRAINT uq_gateway_local_attempt_owner UNIQUE (
        "attempt_no", "provider_code", "operation_scope"
    ),
    CONSTRAINT ck_gateway_local_attempt_no CHECK (
        "attempt_no" ~ '^lwa_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_attempt_installation CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_local_attempt_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_attempt_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9.:-]{1,63}$'
    ),
    CONSTRAINT ck_gateway_local_attempt_version CHECK ("circuit_version" > 0),
    CONSTRAINT ck_gateway_local_attempt_lifecycle CHECK (
        ("status_code" = 'admitted' AND "result_class" IS NULL
            AND "stable_code_sha256" IS NULL AND elapsed_milliseconds IS NULL
            AND observed_at IS NULL)
        OR ("status_code" = 'observed'
            AND "result_class" IN (
                'succeeded', 'definitive_failure',
                'availability_failure', 'result_unknown'
            )
            AND octet_length("stable_code_sha256") = 32
            AND elapsed_milliseconds BETWEEN 0 AND 3600000
            AND observed_at IS NOT NULL)
    )
);

CREATE INDEX idx_gateway_local_attempt_pending
    ON idc_gateway_local_supplier_write_attempt (
        "provider_code", "operation_scope", "status_code", admitted_at, "attempt_no"
    );

-- Purpose: Persists business records for idc_gateway_supplier_health_sample.
-- 用途：持久化 idc_gateway_supplier_health_sample 的业务记录。
CREATE TABLE idc_gateway_supplier_health_sample (
    "sample_no" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "attempt_no" VARCHAR(40) NOT NULL UNIQUE,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "circuit_version" BIGINT NOT NULL,
    "result_class" VARCHAR(24) NOT NULL,
    "stable_code_sha256" BYTEA NOT NULL,
    elapsed_milliseconds INTEGER NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_local_sample_attempt FOREIGN KEY (
        "attempt_no", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_local_supplier_write_attempt (
        "attempt_no", "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_local_sample_circuit FOREIGN KEY (
        "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_supplier_circuit (
        "provider_code", "operation_scope"
    ),
    CONSTRAINT ck_gateway_local_sample_result CHECK (
        "result_class" IN ('succeeded', 'availability_failure', 'result_unknown')
    ),
    CONSTRAINT ck_gateway_local_sample_version CHECK ("circuit_version" > 0),
    CONSTRAINT ck_gateway_local_sample_digest CHECK (
        octet_length("stable_code_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_sample_elapsed CHECK (
        elapsed_milliseconds BETWEEN 0 AND 3600000
    )
);

CREATE INDEX idx_gateway_local_sample_window
    ON idc_gateway_supplier_health_sample (
        "provider_code", "operation_scope", "sample_no" DESC
    );

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_attempt_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."circuit_version" IS DISTINCT FROM NEW."circuit_version"
        OR OLD.probe IS DISTINCT FROM NEW.probe
        OR OLD.admitted_at IS DISTINCT FROM NEW.admitted_at
        OR OLD."status_code" <> 'admitted'
        OR NEW."status_code" <> 'observed'
    THEN
        RAISE EXCEPTION 'local supplier write attempt transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_circuit_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR NEW."policy_version" < OLD."policy_version"
        OR NOT (
            (OLD."status_code" = 'closed' AND NEW."status_code" IN ('closed', 'open'))
            OR (OLD."status_code" = 'open' AND NEW."status_code" IN ('open', 'half_open'))
            OR (OLD."status_code" = 'half_open' AND NEW."status_code" IN ('closed', 'open'))
        )
    THEN
        RAISE EXCEPTION 'local supplier circuit transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_local_attempt_guard
    ON idc_gateway_local_supplier_write_attempt;
CREATE TRIGGER trg_gateway_local_attempt_guard
BEFORE UPDATE ON idc_gateway_local_supplier_write_attempt
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_attempt_update();

DROP TRIGGER IF EXISTS trg_gateway_local_attempt_no_delete
    ON idc_gateway_local_supplier_write_attempt;
CREATE TRIGGER trg_gateway_local_attempt_no_delete
BEFORE DELETE ON idc_gateway_local_supplier_write_attempt
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_local_circuit_guard
    ON idc_gateway_supplier_circuit;
CREATE TRIGGER trg_gateway_local_circuit_guard
BEFORE UPDATE ON idc_gateway_supplier_circuit
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_circuit_update();

DROP TRIGGER IF EXISTS trg_gateway_local_circuit_no_delete
    ON idc_gateway_supplier_circuit;
CREATE TRIGGER trg_gateway_local_circuit_no_delete
BEFORE DELETE ON idc_gateway_supplier_circuit
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
