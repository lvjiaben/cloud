ALTER TABLE idc_gateway_local_supplier_route
    ADD COLUMN "external_ref_ciphertext" BYTEA,
    ADD COLUMN external_ref_nonce BYTEA,
    ADD COLUMN external_ref_cipher_format_version SMALLINT,
    ADD COLUMN external_ref_encryption_key_version INTEGER,
    ADD COLUMN external_ref_hmac_key_version INTEGER,
    ADD COLUMN "external_ref_hmac" BYTEA,
    ADD COLUMN next_query_at TIMESTAMPTZ,
    ADD COLUMN query_failure_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN query_lease_sha256 BYTEA,
    ADD COLUMN query_lease_started_at TIMESTAMPTZ,
    ADD COLUMN query_lease_expires_at TIMESTAMPTZ;

ALTER TABLE idc_gateway_local_supplier_route
    ADD CONSTRAINT ck_gateway_local_route_external_ref CHECK (
        (
            "external_ref_ciphertext" IS NULL
            AND external_ref_nonce IS NULL
            AND external_ref_cipher_format_version IS NULL
            AND external_ref_encryption_key_version IS NULL
            AND external_ref_hmac_key_version IS NULL
            AND "external_ref_hmac" IS NULL
        )
        OR (
            octet_length("external_ref_ciphertext") BETWEEN 17 AND 528
            AND octet_length(external_ref_nonce) = 12
            AND external_ref_cipher_format_version = 1
            AND external_ref_encryption_key_version > 0
            AND external_ref_hmac_key_version > 0
            AND octet_length("external_ref_hmac") = 32
        )
    ),
    ADD CONSTRAINT ck_gateway_local_route_query_count CHECK (
        query_failure_count BETWEEN 0 AND 1000000
    ),
    ADD CONSTRAINT ck_gateway_local_route_query_due CHECK (
        next_query_at IS NULL
        OR (
            "external_ref_hmac" IS NOT NULL
            AND (
                "projected_status" IN ('running', 'result_unknown')
                OR "reconciliation_status" = 'reconciliation_required'
            )
        )
    ),
    ADD CONSTRAINT ck_gateway_local_route_query_lease CHECK (
        (
            query_lease_sha256 IS NULL
            AND query_lease_started_at IS NULL
            AND query_lease_expires_at IS NULL
        )
        OR (
            octet_length(query_lease_sha256) = 32
            AND query_lease_started_at IS NOT NULL
            AND query_lease_expires_at > query_lease_started_at
            AND next_query_at IS NOT NULL
            AND "external_ref_hmac" IS NOT NULL
            AND (
                "projected_status" IN ('running', 'result_unknown')
                OR "reconciliation_status" = 'reconciliation_required'
            )
        )
    ),
    ADD CONSTRAINT ck_gateway_local_route_manual_query CHECK (
        "projected_status" <> 'manual_takeover'
        OR (
            next_query_at IS NULL
            AND query_lease_sha256 IS NULL
            AND query_lease_started_at IS NULL
            AND query_lease_expires_at IS NULL
        )
    );

CREATE UNIQUE INDEX uq_gateway_local_route_external_ref
    ON idc_gateway_local_supplier_route (
        "provider_code", external_ref_hmac_key_version, "external_ref_hmac"
    ) WHERE "external_ref_hmac" IS NOT NULL;

DROP INDEX idx_gateway_local_route_query;
CREATE INDEX idx_gateway_local_route_query_due
    ON idc_gateway_local_supplier_route (
        next_query_at, "provider_code", "route_no"
    ) WHERE next_query_at IS NOT NULL
        AND "external_ref_hmac" IS NOT NULL
        AND (
            "projected_status" IN ('running', 'result_unknown')
            OR "reconciliation_status" = 'reconciliation_required'
        );

ALTER TABLE idc_gateway_local_callback_evidence
    ALTER COLUMN event_key_version DROP NOT NULL,
    ALTER COLUMN "external_event_hmac" DROP NOT NULL,
    ADD COLUMN "projected_status" VARCHAR(24),
    ADD COLUMN "reconciliation_status" VARCHAR(32),
    ADD COLUMN stable_code VARCHAR(32);

UPDATE idc_gateway_local_callback_evidence evidence
SET "projected_status" = COALESCE(
        (
            SELECT delivery."projected_status"
            FROM idc_gateway_local_route_delivery delivery
            WHERE delivery."evidence_no" = evidence."evidence_no"
        ),
        route."projected_status"
    ),
    "reconciliation_status" = COALESCE(
        (
            SELECT delivery."reconciliation_status"
            FROM idc_gateway_local_route_delivery delivery
            WHERE delivery."evidence_no" = evidence."evidence_no"
        ),
        route."reconciliation_status"
    )
FROM idc_gateway_local_supplier_route route
WHERE route."route_no" = evidence."route_no";

