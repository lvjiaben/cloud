-- Governed private files used by staff-managed interface and product configuration.
-- Purpose: Stores Management Configuration Upload records.
-- 用途：保存管理配置上传相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_management_configuration_upload (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "surface_code" VARCHAR(16) NOT NULL,
    "target_no" VARCHAR(40) NOT NULL,
    "target_revision" BIGINT NOT NULL,
    "uploader_user_id" BIGINT NOT NULL,
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
    "expires_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ NULL,
    "bound_revision" BIGINT NULL,
    "bound_at" TIMESTAMPTZ NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_management_configuration_upload UNIQUE ("tenant_id", "upload_no"),
    CONSTRAINT ck_idc_management_configuration_upload_surface CHECK ("surface_code" IN ('interface','product')),
    CONSTRAINT ck_idc_management_configuration_upload_status CHECK ("status_code" IN ('reserved','uploading','ready','bound','revoked','rejected')),
    CONSTRAINT ck_idc_management_configuration_upload_scan CHECK ("scan_status" IN ('pending','not_required','clean','risk','failed')),
    CONSTRAINT ck_idc_management_configuration_upload_size CHECK ("declared_size_bytes" > 0 AND "actual_size_bytes" >= 0),
    CONSTRAINT ck_idc_management_configuration_upload_binding CHECK (("status_code"='bound' AND "bound_revision" IS NOT NULL AND "bound_at" IS NOT NULL) OR ("status_code"<>'bound' AND "bound_revision" IS NULL AND "bound_at" IS NULL))
);

CREATE INDEX IF NOT EXISTS idx_idc_management_configuration_upload_owner
    ON plugin_idc_core_management_configuration_upload ("tenant_id", "uploader_user_id", "surface_code", "target_no", "status_code", "expires_at");
CREATE INDEX IF NOT EXISTS idx_idc_management_configuration_upload_target
    ON plugin_idc_core_management_configuration_upload ("tenant_id", "surface_code", "target_no", "field_key", "status_code");

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_bound_management_configuration_upload()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."status_code"='bound' AND (TG_OP='DELETE' OR NEW IS DISTINCT FROM OLD) THEN
        RAISE EXCEPTION 'bound management configuration upload is immutable evidence';
    END IF;
    RETURN CASE WHEN TG_OP='DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_bound_management_configuration_upload ON plugin_idc_core_management_configuration_upload;
CREATE TRIGGER trg_idc_guard_bound_management_configuration_upload
BEFORE UPDATE OR DELETE ON plugin_idc_core_management_configuration_upload
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_bound_management_configuration_upload();

-- One immutable reference per configuration revision lets a later revision
-- inherit an already-bound file without mutating the original upload evidence.
-- Purpose: Stores Management Configuration File Reference records.
-- 用途：保存管理配置文件引用相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_management_configuration_file_reference (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "surface_code" VARCHAR(16) NOT NULL,
    "target_no" VARCHAR(40) NOT NULL,
    "configuration_revision" BIGINT NOT NULL,
    "upload_no" VARCHAR(40) NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "policy_id" VARCHAR(128) NOT NULL,
    "created_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_management_configuration_file_reference UNIQUE ("tenant_id", "surface_code", "target_no", "configuration_revision", "upload_no"),
    CONSTRAINT fk_idc_management_configuration_file_reference_upload FOREIGN KEY ("tenant_id", "upload_no") REFERENCES plugin_idc_core_management_configuration_upload ("tenant_id", "upload_no"),
    CONSTRAINT ck_idc_management_configuration_file_reference_surface CHECK ("surface_code" IN ('interface','product')),
    CONSTRAINT ck_idc_management_configuration_file_reference_revision CHECK ("configuration_revision" > 0)
);

CREATE INDEX IF NOT EXISTS idx_idc_management_configuration_file_reference_target
    ON plugin_idc_core_management_configuration_file_reference ("tenant_id", "surface_code", "target_no", "configuration_revision");

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_management_configuration_file_reference_insert()
RETURNS TRIGGER AS $$
DECLARE
    upload plugin_idc_core_management_configuration_upload%ROWTYPE;
BEGIN
    -- The IDC owner currently uses the single-merchant tenant 0. Tenant
    -- ownership is nevertheless compared explicitly so a forged reference can
    -- never attach evidence owned by a different tenant if that boundary grows.
    SELECT candidate.*
      INTO upload
      FROM plugin_idc_core_management_configuration_upload AS candidate
     WHERE candidate."tenant_id" = NEW."tenant_id"
       AND candidate."upload_no" = NEW."upload_no"
     FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'management configuration upload evidence is not accessible';
    END IF;
    IF NEW."created_by" <= 0 THEN
        RAISE EXCEPTION 'management configuration file reference requires a direct staff actor';
    END IF;
    IF upload."status_code" IS DISTINCT FROM 'bound' THEN
        RAISE EXCEPTION 'management configuration upload is not bound evidence';
    END IF;
    IF upload."surface_code" IS DISTINCT FROM NEW."surface_code"
       OR upload."target_no" IS DISTINCT FROM NEW."target_no"
       OR upload."field_key" IS DISTINCT FROM NEW."field_key"
       OR upload."policy_id" IS DISTINCT FROM NEW."policy_id" THEN
        RAISE EXCEPTION 'management configuration file reference does not match upload evidence';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_validate_management_configuration_file_reference_insert ON plugin_idc_core_management_configuration_file_reference;
CREATE TRIGGER trg_idc_validate_management_configuration_file_reference_insert
BEFORE INSERT ON plugin_idc_core_management_configuration_file_reference
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_management_configuration_file_reference_insert();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_management_configuration_file_reference()
RETURNS TRIGGER AS $$ BEGIN RAISE EXCEPTION 'management configuration file reference is immutable evidence'; END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_idc_guard_management_configuration_file_reference ON plugin_idc_core_management_configuration_file_reference;
CREATE TRIGGER trg_idc_guard_management_configuration_file_reference
BEFORE UPDATE OR DELETE ON plugin_idc_core_management_configuration_file_reference
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_management_configuration_file_reference();
