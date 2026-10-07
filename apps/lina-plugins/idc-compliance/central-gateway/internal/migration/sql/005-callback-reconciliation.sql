-- Purpose: Persists business records for idc_gateway_supplier_route.
-- 用途：持久化 idc_gateway_supplier_route 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_route (
    "route_no" VARCHAR(40) PRIMARY KEY,
    "attempt_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "instance_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "projected_status" VARCHAR(24) NOT NULL DEFAULT 'running',
    "reconciliation_status" VARCHAR(24) NOT NULL DEFAULT 'none',
    "version" BIGINT NOT NULL DEFAULT 1,
    "external_ref_ciphertext" BYTEA,
    "external_ref_hmac" BYTEA,
    "external_ref_cipher_version" SMALLINT,
    next_query_at TIMESTAMPTZ,
    "query_attempt_count" INTEGER NOT NULL DEFAULT 0,
    "query_lease_token" BYTEA,
    query_lease_started_at TIMESTAMPTZ,
    query_lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_supplier_route_attempt UNIQUE ("attempt_no"),
    CONSTRAINT uq_gateway_supplier_route_request UNIQUE ("request_no"),
    CONSTRAINT uq_gateway_supplier_route_delivery_owner UNIQUE (
        "route_no", "instance_no", "request_no"
    ),
    CONSTRAINT uq_gateway_supplier_route_evidence_owner UNIQUE (
        "route_no", "instance_no", "provider_code"
    ),
    CONSTRAINT fk_gateway_supplier_route_attempt FOREIGN KEY (
        "attempt_no", "request_no", "instance_no", "provider_code", "operation_scope"
    ) REFERENCES idc_gateway_supplier_write_attempt (
        "attempt_no", "request_no", "instance_no", "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_supplier_route_request FOREIGN KEY (
        "request_no", "instance_no", "operation_scope"
    ) REFERENCES idc_gateway_business_request (
        "request_no", "instance_no", "operation_scope"
    ),
    CONSTRAINT ck_gateway_supplier_route_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_supplier_route_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_supplier_route_projection CHECK (
        "projected_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
    ),
    CONSTRAINT ck_gateway_supplier_route_reconciliation CHECK (
        "reconciliation_status" IN ('none', 'query_required', 'manual_takeover')
        AND (
            ("projected_status" = 'manual_takeover'
                AND "reconciliation_status" = 'manual_takeover')
            OR ("projected_status" <> 'manual_takeover'
                AND "reconciliation_status" <> 'manual_takeover')
        )
        AND ("projected_status" <> 'result_unknown'
            OR "reconciliation_status" = 'query_required')
        AND ("reconciliation_status" <> 'query_required' OR next_query_at IS NOT NULL)
        AND ("reconciliation_status" <> 'manual_takeover' OR (
            next_query_at IS NULL AND "query_lease_token" IS NULL
            AND query_lease_started_at IS NULL AND query_lease_expires_at IS NULL
        ))
    ),
    CONSTRAINT ck_gateway_supplier_route_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_supplier_route_external_ref CHECK (
        ("external_ref_ciphertext" IS NULL AND "external_ref_hmac" IS NULL
            AND "external_ref_cipher_version" IS NULL)
        OR ("external_ref_ciphertext" IS NOT NULL AND "external_ref_hmac" IS NOT NULL
            AND "external_ref_cipher_version" IS NOT NULL
            AND octet_length("external_ref_ciphertext") BETWEEN 29 AND 256
            AND octet_length("external_ref_hmac") = 32
            AND "external_ref_cipher_version" > 0)
    ),
    CONSTRAINT ck_gateway_supplier_route_query_count CHECK (
        "query_attempt_count" BETWEEN 0 AND 1000000
    ),
    CONSTRAINT ck_gateway_supplier_route_query_lease CHECK (
        ("query_lease_token" IS NULL AND query_lease_started_at IS NULL
            AND query_lease_expires_at IS NULL)
        OR (octet_length("query_lease_token") = 32
            AND query_lease_started_at IS NOT NULL
            AND query_lease_expires_at > query_lease_started_at
            AND "external_ref_hmac" IS NOT NULL
            AND next_query_at IS NOT NULL
            AND ("projected_status" IN ('running', 'result_unknown')
                OR "reconciliation_status" = 'query_required'))
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_supplier_route_external_ref
    ON idc_gateway_supplier_route ("provider_code", "external_ref_hmac")
    WHERE "external_ref_hmac" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_route_query_due
    ON idc_gateway_supplier_route (
        next_query_at, "provider_code", "route_no"
    )
    WHERE next_query_at IS NOT NULL
        AND "external_ref_hmac" IS NOT NULL
        AND ("projected_status" = 'running'
            OR "reconciliation_status" = 'query_required');

-- Purpose: Persists business records for idc_gateway_supplier_evidence.
-- 用途：持久化 idc_gateway_supplier_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_evidence (
    "evidence_no" VARCHAR(40) PRIMARY KEY,
    "route_no" VARCHAR(40) NOT NULL,
    "instance_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "source_kind" VARCHAR(24) NOT NULL,
    "external_event_hmac" BYTEA,
    "payload_ciphertext" BYTEA NOT NULL,
    "payload_sha256" BYTEA NOT NULL,
    "cipher_version" SMALLINT NOT NULL,
    "normalized_status" VARCHAR(24),
    "decision_code" VARCHAR(32) NOT NULL,
    "route_version" BIGINT NOT NULL,
    supplier_observed_at TIMESTAMPTZ,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_supplier_evidence_route_owner UNIQUE ("evidence_no", "route_no"),
    CONSTRAINT uq_gateway_supplier_evidence_delivery_owner UNIQUE (
        "evidence_no", "route_no", "route_version", "payload_sha256"
    ),
    CONSTRAINT fk_gateway_supplier_evidence_route FOREIGN KEY (
        "route_no", "instance_no", "provider_code"
    ) REFERENCES idc_gateway_supplier_route (
        "route_no", "instance_no", "provider_code"
    ),
    CONSTRAINT ck_gateway_supplier_evidence_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_supplier_evidence_source CHECK (
        "source_kind" IN ('callback', 'active_query', 'manual_takeover')
    ),
    CONSTRAINT ck_gateway_supplier_evidence_event CHECK (
        ("source_kind" = 'callback' AND "external_event_hmac" IS NOT NULL
            AND octet_length("external_event_hmac") = 32)
        OR ("source_kind" <> 'callback' AND "external_event_hmac" IS NULL)
    ),
    CONSTRAINT ck_gateway_supplier_evidence_ciphertext CHECK (
        octet_length("payload_ciphertext") BETWEEN 29 AND 282652
        AND octet_length("payload_sha256") = 32
        AND "cipher_version" > 0
    ),
    CONSTRAINT ck_gateway_supplier_evidence_status CHECK (
        "normalized_status" IS NULL OR "normalized_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown'
        )
    ),
    CONSTRAINT ck_gateway_supplier_evidence_decision CHECK (
        "decision_code" IN (
            'applied', 'duplicate', 'stale', 'conflict',
            'query_failed', 'manual_takeover'
        )
    ),
    CONSTRAINT ck_gateway_supplier_evidence_version CHECK ("route_version" > 0)
);

ALTER TABLE idc_gateway_supplier_evidence
    DROP CONSTRAINT IF EXISTS ck_gateway_supplier_evidence_ciphertext;
ALTER TABLE idc_gateway_supplier_evidence
    ADD CONSTRAINT ck_gateway_supplier_evidence_ciphertext CHECK (
        octet_length("payload_ciphertext") BETWEEN 29 AND 282652
        AND octet_length("payload_sha256") = 32
        AND "cipher_version" > 0
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_supplier_evidence_event
    ON idc_gateway_supplier_evidence ("provider_code", "external_event_hmac")
    WHERE "external_event_hmac" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_evidence_route
    ON idc_gateway_supplier_evidence ("route_no", observed_at, "evidence_no");

-- Purpose: Persists business records for idc_gateway_instance_delivery.
-- 用途：持久化 idc_gateway_instance_delivery 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_instance_delivery (
    "delivery_no" VARCHAR(40) PRIMARY KEY,
    "instance_no" VARCHAR(40) NOT NULL,
    "route_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "route_version" BIGINT NOT NULL,
    "projected_status" VARCHAR(24) NOT NULL,
    "reconciliation_status" VARCHAR(24) NOT NULL,
    "evidence_no" VARCHAR(40) NOT NULL,
    "evidence_sha256" BYTEA NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at TIMESTAMPTZ,
    CONSTRAINT uq_gateway_instance_delivery_version UNIQUE ("route_no", "route_version"),
    CONSTRAINT uq_gateway_instance_delivery_evidence UNIQUE ("evidence_no"),
    CONSTRAINT fk_gateway_instance_delivery_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT fk_gateway_instance_delivery_route FOREIGN KEY (
        "route_no", "instance_no", "request_no"
    ) REFERENCES idc_gateway_supplier_route (
        "route_no", "instance_no", "request_no"
    ),
    CONSTRAINT fk_gateway_instance_delivery_evidence FOREIGN KEY (
        "evidence_no", "route_no", "route_version", "evidence_sha256"
    ) REFERENCES idc_gateway_supplier_evidence (
        "evidence_no", "route_no", "route_version", "payload_sha256"
    ),
    CONSTRAINT ck_gateway_instance_delivery_version CHECK ("route_version" > 0),
    CONSTRAINT ck_gateway_instance_delivery_projection CHECK (
        "projected_status" IN (
            'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
        )
        AND "reconciliation_status" IN ('none', 'query_required', 'manual_takeover')
        AND (
            ("projected_status" = 'manual_takeover'
                AND "reconciliation_status" = 'manual_takeover')
            OR ("projected_status" <> 'manual_takeover'
                AND "reconciliation_status" <> 'manual_takeover')
        )
        AND ("projected_status" <> 'result_unknown'
            OR "reconciliation_status" = 'query_required')
    ),
    CONSTRAINT ck_gateway_instance_delivery_evidence CHECK (
        octet_length("evidence_sha256") = 32
    ),
    CONSTRAINT ck_gateway_instance_delivery_lifecycle CHECK (
        ("status" = 'pending' AND acknowledged_at IS NULL)
        OR ("status" = 'acknowledged' AND acknowledged_at IS NOT NULL)
    )
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'uq_gateway_instance_delivery_evidence'
          AND conrelid = 'idc_gateway_instance_delivery'::regclass
    ) THEN
        ALTER TABLE idc_gateway_instance_delivery
            ADD CONSTRAINT uq_gateway_instance_delivery_evidence UNIQUE ("evidence_no");
    END IF;
END;
$$;

ALTER TABLE idc_gateway_instance_delivery
    DROP CONSTRAINT IF EXISTS fk_gateway_instance_delivery_evidence;
ALTER TABLE idc_gateway_supplier_evidence
    DROP CONSTRAINT IF EXISTS uq_gateway_supplier_evidence_delivery_owner;
ALTER TABLE idc_gateway_supplier_evidence
    ADD CONSTRAINT uq_gateway_supplier_evidence_delivery_owner UNIQUE (
        "evidence_no", "route_no", "route_version", "payload_sha256"
    );
ALTER TABLE idc_gateway_instance_delivery
    ADD CONSTRAINT fk_gateway_instance_delivery_evidence FOREIGN KEY (
        "evidence_no", "route_no", "route_version", "evidence_sha256"
    ) REFERENCES idc_gateway_supplier_evidence (
        "evidence_no", "route_no", "route_version", "payload_sha256"
    );

CREATE INDEX IF NOT EXISTS idx_gateway_instance_delivery_pending
    ON idc_gateway_instance_delivery ("instance_no", "status", created_at, "delivery_no");

-- Purpose: Persists business records for idc_gateway_supplier_takeover.
-- 用途：持久化 idc_gateway_supplier_takeover 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_takeover (
    "takeover_no" VARCHAR(40) PRIMARY KEY,
    "route_no" VARCHAR(40) NOT NULL UNIQUE,
    "operation_hmac" BYTEA NOT NULL UNIQUE,
    "request_sha256" BYTEA NOT NULL,
    "route_version" BIGINT NOT NULL,
    "from_projected_status" VARCHAR(24) NOT NULL,
    "from_reconciliation_status" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "actor_ref" VARCHAR(128) NOT NULL,
    full_auth_at TIMESTAMPTZ NOT NULL,
    "evidence_digest" BYTEA NOT NULL,
    "evidence_count" SMALLINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_supplier_takeover_route_owner UNIQUE ("takeover_no", "route_no"),
    CONSTRAINT fk_gateway_supplier_takeover_route FOREIGN KEY ("route_no")
        REFERENCES idc_gateway_supplier_route ("route_no"),
    CONSTRAINT ck_gateway_supplier_takeover_digest CHECK (
        octet_length("operation_hmac") = 32
        AND octet_length("request_sha256") = 32
        AND octet_length("evidence_digest") = 32
    ),
    CONSTRAINT ck_gateway_supplier_takeover_version CHECK ("route_version" > 0),
    CONSTRAINT ck_gateway_supplier_takeover_from_status CHECK (
        "from_projected_status" IN ('running', 'succeeded', 'failed', 'result_unknown')
        AND "from_reconciliation_status" IN ('none', 'query_required')
        AND ("from_projected_status" IN ('running', 'result_unknown')
            OR "from_reconciliation_status" = 'query_required')
    ),
    CONSTRAINT ck_gateway_supplier_takeover_reason CHECK (
        "reason_code" IN (
            'evidence_conflict', 'query_exhausted', 'provider_unavailable',
            'external_reference_missing', 'legal_review_required', 'security_incident'
        )
    ),
    CONSTRAINT ck_gateway_supplier_takeover_actor CHECK (
        "actor_ref" ~ '^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$'
    ),
    CONSTRAINT ck_gateway_supplier_takeover_evidence_count CHECK (
        "evidence_count" BETWEEN 1 AND 16
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_takeover_evidence.
-- 用途：持久化 idc_gateway_supplier_takeover_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_takeover_evidence (
    "takeover_no" VARCHAR(40) NOT NULL,
    "route_no" VARCHAR(40) NOT NULL,
    "evidence_no" VARCHAR(40) NOT NULL,
    "ordinal" SMALLINT NOT NULL,
    PRIMARY KEY ("takeover_no", "evidence_no"),
    CONSTRAINT uq_gateway_supplier_takeover_ordinal UNIQUE ("takeover_no", "ordinal"),
    CONSTRAINT fk_gateway_takeover_evidence_takeover FOREIGN KEY ("takeover_no", "route_no")
        REFERENCES idc_gateway_supplier_takeover ("takeover_no", "route_no"),
    CONSTRAINT fk_gateway_takeover_evidence_evidence FOREIGN KEY ("evidence_no", "route_no")
        REFERENCES idc_gateway_supplier_evidence ("evidence_no", "route_no"),
    CONSTRAINT ck_gateway_supplier_takeover_ordinal CHECK ("ordinal" BETWEEN 1 AND 16)
);

CREATE OR REPLACE FUNCTION idc_gateway_guard_supplier_route_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR (OLD."external_ref_hmac" IS NOT NULL AND (
            OLD."external_ref_ciphertext" IS DISTINCT FROM NEW."external_ref_ciphertext"
            OR OLD."external_ref_hmac" IS DISTINCT FROM NEW."external_ref_hmac"
            OR OLD."external_ref_cipher_version" IS DISTINCT FROM NEW."external_ref_cipher_version"
        ))
        OR NOT (
            (OLD."projected_status" = 'running' AND NEW."projected_status" IN (
                'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
            ))
            OR (OLD."projected_status" = 'result_unknown' AND NEW."projected_status" IN (
                'result_unknown', 'succeeded', 'failed', 'manual_takeover'
            ))
            OR (OLD."projected_status" IN ('succeeded', 'failed')
                AND (
                    NEW."projected_status" = OLD."projected_status"
                    OR (OLD."reconciliation_status" = 'query_required'
                        AND NEW."projected_status" = 'manual_takeover')
                ))
            OR (OLD."projected_status" = 'manual_takeover'
                AND NEW."projected_status" = 'manual_takeover')
        )
        OR NOT (
            (OLD."reconciliation_status" = 'none'
                AND NEW."reconciliation_status" IN ('none', 'query_required', 'manual_takeover'))
            OR (OLD."reconciliation_status" = 'query_required'
                AND NEW."reconciliation_status" IN ('none', 'query_required', 'manual_takeover'))
            OR (OLD."reconciliation_status" = 'manual_takeover'
                AND NEW."reconciliation_status" = 'manual_takeover')
        )
        OR (
            (OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
                OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status"
                OR OLD."external_ref_ciphertext" IS DISTINCT FROM NEW."external_ref_ciphertext"
                OR OLD."external_ref_hmac" IS DISTINCT FROM NEW."external_ref_hmac"
                OR OLD."external_ref_cipher_version" IS DISTINCT FROM NEW."external_ref_cipher_version")
            AND NEW."version" <> OLD."version" + 1
        )
        OR (
            OLD."projected_status" IS NOT DISTINCT FROM NEW."projected_status"
            AND OLD."reconciliation_status" IS NOT DISTINCT FROM NEW."reconciliation_status"
            AND OLD."external_ref_ciphertext" IS NOT DISTINCT FROM NEW."external_ref_ciphertext"
            AND OLD."external_ref_hmac" IS NOT DISTINCT FROM NEW."external_ref_hmac"
            AND OLD."external_ref_cipher_version" IS NOT DISTINCT FROM NEW."external_ref_cipher_version"
            AND NEW."version" <> OLD."version"
        )
        OR NOT (
            (
                NEW."query_attempt_count" = OLD."query_attempt_count"
                AND OLD."query_lease_token" IS NOT DISTINCT FROM NEW."query_lease_token"
                AND OLD.query_lease_started_at IS NOT DISTINCT FROM NEW.query_lease_started_at
                AND OLD.query_lease_expires_at IS NOT DISTINCT FROM NEW.query_lease_expires_at
            )
            OR (
                NEW."query_attempt_count" = OLD."query_attempt_count" + 1
                AND NEW."query_lease_token" IS NOT NULL
                AND NEW.query_lease_started_at IS NOT NULL
                AND NEW.query_lease_expires_at IS NOT NULL
                AND (OLD."query_lease_token" IS NULL
                    OR OLD."query_lease_token" IS DISTINCT FROM NEW."query_lease_token")
            )
            OR (
                NEW."query_attempt_count" = OLD."query_attempt_count"
                AND OLD."query_lease_token" IS NOT NULL
                AND NEW."query_lease_token" IS NULL
                AND NEW.query_lease_started_at IS NULL
                AND NEW.query_lease_expires_at IS NULL
            )
        )
    THEN
        RAISE EXCEPTION 'gateway supplier route identity or version is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_instance_delivery_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."delivery_no" IS DISTINCT FROM NEW."delivery_no"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."route_version" IS DISTINCT FROM NEW."route_version"
        OR OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
        OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status"
        OR OLD."evidence_no" IS DISTINCT FROM NEW."evidence_no"
        OR OLD."evidence_sha256" IS DISTINCT FROM NEW."evidence_sha256"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status" <> 'pending'
        OR NEW."status" <> 'acknowledged'
    THEN
        RAISE EXCEPTION 'gateway instance delivery transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_reject_supplier_evidence_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway supplier evidence is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_reject_supplier_takeover_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway supplier takeover evidence is append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_supplier_route_guard_update ON idc_gateway_supplier_route;
CREATE TRIGGER trg_gateway_supplier_route_guard_update
BEFORE UPDATE ON idc_gateway_supplier_route
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_supplier_route_update();

DROP TRIGGER IF EXISTS trg_gateway_supplier_route_no_delete ON idc_gateway_supplier_route;
CREATE TRIGGER trg_gateway_supplier_route_no_delete
BEFORE DELETE ON idc_gateway_supplier_route
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_supplier_evidence_immutable ON idc_gateway_supplier_evidence;
CREATE TRIGGER trg_gateway_supplier_evidence_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_evidence
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_supplier_evidence_mutation();

DROP TRIGGER IF EXISTS trg_gateway_instance_delivery_no_delete ON idc_gateway_instance_delivery;
CREATE TRIGGER trg_gateway_instance_delivery_no_delete
BEFORE DELETE ON idc_gateway_instance_delivery
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_instance_delivery_guard_update
    ON idc_gateway_instance_delivery;
CREATE TRIGGER trg_gateway_instance_delivery_guard_update
BEFORE UPDATE ON idc_gateway_instance_delivery
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_instance_delivery_update();

DROP TRIGGER IF EXISTS trg_gateway_supplier_takeover_immutable ON idc_gateway_supplier_takeover;
CREATE TRIGGER trg_gateway_supplier_takeover_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_takeover
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_supplier_takeover_mutation();

DROP TRIGGER IF EXISTS trg_gateway_takeover_evidence_immutable
    ON idc_gateway_supplier_takeover_evidence;
CREATE TRIGGER trg_gateway_takeover_evidence_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_supplier_takeover_evidence
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_supplier_takeover_mutation();