ALTER TABLE idc_gateway_local_callback_evidence
    ALTER COLUMN "projected_status" SET NOT NULL,
    ALTER COLUMN "reconciliation_status" SET NOT NULL,
    DROP CONSTRAINT ck_gateway_local_evidence_source,
    DROP CONSTRAINT ck_gateway_local_evidence_digest,
    DROP CONSTRAINT ck_gateway_local_evidence_status,
    DROP CONSTRAINT ck_gateway_local_evidence_decision,
    ADD CONSTRAINT uq_gateway_local_evidence_route UNIQUE ("evidence_no", "route_no"),
    ADD CONSTRAINT ck_gateway_local_evidence_source CHECK (
        "source_kind" IN ('callback', 'active_query', 'manual_takeover')
    ),
    ADD CONSTRAINT ck_gateway_local_evidence_event CHECK (
        (
            "source_kind" = 'callback'
            AND event_key_version > 0
            AND octet_length("external_event_hmac") = 32
        )
        OR (
            "source_kind" <> 'callback'
            AND event_key_version IS NULL
            AND "external_event_hmac" IS NULL
        )
    ),
    ADD CONSTRAINT ck_gateway_local_evidence_digest CHECK (
        octet_length("payload_sha256") = 32
    ),
    ADD CONSTRAINT ck_gateway_local_evidence_status CHECK (
        "normalized_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
        AND "projected_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
        AND "reconciliation_status" IN (
            'none', 'query_required', 'reconciliation_required', 'manual_takeover'
        )
    ),
    ADD CONSTRAINT ck_gateway_local_evidence_decision CHECK (
        "decision_code" IN (
            'applied', 'stale', 'conflict', 'query_failed', 'manual_takeover'
        )
    ),
    ADD CONSTRAINT ck_gateway_local_evidence_stable_code CHECK (
        (
            "source_kind" = 'active_query'
            AND "decision_code" = 'query_failed'
            AND stable_code IN (
                'transport_unavailable', 'timeout', 'malformed_response',
                'reference_mismatch', 'provider_rejected'
            )
        )
        OR (
            NOT ("source_kind" = 'active_query' AND "decision_code" = 'query_failed')
            AND stable_code IS NULL
        )
    );

