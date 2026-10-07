-- Customer-owned SSH public keys for one installation.
-- Public key material is retained only so an explicitly selected active key can be
-- passed to a compatible Provider. Revocation preserves history and never deletes it.

-- Purpose: Stores customer-owned SSH public keys and their lifecycle metadata.
-- 用途：保存客户 SSH 公钥及其生命周期元数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_ssh_public_key (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "key_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "name" VARCHAR(80) NOT NULL,
    "algorithm_code" VARCHAR(64) NOT NULL,
    "public_key" TEXT NOT NULL,
    "fingerprint_sha256" VARCHAR(128) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_at" TIMESTAMPTZ NULL,
    CONSTRAINT uq_plugin_idc_core_customer_ssh_public_key_no
        UNIQUE ("tenant_id", "key_no"),
    CONSTRAINT fk_plugin_idc_core_customer_ssh_public_key_customer
        FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no"),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_name
        CHECK ("name" = BTRIM("name") AND CHAR_LENGTH("name") BETWEEN 1 AND 80),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_algorithm
        CHECK ("algorithm_code" ~ '^[A-Za-z0-9][A-Za-z0-9@._+-]{0,63}$'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_material
        CHECK (CHAR_LENGTH("public_key") BETWEEN 32 AND 16384 AND "public_key" !~ E'[\\r\\n]'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_fingerprint
        CHECK ("fingerprint_sha256" ~ '^SHA256:[A-Za-z0-9+/]{20,120}$'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_request_fingerprint
        CHECK ("request_fingerprint" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_idempotency_digest
        CHECK ("idempotency_key_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_version
        CHECK ("version" >= 1),
    CONSTRAINT ck_plugin_idc_core_customer_ssh_public_key_revoked
        CHECK ("revoked_at" IS NULL OR "revoked_at" >= "created_at")
);

DO $ssh_public_key_constraint_names$
DECLARE
    constraint_contract RECORD;
    legacy_constraint pg_constraint%ROWTYPE;
    current_constraint pg_constraint%ROWTYPE;
BEGIN
    FOR constraint_contract IN
        SELECT * FROM (VALUES
            ('ck_plugin_idc_core_customer_ssh_public_key_idempotency_',
             'ck_plugin_idc_core_customer_ssh_public_key_idempotency_digest'),
            ('ck_plugin_idc_core_customer_ssh_public_key_request_fing',
             'ck_plugin_idc_core_customer_ssh_public_key_request_fingerprint')
        ) AS contracts(legacy_name, current_name)
    LOOP
        SELECT * INTO legacy_constraint
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS
           AND conname = constraint_contract.legacy_name;
        SELECT * INTO current_constraint
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS
           AND conname = constraint_contract.current_name;

        IF legacy_constraint.oid IS NOT NULL THEN
            IF legacy_constraint.contype <> 'c' THEN
                RAISE EXCEPTION 'legacy SSH public key constraint contract is invalid'
                    USING ERRCODE = '23514';
            END IF;
            IF current_constraint.oid IS NOT NULL THEN
                IF legacy_constraint.conbin::TEXT IS DISTINCT FROM current_constraint.conbin::TEXT THEN
                    RAISE EXCEPTION 'legacy and current SSH public key constraints are not equivalent'
                        USING ERRCODE = '23514';
                END IF;
                EXECUTE format(
                    'ALTER TABLE plugin_idc_core_customer_ssh_public_key DROP CONSTRAINT %I',
                    constraint_contract.legacy_name
                );
            ELSE
                EXECUTE format(
                    'ALTER TABLE plugin_idc_core_customer_ssh_public_key RENAME CONSTRAINT %I TO %I',
                    constraint_contract.legacy_name,
                    constraint_contract.current_name
                );
            END IF;
        END IF;
    END LOOP;
END;
$ssh_public_key_constraint_names$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_idc_core_customer_ssh_public_key_active_name
    ON plugin_idc_core_customer_ssh_public_key
    ("tenant_id", "customer_no", LOWER("name"))
    WHERE "revoked_at" IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_idc_core_customer_ssh_public_key_active_fingerprint
    ON plugin_idc_core_customer_ssh_public_key
    ("tenant_id", "customer_no", "fingerprint_sha256")
    WHERE "revoked_at" IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_plugin_idc_core_customer_ssh_public_key_create_request
    ON plugin_idc_core_customer_ssh_public_key
    ("tenant_id", "customer_no", "idempotency_key_digest");

DO $ssh_public_key_index_names$
DECLARE
    index_contract RECORD;
    legacy_index pg_index%ROWTYPE;
    current_index pg_index%ROWTYPE;
BEGIN
    FOR index_contract IN
        SELECT * FROM (VALUES
            ('uq_plugin_idc_core_customer_ssh_public_key_active_finge',
             'uq_plugin_idc_core_customer_ssh_public_key_active_fingerprint'),
            ('uq_plugin_idc_core_customer_ssh_public_key_create_reque',
             'uq_plugin_idc_core_customer_ssh_public_key_create_request')
        ) AS contracts(legacy_name, current_name)
    LOOP
        SELECT index_row.* INTO legacy_index
          FROM pg_class AS class_row
          JOIN pg_index AS index_row ON index_row.indexrelid = class_row.oid
         WHERE class_row.relnamespace = current_schema()::REGNAMESPACE
           AND class_row.relname = index_contract.legacy_name
           AND index_row.indrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS;
        SELECT index_row.* INTO current_index
          FROM pg_class AS class_row
          JOIN pg_index AS index_row ON index_row.indexrelid = class_row.oid
         WHERE class_row.relnamespace = current_schema()::REGNAMESPACE
           AND class_row.relname = index_contract.current_name
           AND index_row.indrelid = 'plugin_idc_core_customer_ssh_public_key'::REGCLASS;

        IF legacy_index.indexrelid IS NOT NULL THEN
            IF current_index.indexrelid IS NULL
               OR ROW(legacy_index.indisunique, legacy_index.indisvalid,
                      legacy_index.indisready, legacy_index.indkey::TEXT,
                      legacy_index.indclass::TEXT, legacy_index.indcollation::TEXT,
                      legacy_index.indoption::TEXT, legacy_index.indexprs::TEXT,
                      legacy_index.indpred::TEXT)
                  IS DISTINCT FROM
                  ROW(current_index.indisunique, current_index.indisvalid,
                      current_index.indisready, current_index.indkey::TEXT,
                      current_index.indclass::TEXT, current_index.indcollation::TEXT,
                      current_index.indoption::TEXT, current_index.indexprs::TEXT,
                      current_index.indpred::TEXT) THEN
                RAISE EXCEPTION 'legacy and current SSH public key indexes are not equivalent'
                    USING ERRCODE = '23514';
            END IF;
            EXECUTE format('DROP INDEX %I', index_contract.legacy_name);
        END IF;
    END LOOP;
END;
$ssh_public_key_index_names$;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_ssh_public_key_list
    ON plugin_idc_core_customer_ssh_public_key
    ("tenant_id", "customer_no", "id" DESC);
