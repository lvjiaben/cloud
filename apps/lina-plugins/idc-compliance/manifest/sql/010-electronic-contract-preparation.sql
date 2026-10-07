-- Immutable contract preparation snapshot and idempotent launch reservation.

ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD COLUMN IF NOT EXISTS "language_code" VARCHAR(8);

ALTER TABLE plugin_idc_compliance_contract_template_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_ctplv_local_settings;
ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD CONSTRAINT ck_plugin_idc_compliance_ctplv_local_settings
    CHECK (
        ("scope_code" IS NULL AND "signing_order_code" IS NULL AND "language_code" IS NULL) OR
        ("scope_code" IS NOT NULL AND "signing_order_code" IS NOT NULL AND
         "language_code" IN ('zh-CN', 'en-US'))
    );

ALTER TABLE plugin_idc_compliance_contract_template_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_ctplv_configuration;
ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD CONSTRAINT ck_plugin_idc_compliance_ctplv_configuration
    CHECK (
        ("status_code" = 'needs_configuration' AND "scope_code" IS NULL AND
         "signing_order_code" IS NULL AND "language_code" IS NULL AND
         "provider_snapshot_digest" IS NOT NULL) OR
        "status_code" <> 'needs_configuration'
    );

ALTER TABLE plugin_idc_compliance_contract_template_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_ctplv_enable_settings;
ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD CONSTRAINT ck_plugin_idc_compliance_ctplv_enable_settings
    CHECK (
        "status_code" NOT IN ('submission_unavailable', 'submitted', 'enabled', 'disabled') OR
        ("scope_code" IS NOT NULL AND "signing_order_code" IS NOT NULL AND
         "language_code" IN ('zh-CN', 'en-US'))
    );

COMMENT ON COLUMN plugin_idc_compliance_contract_template_version."language_code" IS
    'Explicit contract language; product and service names must use the matching locale';

ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "subject_type_code" VARCHAR(16) NOT NULL DEFAULT 'individual';
ALTER TABLE plugin_idc_compliance_contract
    ALTER COLUMN "subject_type_code" SET DEFAULT 'individual';

UPDATE plugin_idc_compliance_contract
SET "subject_type_code" = 'individual'
WHERE "subject_type_code" = 'personal';
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "signing_order_code" VARCHAR(24) NOT NULL DEFAULT 'customer_first';
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "snapshot_key_ref" VARCHAR(256) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "order_snapshot_sha256" CHAR(64) NOT NULL DEFAULT repeat('0', 64);
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "creation_idempotency_digest" CHAR(64);
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "creation_fingerprint" CHAR(64);
ALTER TABLE plugin_idc_compliance_contract
    ADD COLUMN IF NOT EXISTS "signing_expires_at" TIMESTAMPTZ
    NOT NULL DEFAULT (CURRENT_TIMESTAMP + INTERVAL '7 days');

UPDATE plugin_idc_compliance_contract
SET "signing_expires_at" = "created_at" + INTERVAL '7 days'
WHERE "signing_expires_at" <> "created_at" + INTERVAL '7 days';

ALTER TABLE plugin_idc_compliance_contract
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_contract_subject_type;
ALTER TABLE plugin_idc_compliance_contract
    ADD CONSTRAINT ck_plugin_idc_compliance_contract_subject_type
    CHECK ("subject_type_code" IN ('individual', 'enterprise'));

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract
        ADD CONSTRAINT ck_plugin_idc_compliance_contract_signing_expiry
        CHECK ("signing_expires_at" = "created_at" + INTERVAL '7 days');
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract
        ADD CONSTRAINT ck_plugin_idc_compliance_contract_signing_order
        CHECK ("signing_order_code" IN ('customer_first', 'merchant_first', 'parallel'));
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract
        ADD CONSTRAINT ck_plugin_idc_compliance_contract_snapshot
        CHECK (
            "snapshot_key_ref" <> '' AND
            "order_snapshot_sha256" ~ '^[0-9a-f]{64}$'
        );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract
        ADD CONSTRAINT ck_plugin_idc_compliance_contract_creation
        CHECK (
            ("creation_idempotency_digest" IS NULL AND "creation_fingerprint" IS NULL) OR
            ("creation_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
             "creation_fingerprint" ~ '^[0-9a-f]{64}$')
        );
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_contract_creation
    ON plugin_idc_compliance_contract ("tenant_id", "creation_idempotency_digest")
    WHERE "creation_idempotency_digest" IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_contract_order
    ON plugin_idc_compliance_contract ("tenant_id", "order_no");

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_contract_fulfillment_gate
    ON plugin_idc_compliance_contract (
        "tenant_id", "required_for_fulfillment", "status_code", "order_no", "id"
    ) WHERE "required_for_fulfillment";

COMMENT ON COLUMN plugin_idc_compliance_contract."snapshot_key_ref" IS
    'Opaque secretcap handle for the encrypted contract preparation bundle';
COMMENT ON COLUMN plugin_idc_compliance_contract."order_snapshot_sha256" IS
    'Digest of the canonical immutable core order, service, identity, and amount snapshot';
COMMENT ON COLUMN plugin_idc_compliance_contract."signing_expires_at" IS
    'Fixed signing deadline exactly seven days after authoritative contract creation';

CREATE OR REPLACE FUNCTION plugin_idc_compliance_guard_contract_snapshot()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF (
        to_jsonb(NEW) - ARRAY[
            'status_code', 'launched_at', 'completed_at', 'terminated_at',
            'version', 'updated_at'
        ]
    ) IS DISTINCT FROM (
        to_jsonb(OLD) - ARRAY[
            'status_code', 'launched_at', 'completed_at', 'terminated_at',
            'version', 'updated_at'
        ]
    ) THEN
        RAISE EXCEPTION 'prepared contract business snapshot is immutable';
    END IF;
    IF NEW."version" <= OLD."version" THEN
        RAISE EXCEPTION 'prepared contract version must advance';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_contract_snapshot_guard
    ON plugin_idc_compliance_contract;
CREATE TRIGGER trg_plugin_idc_compliance_contract_snapshot_guard
    BEFORE UPDATE ON plugin_idc_compliance_contract
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_guard_contract_snapshot();
