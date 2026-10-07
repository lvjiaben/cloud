-- Tencent template positions and signer roles are version-bound, append-only projections.

ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD COLUMN IF NOT EXISTS "current_provider_mapping_set_no" VARCHAR(64);

ALTER TABLE plugin_idc_compliance_contract_template_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_ctplv_configuration;
ALTER TABLE plugin_idc_compliance_contract_template_version
    ADD CONSTRAINT ck_plugin_idc_compliance_ctplv_configuration
        CHECK (
            (
                "status_code" = 'needs_configuration' AND
                "provider_snapshot_digest" IS NOT NULL AND
                (
                    ("scope_code" IS NULL AND "signing_order_code" IS NULL AND "language_code" IS NULL) OR
                    ("scope_code" IS NOT NULL AND "signing_order_code" IS NOT NULL AND "language_code" IS NOT NULL)
                )
            ) OR
            "status_code" <> 'needs_configuration'
        );

-- Purpose: Persists business records for plugin_idc_compliance_ctpl_provider_component.
-- 用途：持久化 plugin_idc_compliance_ctpl_provider_component 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_ctpl_provider_component (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "component_ref" VARCHAR(128) NOT NULL,
    "name" VARCHAR(100) NOT NULL,
    "type_code" VARCHAR(64) NOT NULL,
    "required" BOOLEAN NOT NULL,
    "recipient_ref" VARCHAR(128),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplpc_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplpc_ref CHECK (char_length("component_ref") BETWEEN 1 AND 128),
    CONSTRAINT ck_plugin_idc_compliance_ctplpc_name CHECK (char_length("name") BETWEEN 1 AND 100),
    CONSTRAINT ck_plugin_idc_compliance_ctplpc_type CHECK (char_length("type_code") BETWEEN 1 AND 64),
    CONSTRAINT uk_plugin_idc_compliance_ctplpc_ref
        UNIQUE ("template_no", "version_no", "component_ref"),
    CONSTRAINT fk_plugin_idc_compliance_ctplpc_version
        FOREIGN KEY ("template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_version ("template_no", "version_no")
        ON DELETE RESTRICT
);

-- Purpose: Persists business records for plugin_idc_compliance_ctpl_provider_recipient.
-- 用途：持久化 plugin_idc_compliance_ctpl_provider_recipient 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_ctpl_provider_recipient (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "recipient_ref" VARCHAR(128) NOT NULL,
    "sequence_no" INT NOT NULL,
    "type_code" VARCHAR(16) NOT NULL,
    "role_name" VARCHAR(100) NOT NULL,
    "require_sign" BOOLEAN NOT NULL,
    "routing_order" BIGINT,
    "is_promoter" BOOLEAN NOT NULL,
    "sign_component_count" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_ref CHECK (char_length("recipient_ref") BETWEEN 1 AND 128),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_sequence CHECK ("sequence_no" BETWEEN 1 AND 64),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_type CHECK ("type_code" IN ('INDIVIDUAL', 'ENTERPRISE', 'PROMOTER')),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_role CHECK (char_length("role_name") BETWEEN 1 AND 100),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_order CHECK ("routing_order" IS NULL OR "routing_order" BETWEEN 0 AND 1000000),
    CONSTRAINT ck_plugin_idc_compliance_ctplpr_sign_count CHECK ("sign_component_count" >= 0),
    CONSTRAINT uk_plugin_idc_compliance_ctplpr_ref
        UNIQUE ("template_no", "version_no", "recipient_ref"),
    CONSTRAINT uk_plugin_idc_compliance_ctplpr_sequence
        UNIQUE ("template_no", "version_no", "sequence_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplpr_version
        FOREIGN KEY ("template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_version ("template_no", "version_no")
        ON DELETE RESTRICT
);

-- Purpose: Persists business records for plugin_idc_compliance_ctpl_provider_mapping_set.
-- 用途：持久化 plugin_idc_compliance_ctpl_provider_mapping_set 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_ctpl_provider_mapping_set (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_set_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "provider_snapshot_digest" CHAR(64) NOT NULL,
    "command_idempotency_digest" CHAR(64) NOT NULL,
    "command_fingerprint" CHAR(64) NOT NULL,
    "created_by_user_id" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplpms_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplpms_digest CHECK (
        "provider_snapshot_digest" ~ '^[0-9a-f]{64}$' AND
        "command_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
        "command_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_compliance_ctplpms_no UNIQUE ("mapping_set_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplpms_version UNIQUE ("template_no", "version_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplpms_command UNIQUE ("tenant_id", "command_idempotency_digest"),
    CONSTRAINT uk_plugin_idc_compliance_ctplpms_owner UNIQUE ("mapping_set_no", "template_no", "version_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplpms_version
        FOREIGN KEY ("template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_version ("template_no", "version_no")
        ON DELETE RESTRICT
);

-- Purpose: Persists business records for plugin_idc_compliance_ctpl_provider_field_mapping.
-- 用途：持久化 plugin_idc_compliance_ctpl_provider_field_mapping 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_ctpl_provider_field_mapping (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_set_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "component_ref" VARCHAR(128) NOT NULL,
    "value_code" VARCHAR(32) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplpfm_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplpfm_value CHECK (
        "value_code" IN (
            'contract_no', 'customer_name', 'merchant_name', 'order_no',
            'service_lines', 'total_amount', 'currency', 'signing_date'
        )
    ),
    CONSTRAINT uk_plugin_idc_compliance_ctplpfm_component
        UNIQUE ("mapping_set_no", "component_ref"),
    CONSTRAINT fk_plugin_idc_compliance_ctplpfm_set
        FOREIGN KEY ("mapping_set_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_ctpl_provider_mapping_set (
            "mapping_set_no", "template_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_ctplpfm_component
        FOREIGN KEY ("template_no", "version_no", "component_ref")
        REFERENCES plugin_idc_compliance_ctpl_provider_component (
            "template_no", "version_no", "component_ref"
        ) ON DELETE RESTRICT
);

-- Purpose: Persists business records for plugin_idc_compliance_ctpl_provider_recipient_mapping.
-- 用途：持久化 plugin_idc_compliance_ctpl_provider_recipient_mapping 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_ctpl_provider_recipient_mapping (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "mapping_set_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "recipient_ref" VARCHAR(128) NOT NULL,
    "party_code" VARCHAR(16) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplprm_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplprm_party CHECK ("party_code" IN ('customer', 'merchant')),
    CONSTRAINT uk_plugin_idc_compliance_ctplprm_recipient
        UNIQUE ("mapping_set_no", "recipient_ref"),
    CONSTRAINT fk_plugin_idc_compliance_ctplprm_set
        FOREIGN KEY ("mapping_set_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_ctpl_provider_mapping_set (
            "mapping_set_no", "template_no", "version_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_ctplprm_recipient
        FOREIGN KEY ("template_no", "version_no", "recipient_ref")
        REFERENCES plugin_idc_compliance_ctpl_provider_recipient (
            "template_no", "version_no", "recipient_ref"
        ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplpc_version
    ON plugin_idc_compliance_ctpl_provider_component (
        "tenant_id", "template_no", "version_no", "component_ref"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplpr_version
    ON plugin_idc_compliance_ctpl_provider_recipient (
        "tenant_id", "template_no", "version_no", "sequence_no"
    );

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract_template_version
        ADD CONSTRAINT fk_plugin_idc_compliance_ctplv_provider_mapping
        FOREIGN KEY ("current_provider_mapping_set_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_ctpl_provider_mapping_set (
            "mapping_set_no", "template_no", "version_no"
        ) DEFERRABLE INITIALLY DEFERRED;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'contract provider mapping history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplpc_append_only
    ON plugin_idc_compliance_ctpl_provider_component;
CREATE TRIGGER trg_plugin_idc_compliance_ctplpc_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_ctpl_provider_component
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplpr_append_only
    ON plugin_idc_compliance_ctpl_provider_recipient;
CREATE TRIGGER trg_plugin_idc_compliance_ctplpr_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_ctpl_provider_recipient
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplpms_append_only
    ON plugin_idc_compliance_ctpl_provider_mapping_set;
CREATE TRIGGER trg_plugin_idc_compliance_ctplpms_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_ctpl_provider_mapping_set
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplpfm_append_only
    ON plugin_idc_compliance_ctpl_provider_field_mapping;
CREATE TRIGGER trg_plugin_idc_compliance_ctplpfm_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_ctpl_provider_field_mapping
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplprm_append_only
    ON plugin_idc_compliance_ctpl_provider_recipient_mapping;
CREATE TRIGGER trg_plugin_idc_compliance_ctplprm_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_ctpl_provider_recipient_mapping
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_provider_mapping_mutation();

COMMENT ON TABLE plugin_idc_compliance_ctpl_provider_component IS
    'Safe immutable Tencent fill-position and signing-date projection bound to one provider template version';
COMMENT ON TABLE plugin_idc_compliance_ctpl_provider_recipient IS
    'Safe immutable Tencent signing-role projection bound to one provider template version';
COMMENT ON TABLE plugin_idc_compliance_ctpl_provider_mapping_set IS
    'One confirmed local field and signer mapping for one exact provider snapshot';
