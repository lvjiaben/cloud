-- 002: owner-scoped envelope-encrypted secret storage
--
-- Secret plaintext and deployment master keys never enter these tables. Each
-- payload is encrypted with a random data-encryption key (DEK), while one or
-- more versioned master-key wrappers make rotation additive and crash-safe.

-- Purpose: Owner-scoped encrypted secret aggregate without plaintext or master-key material.
-- 用途：保存秘密相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_secret (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "secret_no" VARCHAR(64) NOT NULL,
    "owner_plugin_id" VARCHAR(128) NOT NULL,
    "caller_scope" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'configured',
    "current_version" INT NOT NULL DEFAULT 1,
    "masked_tail" VARCHAR(16) NOT NULL DEFAULT '',
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_secret_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_secret_status CHECK ("status_code" IN ('configured', 'revoked')),
    CONSTRAINT ck_plugin_idc_core_secret_version CHECK ("current_version" > 0),
    CONSTRAINT ck_plugin_idc_core_secret_owner CHECK (length("owner_plugin_id") > 0),
    CONSTRAINT ck_plugin_idc_core_secret_scope CHECK (length("caller_scope") > 0),
    CONSTRAINT ck_plugin_idc_core_secret_revocation CHECK (
        (
            "status_code" = 'configured' AND "revoked_at" IS NULL AND
            (
                "masked_tail" = '****' OR
                (
                    length("masked_tail") = 8 AND left("masked_tail", 4) = '****' AND
                    substring("masked_tail" FROM 5 FOR 4) ~ '^[!-~]{4}$'
                )
            )
        ) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL AND "masked_tail" = '')
    ),
    CONSTRAINT uk_plugin_idc_core_secret_no UNIQUE ("secret_no")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_secret_scope
    ON plugin_idc_core_secret (
        "tenant_id", "owner_plugin_id", "caller_scope", "status_code", "secret_no"
    );

COMMENT ON TABLE plugin_idc_core_secret IS 'Owner-scoped encrypted secret aggregate without plaintext or master-key material';
COMMENT ON COLUMN plugin_idc_core_secret."secret_no" IS 'Opaque secret business number reserved by the core business-number ledger';
COMMENT ON COLUMN plugin_idc_core_secret."owner_plugin_id" IS 'Plugin that owns and authorizes the secret binding';
COMMENT ON COLUMN plugin_idc_core_secret."caller_scope" IS 'Exact least-privilege caller scope bound into encryption AAD';
COMMENT ON COLUMN plugin_idc_core_secret."status_code" IS 'Secret lifecycle state: configured or revoked';
COMMENT ON COLUMN plugin_idc_core_secret."current_version" IS 'Current immutable encrypted payload version';
COMMENT ON COLUMN plugin_idc_core_secret."masked_tail" IS 'Non-secret fixed-prefix display mask with at most four printable trailing bytes';
COMMENT ON COLUMN plugin_idc_core_secret."revoked_at" IS 'Time after which callback access is denied';

