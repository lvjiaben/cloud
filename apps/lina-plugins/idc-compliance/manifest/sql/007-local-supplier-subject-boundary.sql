-- Split merchant-owned supplier credentials from installation-local subject bindings.
-- Supplier subject plaintext remains exclusively inside the local compliance gateway.

ALTER TABLE plugin_idc_compliance_supplier_configuration
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_ready,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_fence,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_subject,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_status,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_last_operation,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_subject_binding_state,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_subject_proof,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_scfg_subject_pending_proof;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_compliance_supplier_configuration'
          AND column_name = 'status_code'
    ) AND NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_compliance_supplier_configuration'
          AND column_name = 'credential_status_code'
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            RENAME COLUMN "status_code" TO "credential_status_code";
    END IF;
END $$;

ALTER TABLE plugin_idc_compliance_supplier_configuration
    DROP COLUMN IF EXISTS "merchant_subject_no",
    ADD COLUMN IF NOT EXISTS "subject_status_code" VARCHAR(32) NOT NULL DEFAULT 'unbound',
    ADD COLUMN IF NOT EXISTS "subject_binding_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "last_operation_kind_code" VARCHAR(32) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS "last_operation_result_code" VARCHAR(32) NOT NULL DEFAULT 'none',
    ADD COLUMN IF NOT EXISTS "last_operation_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "last_operation_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "subject_proof_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "subject_proof_expires_at" TIMESTAMPTZ;

UPDATE plugin_idc_compliance_supplier_configuration
SET "credential_status_code" = CASE "credential_status_code"
    WHEN 'draft' THEN 'unconfigured'
    WHEN 'configured_blocked' THEN 'configured'
    WHEN 'disabled' THEN 'unconfigured'
    ELSE "credential_status_code"
END;

UPDATE plugin_idc_compliance_supplier_configuration
SET "subject_status_code" = 'retryable_failed',
    "last_operation_result_code" = 'retryable_failed',
    "subject_proof_digest" = NULL,
    "subject_proof_expires_at" = NULL
WHERE (
        "subject_status_code" IN ('binding', 'reopening') OR
        "last_operation_result_code" = 'pending'
    )
  AND (
        "subject_proof_digest" IS NULL OR
        "subject_proof_expires_at" IS NULL
    );

ALTER TABLE plugin_idc_compliance_supplier_configuration
    ALTER COLUMN "credential_status_code" SET DEFAULT 'unconfigured';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_credential_status'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_credential_status
            CHECK ("credential_status_code" IN (
                'unconfigured', 'updating', 'configured', 'retryable_failed'
            ));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_subject_status'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_subject_status
            CHECK ("subject_status_code" IN (
                'unbound', 'binding', 'active', 'revoked', 'reopening', 'retryable_failed'
            ));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_binding_no'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_binding_no
            CHECK (
                "subject_binding_no" IS NULL OR
                "subject_binding_no" ~ '^lsb_[0-9a-f]{32}$'
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_last_operation'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_last_operation
            CHECK (
                "last_operation_kind_code" IN ('none', 'subject_bind', 'subject_revoke', 'subject_reopen') AND
                "last_operation_result_code" IN ('none', 'pending', 'succeeded', 'retryable_failed') AND
                ("last_operation_no" IS NULL OR "last_operation_no" ~ '^lsi_[0-9a-f]{32}$') AND
                (
                    (
                        "last_operation_kind_code" = 'none' AND
                        "last_operation_result_code" = 'none' AND
                        "last_operation_no" IS NULL AND
                        "last_operation_at" IS NULL
                    ) OR
                    (
                        "last_operation_kind_code" <> 'none' AND
                        "last_operation_result_code" <> 'none' AND
                        "last_operation_at" IS NOT NULL
                    )
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_subject_binding_state'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_subject_binding_state
            CHECK (
                (
                    "subject_status_code" IN ('unbound', 'binding') AND
                    "subject_binding_no" IS NULL
                ) OR
                (
                    "subject_status_code" IN ('active', 'revoked', 'reopening') AND
                    "subject_binding_no" IS NOT NULL
                ) OR
                "subject_status_code" = 'retryable_failed'
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_subject_proof'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_subject_proof
            CHECK (
                (
                    "subject_proof_digest" IS NULL AND
                    "subject_proof_expires_at" IS NULL
                ) OR
                (
                    "subject_proof_digest" ~ '^[0-9a-f]{64}$' AND
                    "subject_proof_expires_at" IS NOT NULL
                )
            ) NOT VALID;
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            VALIDATE CONSTRAINT ck_plugin_idc_compliance_scfg_subject_proof;
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_subject_pending_proof
            CHECK (
                "last_operation_result_code" <> 'pending' OR
                (
                    "subject_status_code" IN ('binding', 'reopening') AND
                    "subject_proof_digest" IS NOT NULL AND
                    "subject_proof_expires_at" IS NOT NULL
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_credential_ready'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_credential_ready
            CHECK (
                "credential_status_code" <> 'configured' OR
                (
                    "credential_secret_ref" IS NOT NULL AND
                    "credential_generation" > 0 AND
                    "adapter_enabled" = FALSE
                )
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_scfg_credential_fence'
          AND conrelid = 'plugin_idc_compliance_supplier_configuration'::regclass
    ) THEN
        ALTER TABLE plugin_idc_compliance_supplier_configuration
            ADD CONSTRAINT ck_plugin_idc_compliance_scfg_credential_fence
            CHECK (
                "credential_status_code" <> 'updating' OR
                (
                    "last_mutation_digest" IS NOT NULL AND
                    "mutation_operation_key" IS NOT NULL AND
                    "mutation_operation_key" ~ '^[0-9a-f]{64}$' AND
                    "adapter_enabled" = FALSE
                )
            );
    END IF;
END $$;

COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."credential_status_code" IS
    'Credential lifecycle only; independent from supplier subject binding state';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."subject_binding_no" IS
    'Opaque installation-local gateway binding identifier; supplier subject plaintext is forbidden';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."last_operation_no" IS
    'Opaque plugin-local operation identifier; provider payloads and subject identifiers are forbidden';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."subject_proof_digest" IS
    'Digest of the current server-only single-use proof; raw proof tokens are forbidden';
