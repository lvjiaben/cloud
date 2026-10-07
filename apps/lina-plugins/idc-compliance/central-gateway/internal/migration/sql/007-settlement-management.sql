-- Purpose: Persists business records for idc_gateway_settlement_management_operation.
-- 用途：持久化 idc_gateway_settlement_management_operation 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_management_operation (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "operation_kind" VARCHAR(32) NOT NULL,
    "operation_hmac" BYTEA NOT NULL,
    "request_sha256" BYTEA NOT NULL,
    "result_kind" VARCHAR(24) NOT NULL,
    "result_ref" VARCHAR(128) NOT NULL,
    "actor_ref" VARCHAR(128) NOT NULL,
    full_auth_at TIMESTAMPTZ NOT NULL,
    "evidence_sha256" BYTEA NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_settlement_management_operation UNIQUE ("operation_hmac"),
    CONSTRAINT uq_gateway_settlement_management_result UNIQUE (
        "operation_no", "operation_kind", "result_kind", "result_ref"
    ),
    CONSTRAINT ck_gateway_settlement_management_kind CHECK (
        "operation_kind" IN (
            'prepaid_fund', 'prepaid_adjustment', 'postpaid_approval',
            'postpaid_adjustment', 'price_publish', 'provider_availability'
        )
    ),
    CONSTRAINT ck_gateway_settlement_management_result CHECK (
        ("operation_kind" IN (
            'prepaid_fund', 'prepaid_adjustment', 'postpaid_approval',
            'postpaid_adjustment'
        ) AND "result_kind" = 'account')
        OR ("operation_kind" = 'price_publish' AND "result_kind" = 'price_version')
        OR ("operation_kind" = 'provider_availability' AND "result_kind" = 'provider_gate')
    ),
    CONSTRAINT ck_gateway_settlement_management_digest CHECK (
        octet_length("operation_hmac") = 32
        AND octet_length("request_sha256") = 32
        AND octet_length("evidence_sha256") = 32
    ),
    CONSTRAINT ck_gateway_settlement_management_refs CHECK (
        "result_ref" = btrim("result_ref") AND char_length("result_ref") BETWEEN 1 AND 128
        AND "actor_ref" = btrim("actor_ref") AND char_length("actor_ref") BETWEEN 1 AND 128
        AND "reason_code" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_settlement_management_auth CHECK (
        created_at >= full_auth_at
        AND created_at - full_auth_at <= INTERVAL '10 minutes'
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_management_actor
    ON idc_gateway_settlement_management_operation (
        "actor_ref", created_at DESC, "operation_no"
    );

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_management_result
    ON idc_gateway_settlement_management_operation (
        "result_kind", "result_ref", created_at, "operation_no"
    );

ALTER TABLE idc_gateway_price_version
    ADD COLUMN IF NOT EXISTS management_operation_no VARCHAR(40);
ALTER TABLE idc_gateway_price_version
    DROP CONSTRAINT IF EXISTS fk_gateway_price_version_management_operation;
ALTER TABLE idc_gateway_price_version
    ADD CONSTRAINT fk_gateway_price_version_management_operation FOREIGN KEY (
        management_operation_no
    ) REFERENCES idc_gateway_settlement_management_operation ("operation_no");

ALTER TABLE idc_gateway_settlement_ledger_entry
    ADD COLUMN IF NOT EXISTS management_operation_no VARCHAR(40);
ALTER TABLE idc_gateway_settlement_ledger_entry
    DROP CONSTRAINT IF EXISTS fk_gateway_settlement_ledger_management_operation;
ALTER TABLE idc_gateway_settlement_ledger_entry
    ADD CONSTRAINT fk_gateway_settlement_ledger_management_operation FOREIGN KEY (
        management_operation_no
    ) REFERENCES idc_gateway_settlement_management_operation ("operation_no");

-- Purpose: Persists business records for idc_gateway_provider_billing_gate.
-- 用途：持久化 idc_gateway_provider_billing_gate 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_provider_billing_gate (
    "gate_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "status" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "last_management_operation_no" VARCHAR(40) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_provider_billing_gate UNIQUE (
        "provider_code", "operation_scope"
    ),
    CONSTRAINT fk_gateway_provider_billing_gate_operation FOREIGN KEY (
        "last_management_operation_no"
    ) REFERENCES idc_gateway_settlement_management_operation ("operation_no")
        DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT ck_gateway_provider_billing_gate_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_provider_billing_gate_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_provider_billing_gate_status CHECK (
        "status" IN ('enabled', 'disabled')
    ),
    CONSTRAINT ck_gateway_provider_billing_gate_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_provider_billing_gate_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_provider_billing_gate_time CHECK (
        updated_at >= created_at
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_provider_billing_gate_status
    ON idc_gateway_provider_billing_gate (
        "status", "provider_code", "operation_scope", "gate_no"
    );

-- Purpose: Persists business records for idc_gateway_settlement_management_account_result.
-- 用途：持久化 idc_gateway_settlement_management_account_result 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_management_account_result (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "operation_kind" VARCHAR(32) NOT NULL,
    "result_kind" VARCHAR(24) NOT NULL DEFAULT 'account',
    "account_no" VARCHAR(40) NOT NULL,
    "ledger_entry_no" VARCHAR(40),
    "settlement_mode" VARCHAR(16) NOT NULL,
    "prepaid_available_minor" BIGINT NOT NULL,
    "held_minor" BIGINT NOT NULL,
    "postpaid_outstanding_minor" BIGINT NOT NULL,
    "postpaid_limit_minor" BIGINT NOT NULL,
    postpaid_valid_until TIMESTAMPTZ,
    "account_version" BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_gateway_settlement_management_account_operation FOREIGN KEY (
        "operation_no", "operation_kind", "result_kind", "account_no"
    ) REFERENCES idc_gateway_settlement_management_operation (
        "operation_no", "operation_kind", "result_kind", "result_ref"
    ),
    CONSTRAINT fk_gateway_settlement_management_account FOREIGN KEY ("account_no")
        REFERENCES idc_gateway_settlement_account ("account_no"),
    CONSTRAINT fk_gateway_settlement_management_account_ledger FOREIGN KEY (
        "ledger_entry_no", "account_no"
    ) REFERENCES idc_gateway_settlement_ledger_entry (
        "ledger_entry_no", "account_no"
    ),
    CONSTRAINT ck_gateway_settlement_management_account_kind CHECK (
        "operation_kind" IN (
            'prepaid_fund', 'prepaid_adjustment', 'postpaid_approval',
            'postpaid_adjustment'
        )
        AND "result_kind" = 'account'
        AND (("operation_kind" = 'postpaid_approval' AND "ledger_entry_no" IS NULL)
            OR ("operation_kind" <> 'postpaid_approval' AND "ledger_entry_no" IS NOT NULL))
    ),
    CONSTRAINT ck_gateway_settlement_management_account_amounts CHECK (
        "prepaid_available_minor" BETWEEN 0 AND 9000000000000000
        AND "held_minor" BETWEEN 0 AND 9000000000000000
        AND "postpaid_outstanding_minor" BETWEEN 0 AND 9000000000000000
        AND "postpaid_limit_minor" BETWEEN 0 AND 9000000000000000
    ),
    CONSTRAINT ck_gateway_settlement_management_account_projection CHECK (
        ("settlement_mode" = 'prepaid' AND "postpaid_outstanding_minor" = 0
            AND "postpaid_limit_minor" = 0 AND postpaid_valid_until IS NULL)
        OR ("settlement_mode" = 'postpaid' AND "prepaid_available_minor" = 0
            AND "postpaid_limit_minor" > 0 AND postpaid_valid_until IS NOT NULL
            AND "held_minor"::numeric + "postpaid_outstanding_minor"::numeric
                <= "postpaid_limit_minor"::numeric)
    ),
    CONSTRAINT ck_gateway_settlement_management_account_version CHECK (
        "account_version" > 0
    )
);

-- Purpose: Persists business records for idc_gateway_settlement_management_price_result.
-- 用途：持久化 idc_gateway_settlement_management_price_result 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_management_price_result (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "operation_kind" VARCHAR(32) NOT NULL DEFAULT 'price_publish',
    "result_kind" VARCHAR(24) NOT NULL DEFAULT 'price_version',
    "price_version_no" VARCHAR(40) NOT NULL UNIQUE,
    "item_count" SMALLINT NOT NULL,
    published_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_gateway_settlement_management_price_operation FOREIGN KEY (
        "operation_no", "operation_kind", "result_kind", "price_version_no"
    ) REFERENCES idc_gateway_settlement_management_operation (
        "operation_no", "operation_kind", "result_kind", "result_ref"
    ),
    CONSTRAINT fk_gateway_settlement_management_price FOREIGN KEY (
        "price_version_no"
    ) REFERENCES idc_gateway_price_version ("price_version_no"),
    CONSTRAINT ck_gateway_settlement_management_price_kind CHECK (
        "operation_kind" = 'price_publish' AND "result_kind" = 'price_version'
    ),
    CONSTRAINT ck_gateway_settlement_management_price_count CHECK (
        "item_count" BETWEEN 1 AND 16
    ),
    CONSTRAINT ck_gateway_settlement_management_price_time CHECK (
        created_at = published_at
    )
);

-- Purpose: Persists business records for idc_gateway_settlement_management_gate_result.
-- 用途：持久化 idc_gateway_settlement_management_gate_result 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_management_gate_result (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "operation_kind" VARCHAR(32) NOT NULL DEFAULT 'provider_availability',
    "result_kind" VARCHAR(24) NOT NULL DEFAULT 'provider_gate',
    "gate_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL,
    "gate_version" BIGINT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_gateway_settlement_management_gate_operation FOREIGN KEY (
        "operation_no", "operation_kind", "result_kind", "gate_no"
    ) REFERENCES idc_gateway_settlement_management_operation (
        "operation_no", "operation_kind", "result_kind", "result_ref"
    ),
    CONSTRAINT fk_gateway_settlement_management_gate FOREIGN KEY ("gate_no")
        REFERENCES idc_gateway_provider_billing_gate ("gate_no"),
    CONSTRAINT ck_gateway_settlement_management_gate_kind CHECK (
        "operation_kind" = 'provider_availability' AND "result_kind" = 'provider_gate'
    ),
    CONSTRAINT ck_gateway_settlement_management_gate_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_settlement_management_gate_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_settlement_management_gate_version CHECK (
        "gate_version" > 0
    ),
    CONSTRAINT ck_gateway_settlement_management_gate_time CHECK (
        created_at = updated_at
    )
);

CREATE OR REPLACE FUNCTION idc_gateway_reject_settlement_management_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway settlement management history is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_price_version_update()
RETURNS TRIGGER AS $$
DECLARE
    management_proof_count INTEGER;
BEGIN
    IF TG_OP = 'INSERT' THEN
        SELECT COUNT(*)
        INTO management_proof_count
        FROM idc_gateway_settlement_management_operation
        WHERE "operation_no" = NEW.management_operation_no
          AND "operation_kind" = 'price_publish'
          AND "result_kind" = 'price_version'
          AND "result_ref" = NEW."price_version_no"
          AND "actor_ref" = NEW."published_by_actor_ref"
          AND "evidence_sha256" = NEW."published_authorization_sha256";
        IF NEW."status" <> 'draft' OR NEW.published_at IS NOT NULL
            OR management_proof_count <> 1
        THEN
            RAISE EXCEPTION 'gateway price version must start with exact management proof';
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE'
        OR OLD."price_version_no" IS DISTINCT FROM NEW."price_version_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."currency_code" IS DISTINCT FROM NEW."currency_code"
        OR OLD."published_by_actor_ref" IS DISTINCT FROM NEW."published_by_actor_ref"
        OR OLD."published_authorization_sha256"
            IS DISTINCT FROM NEW."published_authorization_sha256"
        OR OLD.management_operation_no IS DISTINCT FROM NEW.management_operation_no
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status" <> 'draft' OR NEW."status" <> 'published'
        OR OLD.published_at IS NOT NULL OR NEW.published_at IS NULL
    THEN
        RAISE EXCEPTION 'gateway published price version is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_management_ledger()
RETURNS TRIGGER AS $$
DECLARE
    proof_count INTEGER;
BEGIN
    IF NEW."entry_kind" IN (
        'prepaid_fund', 'prepaid_adjustment', 'postpaid_adjustment'
    ) THEN
        SELECT COUNT(*)
        INTO proof_count
        FROM idc_gateway_settlement_management_operation
        WHERE "operation_no" = NEW.management_operation_no
          AND "operation_kind" = NEW."entry_kind"
          AND "operation_hmac" = NEW."operation_hmac"
          AND "request_sha256" = NEW."request_sha256"
          AND "result_kind" = 'account'
          AND "result_ref" = NEW."account_no";
        IF proof_count <> 1 THEN
            RAISE EXCEPTION 'gateway management ledger has no exact operation proof';
        END IF;
    ELSIF NEW.management_operation_no IS NOT NULL THEN
        RAISE EXCEPTION 'gateway non-management ledger cannot claim management proof';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_settlement_management_result()
RETURNS TRIGGER AS $$
DECLARE
    result_count INTEGER;
BEGIN
    IF NEW."result_kind" = 'account' THEN
        SELECT COUNT(*)
        INTO result_count
        FROM idc_gateway_settlement_management_account_result
        WHERE "operation_no" = NEW."operation_no"
          AND "operation_kind" = NEW."operation_kind"
          AND "account_no" = NEW."result_ref";
    ELSIF NEW."result_kind" = 'price_version' THEN
        SELECT COUNT(*)
        INTO result_count
        FROM idc_gateway_settlement_management_price_result
        WHERE "operation_no" = NEW."operation_no"
          AND "price_version_no" = NEW."result_ref";
    ELSIF NEW."result_kind" = 'provider_gate' THEN
        SELECT COUNT(*)
        INTO result_count
        FROM idc_gateway_settlement_management_gate_result
        WHERE "operation_no" = NEW."operation_no"
          AND "gate_no" = NEW."result_ref";
    ELSE
        RAISE EXCEPTION 'gateway settlement management result kind is invalid';
    END IF;
    IF result_count <> 1 THEN
        RAISE EXCEPTION 'gateway settlement management operation has no exact result';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_management_account_result()
RETURNS TRIGGER AS $$
DECLARE
    projection_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO projection_count
    FROM idc_gateway_settlement_account AS account
    JOIN idc_gateway_settlement_management_operation AS "operation"
      ON "operation"."operation_no" = NEW."operation_no"
    WHERE account."account_no" = NEW."account_no"
      AND account."settlement_mode" = NEW."settlement_mode"
      AND account."prepaid_available_minor" = NEW."prepaid_available_minor"
      AND account."held_minor" = NEW."held_minor"
      AND account."postpaid_outstanding_minor" = NEW."postpaid_outstanding_minor"
      AND account."postpaid_limit_minor" = NEW."postpaid_limit_minor"
      AND account.postpaid_valid_until IS NOT DISTINCT FROM NEW.postpaid_valid_until
      AND account."version" = NEW."account_version"
      AND account.updated_at = NEW.created_at
      AND "operation".created_at = NEW.created_at;
    IF projection_count <> 1 THEN
        RAISE EXCEPTION 'gateway management account result has no exact projection';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_management_price_result()
RETURNS TRIGGER AS $$
DECLARE
    projection_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO projection_count
    FROM idc_gateway_price_version AS "price"
    JOIN idc_gateway_settlement_management_operation AS "operation"
      ON "operation"."operation_no" = NEW."operation_no"
    WHERE "price"."price_version_no" = NEW."price_version_no"
      AND "price"."status" = 'published'
      AND "price".management_operation_no = NEW."operation_no"
      AND "price".published_at = NEW.published_at
      AND "operation".created_at = NEW.created_at
      AND (SELECT COUNT(*) FROM idc_gateway_price_item AS item
           WHERE item."price_version_no" = "price"."price_version_no") = NEW."item_count";
    IF projection_count <> 1 THEN
        RAISE EXCEPTION 'gateway management price result has no exact projection';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_management_gate_result()
RETURNS TRIGGER AS $$
DECLARE
    projection_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO projection_count
    FROM idc_gateway_provider_billing_gate AS gate
    JOIN idc_gateway_settlement_management_operation AS "operation"
      ON "operation"."operation_no" = NEW."operation_no"
    WHERE gate."gate_no" = NEW."gate_no"
      AND gate."provider_code" = NEW."provider_code"
      AND gate."operation_scope" = NEW."operation_scope"
      AND (gate."status" = 'enabled') = NEW.enabled
      AND gate."version" = NEW."gate_version"
      AND gate."last_management_operation_no" = NEW."operation_no"
      AND gate.updated_at = NEW.updated_at
      AND "operation".created_at = NEW.created_at;
    IF projection_count <> 1 THEN
        RAISE EXCEPTION 'gateway management gate result has no exact projection';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_provider_billing_gate()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."version" <> 1 OR NEW.created_at <> NEW.updated_at THEN
            RAISE EXCEPTION 'gateway provider billing gate must start at version one';
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE'
        OR OLD."gate_no" IS DISTINCT FROM NEW."gate_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR NEW."version" <> OLD."version" + 1
        OR NEW.updated_at <= OLD.updated_at
        OR NEW."last_management_operation_no"
            IS NOT DISTINCT FROM OLD."last_management_operation_no"
    THEN
        RAISE EXCEPTION 'gateway provider billing gate transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_provider_billing_gate_operation()
RETURNS TRIGGER AS $$
DECLARE
    proof_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO proof_count
    FROM idc_gateway_settlement_management_operation
    WHERE "operation_no" = NEW."last_management_operation_no"
      AND "operation_kind" = 'provider_availability'
      AND "result_kind" = 'provider_gate'
      AND "result_ref" = NEW."gate_no"
      AND created_at = NEW.updated_at;
    IF proof_count <> 1 THEN
        RAISE EXCEPTION 'gateway provider billing gate has no exact management proof';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_immutable
    ON idc_gateway_settlement_management_operation;
CREATE TRIGGER trg_gateway_settlement_management_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_management_operation
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_management_mutation();

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_account_result_immutable
    ON idc_gateway_settlement_management_account_result;
CREATE TRIGGER trg_gateway_settlement_management_account_result_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_management_account_result
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_management_mutation();

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_price_result_immutable
    ON idc_gateway_settlement_management_price_result;
CREATE TRIGGER trg_gateway_settlement_management_price_result_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_management_price_result
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_management_mutation();

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_gate_result_immutable
    ON idc_gateway_settlement_management_gate_result;
CREATE TRIGGER trg_gateway_settlement_management_gate_result_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_management_gate_result
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_management_mutation();

DROP TRIGGER IF EXISTS trg_gateway_price_version_immutable ON idc_gateway_price_version;
CREATE TRIGGER trg_gateway_price_version_immutable
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_price_version
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_price_version_update();

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_ledger
    ON idc_gateway_settlement_ledger_entry;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_management_ledger
AFTER INSERT ON idc_gateway_settlement_ledger_entry
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_management_ledger();

DROP TRIGGER IF EXISTS trg_gateway_settlement_management_result
    ON idc_gateway_settlement_management_operation;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_management_result
AFTER INSERT ON idc_gateway_settlement_management_operation
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_management_result();

DROP TRIGGER IF EXISTS trg_gateway_management_account_result_projection
    ON idc_gateway_settlement_management_account_result;
CREATE CONSTRAINT TRIGGER trg_gateway_management_account_result_projection
AFTER INSERT ON idc_gateway_settlement_management_account_result
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_management_account_result();

DROP TRIGGER IF EXISTS trg_gateway_management_price_result_projection
    ON idc_gateway_settlement_management_price_result;
CREATE CONSTRAINT TRIGGER trg_gateway_management_price_result_projection
AFTER INSERT ON idc_gateway_settlement_management_price_result
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_management_price_result();

DROP TRIGGER IF EXISTS trg_gateway_management_gate_result_projection
    ON idc_gateway_settlement_management_gate_result;
CREATE CONSTRAINT TRIGGER trg_gateway_management_gate_result_projection
AFTER INSERT ON idc_gateway_settlement_management_gate_result
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_management_gate_result();

DROP TRIGGER IF EXISTS trg_gateway_provider_billing_gate_guard
    ON idc_gateway_provider_billing_gate;
CREATE TRIGGER trg_gateway_provider_billing_gate_guard
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_provider_billing_gate
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_provider_billing_gate();

DROP TRIGGER IF EXISTS trg_gateway_provider_billing_gate_operation
    ON idc_gateway_provider_billing_gate;
CREATE CONSTRAINT TRIGGER trg_gateway_provider_billing_gate_operation
AFTER INSERT OR UPDATE ON idc_gateway_provider_billing_gate
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_provider_billing_gate_operation();