-- Purpose: Immutable AES-256-GCM encrypted secret payload versions.
-- 用途：保存秘密版本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_secret_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "secret_no" VARCHAR(64) NOT NULL,
    "version" INT NOT NULL,
    "envelope_version" INT NOT NULL DEFAULT 1,
    "cipher_algorithm" VARCHAR(24) NOT NULL DEFAULT 'AES-256-GCM',
    "payload_nonce" CHAR(24) NOT NULL,
    "ciphertext" TEXT NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_operation" VARCHAR(16) NOT NULL,
    "request_parent_version" INT NOT NULL,
    "request_expected_updated_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_secret_version_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_secret_payload_version CHECK ("version" > 0),
    CONSTRAINT ck_plugin_idc_core_secret_envelope_version CHECK ("envelope_version" = 1),
    CONSTRAINT ck_plugin_idc_core_secret_cipher CHECK ("cipher_algorithm" = 'AES-256-GCM'),
    CONSTRAINT ck_plugin_idc_core_secret_payload_nonce CHECK ("payload_nonce" ~ '^[0-9a-f]{24}$'),
    CONSTRAINT ck_plugin_idc_core_secret_ciphertext CHECK (length("ciphertext") BETWEEN 32 AND 2097184 AND length("ciphertext") % 2 = 0 AND "ciphertext" ~ '^[0-9a-f]+$'),
    CONSTRAINT ck_plugin_idc_core_secret_fingerprint CHECK ("request_fingerprint" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_secret_idempotency CHECK ("idempotency_key_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_secret_request_operation CHECK ("request_operation" IN ('create', 'update')),
    CONSTRAINT ck_plugin_idc_core_secret_request_fence CHECK (
        (
            "request_operation" = 'create' AND "version" = 1 AND
            "request_parent_version" = 0 AND "request_expected_updated_at" IS NULL
        ) OR
        (
            "request_operation" = 'update' AND "request_parent_version" > 0 AND
            "version" = "request_parent_version" + 1 AND
            "request_expected_updated_at" IS NOT NULL
        )
    ),
    CONSTRAINT uk_plugin_idc_core_secret_payload UNIQUE ("secret_no", "version"),
    CONSTRAINT uk_plugin_idc_core_secret_request UNIQUE ("tenant_id", "secret_no", "idempotency_key_digest"),
    CONSTRAINT fk_plugin_idc_core_secret_version_secret FOREIGN KEY ("secret_no")
        REFERENCES plugin_idc_core_secret ("secret_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_secret_version_lookup
    ON plugin_idc_core_secret_version ("tenant_id", "secret_no", "version");

COMMENT ON TABLE plugin_idc_core_secret_version IS 'Immutable AES-256-GCM encrypted secret payload versions';
COMMENT ON COLUMN plugin_idc_core_secret_version."envelope_version" IS 'Canonical envelope and AAD format version';
COMMENT ON COLUMN plugin_idc_core_secret_version."payload_nonce" IS 'Canonical hexadecimal encoding of the random 96-bit AES-GCM payload nonce';
COMMENT ON COLUMN plugin_idc_core_secret_version."ciphertext" IS 'Canonical hexadecimal authenticated ciphertext containing no plaintext key material';
COMMENT ON COLUMN plugin_idc_core_secret_version."request_fingerprint" IS 'DEK-keyed HMAC digest used to reject changed plaintext under one idempotency key';
COMMENT ON COLUMN plugin_idc_core_secret_version."idempotency_key_digest" IS 'SHA-256 digest of the caller-supplied idempotency key';
COMMENT ON COLUMN plugin_idc_core_secret_version."request_operation" IS 'Authenticated write operation discriminator: create or update';
COMMENT ON COLUMN plugin_idc_core_secret_version."request_parent_version" IS 'Authenticated immutable version fence observed before this write';
COMMENT ON COLUMN plugin_idc_core_secret_version."request_expected_updated_at" IS 'Authenticated optimistic timestamp fence supplied for an update';

-- Purpose: Append-only versioned DEK wrappers used for additive master-key rotation.
-- 用途：保存秘密密钥封装相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_secret_key_wrap (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "secret_no" VARCHAR(64) NOT NULL,
    "secret_version" INT NOT NULL,
    "envelope_version" INT NOT NULL DEFAULT 1,
    "master_key_id" VARCHAR(128) NOT NULL,
    "wrap_algorithm" VARCHAR(24) NOT NULL DEFAULT 'AES-256-GCM',
    "wrap_nonce" CHAR(24) NOT NULL,
    "wrapped_dek" CHAR(96) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_secret_wrap_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_secret_wrap_version CHECK ("secret_version" > 0),
    CONSTRAINT ck_plugin_idc_core_secret_wrap_envelope CHECK ("envelope_version" = 1),
    CONSTRAINT ck_plugin_idc_core_secret_wrap_algorithm CHECK ("wrap_algorithm" = 'AES-256-GCM'),
    CONSTRAINT ck_plugin_idc_core_secret_wrap_key_id CHECK (length("master_key_id") > 0),
    CONSTRAINT ck_plugin_idc_core_secret_wrap_nonce CHECK ("wrap_nonce" ~ '^[0-9a-f]{24}$'),
    CONSTRAINT ck_plugin_idc_core_secret_wrapped_dek CHECK ("wrapped_dek" ~ '^[0-9a-f]{96}$'),
    CONSTRAINT uk_plugin_idc_core_secret_key_wrap UNIQUE ("secret_no", "secret_version", "master_key_id"),
    CONSTRAINT fk_plugin_idc_core_secret_wrap_payload FOREIGN KEY ("secret_no", "secret_version")
        REFERENCES plugin_idc_core_secret_version ("secret_no", "version") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_secret_wrap_lookup
    ON plugin_idc_core_secret_key_wrap ("tenant_id", "secret_no", "secret_version", "id" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_secret_wrap_rotation
    ON plugin_idc_core_secret_key_wrap ("tenant_id", "master_key_id", "id");

COMMENT ON TABLE plugin_idc_core_secret_key_wrap IS 'Append-only versioned DEK wrappers used for additive master-key rotation';
COMMENT ON COLUMN plugin_idc_core_secret_key_wrap."master_key_id" IS 'External deployment key identifier; master-key bytes are never persisted';
COMMENT ON COLUMN plugin_idc_core_secret_key_wrap."wrap_nonce" IS 'Canonical hexadecimal encoding of the random 96-bit AES-GCM wrapper nonce';
COMMENT ON COLUMN plugin_idc_core_secret_key_wrap."wrapped_dek" IS 'Canonical hexadecimal authenticated 32-byte DEK ciphertext plus the 16-byte GCM tag';

-- Purpose: Database-shared epoch and write fence for crash-safe cross-instance master-key rotation.
-- 用途：保存秘密轮换状态相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_secret_rotation_state (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "owner_plugin_id" VARCHAR(128) NOT NULL,
    "caller_scope" VARCHAR(128) NOT NULL,
    "epoch" BIGINT NOT NULL DEFAULT 1,
    "active_key_id" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'rotating',
    "write_fence" BIGINT NOT NULL DEFAULT 0,
    "verification_cursor_id" BIGINT NOT NULL DEFAULT 0,
    "verification_write_fence" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_secret_rotation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_epoch CHECK ("epoch" > 0),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_key CHECK (length("active_key_id") > 0),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_status CHECK ("status_code" IN ('rotating', 'stable')),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_fences CHECK (
        "write_fence" >= 0 AND "verification_cursor_id" >= 0 AND
        "verification_write_fence" >= 0 AND "verification_write_fence" <= "write_fence"
    ),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_owner CHECK (length("owner_plugin_id") > 0),
    CONSTRAINT ck_plugin_idc_core_secret_rotation_scope CHECK (length("caller_scope") > 0),
    CONSTRAINT uk_plugin_idc_core_secret_rotation_scope UNIQUE (
        "tenant_id", "owner_plugin_id", "caller_scope"
    )
);

COMMENT ON TABLE plugin_idc_core_secret_rotation_state IS 'Database-shared epoch and write fence for crash-safe cross-instance master-key rotation';
COMMENT ON COLUMN plugin_idc_core_secret_rotation_state."epoch" IS 'Monotonic active-key verification generation';
COMMENT ON COLUMN plugin_idc_core_secret_rotation_state."active_key_id" IS 'Active key identifier certified by the current generation';
COMMENT ON COLUMN plugin_idc_core_secret_rotation_state."write_fence" IS 'Monotonic payload mutation fence shared by all service instances';
COMMENT ON COLUMN plugin_idc_core_secret_rotation_state."verification_cursor_id" IS 'Last immutable payload row authenticated in the current generation';
COMMENT ON COLUMN plugin_idc_core_secret_rotation_state."verification_write_fence" IS 'Write fence captured when the current full verification pass began';

-- Every payload append invalidates an in-progress full verification pass. An
-- update or delete is out-of-band corruption for this append-only model and
-- therefore invalidates even a previously stable certification. Keeping this
-- fence in PostgreSQL coordinates independent service processes and test/admin
-- SQL alike; application memory is never the source of rotation truth.
CREATE OR REPLACE FUNCTION plugin_idc_core_invalidate_secret_rotation()
RETURNS TRIGGER AS $$
DECLARE
    affected_secret_no VARCHAR(64);
BEGIN
    IF TG_OP = 'DELETE' THEN
        affected_secret_no := OLD."secret_no";
    ELSE
        affected_secret_no := NEW."secret_no";
    END IF;

    UPDATE plugin_idc_core_secret_rotation_state AS rotation
    SET
        "status_code" = 'rotating',
        "write_fence" = rotation."write_fence" + 1,
        "verification_cursor_id" = 0,
        "verification_write_fence" = rotation."write_fence" + 1
    FROM plugin_idc_core_secret AS aggregate
    WHERE aggregate."secret_no" = affected_secret_no
      AND rotation."tenant_id" = aggregate."tenant_id"
      AND rotation."owner_plugin_id" = aggregate."owner_plugin_id"
      AND rotation."caller_scope" = aggregate."caller_scope";

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $legacy_secret_trigger_names$
DECLARE
    trigger_contract RECORD;
    legacy_trigger pg_trigger%ROWTYPE;
    current_trigger pg_trigger%ROWTYPE;
BEGIN
    FOR trigger_contract IN
        SELECT * FROM (VALUES
            ('plugin_idc_core_secret_version'::REGCLASS,
             'trg_plugin_linapro_idc_core_secret_version_rotation_fence',
             'trg_plugin_idc_core_secret_version_rotation_fence',
             'plugin_idc_core_invalidate_secret_rotation()'::REGPROCEDURE),
            ('plugin_idc_core_secret_key_wrap'::REGCLASS,
             'trg_plugin_linapro_idc_core_secret_wrap_rotation_fence',
             'trg_plugin_idc_core_secret_wrap_rotation_fence',
             'plugin_idc_core_invalidate_secret_rotation()'::REGPROCEDURE)
        ) AS contracts(table_oid, legacy_name, current_name, function_oid)
    LOOP
        SELECT * INTO legacy_trigger
          FROM pg_trigger
         WHERE tgrelid = trigger_contract.table_oid
           AND tgname = trigger_contract.legacy_name;
        IF FOUND THEN
            IF legacy_trigger.tgisinternal
               OR legacy_trigger.tgconstraint <> 0
               OR legacy_trigger.tgfoid <> trigger_contract.function_oid THEN
                RAISE EXCEPTION 'legacy secret trigger catalog contract is invalid'
                    USING ERRCODE = '23514';
            END IF;

            SELECT * INTO current_trigger
              FROM pg_trigger
             WHERE tgrelid = trigger_contract.table_oid
               AND tgname = trigger_contract.current_name;
            IF FOUND AND (
                current_trigger.tgisinternal
                OR current_trigger.tgconstraint <> 0
                OR current_trigger.tgfoid <> trigger_contract.function_oid
                OR ROW(current_trigger.tgtype, current_trigger.tgnargs,
                       current_trigger.tgargs, current_trigger.tgattr::TEXT,
                       current_trigger.tgqual::TEXT, current_trigger.tgoldtable,
                       current_trigger.tgnewtable)
                   IS DISTINCT FROM
                   ROW(legacy_trigger.tgtype, legacy_trigger.tgnargs,
                       legacy_trigger.tgargs, legacy_trigger.tgattr::TEXT,
                       legacy_trigger.tgqual::TEXT, legacy_trigger.tgoldtable,
                       legacy_trigger.tgnewtable)
            ) THEN
                RAISE EXCEPTION 'legacy and current secret triggers are not equivalent'
                    USING ERRCODE = '23514';
            END IF;

            EXECUTE format(
                'DROP TRIGGER %I ON %s',
                trigger_contract.legacy_name,
                trigger_contract.table_oid
            );
        END IF;
    END LOOP;
END;
$legacy_secret_trigger_names$;

DROP TRIGGER IF EXISTS trg_plugin_idc_core_secret_version_rotation_fence
    ON plugin_idc_core_secret_version;
CREATE TRIGGER trg_plugin_idc_core_secret_version_rotation_fence
AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_secret_version
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_invalidate_secret_rotation();

DROP TRIGGER IF EXISTS trg_plugin_idc_core_secret_wrap_rotation_fence
    ON plugin_idc_core_secret_key_wrap;
CREATE TRIGGER trg_plugin_idc_core_secret_wrap_rotation_fence
AFTER UPDATE OR DELETE ON plugin_idc_core_secret_key_wrap
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_invalidate_secret_rotation();
