-- Purpose: Persists business records for idc_gateway_local_supplier_instance.
-- 用途：持久化 idc_gateway_local_supplier_instance 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_local_supplier_instance (
    "installation_id" VARCHAR(70) PRIMARY KEY,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_local_subject_instance_id CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_local_subject_instance_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_gateway_local_subject_instance_lifecycle CHECK (
        ("status_code" = 'active' AND revoked_at IS NULL AND "revoke_reason_code" IS NULL)
        OR ("status_code" = 'revoked' AND revoked_at IS NOT NULL
            AND "revoke_reason_code" = 'instance_revoked')
    ),
    CONSTRAINT ck_gateway_local_subject_instance_version CHECK ("version" > 0)
);

-- Purpose: Persists business records for idc_gateway_local_supplier_subject.
-- 用途：持久化 idc_gateway_local_supplier_subject 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_local_supplier_subject (
    "binding_no" VARCHAR(40) PRIMARY KEY,
    "installation_id" VARCHAR(70) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "subject_ciphertext" BYTEA NOT NULL,
    "subject_nonce" BYTEA NOT NULL,
    "cipher_format_version" SMALLINT NOT NULL,
    encryption_key_version INTEGER NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "reopened_from_binding_no" VARCHAR(40),
    bound_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMPTZ,
    "revoke_reason_code" VARCHAR(40),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_gateway_local_subject_owner UNIQUE (
        "binding_no", "installation_id", "provider_code"
    ),
    CONSTRAINT uq_gateway_local_subject_provider UNIQUE ("binding_no", "provider_code"),
    CONSTRAINT fk_gateway_local_subject_instance FOREIGN KEY ("installation_id")
        REFERENCES idc_gateway_local_supplier_instance ("installation_id"),
    CONSTRAINT fk_gateway_local_subject_reopened FOREIGN KEY (
        "reopened_from_binding_no", "installation_id", "provider_code"
    ) REFERENCES idc_gateway_local_supplier_subject (
        "binding_no", "installation_id", "provider_code"
    ),
    CONSTRAINT ck_gateway_local_subject_no CHECK ("binding_no" ~ '^lsb_[0-9a-f]{32}$'),
    CONSTRAINT ck_gateway_local_subject_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_subject_cipher CHECK (
        octet_length("subject_ciphertext") >= 16
        AND octet_length("subject_nonce") = 12
        AND "cipher_format_version" = 1
        AND encryption_key_version > 0
    ),
    CONSTRAINT ck_gateway_local_subject_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_gateway_local_subject_lifecycle CHECK (
        ("status_code" = 'active' AND revoked_at IS NULL AND "revoke_reason_code" IS NULL)
        OR ("status_code" = 'revoked' AND revoked_at IS NOT NULL
            AND "revoke_reason_code" IN (
                'subject_replaced', 'supplier_authorization_revoked',
                'security_compromise', 'instance_revoked'
            ))
    ),
    CONSTRAINT ck_gateway_local_subject_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_local_subject_reopen CHECK (
        "reopened_from_binding_no" IS NULL OR "reopened_from_binding_no" <> "binding_no"
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_local_subject_instance_active
    ON idc_gateway_local_supplier_subject ("installation_id", "provider_code")
    WHERE "status_code" = 'active';

CREATE INDEX IF NOT EXISTS idx_gateway_local_subject_history
    ON idc_gateway_local_supplier_subject (
        "installation_id", "provider_code", "status_code", bound_at DESC, "binding_no" DESC
    );

-- Purpose: Persists business records for idc_gateway_local_subject_lookup.
-- 用途：持久化 idc_gateway_local_subject_lookup 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_local_subject_lookup (
    "binding_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    lookup_key_version INTEGER NOT NULL,
    "subject_lookup_hmac" BYTEA NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("binding_no", lookup_key_version),
    CONSTRAINT fk_gateway_local_subject_lookup_binding FOREIGN KEY (
        "binding_no", "provider_code"
    ) REFERENCES idc_gateway_local_supplier_subject (
        "binding_no", "provider_code"
    ),
    CONSTRAINT ck_gateway_local_subject_lookup_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_local_subject_lookup_version CHECK (lookup_key_version > 0),
    CONSTRAINT ck_gateway_local_subject_lookup_digest CHECK (
        octet_length("subject_lookup_hmac") = 32
    ),
    CONSTRAINT ck_gateway_local_subject_lookup_status CHECK (
        "status_code" IN ('active', 'revoked')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_local_subject_lookup_active
    ON idc_gateway_local_subject_lookup (
        "provider_code", lookup_key_version, "subject_lookup_hmac"
    ) WHERE "status_code" = 'active';

CREATE INDEX IF NOT EXISTS idx_gateway_local_subject_lookup_binding
    ON idc_gateway_local_subject_lookup (
        "provider_code", lookup_key_version, "subject_lookup_hmac", "status_code", "binding_no"
    );

-- Purpose: Persists business records for idc_gateway_local_identity_operation.
-- 用途：持久化 idc_gateway_local_identity_operation 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_local_identity_operation (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "installation_id" VARCHAR(70) NOT NULL,
    "operation_kind" VARCHAR(32) NOT NULL,
    "provider_code" VARCHAR(32),
    "binding_no" VARCHAR(40),
    "previous_binding_no" VARCHAR(40),
    idempotency_key_version INTEGER NOT NULL,
    "idempotency_hmac" BYTEA NOT NULL,
    "request_sha256" BYTEA NOT NULL,
    "reason_code" VARCHAR(40),
    completed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_local_identity_instance FOREIGN KEY ("installation_id")
        REFERENCES idc_gateway_local_supplier_instance ("installation_id"),
    CONSTRAINT fk_gateway_local_identity_binding FOREIGN KEY (
        "binding_no", "installation_id", "provider_code"
    ) REFERENCES idc_gateway_local_supplier_subject (
        "binding_no", "installation_id", "provider_code"
    ),
    CONSTRAINT fk_gateway_local_identity_previous FOREIGN KEY (
        "previous_binding_no", "installation_id", "provider_code"
    ) REFERENCES idc_gateway_local_supplier_subject (
        "binding_no", "installation_id", "provider_code"
    ),
    CONSTRAINT uq_gateway_local_identity_idempotency UNIQUE (
        idempotency_key_version, "idempotency_hmac"
    ),
    CONSTRAINT ck_gateway_local_identity_operation_no CHECK (
        "operation_no" ~ '^lsi_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_local_identity_kind CHECK (
        "operation_kind" IN (
            'subject_bind', 'subject_revoke', 'subject_reopen', 'instance_revoke'
        )
    ),
    CONSTRAINT ck_gateway_local_identity_provider CHECK (
        ("operation_kind" = 'instance_revoke' AND "provider_code" IS NULL)
        OR ("operation_kind" <> 'instance_revoke'
            AND "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice'))
    ),
    CONSTRAINT ck_gateway_local_identity_result CHECK (
        ("operation_kind" = 'instance_revoke' AND "binding_no" IS NULL
            AND "previous_binding_no" IS NULL)
        OR ("operation_kind" = 'subject_reopen' AND "binding_no" IS NOT NULL
            AND "previous_binding_no" IS NOT NULL)
        OR ("operation_kind" IN ('subject_bind', 'subject_revoke')
            AND "binding_no" IS NOT NULL AND "previous_binding_no" IS NULL)
    ),
    CONSTRAINT ck_gateway_local_identity_idempotency CHECK (
        idempotency_key_version > 0
        AND octet_length("idempotency_hmac") = 32
        AND octet_length("request_sha256") = 32
    ),
    CONSTRAINT ck_gateway_local_identity_reason CHECK (
        ("operation_kind" = 'subject_bind' AND "reason_code" IS NULL)
        OR ("operation_kind" = 'subject_revoke' AND "reason_code" IN (
            'subject_replaced', 'supplier_authorization_revoked',
            'security_compromise', 'instance_revoked'
        ))
        OR ("operation_kind" = 'subject_reopen' AND "reason_code" IN (
            'replacement_approved', 'supplier_reauthorized', 'security_recovered'
        ))
        OR ("operation_kind" = 'instance_revoke' AND "reason_code" = 'instance_revoked')
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_local_identity_history
    ON idc_gateway_local_identity_operation (
        "installation_id", "operation_kind", completed_at DESC, "operation_no" DESC
    );

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_instance_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status_code" <> 'active'
        OR NEW."status_code" <> 'revoked'
        OR NEW."revoke_reason_code" <> 'instance_revoked'
        OR NEW.revoked_at IS NULL
        OR NEW."version" <> OLD."version" + 1
    THEN
        RAISE EXCEPTION 'local supplier instance lifecycle is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_cascade_local_instance_revoke()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE idc_gateway_local_supplier_subject
    SET "status_code" = 'revoked', revoked_at = NEW.revoked_at,
        "revoke_reason_code" = 'instance_revoked', "version" = "version" + 1
    WHERE "installation_id" = NEW."installation_id" AND "status_code" = 'active';

    UPDATE idc_gateway_local_subject_lookup lookup
    SET "status_code" = 'revoked'
    FROM idc_gateway_local_supplier_subject "subject"
    WHERE "subject"."binding_no" = lookup."binding_no"
      AND "subject"."installation_id" = NEW."installation_id"
      AND lookup."status_code" = 'active';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_subject_insert()
RETURNS TRIGGER AS $$
DECLARE
    "current_status" VARCHAR(16);
    previous_installation VARCHAR(70);
    previous_provider VARCHAR(32);
    "previous_status" VARCHAR(16);
BEGIN
    SELECT "status_code" INTO "current_status"
    FROM idc_gateway_local_supplier_instance
    WHERE "installation_id" = NEW."installation_id"
    FOR SHARE;
    IF "current_status" IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'local supplier instance is not active';
    END IF;
    IF NEW."reopened_from_binding_no" IS NOT NULL THEN
        SELECT "installation_id", "provider_code", "status_code"
        INTO previous_installation, previous_provider, "previous_status"
        FROM idc_gateway_local_supplier_subject
        WHERE "binding_no" = NEW."reopened_from_binding_no"
        FOR SHARE;
        IF previous_installation IS DISTINCT FROM NEW."installation_id"
            OR previous_provider IS DISTINCT FROM NEW."provider_code"
            OR "previous_status" IS DISTINCT FROM 'revoked'
        THEN
            RAISE EXCEPTION 'local supplier subject reopen source is invalid';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_subject_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."binding_no" IS DISTINCT FROM NEW."binding_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."subject_ciphertext" IS DISTINCT FROM NEW."subject_ciphertext"
        OR OLD."subject_nonce" IS DISTINCT FROM NEW."subject_nonce"
        OR OLD."cipher_format_version" IS DISTINCT FROM NEW."cipher_format_version"
        OR OLD.encryption_key_version IS DISTINCT FROM NEW.encryption_key_version
        OR OLD."reopened_from_binding_no" IS DISTINCT FROM NEW."reopened_from_binding_no"
        OR OLD.bound_at IS DISTINCT FROM NEW.bound_at
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status_code" <> 'active'
        OR NEW."status_code" <> 'revoked'
        OR NEW.revoked_at IS NULL
        OR NEW."revoke_reason_code" NOT IN (
            'subject_replaced', 'supplier_authorization_revoked',
            'security_compromise', 'instance_revoked'
        )
        OR NEW."version" <> OLD."version" + 1
    THEN
        RAISE EXCEPTION 'local supplier subject identity is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_reject_local_identity_change()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'local supplier identity operation is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_identity_insert()
RETURNS TRIGGER AS $$
DECLARE
    instance_status VARCHAR(16);
    subject_status VARCHAR(16);
    subject_reason VARCHAR(40);
    subject_reopened_from VARCHAR(40);
    "previous_status" VARCHAR(16);
BEGIN
    SELECT "status_code" INTO instance_status
    FROM idc_gateway_local_supplier_instance
    WHERE "installation_id" = NEW."installation_id"
    FOR SHARE;

    IF NEW."operation_kind" = 'instance_revoke' THEN
        IF instance_status IS DISTINCT FROM 'revoked' THEN
            RAISE EXCEPTION 'local supplier instance revoke operation is invalid';
        END IF;
        RETURN NEW;
    END IF;

    SELECT "status_code", "revoke_reason_code", "reopened_from_binding_no"
    INTO subject_status, subject_reason, subject_reopened_from
    FROM idc_gateway_local_supplier_subject
    WHERE "binding_no" = NEW."binding_no"
      AND "installation_id" = NEW."installation_id"
      AND "provider_code" = NEW."provider_code"
    FOR SHARE;

    IF NEW."operation_kind" = 'subject_bind'
        AND (subject_status IS DISTINCT FROM 'active'
            OR subject_reopened_from IS NOT NULL)
    THEN
        RAISE EXCEPTION 'local supplier bind operation is invalid';
    ELSIF NEW."operation_kind" = 'subject_revoke'
        AND (subject_status IS DISTINCT FROM 'revoked'
            OR subject_reason IS DISTINCT FROM NEW."reason_code")
    THEN
        RAISE EXCEPTION 'local supplier revoke operation is invalid';
    ELSIF NEW."operation_kind" = 'subject_reopen' THEN
        SELECT "status_code" INTO "previous_status"
        FROM idc_gateway_local_supplier_subject
        WHERE "binding_no" = NEW."previous_binding_no"
          AND "installation_id" = NEW."installation_id"
          AND "provider_code" = NEW."provider_code"
        FOR SHARE;
        IF subject_status IS DISTINCT FROM 'active'
            OR subject_reopened_from IS DISTINCT FROM NEW."previous_binding_no"
            OR "previous_status" IS DISTINCT FROM 'revoked'
        THEN
            RAISE EXCEPTION 'local supplier reopen operation is invalid';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_lookup_insert()
RETURNS TRIGGER AS $$
DECLARE
    subject_status VARCHAR(16);
BEGIN
    SELECT "status_code" INTO subject_status
    FROM idc_gateway_local_supplier_subject
    WHERE "binding_no" = NEW."binding_no"
    FOR SHARE;
    IF subject_status IS DISTINCT FROM 'active' OR NEW."status_code" <> 'active' THEN
        RAISE EXCEPTION 'local supplier lookup requires an active subject';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_lookup_update()
RETURNS TRIGGER AS $$
DECLARE
    subject_status VARCHAR(16);
BEGIN
    SELECT "status_code" INTO subject_status
    FROM idc_gateway_local_supplier_subject
    WHERE "binding_no" = OLD."binding_no"
    FOR SHARE;
    IF OLD."binding_no" IS DISTINCT FROM NEW."binding_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD.lookup_key_version IS DISTINCT FROM NEW.lookup_key_version
        OR OLD."subject_lookup_hmac" IS DISTINCT FROM NEW."subject_lookup_hmac"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status_code" <> 'active'
        OR NEW."status_code" <> 'revoked'
        OR subject_status IS DISTINCT FROM 'revoked'
    THEN
        RAISE EXCEPTION 'local supplier lookup identity is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_cascade_local_subject_revoke()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE idc_gateway_local_subject_lookup
    SET "status_code" = 'revoked'
    WHERE "binding_no" = NEW."binding_no" AND "status_code" = 'active';
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_local_instance_guard
    ON idc_gateway_local_supplier_instance;
CREATE TRIGGER trg_gateway_local_instance_guard
BEFORE UPDATE ON idc_gateway_local_supplier_instance
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_instance_update();

DROP TRIGGER IF EXISTS trg_gateway_local_instance_cascade
    ON idc_gateway_local_supplier_instance;
CREATE TRIGGER trg_gateway_local_instance_cascade
AFTER UPDATE ON idc_gateway_local_supplier_instance
FOR EACH ROW EXECUTE FUNCTION idc_gateway_cascade_local_instance_revoke();

DROP TRIGGER IF EXISTS trg_gateway_local_subject_insert
    ON idc_gateway_local_supplier_subject;
CREATE TRIGGER trg_gateway_local_subject_insert
BEFORE INSERT ON idc_gateway_local_supplier_subject
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_subject_insert();

DROP TRIGGER IF EXISTS trg_gateway_local_subject_guard
    ON idc_gateway_local_supplier_subject;
CREATE TRIGGER trg_gateway_local_subject_guard
BEFORE UPDATE ON idc_gateway_local_supplier_subject
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_subject_update();

DROP TRIGGER IF EXISTS trg_gateway_local_subject_cascade
    ON idc_gateway_local_supplier_subject;
CREATE TRIGGER trg_gateway_local_subject_cascade
AFTER UPDATE ON idc_gateway_local_supplier_subject
FOR EACH ROW EXECUTE FUNCTION idc_gateway_cascade_local_subject_revoke();

DROP TRIGGER IF EXISTS trg_gateway_local_lookup_insert
    ON idc_gateway_local_subject_lookup;
CREATE TRIGGER trg_gateway_local_lookup_insert
BEFORE INSERT ON idc_gateway_local_subject_lookup
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_lookup_insert();

DROP TRIGGER IF EXISTS trg_gateway_local_lookup_guard
    ON idc_gateway_local_subject_lookup;
CREATE TRIGGER trg_gateway_local_lookup_guard
BEFORE UPDATE ON idc_gateway_local_subject_lookup
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_lookup_update();

DROP TRIGGER IF EXISTS trg_gateway_local_identity_immutable
    ON idc_gateway_local_identity_operation;
CREATE TRIGGER trg_gateway_local_identity_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_local_identity_operation
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_identity_change();

DROP TRIGGER IF EXISTS trg_gateway_local_identity_insert
    ON idc_gateway_local_identity_operation;
CREATE TRIGGER trg_gateway_local_identity_insert
BEFORE INSERT ON idc_gateway_local_identity_operation
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_local_identity_insert();

DROP TRIGGER IF EXISTS trg_gateway_local_instance_no_delete
    ON idc_gateway_local_supplier_instance;
CREATE TRIGGER trg_gateway_local_instance_no_delete
BEFORE DELETE ON idc_gateway_local_supplier_instance
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_identity_change();

DROP TRIGGER IF EXISTS trg_gateway_local_subject_no_delete
    ON idc_gateway_local_supplier_subject;
CREATE TRIGGER trg_gateway_local_subject_no_delete
BEFORE DELETE ON idc_gateway_local_supplier_subject
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_identity_change();

DROP TRIGGER IF EXISTS trg_gateway_local_lookup_no_delete
    ON idc_gateway_local_subject_lookup;
CREATE TRIGGER trg_gateway_local_lookup_no_delete
BEFORE DELETE ON idc_gateway_local_subject_lookup
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_identity_change();
