-- 075: Persist bounded, versioned structured configuration without reusing the
-- legacy server component_type_code/normalized_value model. Secret plaintext
-- never enters these tables; only owner-authorized secretcap references do.

-- Purpose: Append-only structured interface configuration revisions; public values exclude secret plaintext and opaque secret references.
-- 用途：保存商品自动化接口配置修订相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_configuration_revision (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "revision" BIGINT NOT NULL,
    "schema_version" VARCHAR(32) NOT NULL,
    "declaration_snapshot" JSONB NOT NULL,
    "public_value_snapshot" JSONB NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "value_digest" CHAR(64) NOT NULL,
    "secret_field_count" INT NOT NULL DEFAULT 0,
    "created_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_interface_configuration_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_interface_configuration_revision CHECK ("revision" > 0),
    CONSTRAINT ck_idc_interface_configuration_actor CHECK ("created_by" > 0),
    CONSTRAINT ck_idc_interface_configuration_schema CHECK (
        btrim("schema_version") <> '' AND length("schema_version") <= 32
        AND jsonb_typeof("declaration_snapshot") = 'object'
        AND octet_length("declaration_snapshot"::TEXT) <= 262144
    ),
    CONSTRAINT ck_idc_interface_configuration_values CHECK (
        jsonb_typeof("public_value_snapshot") = 'object'
        AND octet_length("public_value_snapshot"::TEXT) <= 262144
        AND "secret_field_count" BETWEEN 0 AND 128
    ),
    CONSTRAINT ck_idc_interface_configuration_digests CHECK (
        "declaration_digest" ~ '^[0-9a-f]{64}$'
        AND "value_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_interface_configuration_revision UNIQUE (
        "tenant_id", "interface_instance_no", "revision"
    ),
    CONSTRAINT fk_idc_interface_configuration_interface FOREIGN KEY (
        "interface_instance_no"
    ) REFERENCES plugin_idc_core_product_automation_interface_instance (
        "interface_instance_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_interface_configuration_latest
    ON plugin_idc_core_product_automation_interface_configuration_revision (
        "tenant_id", "interface_instance_no", "revision" DESC
    );

-- Purpose: Owner-private secretcap references for one immutable interface configuration revision; never returned through management or public APIs.
-- 用途：保存商品自动化接口秘密引用相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_secret_reference (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "configuration_revision_id" BIGINT NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "secret_ref" VARCHAR(512) NOT NULL,
    "configured" BOOLEAN NOT NULL DEFAULT TRUE,
    "masked_tail" VARCHAR(32) NOT NULL DEFAULT '',
    "secret_updated_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_interface_secret_reference_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_interface_secret_reference_field CHECK (
        "field_key" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
        AND length("field_key") <= 64
    ),
    CONSTRAINT ck_idc_interface_secret_reference_value CHECK (
        "configured" AND btrim("secret_ref") <> ''
        AND length("secret_ref") <= 512
        AND length("masked_tail") <= 32
    ),
    CONSTRAINT uk_idc_interface_secret_reference_field UNIQUE (
        "configuration_revision_id", "field_key"
    ),
    CONSTRAINT fk_idc_interface_secret_reference_revision FOREIGN KEY (
        "configuration_revision_id"
    ) REFERENCES plugin_idc_core_product_automation_interface_configuration_revision (
        "id"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_interface_secret_reference_revision
    ON plugin_idc_core_product_automation_interface_secret_reference (
        "tenant_id", "configuration_revision_id", "field_key"
    );

-- Owner-private convergence journal for secret create/recover/revoke. It
-- stores only stable operation identities and opaque owner routing refs;
-- plaintext, request values, masked tails, and user-visible audit data are
-- deliberately excluded.
-- Purpose: Stores Product Automation Interface Secret Operation records.
-- 用途：保存商品自动化接口秘密操作相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_secret_operation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_no" VARCHAR(40) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "expected_revision" BIGINT NOT NULL,
    "target_revision" BIGINT NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'prepared',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_owner" VARCHAR(64) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "last_error_digest" CHAR(64),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "changed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_interface_secret_operation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_interface_secret_operation_no CHECK ("operation_no" ~ '^pso_[0-9a-f]{32}$'),
    CONSTRAINT ck_idc_interface_secret_operation_revision CHECK (
        "expected_revision" >= 0 AND "target_revision" = "expected_revision" + 1
    ),
    CONSTRAINT ck_idc_interface_secret_operation_status CHECK (
        "status_code" IN ('prepared', 'secrets_ready', 'compensation_pending', 'commit_succeeded', 'compensated', 'manual_required')
    ),
    CONSTRAINT ck_idc_interface_secret_operation_attempt CHECK ("attempt_count" BETWEEN 0 AND 5),
    CONSTRAINT ck_idc_interface_secret_operation_lease CHECK (
        ("lease_owner" = '' AND "lease_expires_at" IS NULL)
        OR (btrim("lease_owner") <> '' AND length("lease_owner") <= 64 AND "lease_expires_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_interface_secret_operation_error CHECK (
        "last_error_digest" IS NULL OR "last_error_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_interface_secret_operation_no UNIQUE ("tenant_id", "operation_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_interface_secret_operation_active
    ON plugin_idc_core_product_automation_interface_secret_operation ("tenant_id", "interface_instance_no")
    WHERE "status_code" IN ('prepared', 'secrets_ready', 'compensation_pending', 'manual_required');

CREATE INDEX IF NOT EXISTS idx_idc_interface_secret_operation_recovery
    ON plugin_idc_core_product_automation_interface_secret_operation (
        "tenant_id", "status_code", "next_attempt_at", "lease_expires_at", "id"
    );

-- Purpose: Stores Product Automation Interface Secret Operation Field records.
-- 用途：保存商品自动化接口秘密操作字段相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_secret_operation_field (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_id" BIGINT NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "secret_ref" VARCHAR(512),
    "secret_updated_at" TIMESTAMPTZ,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'prepared',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "changed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_interface_secret_operation_field_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_interface_secret_operation_field_key CHECK (
        "field_key" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND length("field_key") <= 64
    ),
    CONSTRAINT ck_idc_interface_secret_operation_field_status CHECK (
        "status_code" IN ('prepared', 'created', 'revoked')
    ),
    CONSTRAINT ck_idc_interface_secret_operation_field_ref CHECK (
        ("status_code" = 'prepared' AND "secret_ref" IS NULL AND "secret_updated_at" IS NULL)
        OR ("status_code" IN ('created', 'revoked') AND btrim("secret_ref") <> '' AND length("secret_ref") <= 512 AND "secret_updated_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_interface_secret_operation_field UNIQUE ("operation_id", "field_key"),
    CONSTRAINT fk_idc_interface_secret_operation_field_operation FOREIGN KEY ("operation_id")
        REFERENCES plugin_idc_core_product_automation_interface_secret_operation ("id") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_interface_secret_operation_field_operation
    ON plugin_idc_core_product_automation_interface_secret_operation_field ("tenant_id", "operation_id", "field_key");

-- Purpose: Draft-editable and published-immutable frozen plugin declaration, merchant configuration, customer schema, and exact interface configuration revision.
-- 用途：保存目录商品版本配置相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_configuration (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "version_no" VARCHAR(40) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "interface_configuration_revision" BIGINT NOT NULL,
    "product_schema_version" VARCHAR(32) NOT NULL,
    "product_declaration_snapshot" JSONB NOT NULL,
    "merchant_value_snapshot" JSONB NOT NULL,
    "customer_schema_version" VARCHAR(32) NOT NULL,
    "customer_declaration_snapshot" JSONB NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "merchant_value_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "changed_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "changed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_version_configuration_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_version_configuration_revision CHECK (
        "interface_configuration_revision" > 0 AND "version" > 0
    ),
    CONSTRAINT ck_idc_product_version_configuration_actor CHECK ("changed_by" > 0),
    CONSTRAINT ck_idc_product_version_configuration_schema CHECK (
        btrim("product_schema_version") <> ''
        AND btrim("customer_schema_version") <> ''
        AND jsonb_typeof("product_declaration_snapshot") = 'object'
        AND jsonb_typeof("merchant_value_snapshot") = 'object'
        AND jsonb_typeof("customer_declaration_snapshot") = 'object'
        AND octet_length("product_declaration_snapshot"::TEXT) <= 262144
        AND octet_length("merchant_value_snapshot"::TEXT) <= 262144
        AND octet_length("customer_declaration_snapshot"::TEXT) <= 262144
    ),
    CONSTRAINT ck_idc_product_version_configuration_digests CHECK (
        "declaration_digest" ~ '^[0-9a-f]{64}$'
        AND "merchant_value_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_product_version_configuration_version UNIQUE (
        "tenant_id", "version_no"
    ),
    CONSTRAINT fk_idc_product_version_configuration_version FOREIGN KEY (
        "version_no"
    ) REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_version_configuration_binding FOREIGN KEY (
        "version_no"
    ) REFERENCES plugin_idc_core_catalog_product_version_fulfillment_binding (
        "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_product_version_configuration_interface_revision FOREIGN KEY (
        "tenant_id", "interface_instance_no", "interface_configuration_revision"
    ) REFERENCES plugin_idc_core_product_automation_interface_configuration_revision (
        "tenant_id", "interface_instance_no", "revision"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_product_version_configuration_interface
    ON plugin_idc_core_catalog_product_version_configuration (
        "tenant_id", "interface_instance_no", "interface_configuration_revision", "version_no"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_interface_configuration_revision()
RETURNS TRIGGER AS $$
DECLARE
    "expected_revision" BIGINT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        RAISE EXCEPTION 'interface configuration revisions are append-only';
    END IF;
    SELECT COALESCE(MAX("revision"), 0) + 1
      INTO "expected_revision"
      FROM plugin_idc_core_product_automation_interface_configuration_revision
     WHERE "tenant_id" = NEW."tenant_id"
       AND "interface_instance_no" = NEW."interface_instance_no";
    IF NEW."revision" <> "expected_revision" THEN
        RAISE EXCEPTION 'interface configuration revision is not contiguous';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_interface_configuration_revision
    ON plugin_idc_core_product_automation_interface_configuration_revision;
CREATE TRIGGER trg_idc_guard_interface_configuration_revision
BEFORE INSERT OR UPDATE OR DELETE
ON plugin_idc_core_product_automation_interface_configuration_revision
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_interface_configuration_revision();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_interface_secret_reference()
RETURNS TRIGGER AS $$
DECLARE
    expected_count INT;
    actual_count INT;
BEGIN
    IF TG_OP <> 'INSERT' THEN
        RAISE EXCEPTION 'interface secret references are append-only';
    END IF;
    SELECT "secret_field_count"
      INTO expected_count
      FROM plugin_idc_core_product_automation_interface_configuration_revision
     WHERE "id" = NEW."configuration_revision_id"
       AND "tenant_id" = NEW."tenant_id"
     FOR UPDATE;
    IF expected_count IS NULL THEN
        RAISE EXCEPTION 'interface configuration revision is unavailable';
    END IF;
    SELECT count(*)
      INTO actual_count
      FROM plugin_idc_core_product_automation_interface_secret_reference
     WHERE "tenant_id" = NEW."tenant_id"
       AND "configuration_revision_id" = NEW."configuration_revision_id";
    IF actual_count >= expected_count OR actual_count >= 128 THEN
        RAISE EXCEPTION 'interface secret reference capacity is exhausted';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_interface_secret_reference
    ON plugin_idc_core_product_automation_interface_secret_reference;
CREATE TRIGGER trg_idc_guard_interface_secret_reference
BEFORE INSERT OR UPDATE OR DELETE
ON plugin_idc_core_product_automation_interface_secret_reference
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_interface_secret_reference();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_product_version_configuration()
RETURNS TRIGGER AS $$
DECLARE
    target_version_status VARCHAR(16);
    binding_interface_no VARCHAR(40);
BEGIN
    SELECT version_row."status_code", binding."interface_instance_no"
      INTO target_version_status, binding_interface_no
      FROM plugin_idc_core_catalog_product_version AS version_row
      JOIN plugin_idc_core_catalog_product_version_fulfillment_binding AS binding
        ON binding."tenant_id" = version_row."tenant_id"
       AND binding."version_no" = version_row."version_no"
     WHERE version_row."tenant_id" = COALESCE(NEW."tenant_id", OLD."tenant_id")
       AND version_row."version_no" = COALESCE(NEW."version_no", OLD."version_no")
     FOR UPDATE OF version_row, binding;
    IF target_version_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'published product version configuration is immutable';
    END IF;
    IF TG_OP <> 'DELETE' AND binding_interface_no IS DISTINCT FROM NEW."interface_instance_no" THEN
        RAISE EXCEPTION 'product version configuration does not match its fulfillment binding';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        NEW."changed_at" := CURRENT_TIMESTAMP;
        IF NEW."id" IS DISTINCT FROM OLD."id"
           OR NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
           OR NEW."version_no" IS DISTINCT FROM OLD."version_no"
           OR NEW."version" <> OLD."version" + 1 THEN
            RAISE EXCEPTION 'product version configuration revision is invalid';
        END IF;
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_product_version_configuration
    ON plugin_idc_core_catalog_product_version_configuration;
CREATE TRIGGER trg_idc_guard_product_version_configuration
BEFORE INSERT OR UPDATE OR DELETE
ON plugin_idc_core_catalog_product_version_configuration
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_product_version_configuration();

COMMENT ON TABLE plugin_idc_core_product_automation_interface_configuration_revision IS
    'Append-only structured interface configuration revisions; public values exclude secret plaintext and opaque secret references';
COMMENT ON TABLE plugin_idc_core_product_automation_interface_secret_reference IS
    'Owner-private secretcap references for one immutable interface configuration revision; never returned through management or public APIs';
COMMENT ON TABLE plugin_idc_core_catalog_product_version_configuration IS
    'Draft-editable and published-immutable frozen plugin declaration, merchant configuration, customer schema, and exact interface configuration revision';

-- Governed purchase-configuration file uploads belong to this same structured
-- configuration iteration. Keep them in this single migration file.
-- Purpose: Stores Purchase Configuration Upload records.
-- 用途：保存购买配置上传相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_purchase_configuration_upload (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "uploader_user_id" BIGINT NOT NULL,
    "product_version_no" VARCHAR(40) NOT NULL,
    "configuration_revision" VARCHAR(128) NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "policy_id" VARCHAR(128) NOT NULL,
    "policy_snapshot" JSONB NOT NULL,
    "file_name" VARCHAR(255) NOT NULL,
    "declared_size_bytes" BIGINT NOT NULL,
    "actual_size_bytes" BIGINT NOT NULL DEFAULT 0,
    "media_type" VARCHAR(128) NOT NULL DEFAULT '',
    "sha256_hex" VARCHAR(64) NOT NULL DEFAULT '',
    "object_ref" VARCHAR(512) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'reserved',
    "scan_status" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "bound_owner_type" VARCHAR(32) NOT NULL DEFAULT '',
    "bound_owner_no" VARCHAR(64) NOT NULL DEFAULT '',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ NULL,
    "consumed_at" TIMESTAMPTZ NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_core_purchase_configuration_upload_no UNIQUE ("tenant_id", "upload_no"),
    CONSTRAINT ck_plugin_idc_core_purchase_configuration_upload_size CHECK ("declared_size_bytes" > 0 AND "actual_size_bytes" >= 0),
    CONSTRAINT ck_plugin_idc_core_purchase_configuration_upload_status CHECK ("status_code" IN ('reserved', 'uploading', 'ready', 'consumed', 'revoked', 'rejected', 'expired')),
    CONSTRAINT ck_plugin_idc_core_purchase_configuration_upload_scan CHECK ("scan_status" IN ('pending', 'not_required', 'clean', 'risk', 'failed')),
    CONSTRAINT ck_plugin_idc_core_purchase_configuration_upload_binding CHECK (
        ("status_code" = 'consumed' AND "bound_owner_type" = 'order' AND "bound_owner_no" <> '' AND "consumed_at" IS NOT NULL)
        OR ("status_code" <> 'consumed' AND "bound_owner_type" = '' AND "bound_owner_no" = '' AND "consumed_at" IS NULL)
    )
);

ALTER TABLE plugin_idc_core_purchase_configuration_upload
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_purchase_configuration_upload_status;
ALTER TABLE plugin_idc_core_purchase_configuration_upload
    ADD CONSTRAINT ck_plugin_idc_core_purchase_configuration_upload_status
    CHECK ("status_code" IN ('reserved', 'uploading', 'ready', 'consumed', 'revoked', 'rejected', 'expired'));

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_purchase_configuration_upload_owner
    ON plugin_idc_core_purchase_configuration_upload ("tenant_id", "customer_no", "uploader_user_id", "status_code", "expires_at");

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_purchase_configuration_upload_target
    ON plugin_idc_core_purchase_configuration_upload ("tenant_id", "product_version_no", "configuration_revision", "field_key", "status_code");

-- Generic transaction evidence is a sidecar to the legacy server transaction
-- graph during FB-52. It contains only compiler-produced typed documents and
-- must not acquire CPU, resource-pool, or server-type columns.
-- Purpose: Immutable generic quote-item configuration evidence produced only by the governed thirteen-type compiler and bound once by checkout.
-- 用途：保存交易结构化配置快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_structured_configuration_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "uploader_user_id" BIGINT NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "quote_item_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "configuration_revision" VARCHAR(128) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "interface_configuration_revision" BIGINT NOT NULL,
    "customer_declaration_digest" CHAR(64) NOT NULL,
    "candidate_snapshot_digest" CHAR(64) NOT NULL,
    "customer_value_digest" CHAR(64) NOT NULL,
    "candidate_snapshot" JSONB NOT NULL,
    "typed_value_snapshot" JSONB NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "order_no" VARCHAR(40),
    "order_item_no" VARCHAR(40),
    "bound_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_com_structured_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT uk_idc_com_structured_snapshot_quote_item UNIQUE ("quote_item_no"),
    CONSTRAINT uk_idc_com_structured_snapshot_order_item UNIQUE ("order_item_no"),
    CONSTRAINT ck_idc_com_structured_snapshot_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_structured_snapshot_actor CHECK ("uploader_user_id" > 0),
    CONSTRAINT ck_idc_com_structured_snapshot_revision CHECK ("interface_configuration_revision" > 0),
    CONSTRAINT ck_idc_com_structured_snapshot_digests CHECK (
        "customer_declaration_digest" ~ '^[0-9a-f]{64}$'
        AND "candidate_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "customer_value_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_structured_snapshot_documents CHECK (
        jsonb_typeof("candidate_snapshot") = 'object'
        AND jsonb_typeof("typed_value_snapshot") = 'object'
        AND octet_length("candidate_snapshot"::TEXT) <= 1048576
        AND octet_length("typed_value_snapshot"::TEXT) <= 1048576
    ),
    CONSTRAINT ck_idc_com_structured_snapshot_binding CHECK (
        ("order_no" IS NULL AND "order_item_no" IS NULL AND "bound_at" IS NULL)
        OR ("order_no" IS NOT NULL AND "order_item_no" IS NOT NULL AND "bound_at" IS NOT NULL)
    ),
    CONSTRAINT fk_idc_com_structured_snapshot_quote_item FOREIGN KEY ("quote_item_no")
        REFERENCES plugin_idc_core_commerce_quote_item_snapshot ("quote_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_structured_snapshot_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_structured_snapshot_order_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item ("order_item_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_structured_snapshot_order
    ON plugin_idc_core_commerce_structured_configuration_snapshot ("tenant_id", "order_no", "order_item_no");

-- Purpose: Bounded file-token and upload-policy evidence for one generic configuration snapshot.
-- 用途：保存交易结构化配置文件快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_structured_configuration_file_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "field_sequence" INT NOT NULL,
    "file_sequence" INT NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "upload_no" VARCHAR(40) NOT NULL,
    "upload_policy_snapshot" JSONB NOT NULL,
    CONSTRAINT uk_idc_com_structured_file_position UNIQUE ("snapshot_no", "field_sequence", "file_sequence"),
    CONSTRAINT uk_idc_com_structured_file_upload UNIQUE ("upload_no"),
    CONSTRAINT ck_idc_com_structured_file_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_structured_file_sequence CHECK (
        "field_sequence" BETWEEN 1 AND 128 AND "file_sequence" BETWEEN 1 AND 100
    ),
    CONSTRAINT ck_idc_com_structured_file_policy CHECK (
        jsonb_typeof("upload_policy_snapshot") = 'object'
        AND octet_length("upload_policy_snapshot"::TEXT) <= 65536
    ),
    CONSTRAINT fk_idc_com_structured_file_snapshot FOREIGN KEY ("snapshot_no")
        REFERENCES plugin_idc_core_commerce_structured_configuration_snapshot ("snapshot_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_structured_file_upload FOREIGN KEY ("tenant_id", "upload_no")
        REFERENCES plugin_idc_core_purchase_configuration_upload ("tenant_id", "upload_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_structured_file_snapshot
    ON plugin_idc_core_commerce_structured_configuration_file_snapshot ("tenant_id", "snapshot_no", "field_sequence", "file_sequence");

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_structured_configuration_file_snapshot()
RETURNS TRIGGER AS $$
DECLARE
    parent_order_no VARCHAR(40);
BEGIN
    IF TG_OP = 'UPDATE' OR TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'structured transaction file snapshots are immutable evidence';
    END IF;
    SELECT "order_no" INTO parent_order_no
    FROM plugin_idc_core_commerce_structured_configuration_snapshot
    WHERE "tenant_id"=NEW."tenant_id" AND "snapshot_no"=NEW."snapshot_no"
    FOR UPDATE;
    IF NOT FOUND OR parent_order_no IS NOT NULL THEN
        RAISE EXCEPTION 'structured transaction file snapshot parent is already bound or unavailable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_structured_configuration_file_snapshot
    ON plugin_idc_core_commerce_structured_configuration_file_snapshot;
CREATE TRIGGER trg_idc_guard_structured_configuration_file_snapshot
BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_commerce_structured_configuration_file_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_structured_configuration_file_snapshot();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_consumed_purchase_configuration_upload()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."status_code" = 'consumed' THEN
        IF TG_OP = 'DELETE' OR NEW IS DISTINCT FROM OLD THEN
            RAISE EXCEPTION 'consumed purchase configuration upload is immutable order evidence';
        END IF;
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_consumed_purchase_configuration_upload
    ON plugin_idc_core_purchase_configuration_upload;
CREATE TRIGGER trg_idc_guard_consumed_purchase_configuration_upload
BEFORE UPDATE OR DELETE ON plugin_idc_core_purchase_configuration_upload
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_consumed_purchase_configuration_upload();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_structured_configuration_snapshot()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'structured transaction snapshots are immutable evidence';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF OLD."order_no" IS NOT NULL
           OR NEW."order_no" IS NULL OR NEW."order_item_no" IS NULL OR NEW."bound_at" IS NULL
           OR NEW."id" IS DISTINCT FROM OLD."id"
           OR NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
           OR NEW."snapshot_no" IS DISTINCT FROM OLD."snapshot_no"
           OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
           OR NEW."uploader_user_id" IS DISTINCT FROM OLD."uploader_user_id"
           OR NEW."quote_no" IS DISTINCT FROM OLD."quote_no"
           OR NEW."quote_item_no" IS DISTINCT FROM OLD."quote_item_no"
           OR NEW."product_no" IS DISTINCT FROM OLD."product_no"
           OR NEW."version_no" IS DISTINCT FROM OLD."version_no"
           OR NEW."configuration_revision" IS DISTINCT FROM OLD."configuration_revision"
           OR NEW."interface_instance_no" IS DISTINCT FROM OLD."interface_instance_no"
           OR NEW."interface_configuration_revision" IS DISTINCT FROM OLD."interface_configuration_revision"
           OR NEW."customer_declaration_digest" IS DISTINCT FROM OLD."customer_declaration_digest"
           OR NEW."candidate_snapshot_digest" IS DISTINCT FROM OLD."candidate_snapshot_digest"
           OR NEW."customer_value_digest" IS DISTINCT FROM OLD."customer_value_digest"
           OR NEW."candidate_snapshot" IS DISTINCT FROM OLD."candidate_snapshot"
           OR NEW."typed_value_snapshot" IS DISTINCT FROM OLD."typed_value_snapshot"
           OR NEW."content_digest" IS DISTINCT FROM OLD."content_digest"
           OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
            RAISE EXCEPTION 'structured transaction snapshot mutation is forbidden';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_structured_configuration_snapshot
    ON plugin_idc_core_commerce_structured_configuration_snapshot;
CREATE TRIGGER trg_idc_guard_structured_configuration_snapshot
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_structured_configuration_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_structured_configuration_snapshot();

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_structured_file_order_evidence()
RETURNS TRIGGER AS $$
DECLARE
    affected_upload_no VARCHAR(40);
    affected_snapshot_no VARCHAR(40);
BEGIN
    IF TG_TABLE_NAME = 'plugin_idc_core_purchase_configuration_upload' THEN
        affected_upload_no := COALESCE(NEW."upload_no", OLD."upload_no");
    ELSE
        affected_snapshot_no := COALESCE(NEW."snapshot_no", OLD."snapshot_no");
    END IF;

    IF EXISTS (
        SELECT 1
          FROM plugin_idc_core_purchase_configuration_upload upload
          LEFT JOIN plugin_idc_core_commerce_structured_configuration_file_snapshot file
            ON file."tenant_id"=upload."tenant_id" AND file."upload_no"=upload."upload_no"
          LEFT JOIN plugin_idc_core_commerce_structured_configuration_snapshot parent
            ON parent."tenant_id"=file."tenant_id" AND parent."snapshot_no"=file."snapshot_no"
         WHERE (affected_upload_no IS NOT NULL AND upload."upload_no"=affected_upload_no)
            OR (affected_snapshot_no IS NOT NULL AND parent."snapshot_no"=affected_snapshot_no)
         GROUP BY upload."id", upload."status_code", upload."bound_owner_no", upload."customer_no",
                  upload."uploader_user_id", upload."product_version_no", upload."configuration_revision",
                  upload."field_key", parent."snapshot_no", parent."order_no", parent."customer_no",
                  parent."uploader_user_id", parent."version_no", parent."configuration_revision", file."field_key"
        HAVING (upload."status_code"='consumed' AND (
                    count(file."id") <> 1
                    OR parent."order_no" IS NULL
                    OR parent."order_no" IS DISTINCT FROM upload."bound_owner_no"
                    OR parent."customer_no" IS DISTINCT FROM upload."customer_no"
                    OR parent."uploader_user_id" IS DISTINCT FROM upload."uploader_user_id"
                    OR parent."version_no" IS DISTINCT FROM upload."product_version_no"
                    OR parent."configuration_revision" IS DISTINCT FROM upload."configuration_revision"
                    OR file."field_key" IS DISTINCT FROM upload."field_key"
                ))
            OR (parent."order_no" IS NOT NULL AND (
                    upload."status_code" IS DISTINCT FROM 'consumed'
                    OR upload."bound_owner_no" IS DISTINCT FROM parent."order_no"
                    OR upload."customer_no" IS DISTINCT FROM parent."customer_no"
                    OR upload."uploader_user_id" IS DISTINCT FROM parent."uploader_user_id"
                ))
    ) THEN
        RAISE EXCEPTION 'structured configuration file order evidence is incomplete or mismatched';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_purchase_configuration_upload'::regclass
           AND conname = 'trg_idc_validate_upload_file_order_evidence' AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_purchase_configuration_upload'::regclass
               AND constraint_row.conname = 'trg_idc_validate_upload_file_order_evidence'
               AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_validate_upload_file_order_evidence'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_structured_file_order_evidence()'::regprocedure
        ) THEN RAISE EXCEPTION 'constraint trigger trg_idc_validate_upload_file_order_evidence catalog contract is invalid' USING ERRCODE = '23514'; END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_validate_upload_file_order_evidence
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_purchase_configuration_upload
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_structured_file_order_evidence();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_commerce_structured_configuration_snapshot'::regclass
           AND conname = 'trg_idc_validate_snapshot_file_order_evidence' AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_structured_configuration_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_validate_snapshot_file_order_evidence'
               AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_validate_snapshot_file_order_evidence'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_structured_file_order_evidence()'::regprocedure
        ) THEN RAISE EXCEPTION 'constraint trigger trg_idc_validate_snapshot_file_order_evidence catalog contract is invalid' USING ERRCODE = '23514'; END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_validate_snapshot_file_order_evidence
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_commerce_structured_configuration_snapshot
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_structured_file_order_evidence();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_commerce_structured_configuration_file_snapshot'::regclass
           AND conname = 'trg_idc_validate_file_snapshot_order_evidence' AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_commerce_structured_configuration_file_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_validate_file_snapshot_order_evidence'
               AND constraint_row.contype = 't' AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal AND trigger_row.tgname = 'trg_idc_validate_file_snapshot_order_evidence'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_structured_file_order_evidence()'::regprocedure
        ) THEN RAISE EXCEPTION 'constraint trigger trg_idc_validate_file_snapshot_order_evidence catalog contract is invalid' USING ERRCODE = '23514'; END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_validate_file_snapshot_order_evidence
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_commerce_structured_configuration_file_snapshot
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_structured_file_order_evidence();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_commerce_structured_configuration_snapshot IS
    'Immutable generic quote-item configuration evidence produced only by the governed thirteen-type compiler and bound once by checkout';
COMMENT ON TABLE plugin_idc_core_commerce_structured_configuration_file_snapshot IS
    'Bounded file-token and upload-policy evidence for one generic configuration snapshot';
