-- Purpose: Persists business records for plugin_idc_compliance_supplier_configuration.
-- 用途：持久化 plugin_idc_compliance_supplier_configuration 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_supplier_configuration (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "configuration_no" VARCHAR(96) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'draft',
    "merchant_subject_no" VARCHAR(128) NOT NULL DEFAULT '',
    "credential_secret_ref" VARCHAR(256),
    "credential_generation" BIGINT NOT NULL DEFAULT 0,
    "capability_evidence_code" VARCHAR(64) NOT NULL DEFAULT 'external_confirmation_required',
    "adapter_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
    "last_mutation_digest" CHAR(64),
    "mutation_operation_key" CHAR(64),
    "mutation_secret_updated_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_scfg_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_scfg_provider
        CHECK ("provider_code" IN ('nuonuo_invoice')),
    CONSTRAINT ck_plugin_idc_compliance_scfg_status
        CHECK ("status_code" IN ('draft', 'updating', 'configured_blocked', 'disabled')),
    CONSTRAINT ck_plugin_idc_compliance_scfg_subject
        CHECK (
            "merchant_subject_no" = '' OR
            (
                length("merchant_subject_no") = 37 AND
                "merchant_subject_no" ~ '^msub_[0-9a-f]{32}$'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_scfg_generation
        CHECK ("credential_generation" >= 0),
    CONSTRAINT ck_plugin_idc_compliance_scfg_evidence
        CHECK ("capability_evidence_code" = 'external_confirmation_required'),
    CONSTRAINT ck_plugin_idc_compliance_scfg_adapter
        CHECK ("adapter_enabled" = FALSE),
    CONSTRAINT ck_plugin_idc_compliance_scfg_ready
        CHECK (
            "status_code" <> 'configured_blocked' OR
            (
                "merchant_subject_no" <> '' AND
                "credential_secret_ref" IS NOT NULL AND
                "credential_generation" > 0 AND
                "adapter_enabled" = FALSE
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_scfg_digest
        CHECK (
            "last_mutation_digest" IS NULL OR
            "last_mutation_digest" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_scfg_fence
        CHECK (
            "status_code" <> 'updating' OR
            (
                "last_mutation_digest" IS NOT NULL AND
                "mutation_operation_key" IS NOT NULL AND
                "mutation_operation_key" ~ '^[0-9a-f]{64}$' AND
                "adapter_enabled" = FALSE
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_scfg_version
        CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_scfg_no
        UNIQUE ("configuration_no"),
    CONSTRAINT uk_plugin_idc_compliance_scfg_provider
        UNIQUE ("tenant_id", "provider_code")
);

COMMENT ON TABLE plugin_idc_compliance_supplier_configuration IS
    'Installation-local merchant-owned supplier configuration; adapters remain database-enforced off until external capability evidence is separately confirmed';
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name = 'plugin_idc_compliance_supplier_configuration'
          AND column_name = 'merchant_subject_no'
    ) THEN
        COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."merchant_subject_no" IS
            'Opaque merchant-owned local subject reference; external taxpayer identifiers and legal-profile plaintext are forbidden';
    END IF;
END
$$;
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."credential_secret_ref" IS
    'Sensitive secretcap routing reference for one provider-neutral credential envelope; plaintext and ciphertext are forbidden';
COMMENT ON COLUMN plugin_idc_compliance_supplier_configuration."capability_evidence_code" IS
    'Fail-closed external evidence gate; this migration deliberately permits only external_confirmation_required';

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_scfg_gate
    ON plugin_idc_compliance_supplier_configuration (
        "tenant_id", "adapter_enabled", "capability_evidence_code", "provider_code"
    );