-- Purpose: Persists business records for idc_gateway_local_supplier_takeover.
-- 用途：持久化 idc_gateway_local_supplier_takeover 的业务记录。
CREATE TABLE idc_gateway_local_supplier_takeover (
    "takeover_no" VARCHAR(40) PRIMARY KEY,
    "route_no" VARCHAR(40) NOT NULL UNIQUE,
    "request_no" VARCHAR(40) NOT NULL,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    operation_key_version INTEGER NOT NULL,
    "operation_hmac" BYTEA NOT NULL,
    "request_sha256" BYTEA NOT NULL,
    "expected_route_version" BIGINT NOT NULL,
    "committed_route_version" BIGINT NOT NULL,
    "from_projected_status" VARCHAR(24) NOT NULL,
    "from_reconciliation_status" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(40) NOT NULL,
    "employee_principal_no" VARCHAR(128) NOT NULL,
    full_auth_at TIMESTAMPTZ NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    "evidence_count" SMALLINT NOT NULL,
    "decision_evidence_no" VARCHAR(40) NOT NULL UNIQUE,
    "decision_evidence_sha256" BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_local_takeover_operation UNIQUE (
        operation_key_version, "operation_hmac"
    ),
    CONSTRAINT uq_gateway_local_takeover_owner UNIQUE (
        "takeover_no", "route_no"
    ),
    CONSTRAINT fk_gateway_local_takeover_route FOREIGN KEY (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_local_supplier_route (
        "route_no", "request_no", "installation_id", "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_local_takeover_decision FOREIGN KEY (
        "decision_evidence_no", "route_no", "request_no", "installation_id",
        "provider_code", "operation_scope", "committed_route_version",
        "decision_evidence_sha256"
    ) REFERENCES idc_gateway_local_callback_evidence (
        "evidence_no", "route_no", "request_no", "installation_id",
        "provider_code", "operation_scope", "route_version", "payload_sha256"
    ),
    CONSTRAINT ck_gateway_local_takeover_no CHECK (
        "takeover_no" ~ '^lto_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_takeover_digest CHECK (
        operation_key_version > 0
        AND octet_length("operation_hmac") = 32
        AND octet_length("request_sha256") = 32
        AND octet_length("evidence_set_sha256") = 32
        AND octet_length("decision_evidence_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_takeover_version CHECK (
        "expected_route_version" > 0
        AND "committed_route_version" = "expected_route_version" + 1
    ),
    CONSTRAINT ck_gateway_local_takeover_from CHECK (
        "from_projected_status" IN ('running', 'succeeded', 'failed', 'result_unknown')
        AND "from_reconciliation_status" IN (
            'none', 'query_required', 'reconciliation_required'
        )
        AND (
            "from_projected_status" IN ('running', 'result_unknown')
            OR "from_reconciliation_status" = 'reconciliation_required'
        )
    ),
    CONSTRAINT ck_gateway_local_takeover_reason CHECK (
        "reason_code" IN (
            'result_remains_unknown', 'terminal_result_conflict',
            'query_reference_missing', 'supplier_query_unavailable',
            'compliance_security_review'
        )
    ),
    CONSTRAINT ck_gateway_local_takeover_employee CHECK (
        "employee_principal_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'
    ),
    CONSTRAINT ck_gateway_local_takeover_auth CHECK (
        full_auth_at >= created_at - INTERVAL '5 minutes'
        AND full_auth_at <= created_at + INTERVAL '15 seconds'
    ),
    CONSTRAINT ck_gateway_local_takeover_evidence_count CHECK (
        "evidence_count" BETWEEN 1 AND 16
    )
);

-- Purpose: Persists business records for idc_gateway_local_takeover_evidence.
-- 用途：持久化 idc_gateway_local_takeover_evidence 的业务记录。
CREATE TABLE idc_gateway_local_takeover_evidence (
    "takeover_no" VARCHAR(40) NOT NULL,
    "route_no" VARCHAR(40) NOT NULL,
    "evidence_no" VARCHAR(40) NOT NULL,
    "ordinal" SMALLINT NOT NULL,
    PRIMARY KEY ("takeover_no", "evidence_no"),
    CONSTRAINT uq_gateway_local_takeover_ordinal UNIQUE ("takeover_no", "ordinal"),
    CONSTRAINT fk_gateway_local_takeover_record FOREIGN KEY (
        "takeover_no", "route_no"
    ) REFERENCES idc_gateway_local_supplier_takeover ("takeover_no", "route_no"),
    CONSTRAINT fk_gateway_local_takeover_evidence FOREIGN KEY (
        "evidence_no", "route_no"
    ) REFERENCES idc_gateway_local_callback_evidence ("evidence_no", "route_no"),
    CONSTRAINT ck_gateway_local_takeover_ordinal CHECK (
        "ordinal" BETWEEN 1 AND 16
    )
);

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_route_update()
RETURNS TRIGGER AS $$
DECLARE
    state_changed BOOLEAN;
    reference_changed BOOLEAN;
BEGIN
    state_changed := OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
        OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status";
    reference_changed := OLD."external_ref_ciphertext" IS DISTINCT FROM NEW."external_ref_ciphertext"
        OR OLD.external_ref_nonce IS DISTINCT FROM NEW.external_ref_nonce
        OR OLD.external_ref_cipher_format_version IS DISTINCT FROM NEW.external_ref_cipher_format_version
        OR OLD.external_ref_encryption_key_version IS DISTINCT FROM NEW.external_ref_encryption_key_version
        OR OLD.external_ref_hmac_key_version IS DISTINCT FROM NEW.external_ref_hmac_key_version
        OR OLD."external_ref_hmac" IS DISTINCT FROM NEW."external_ref_hmac";

    IF OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR (OLD."external_ref_hmac" IS NOT NULL AND reference_changed)
        OR NOT (
            (OLD."projected_status" = 'running' AND NEW."projected_status" IN (
                'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
            ))
            OR (OLD."projected_status" = 'result_unknown' AND NEW."projected_status" IN (
                'result_unknown', 'succeeded', 'failed', 'manual_takeover'
            ))
            OR (
                OLD."projected_status" IN ('succeeded', 'failed')
                AND (
                    NEW."projected_status" = OLD."projected_status"
                    OR (
                        OLD."reconciliation_status" = 'reconciliation_required'
                        AND NEW."projected_status" = 'manual_takeover'
                    )
                )
            )
            OR (
                OLD."projected_status" = 'manual_takeover'
                AND NEW."projected_status" = 'manual_takeover'
            )
        )
        OR NOT (
            (OLD."reconciliation_status" = 'none' AND NEW."reconciliation_status" IN (
                'none', 'query_required', 'reconciliation_required', 'manual_takeover'
            ))
            OR (OLD."reconciliation_status" = 'query_required' AND NEW."reconciliation_status" IN (
                'none', 'query_required', 'reconciliation_required', 'manual_takeover'
            ))
            OR (OLD."reconciliation_status" = 'reconciliation_required'
                AND NEW."reconciliation_status" IN (
                    'none', 'reconciliation_required', 'manual_takeover'
                ))
            OR (OLD."reconciliation_status" = 'manual_takeover'
                AND NEW."reconciliation_status" = 'manual_takeover')
        )
        OR ((state_changed OR reference_changed) AND NEW."version" <> OLD."version" + 1)
        OR (NOT state_changed AND NOT reference_changed AND NEW."version" <> OLD."version")
    THEN
        RAISE EXCEPTION 'local supplier route transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_gateway_local_takeover_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_local_supplier_takeover
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();

CREATE TRIGGER trg_gateway_local_takeover_evidence_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_local_takeover_evidence
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
