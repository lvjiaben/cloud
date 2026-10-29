-- Enterprise verification submission and encrypted-material metadata. Raw
-- legal values and document bytes remain in encrypted plugin storage only.

-- Purpose: Persists business records for plugin_idc_compliance_enterprise_encryption_key.
-- 用途：持久化 plugin_idc_compliance_enterprise_encryption_key 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_enterprise_encryption_key (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "key_no" VARCHAR(64) NOT NULL,
    "secret_ref" VARCHAR(256) NOT NULL,
    "generation" BIGINT NOT NULL DEFAULT 1,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ent_key_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ent_key_no CHECK (
        "key_no" ~ '^enterprise_material_key_v[0-9]+$'
    ),
    CONSTRAINT ck_plugin_idc_compliance_ent_key_ref CHECK (
        length(btrim("secret_ref")) BETWEEN 1 AND 256
    ),
    CONSTRAINT ck_plugin_idc_compliance_ent_key_generation CHECK ("generation" > 0),
    CONSTRAINT ck_plugin_idc_compliance_ent_key_status CHECK (
        "status_code" IN ('active', 'retired')
    ),
    CONSTRAINT ck_plugin_idc_compliance_ent_key_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_ent_key_no UNIQUE ("key_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_ent_key_active
    ON plugin_idc_compliance_enterprise_encryption_key ("tenant_id")
    WHERE "status_code" = 'active';

COMMENT ON TABLE plugin_idc_compliance_enterprise_encryption_key IS
    'Opaque secretcap routing reference for enterprise material encryption; plaintext keys and ciphertext are forbidden';
COMMENT ON COLUMN plugin_idc_compliance_enterprise_encryption_key."secret_ref" IS
    'Sensitive secretcap handle reference; never expose through API, logs, audit, tasks or errors';

-- Purpose: Persists business records for plugin_idc_compliance_enterprise_material_upload.
-- 用途：持久化 plugin_idc_compliance_enterprise_material_upload 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_enterprise_material_upload (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "material_no" VARCHAR(64) NOT NULL,
    "case_no" VARCHAR(64) NOT NULL,
    "case_subject_type_code" VARCHAR(16) NOT NULL DEFAULT 'enterprise',
    "material_type_code" VARCHAR(40) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'reserved',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_subject
        CHECK ("case_subject_type_code" = 'enterprise'),
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_type CHECK (
        "material_type_code" IN (
            'business_license', 'legal_representative_id', 'agent_id',
            'authorization_letter', 'organization_certificate', 'other'
        )
    ),
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_digest CHECK (
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_status
        CHECK ("status_code" IN ('reserved', 'completed')),
    CONSTRAINT ck_plugin_idc_compliance_ent_upload_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_ent_upload_material UNIQUE ("material_no"),
    CONSTRAINT uk_plugin_idc_compliance_ent_upload_case_material
        UNIQUE ("material_no", "case_no"),
    CONSTRAINT uk_plugin_idc_compliance_ent_upload_request
        UNIQUE ("tenant_id", "case_no", "idempotency_key_digest"),
    CONSTRAINT fk_plugin_idc_compliance_ent_upload_case
        FOREIGN KEY ("case_no", "case_subject_type_code")
        REFERENCES plugin_idc_compliance_verification_case ("case_no", "subject_type_code")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ent_upload_case
    ON plugin_idc_compliance_enterprise_material_upload (
        "tenant_id", "case_no", "status_code", "created_at", "id"
    );

COMMENT ON TABLE plugin_idc_compliance_enterprise_material_upload IS
    'Non-sensitive upload reservation; original idempotency keys and document bytes are forbidden';

ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "legal_change_request_no" VARCHAR(40) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "request_idempotency_key_digest" CHAR(64);
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "request_fingerprint" CHAR(64);
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "submission_idempotency_key_digest" CHAR(64);
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "submission_fingerprint" CHAR(64);
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "encryption_key_no" VARCHAR(64);
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "snapshot_stored_byte_size" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "applicant_role_code" VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_compliance_verification_case
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_case_enterprise_submission;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD CONSTRAINT ck_plugin_idc_compliance_case_enterprise_submission CHECK (
        (
            "subject_type_code" = 'personal' AND
            "legal_change_request_no" = '' AND
            "request_idempotency_key_digest" IS NULL AND
            "request_fingerprint" IS NULL AND
            "submission_idempotency_key_digest" IS NULL AND
            "submission_fingerprint" IS NULL AND
            "encryption_key_no" IS NULL AND
            "snapshot_stored_byte_size" = 0 AND
            "applicant_role_code" = ''
        ) OR
        (
            "subject_type_code" = 'enterprise' AND
            ("legal_change_request_no" = '' OR
                "legal_change_request_no" ~ '^LCR_[0-9a-f]{32}$') AND
            "request_idempotency_key_digest" IS NOT NULL AND
            "request_idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
            "request_fingerprint" IS NOT NULL AND
            "request_fingerprint" ~ '^[0-9a-f]{64}$' AND
            (
                ("submission_idempotency_key_digest" IS NULL AND
                    "submission_fingerprint" IS NULL) OR
                ("submission_idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
                    "submission_fingerprint" ~ '^[0-9a-f]{64}$')
            ) AND
            "encryption_key_no" IS NOT NULL AND
            "encryption_key_no" ~ '^enterprise_material_key_v[0-9]+$' AND
            "snapshot_stored_byte_size" >= 0 AND
            "applicant_role_code" IN ('', 'legal_representative', 'authorized_agent')
        )
    );

ALTER TABLE plugin_idc_compliance_verification_case
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_verification_snapshot_size;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD CONSTRAINT ck_plugin_idc_compliance_verification_snapshot_size CHECK (
        ("subject_type_code" = 'personal' AND "snapshot_stored_byte_size" = 0) OR
        ("subject_type_code" = 'enterprise' AND (
            ("legal_profile_snapshot_ref" IS NULL AND "snapshot_stored_byte_size" = 0) OR
            ("legal_profile_snapshot_ref" IS NOT NULL AND "snapshot_stored_byte_size" > 0)
        ))
    );

ALTER TABLE plugin_idc_compliance_verification_case
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_compliance_verification_ent_key;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD CONSTRAINT fk_plugin_idc_compliance_verification_ent_key
        FOREIGN KEY ("encryption_key_no")
        REFERENCES plugin_idc_compliance_enterprise_encryption_key ("key_no")
        ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_verification_ent_request
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "customer_no", "subject_type_code", "request_idempotency_key_digest"
    ) WHERE "subject_type_code" = 'enterprise';
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_verification_ent_legal_change
    ON plugin_idc_compliance_verification_case ("tenant_id", "legal_change_request_no")
    WHERE "subject_type_code" = 'enterprise' AND "legal_change_request_no" <> '';

ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD COLUMN IF NOT EXISTS "encryption_key_no" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD COLUMN IF NOT EXISTS "stored_byte_size" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD COLUMN IF NOT EXISTS "idempotency_key_digest" CHAR(64);
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD COLUMN IF NOT EXISTS "request_fingerprint" CHAR(64);

ALTER TABLE plugin_idc_compliance_enterprise_material
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_enterprise_material_envelope;
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD CONSTRAINT ck_plugin_idc_compliance_enterprise_material_envelope CHECK (
        "encryption_key_no" ~ '^enterprise_material_key_v[0-9]+$' AND
        "stored_byte_size" > "byte_size" AND
        "idempotency_key_digest" IS NOT NULL AND
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" IS NOT NULL AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    );

ALTER TABLE plugin_idc_compliance_enterprise_material
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_compliance_enterprise_material_ent_key;
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD CONSTRAINT fk_plugin_idc_compliance_enterprise_material_ent_key
        FOREIGN KEY ("encryption_key_no")
        REFERENCES plugin_idc_compliance_enterprise_encryption_key ("key_no")
        ON DELETE RESTRICT;

ALTER TABLE plugin_idc_compliance_enterprise_material
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_compliance_ent_material_upload;
ALTER TABLE plugin_idc_compliance_enterprise_material
    ADD CONSTRAINT fk_plugin_idc_compliance_ent_material_upload
        FOREIGN KEY ("material_no", "case_no")
        REFERENCES plugin_idc_compliance_enterprise_material_upload ("material_no", "case_no")
        ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_enterprise_material_request
    ON plugin_idc_compliance_enterprise_material (
        "tenant_id", "case_no", "idempotency_key_digest"
    );

COMMENT ON COLUMN plugin_idc_compliance_enterprise_material."content_sha256" IS
    'SHA-256 of the encrypted storage envelope, never a digest derived directly from document plaintext';
COMMENT ON COLUMN plugin_idc_compliance_enterprise_material."stored_byte_size" IS
    'Encrypted envelope byte size confirmed against private object storage metadata';
COMMENT ON COLUMN plugin_idc_compliance_enterprise_material."encryption_key_no" IS
    'Low-entropy keyring generation code; the secretcap handle remains in the keyring table only';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, '企业认证案件状态', 'idc_compliance_enterprise_verification_status', 1, 1, FALSE,
     '企业人工认证案件的完整生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证材料类型', 'idc_compliance_enterprise_material_type', 1, 1, FALSE,
     '企业人工认证允许提交的材料分类', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证材料状态', 'idc_compliance_enterprise_material_status', 1, 1, FALSE,
     '企业认证加密材料的保留与处置状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证上传预约状态', 'idc_compliance_enterprise_material_upload_status', 1, 1, FALSE,
     '服务端分配材料编号后的上传预约状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证申请人角色', 'idc_compliance_enterprise_applicant_role', 1, 1, FALSE,
     '企业认证资料提交人的法定角色', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证材料加密状态', 'idc_compliance_enterprise_material_encryption_state', 1, 1, FALSE,
     '企业认证材料在私有对象存储中的加密状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业认证密钥状态', 'idc_compliance_enterprise_encryption_key_status', 1, 1, FALSE,
     '安装实例企业认证材料密钥代次状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_enterprise_verification_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '待审核', 'pending', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '审核中', 'verifying', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '需要复核', 'review_required', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '已认证', 'verified', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '已驳回', 'rejected', 6, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '已过期', 'expired', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_verification_status', '已取消', 'canceled', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '营业执照', 'business_license', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '法定代表人身份证明', 'legal_representative_id', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '经办人身份证明', 'agent_id', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '授权委托书', 'authorization_letter', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '组织机构证明', 'organization_certificate', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_type', '其他材料', 'other', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '隔离确认中', 'quarantined', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '可用于审核', 'available', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '已被替代', 'superseded', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '待删除', 'deletion_pending', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '已删除', 'deleted', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_status', '法律保全', 'legal_hold', 6, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_upload_status', '已预约', 'reserved', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_upload_status', '已完成', 'completed', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_applicant_role', '法定代表人', 'legal_representative', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_applicant_role', '授权经办人', 'authorized_agent', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_material_encryption_state', '已加密', 'encrypted', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_encryption_key_status', '使用中', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_encryption_key_status', '已停用', 'retired', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
