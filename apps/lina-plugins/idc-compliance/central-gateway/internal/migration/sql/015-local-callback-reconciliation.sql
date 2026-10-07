ALTER TABLE idc_gateway_local_supplier_write_attempt
    ADD CONSTRAINT uq_gateway_local_attempt_route_owner UNIQUE (
        "attempt_no", "request_no", "installation_id", "provider_code", "operation_scope"
    );

-- Purpose: Persists business records for idc_gateway_local_supplier_route.
-- 用途：持久化 idc_gateway_local_supplier_route 的业务记录。
CREATE TABLE idc_gateway_local_supplier_route (
    "route_no" VARCHAR(40) PRIMARY KEY,
    "attempt_no" VARCHAR(40) NOT NULL UNIQUE,
    "request_no" VARCHAR(40) NOT NULL UNIQUE,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "projected_status" VARCHAR(24) NOT NULL DEFAULT 'running',
    "reconciliation_status" VARCHAR(32) NOT NULL DEFAULT 'none',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_gateway_local_route_owner UNIQUE (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_local_route_attempt FOREIGN KEY (
        "attempt_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_local_supplier_write_attempt (
        "attempt_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ),
    CONSTRAINT ck_gateway_local_route_no CHECK (
        "route_no" ~ '^lwr_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_route_installation CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_local_route_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_route_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9.:-]{1,63}$'
    ),
    CONSTRAINT ck_gateway_local_route_projection CHECK (
        "projected_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
        AND "reconciliation_status" IN (
            'none', 'query_required', 'reconciliation_required', 'manual_takeover'
        )
        AND ("projected_status" <> 'result_unknown'
            OR "reconciliation_status" IN ('query_required', 'reconciliation_required'))
        AND ("projected_status" <> 'manual_takeover'
            OR "reconciliation_status" = 'manual_takeover')
        AND ("reconciliation_status" <> 'manual_takeover'
            OR "projected_status" = 'manual_takeover')
    ),
    CONSTRAINT ck_gateway_local_route_version CHECK ("version" > 0)
);

CREATE INDEX idx_gateway_local_route_query
    ON idc_gateway_local_supplier_route (
        "reconciliation_status", updated_at, "route_no"
    ) WHERE "reconciliation_status" IN ('query_required', 'reconciliation_required');

-- Purpose: Persists business records for idc_gateway_local_callback_evidence.
-- 用途：持久化 idc_gateway_local_callback_evidence 的业务记录。
CREATE TABLE idc_gateway_local_callback_evidence (
    "evidence_no" VARCHAR(40) PRIMARY KEY,
    "route_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "source_kind" VARCHAR(24) NOT NULL DEFAULT 'callback',
    event_key_version INTEGER NOT NULL,
    "external_event_hmac" BYTEA NOT NULL,
    "payload_ciphertext" BYTEA NOT NULL,
    "payload_nonce" BYTEA NOT NULL,
    "payload_sha256" BYTEA NOT NULL,
    "cipher_format_version" SMALLINT NOT NULL,
    encryption_key_version INTEGER NOT NULL,
    "normalized_status" VARCHAR(24) NOT NULL,
    "decision_code" VARCHAR(24) NOT NULL,
    "route_version" BIGINT NOT NULL,
    supplier_observed_at TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_local_evidence_event UNIQUE (
        "provider_code", event_key_version, "external_event_hmac"
    ),
    CONSTRAINT uq_gateway_local_evidence_owner UNIQUE (
        "evidence_no", "route_no", "request_no", "installation_id",
        "provider_code", "operation_scope", "route_version", "payload_sha256"
    ),
    CONSTRAINT fk_gateway_local_evidence_route FOREIGN KEY (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_local_supplier_route (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ),
    CONSTRAINT ck_gateway_local_evidence_no CHECK (
        "evidence_no" ~ '^lev_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_evidence_source CHECK (
        "source_kind" IN ('callback', 'active_query')
    ),
    CONSTRAINT ck_gateway_local_evidence_digest CHECK (
        event_key_version > 0
        AND octet_length("external_event_hmac") = 32
        AND octet_length("payload_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_evidence_cipher CHECK (
        octet_length("payload_ciphertext") BETWEEN 17 AND 299024
        AND octet_length("payload_nonce") = 12
        AND "cipher_format_version" = 1
        AND encryption_key_version > 0
    ),
    CONSTRAINT ck_gateway_local_evidence_status CHECK (
        "normalized_status" IN ('running', 'succeeded', 'failed', 'result_unknown')
    ),
    CONSTRAINT ck_gateway_local_evidence_decision CHECK (
        "decision_code" IN ('applied', 'stale', 'conflict')
    ),
    CONSTRAINT ck_gateway_local_evidence_version CHECK ("route_version" > 0)
);

CREATE INDEX idx_gateway_local_evidence_route
    ON idc_gateway_local_callback_evidence (
        "route_no", observed_at DESC, "evidence_no" DESC
    );

-- Purpose: Persists business records for idc_gateway_local_route_delivery.
-- 用途：持久化 idc_gateway_local_route_delivery 的业务记录。
CREATE TABLE idc_gateway_local_route_delivery (
    "delivery_no" VARCHAR(40) PRIMARY KEY,
    "route_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "route_version" BIGINT NOT NULL,
    "projected_status" VARCHAR(24) NOT NULL,
    "reconciliation_status" VARCHAR(32) NOT NULL,
    "evidence_no" VARCHAR(40) NOT NULL UNIQUE,
    "evidence_sha256" BYTEA NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at TIMESTAMPTZ,
    CONSTRAINT uq_gateway_local_delivery_version UNIQUE ("route_no", "route_version"),
    CONSTRAINT fk_gateway_local_delivery_route FOREIGN KEY (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_local_supplier_route (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_local_delivery_evidence FOREIGN KEY (
        "evidence_no", "route_no", "request_no", "installation_id", "provider_code",
        "operation_scope", "route_version", "evidence_sha256"
    ) REFERENCES idc_gateway_local_callback_evidence (
        "evidence_no", "route_no", "request_no", "installation_id", "provider_code",
        "operation_scope", "route_version", "payload_sha256"
    ),
    CONSTRAINT ck_gateway_local_delivery_no CHECK (
        "delivery_no" ~ '^ldv_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_delivery_projection CHECK (
        "projected_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
        AND "reconciliation_status" IN (
            'none', 'query_required', 'reconciliation_required', 'manual_takeover'
        )
        AND "route_version" > 1
    ),
    CONSTRAINT ck_gateway_local_delivery_digest CHECK (
        octet_length("evidence_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_delivery_lifecycle CHECK (
        ("status_code" = 'pending' AND acknowledged_at IS NULL)
        OR ("status_code" = 'acknowledged' AND acknowledged_at IS NOT NULL)
    )
);

CREATE INDEX idx_gateway_local_delivery_pending
    ON idc_gateway_local_route_delivery (
        "installation_id", "status_code", created_at, "delivery_no"
    ) WHERE "status_code" = 'pending';

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_route_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR NOT (
            (OLD."projected_status" = 'running' AND NEW."projected_status" IN (
                'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
            ))
            OR (OLD."projected_status" = 'result_unknown' AND NEW."projected_status" IN (
                'result_unknown', 'succeeded', 'failed', 'manual_takeover'
            ))
            OR (OLD."projected_status" IN ('succeeded', 'failed')
                AND NEW."projected_status" IN (OLD."projected_status", 'manual_takeover'))
            OR (OLD."projected_status" = 'manual_takeover'
                AND NEW."projected_status" = 'manual_takeover')
        )
        OR (
            (OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
                OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status")
            AND NEW."version" <> OLD."version" + 1
        )
        OR (
            OLD."projected_status" IS NOT DISTINCT FROM NEW."projected_status"
            AND OLD."reconciliation_status" IS NOT DISTINCT FROM NEW."reconciliation_status"
            AND NEW."version" <> OLD."version"
        )
    THEN
        RAISE EXCEPTION 'local supplier route transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_delivery_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."delivery_no" IS DISTINCT FROM NEW."delivery_no"
        OR OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."route_version" IS DISTINCT FROM NEW."route_version"
        OR OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
        OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status"
        OR OLD."evidence_no" IS DISTINCT FROM NEW."evidence_no"
        OR OLD."evidence_sha256" IS DISTINCT FROM NEW."evidence_sha256"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status_code" <> 'pending'
        OR NEW."status_code" <> 'acknowledged'
        OR NEW.acknowledged_at IS NULL
    THEN
        RAISE EXCEPTION 'local route delivery transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_local_route_guard
    ON idc_gateway_local_supplier_route;
CREATE TRIGGER trg_gateway_local_route_guard
BEFORE UPDATE ON idc_gateway_local_supplier_route
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_route_update();

DROP TRIGGER IF EXISTS trg_gateway_local_route_no_delete
    ON idc_gateway_local_supplier_route;
CREATE TRIGGER trg_gateway_local_route_no_delete
BEFORE DELETE ON idc_gateway_local_supplier_route
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_local_evidence_immutable
    ON idc_gateway_local_callback_evidence;
CREATE TRIGGER trg_gateway_local_evidence_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_local_callback_evidence
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

DROP TRIGGER IF EXISTS trg_gateway_local_delivery_guard
    ON idc_gateway_local_route_delivery;
CREATE TRIGGER trg_gateway_local_delivery_guard
BEFORE UPDATE ON idc_gateway_local_route_delivery
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_delivery_update();

DROP TRIGGER IF EXISTS trg_gateway_local_delivery_no_delete
    ON idc_gateway_local_route_delivery;
CREATE TRIGGER trg_gateway_local_delivery_no_delete
BEFORE DELETE ON idc_gateway_local_route_delivery
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
