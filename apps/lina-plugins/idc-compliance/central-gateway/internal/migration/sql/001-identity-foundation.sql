-- Purpose: Persists business records for idc_gateway_registration_token.
-- 用途：持久化 idc_gateway_registration_token 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_registration_token (
    "registration_no" VARCHAR(40) PRIMARY KEY,
    "sales_record_ref" VARCHAR(128) NOT NULL,
    "merchant_ref" VARCHAR(128) NOT NULL,
    "token_digest" BYTEA NOT NULL,
    "issue_operation_hmac" BYTEA NOT NULL,
    "issue_request_sha256" BYTEA NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'issued',
    expires_at TIMESTAMPTZ NOT NULL,
    "request_sha256" BYTEA,
    consumed_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_registration_digest UNIQUE ("token_digest"),
    CONSTRAINT uq_gateway_registration_operation UNIQUE ("issue_operation_hmac"),
    CONSTRAINT ck_gateway_registration_digest CHECK (octet_length("token_digest") = 32),
    CONSTRAINT ck_gateway_registration_operation CHECK (octet_length("issue_operation_hmac") = 32),
    CONSTRAINT ck_gateway_registration_issue_request CHECK (octet_length("issue_request_sha256") = 32),
    CONSTRAINT ck_gateway_registration_status CHECK ("status" IN ('issued', 'consumed', 'expired', 'revoked')),
    CONSTRAINT ck_gateway_registration_request CHECK ("request_sha256" IS NULL OR octet_length("request_sha256") = 32),
    CONSTRAINT ck_gateway_registration_lifecycle CHECK (
        ("status" = 'issued' AND "request_sha256" IS NULL AND consumed_at IS NULL AND revoked_at IS NULL)
        OR ("status" = 'consumed' AND "request_sha256" IS NOT NULL AND consumed_at IS NOT NULL AND revoked_at IS NULL)
        OR ("status" = 'expired' AND "request_sha256" IS NULL AND consumed_at IS NULL AND revoked_at IS NULL)
        OR ("status" = 'revoked' AND "request_sha256" IS NULL AND consumed_at IS NULL AND revoked_at IS NOT NULL
            AND "revoke_reason_code" IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_registration_active_sale
    ON idc_gateway_registration_token ("sales_record_ref")
    WHERE "status" = 'issued';

CREATE INDEX IF NOT EXISTS idx_gateway_registration_expiry
    ON idc_gateway_registration_token ("status", expires_at, "registration_no");

-- Purpose: Persists business records for idc_gateway_instance.
-- 用途：持久化 idc_gateway_instance 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_instance (
    "instance_no" VARCHAR(40) PRIMARY KEY,
    "sales_record_ref" VARCHAR(128) NOT NULL,
    "merchant_ref" VARCHAR(128) NOT NULL,
    "registration_no" VARCHAR(40) NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'active',
    registered_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_instance_sale UNIQUE ("sales_record_ref"),
    CONSTRAINT uq_gateway_instance_registration UNIQUE ("registration_no"),
    CONSTRAINT fk_gateway_instance_registration FOREIGN KEY ("registration_no")
        REFERENCES idc_gateway_registration_token ("registration_no"),
    CONSTRAINT ck_gateway_instance_status CHECK ("status" IN ('active', 'revoked')),
    CONSTRAINT ck_gateway_instance_lifecycle CHECK (
        ("status" = 'active' AND revoked_at IS NULL AND "revoke_reason_code" IS NULL)
        OR ("status" = 'revoked' AND revoked_at IS NOT NULL AND "revoke_reason_code" IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_instance_merchant
    ON idc_gateway_instance ("merchant_ref", "status", "instance_no");

-- Purpose: Persists business records for idc_gateway_instance_key.
-- 用途：持久化 idc_gateway_instance_key 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_instance_key (
    "key_no" VARCHAR(40) PRIMARY KEY,
    "instance_no" VARCHAR(40) NOT NULL,
    "generation" BIGINT NOT NULL,
    "public_key" BYTEA NOT NULL,
    "public_key_sha256" BYTEA NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'active',
    activated_at TIMESTAMPTZ,
    retired_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_instance_key_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT uq_gateway_instance_key_generation UNIQUE ("instance_no", "generation"),
    CONSTRAINT uq_gateway_instance_key_fingerprint UNIQUE ("public_key_sha256"),
    CONSTRAINT ck_gateway_instance_key_generation CHECK ("generation" > 0),
    CONSTRAINT ck_gateway_instance_key_public CHECK (octet_length("public_key") = 32),
    CONSTRAINT ck_gateway_instance_key_fingerprint CHECK (octet_length("public_key_sha256") = 32),
    CONSTRAINT ck_gateway_instance_key_status CHECK ("status" IN ('pending', 'active', 'retired', 'revoked')),
    CONSTRAINT ck_gateway_instance_key_lifecycle CHECK (
        ("status" = 'pending' AND activated_at IS NULL AND retired_at IS NULL AND revoked_at IS NULL
            AND "revoke_reason_code" IS NULL)
        OR ("status" = 'active' AND activated_at IS NOT NULL AND retired_at IS NULL AND revoked_at IS NULL
            AND "revoke_reason_code" IS NULL)
        OR ("status" = 'retired' AND activated_at IS NOT NULL AND retired_at IS NOT NULL AND revoked_at IS NULL
            AND "revoke_reason_code" IS NULL)
        OR ("status" = 'revoked' AND revoked_at IS NOT NULL AND "revoke_reason_code" IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_instance_key_active
    ON idc_gateway_instance_key ("instance_no")
    WHERE "status" = 'active';

CREATE INDEX IF NOT EXISTS idx_gateway_instance_key_status
    ON idc_gateway_instance_key ("instance_no", "status", "generation" DESC);

-- Purpose: Persists business records for idc_gateway_subject_binding.
-- 用途：持久化 idc_gateway_subject_binding 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_subject_binding (
    "binding_no" VARCHAR(40) PRIMARY KEY,
    "instance_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "subject_ciphertext" BYTEA NOT NULL,
    "subject_lookup_hmac" BYTEA NOT NULL,
    "cipher_version" SMALLINT NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'active',
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_subject_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT ck_gateway_subject_provider CHECK ("provider_code" IN ('tencent_essbasic', 'nuonuo')),
    CONSTRAINT ck_gateway_subject_lookup CHECK (octet_length("subject_lookup_hmac") = 32),
    CONSTRAINT ck_gateway_subject_cipher CHECK (octet_length("subject_ciphertext") >= 32 AND "cipher_version" > 0),
    CONSTRAINT ck_gateway_subject_status CHECK ("status" IN ('active', 'revoked')),
    CONSTRAINT ck_gateway_subject_lifecycle CHECK (
        ("status" = 'active' AND revoked_at IS NULL AND "revoke_reason_code" IS NULL)
        OR ("status" = 'revoked' AND revoked_at IS NOT NULL AND "revoke_reason_code" IS NOT NULL)
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_subject_active_instance
    ON idc_gateway_subject_binding ("instance_no", "provider_code")
    WHERE "status" = 'active';

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_subject_active_reference
    ON idc_gateway_subject_binding ("provider_code", "subject_lookup_hmac")
    WHERE "status" = 'active';

-- Purpose: Persists business records for idc_gateway_identity_event.
-- 用途：持久化 idc_gateway_identity_event 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_identity_event (
    "event_no" VARCHAR(40) PRIMARY KEY,
    "aggregate_kind" VARCHAR(24) NOT NULL,
    "aggregate_no" VARCHAR(40) NOT NULL,
    "event_code" VARCHAR(64) NOT NULL,
    "actor_kind" VARCHAR(24) NOT NULL,
    "actor_ref" VARCHAR(128),
    "reason_code" VARCHAR(64),
    "evidence_sha256" BYTEA,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_identity_event_aggregate CHECK (
        "aggregate_kind" IN ('registration', 'instance', 'instance_key', 'subject_binding')
    ),
    CONSTRAINT ck_gateway_identity_event_actor CHECK (
        "actor_kind" IN ('platform_sales', 'instance', 'operator', 'system')
    ),
    CONSTRAINT ck_gateway_identity_event_evidence CHECK (
        "evidence_sha256" IS NULL OR octet_length("evidence_sha256") = 32
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_identity_event_aggregate
    ON idc_gateway_identity_event ("aggregate_kind", "aggregate_no", occurred_at, "event_no");

CREATE OR REPLACE FUNCTION idc_gateway_reject_delete()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway identity records cannot be physically deleted';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_reject_event_change()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway identity events are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_registration_no_delete ON idc_gateway_registration_token;
CREATE TRIGGER trg_gateway_registration_no_delete
BEFORE DELETE ON idc_gateway_registration_token
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_instance_no_delete ON idc_gateway_instance;
CREATE TRIGGER trg_gateway_instance_no_delete
BEFORE DELETE ON idc_gateway_instance
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_instance_key_no_delete ON idc_gateway_instance_key;
CREATE TRIGGER trg_gateway_instance_key_no_delete
BEFORE DELETE ON idc_gateway_instance_key
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_subject_no_delete ON idc_gateway_subject_binding;
CREATE TRIGGER trg_gateway_subject_no_delete
BEFORE DELETE ON idc_gateway_subject_binding
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_delete();

DROP TRIGGER IF EXISTS trg_gateway_identity_event_immutable ON idc_gateway_identity_event;
CREATE TRIGGER trg_gateway_identity_event_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_identity_event
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_event_change();
