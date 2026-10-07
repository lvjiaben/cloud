-- Installation-local electronic-contract template catalog and immutable authoring revisions.
-- Contract bodies stay in encrypted private object storage; database rows contain only opaque
-- references, integrity metadata, safe list projections, and normalized field coordinates.

-- Purpose: Persists business records for plugin_idc_compliance_contract_template.
-- 用途：持久化 plugin_idc_compliance_contract_template 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_template (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "template_no" VARCHAR(64) NOT NULL,
    "origin_code" VARCHAR(16) NOT NULL,
    "provider_code" VARCHAR(32),
    "provider_template_ref" VARCHAR(256),
    "provider_state_code" VARCHAR(24),
    "provider_last_seen_sync_no" VARCHAR(64),
    "provider_updated_at" TIMESTAMPTZ,
    "creation_idempotency_digest" CHAR(64),
    "creation_fingerprint" CHAR(64),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctpl_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctpl_origin
        CHECK ("origin_code" IN ('local', 'tencent_synced')),
    CONSTRAINT ck_plugin_idc_compliance_ctpl_provider
        CHECK (
            (
                "origin_code" = 'local' AND
                "provider_code" IS NULL AND
                "provider_template_ref" IS NULL AND
                "provider_state_code" IS NULL AND
                "provider_last_seen_sync_no" IS NULL AND
                "provider_updated_at" IS NULL
            ) OR
            (
                "origin_code" = 'tencent_synced' AND
                "provider_code" = 'tencent_essbasic' AND
                "provider_template_ref" IS NOT NULL AND
                "provider_state_code" IN ('active', 'disabled', 'offline') AND
                "provider_last_seen_sync_no" IS NOT NULL AND
                "provider_updated_at" IS NOT NULL
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctpl_creation
        CHECK (
            ("creation_idempotency_digest" IS NULL AND "creation_fingerprint" IS NULL) OR
            ("creation_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
             "creation_fingerprint" ~ '^[0-9a-f]{64}$')
        ),
    CONSTRAINT uk_plugin_idc_compliance_ctpl_no UNIQUE ("template_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctpl_provider_ref
        UNIQUE ("provider_code", "provider_template_ref")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctpl_sync
    ON plugin_idc_compliance_contract_template (
        "tenant_id", "provider_code", "provider_state_code", "provider_last_seen_sync_no"
    );
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_ctpl_creation
    ON plugin_idc_compliance_contract_template (
        "tenant_id", "creation_idempotency_digest"
    )
    WHERE "creation_idempotency_digest" IS NOT NULL;

COMMENT ON TABLE plugin_idc_compliance_contract_template IS
    'Installation-local logical contract template; Tencent-synced bodies remain provider-owned and read-only';
COMMENT ON COLUMN plugin_idc_compliance_contract_template."provider_template_ref" IS
    'Opaque Tencent template identifier; it is never accepted from browser create or update commands';

-- Purpose: Persists business records for plugin_idc_compliance_contract_template_version.
-- 用途：持久化 plugin_idc_compliance_contract_template_version 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_template_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "name" VARCHAR(200) NOT NULL,
    "scope_code" VARCHAR(16),
    "signing_order_code" VARCHAR(24),
    "language_code" VARCHAR(8),
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'draft',
    "provider_version" VARCHAR(32),
    "provider_snapshot_digest" CHAR(64),
    "provider_customer_signature_count" INT NOT NULL DEFAULT 0,
    "provider_customer_seal_count" INT NOT NULL DEFAULT 0,
    "provider_merchant_signature_count" INT NOT NULL DEFAULT 0,
    "provider_merchant_seal_count" INT NOT NULL DEFAULT 0,
    "current_content_revision_no" VARCHAR(64),
    "current_field_set_no" VARCHAR(64),
    "preview_confirmed" BOOLEAN NOT NULL DEFAULT FALSE,
    "used_count" BIGINT NOT NULL DEFAULT 0,
    "lock_version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplv_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_version CHECK ("version_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_name CHECK (char_length("name") BETWEEN 1 AND 200),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_scope
        CHECK (
            "scope_code" IS NULL OR
            "scope_code" IN ('personal', 'enterprise', 'all_customers')
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_order
        CHECK (
            "signing_order_code" IS NULL OR
            "signing_order_code" IN ('customer_first', 'merchant_first', 'parallel')
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_language
        CHECK ("language_code" IS NULL OR "language_code" IN ('zh-CN', 'en-US')),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_local_settings
        CHECK (
            ("scope_code" IS NULL AND "signing_order_code" IS NULL AND "language_code" IS NULL) OR
            ("scope_code" IS NOT NULL AND "signing_order_code" IS NOT NULL AND
             "language_code" IS NOT NULL)
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_status
        CHECK (
            "status_code" IN (
                'draft', 'needs_configuration', 'submission_unavailable',
                'submitted', 'enabled', 'disabled'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_provider_snapshot
        CHECK ("provider_snapshot_digest" IS NULL OR "provider_snapshot_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_provider_fields
        CHECK (
            "provider_customer_signature_count" >= 0 AND
            "provider_customer_seal_count" >= 0 AND
            "provider_merchant_signature_count" >= 0 AND
            "provider_merchant_seal_count" >= 0 AND
            (
                "provider_snapshot_digest" IS NOT NULL OR
                (
                    "provider_customer_signature_count" = 0 AND
                    "provider_customer_seal_count" = 0 AND
                    "provider_merchant_signature_count" = 0 AND
                    "provider_merchant_seal_count" = 0
                )
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_configuration
        CHECK (
            ("status_code" = 'needs_configuration' AND "scope_code" IS NULL AND
             "signing_order_code" IS NULL AND "language_code" IS NULL AND
             "provider_snapshot_digest" IS NOT NULL) OR
            "status_code" <> 'needs_configuration'
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_enable_settings
        CHECK (
            "status_code" NOT IN ('submission_unavailable', 'submitted', 'enabled', 'disabled') OR
            ("scope_code" IS NOT NULL AND "signing_order_code" IS NOT NULL AND
             "language_code" IS NOT NULL)
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_used CHECK ("used_count" >= 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_lock CHECK ("lock_version" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplv_content
        CHECK (
            (
                "status_code" IN ('submitted', 'enabled', 'disabled') AND
                "current_content_revision_no" IS NOT NULL AND
                "current_field_set_no" IS NOT NULL AND
                "preview_confirmed" = TRUE
            ) OR
            (
                "status_code" IN ('submitted', 'enabled', 'disabled') AND
                "provider_snapshot_digest" IS NOT NULL AND
                "current_content_revision_no" IS NULL AND
                "current_field_set_no" IS NULL AND
                "preview_confirmed" = FALSE
            ) OR
            "status_code" IN ('draft', 'needs_configuration', 'submission_unavailable')
        ),
    CONSTRAINT uk_plugin_idc_compliance_ctplv_no
        UNIQUE ("template_no", "version_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplv_template
        FOREIGN KEY ("template_no")
        REFERENCES plugin_idc_compliance_contract_template ("template_no")
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_ctplv_enabled
    ON plugin_idc_compliance_contract_template_version ("tenant_id", "template_no")
    WHERE "status_code" = 'enabled';
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_ctplv_provider_snapshot
    ON plugin_idc_compliance_contract_template_version (
        "tenant_id", "template_no", "provider_snapshot_digest"
    )
    WHERE "provider_snapshot_digest" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplv_list
    ON plugin_idc_compliance_contract_template_version (
        "tenant_id", "status_code", "scope_code", "updated_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplv_name
    ON plugin_idc_compliance_contract_template_version (
        "tenant_id", lower("name") text_pattern_ops, "updated_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_compliance_contract_template_version IS
    'Local immutable-after-submission template version and safe management-list projection';
COMMENT ON COLUMN plugin_idc_compliance_contract_template_version."current_content_revision_no" IS
    'Opaque pointer to the latest append-only encrypted source and PDF revision';

-- Purpose: Persists business records for plugin_idc_compliance_contract_template_content.
-- 用途：持久化 plugin_idc_compliance_contract_template_content 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_template_content (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "content_revision_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "revision_no" BIGINT NOT NULL,
    "command_idempotency_digest" CHAR(64) NOT NULL,
    "command_fingerprint" CHAR(64) NOT NULL,
    "source_mode_code" VARCHAR(16) NOT NULL,
    "encryption_key_no" VARCHAR(256) NOT NULL,
    "source_object_ref" VARCHAR(512) NOT NULL,
    "source_sha256" CHAR(64) NOT NULL,
    "source_media_type" VARCHAR(128) NOT NULL,
    "source_byte_size" BIGINT NOT NULL,
    "pdf_object_ref" VARCHAR(512),
    "pdf_sha256" CHAR(64),
    "pdf_byte_size" BIGINT,
    "pdf_page_count" INT,
    "conversion_status_code" VARCHAR(24) NOT NULL,
    "provider_file_ref" VARCHAR(256),
    "created_by_user_id" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplc_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_revision CHECK ("revision_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_command
        CHECK (
            "command_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
            "command_fingerprint" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_mode
        CHECK ("source_mode_code" IN ('uploaded_file', 'online_editor')),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_source_digest
        CHECK ("source_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_source_size CHECK ("source_byte_size" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_conversion
        CHECK ("conversion_status_code" IN ('source_saved', 'converting', 'ready', 'retryable_failed')),
    CONSTRAINT ck_plugin_idc_compliance_ctplc_pdf
        CHECK (
            (
                "conversion_status_code" = 'ready' AND
                "pdf_object_ref" IS NOT NULL AND
                "pdf_sha256" ~ '^[0-9a-f]{64}$' AND
                "pdf_byte_size" > 0 AND
                "pdf_page_count" > 0
            ) OR
            (
                "conversion_status_code" <> 'ready' AND
                "pdf_object_ref" IS NULL AND
                "pdf_sha256" IS NULL AND
                "pdf_byte_size" IS NULL AND
                "pdf_page_count" IS NULL
            )
        ),
    CONSTRAINT uk_plugin_idc_compliance_ctplc_no UNIQUE ("content_revision_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplc_command
        UNIQUE ("tenant_id", "command_idempotency_digest"),
    CONSTRAINT uk_plugin_idc_compliance_ctplc_version_revision
        UNIQUE ("template_no", "version_no", "revision_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplc_owner
        UNIQUE ("content_revision_no", "template_no", "version_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplc_version
        FOREIGN KEY ("template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_version ("template_no", "version_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplc_version
    ON plugin_idc_compliance_contract_template_content (
        "tenant_id", "template_no", "version_no", "revision_no" DESC
    );

COMMENT ON TABLE plugin_idc_compliance_contract_template_content IS
    'Append-only template source revision; body bytes are encrypted in private object storage and never stored in this table';

-- Purpose: Persists business records for plugin_idc_compliance_contract_template_field_set.
-- 用途：持久化 plugin_idc_compliance_contract_template_field_set 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_template_field_set (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "field_set_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "content_revision_no" VARCHAR(64) NOT NULL,
    "command_idempotency_digest" CHAR(64) NOT NULL,
    "command_fingerprint" CHAR(64) NOT NULL,
    "created_by_user_id" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplfs_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplfs_command
        CHECK (
            "command_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
            "command_fingerprint" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT uk_plugin_idc_compliance_ctplfs_no UNIQUE ("field_set_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplfs_command
        UNIQUE ("tenant_id", "command_idempotency_digest"),
    CONSTRAINT uk_plugin_idc_compliance_ctplfs_owner
        UNIQUE ("field_set_no", "template_no", "version_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplfs_content
        FOREIGN KEY ("content_revision_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_content (
            "content_revision_no", "template_no", "version_no"
        )
        ON DELETE RESTRICT
);

-- Purpose: Persists business records for plugin_idc_compliance_contract_template_field.
-- 用途：持久化 plugin_idc_compliance_contract_template_field 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_template_field (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "field_no" VARCHAR(64) NOT NULL,
    "field_set_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "sequence_no" INT NOT NULL,
    "field_type_code" VARCHAR(32) NOT NULL,
    "party_code" VARCHAR(16) NOT NULL,
    "variable_code" VARCHAR(64),
    "page_no" INT NOT NULL,
    "position_x_ppm" INT NOT NULL,
    "position_y_ppm" INT NOT NULL,
    "width_ppm" INT NOT NULL,
    "height_ppm" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ctplf_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_sequence CHECK ("sequence_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_type
        CHECK (
            "field_type_code" IN (
                'text_variable', 'signature', 'enterprise_seal', 'signing_date'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_party
        CHECK ("party_code" IN ('customer', 'merchant')),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_variable
        CHECK (
            ("field_type_code" = 'text_variable' AND "variable_code" IS NOT NULL) OR
            ("field_type_code" <> 'text_variable' AND "variable_code" IS NULL)
        ),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_page CHECK ("page_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ctplf_position
        CHECK (
            "position_x_ppm" BETWEEN 0 AND 1000000 AND
            "position_y_ppm" BETWEEN 0 AND 1000000 AND
            "width_ppm" BETWEEN 1 AND 1000000 AND
            "height_ppm" BETWEEN 1 AND 1000000 AND
            "position_x_ppm" + "width_ppm" <= 1000000 AND
            "position_y_ppm" + "height_ppm" <= 1000000
        ),
    CONSTRAINT uk_plugin_idc_compliance_ctplf_no UNIQUE ("field_no"),
    CONSTRAINT uk_plugin_idc_compliance_ctplf_slot
        UNIQUE ("field_set_no", "sequence_no"),
    CONSTRAINT fk_plugin_idc_compliance_ctplf_set
        FOREIGN KEY ("field_set_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_field_set (
            "field_set_no", "template_no", "version_no"
        )
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ctplf_set
    ON plugin_idc_compliance_contract_template_field (
        "tenant_id", "field_set_no", "sequence_no"
    );

COMMENT ON TABLE plugin_idc_compliance_contract_template_field_set IS
    'Append-only field placement revision bound to one immutable PDF content revision';
COMMENT ON TABLE plugin_idc_compliance_contract_template_field IS
    'Normalized contract variable and signing coordinates; one million units represent full page width or height';

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract_template_version
        ADD CONSTRAINT fk_plugin_idc_compliance_ctplv_content
        FOREIGN KEY ("current_content_revision_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_content (
            "content_revision_no", "template_no", "version_no"
        )
        DEFERRABLE INITIALLY DEFERRED;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract_template_version
        ADD CONSTRAINT fk_plugin_idc_compliance_ctplv_field_set
        FOREIGN KEY ("current_field_set_no", "template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_field_set (
            "field_set_no", "template_no", "version_no"
        )
        DEFERRABLE INITIALLY DEFERRED;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

-- Backfill safe disabled projections for contract versions created before the template catalog existed.
INSERT INTO plugin_idc_compliance_contract_template (
    "tenant_id", "template_no", "origin_code"
)
SELECT DISTINCT 0, contract."template_no", 'local'
FROM plugin_idc_compliance_contract contract
ON CONFLICT ("template_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_contract_template_version (
    "tenant_id", "template_no", "version_no", "name", "scope_code",
    "signing_order_code", "language_code", "status_code", "used_count"
)
SELECT
    0,
    contract."template_no",
    contract."template_version",
    contract."template_no",
    NULL,
    NULL,
    NULL,
    'draft',
    COUNT(*)
FROM plugin_idc_compliance_contract contract
GROUP BY contract."template_no", contract."template_version"
ON CONFLICT ("template_no", "version_no") DO NOTHING;

DO $$
BEGIN
    ALTER TABLE plugin_idc_compliance_contract
        ADD CONSTRAINT fk_plugin_idc_compliance_contract_template_version
        FOREIGN KEY ("template_no", "template_version")
        REFERENCES plugin_idc_compliance_contract_template_version (
            "template_no", "version_no"
        )
        ON DELETE RESTRICT;
EXCEPTION
    WHEN duplicate_object THEN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_template_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'contract template authoring history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplc_append_only
    ON plugin_idc_compliance_contract_template_content;
CREATE TRIGGER trg_plugin_idc_compliance_ctplc_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_contract_template_content
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_template_history_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplfs_append_only
    ON plugin_idc_compliance_contract_template_field_set;
CREATE TRIGGER trg_plugin_idc_compliance_ctplfs_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_contract_template_field_set
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_template_history_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplf_append_only
    ON plugin_idc_compliance_contract_template_field;
CREATE TRIGGER trg_plugin_idc_compliance_ctplf_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_contract_template_field
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_template_history_mutation();

CREATE OR REPLACE FUNCTION plugin_idc_compliance_guard_template_version()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD."status_code" IN ('submitted', 'enabled', 'disabled') THEN
        IF (
            to_jsonb(NEW) - ARRAY['status_code', 'used_count', 'lock_version', 'updated_at']
        ) IS DISTINCT FROM (
            to_jsonb(OLD) - ARRAY['status_code', 'used_count', 'lock_version', 'updated_at']
        ) THEN
            RAISE EXCEPTION 'submitted contract template version is immutable';
        END IF;
        IF NEW."status_code" NOT IN ('submitted', 'enabled', 'disabled') THEN
            RAISE EXCEPTION 'submitted contract template version cannot return to draft';
        END IF;
    END IF;
    IF NEW."lock_version" <= OLD."lock_version" THEN
        RAISE EXCEPTION 'contract template version lock must advance';
    END IF;
    IF NEW."used_count" < OLD."used_count" THEN
        RAISE EXCEPTION 'contract template use count cannot decrease';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_ctplv_guard
    ON plugin_idc_compliance_contract_template_version;
CREATE TRIGGER trg_plugin_idc_compliance_ctplv_guard
    BEFORE UPDATE ON plugin_idc_compliance_contract_template_version
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_guard_template_version();
