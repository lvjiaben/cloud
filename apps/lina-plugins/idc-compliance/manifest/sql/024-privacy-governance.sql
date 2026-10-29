-- Required authentication and invoice material retention, legal holds and confirmed deletion.
-- The tables contain governance metadata only. Raw identity values, original
-- files, private object paths, access tokens, and secret references are forbidden.

ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "retention_until" TIMESTAMPTZ;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "legal_hold" BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_idc_compliance_verification_case_retention
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "subject_type_code", "retention_until", "id"
    ) WHERE "retention_until" IS NOT NULL AND NOT "legal_hold";

-- Purpose: Persists business records for plugin_idc_compliance_privacy_retention_policy.
-- 用途：持久化 plugin_idc_compliance_privacy_retention_policy 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_retention_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "data_type_code" VARCHAR(40) NOT NULL,
    "version" BIGINT NOT NULL,
    "retention_days" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_by_user_id" INT NOT NULL,
    "superseded_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_retention_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_retention_type CHECK (
        "data_type_code" IN (
            'personal_verification_material', 'enterprise_verification_material',
            'tax_invoice_evidence'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_retention_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_compliance_privacy_retention_days CHECK ("retention_days" > 0),
    CONSTRAINT ck_idc_compliance_privacy_retention_status CHECK ("status_code" IN ('active', 'superseded')),
    CONSTRAINT ck_idc_compliance_privacy_retention_superseded CHECK (
        ("status_code" = 'active' AND "superseded_at" IS NULL) OR
        ("status_code" = 'superseded' AND "superseded_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_compliance_privacy_retention_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_compliance_privacy_retention_version UNIQUE ("tenant_id", "data_type_code", "version")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_privacy_retention_active
    ON plugin_idc_compliance_privacy_retention_policy ("tenant_id", "data_type_code")
    WHERE "status_code" = 'active';

-- Purpose: Persists business records for plugin_idc_compliance_privacy_retention_hold.
-- 用途：持久化 plugin_idc_compliance_privacy_retention_hold 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_retention_hold (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "hold_no" VARCHAR(40) NOT NULL,
    "target_type_code" VARCHAR(40) NOT NULL,
    "target_no" VARCHAR(64) NOT NULL,
    "reason_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_by_owner" VARCHAR(64) NOT NULL,
    "released_by_owner" VARCHAR(64) NOT NULL DEFAULT '',
    "released_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_hold_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_hold_target_type CHECK (
        "target_type_code" IN (
            'personal_verification_case', 'enterprise_verification_case', 'enterprise_verification_material',
            'tax_invoice_file'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_hold_reason CHECK (
        "reason_code" IN ('tax', 'dispute', 'audit', 'legal_hold')
    ),
    CONSTRAINT ck_idc_compliance_privacy_hold_status CHECK ("status_code" IN ('active', 'released')),
    CONSTRAINT ck_idc_compliance_privacy_hold_owner CHECK (
        length(btrim("created_by_owner")) BETWEEN 1 AND 64 AND
        length("released_by_owner") <= 64
    ),
    CONSTRAINT ck_idc_compliance_privacy_hold_release CHECK (
        ("status_code" = 'active' AND "released_by_owner" = '' AND "released_at" IS NULL) OR
        ("status_code" = 'released' AND length(btrim("released_by_owner")) > 0 AND "released_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_compliance_privacy_hold_no UNIQUE ("hold_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_privacy_hold_active
    ON plugin_idc_compliance_privacy_retention_hold (
        "tenant_id", "target_type_code", "target_no", "reason_code"
    ) WHERE "status_code" = 'active';

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_hold_target
    ON plugin_idc_compliance_privacy_retention_hold (
        "tenant_id", "target_type_code", "target_no", "status_code", "id"
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_deletion_task.
-- 用途：持久化 plugin_idc_compliance_privacy_deletion_task 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_deletion_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "data_type_code" VARCHAR(40) NOT NULL,
    "target_type_code" VARCHAR(40) NOT NULL,
    "target_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "due_at" TIMESTAMPTZ NOT NULL,
    "hold_reason_code" VARCHAR(32) NOT NULL DEFAULT '',
    "confirmed_by_user_id" INT NOT NULL DEFAULT 0,
    "confirmed_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_deletion_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_deletion_data_type CHECK (
        "data_type_code" IN (
            'personal_verification_material', 'enterprise_verification_material',
            'tax_invoice_evidence'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_target_type CHECK (
        "target_type_code" IN (
            'personal_verification_case', 'enterprise_verification_case', 'enterprise_verification_material',
            'tax_invoice_file'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_status CHECK (
        "status_code" IN ('pending', 'blocked', 'confirmed', 'completed')
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_hold CHECK (
        "hold_reason_code" IN ('', 'tax', 'dispute', 'audit', 'legal_hold')
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_state CHECK (
        ("status_code" = 'pending' AND "hold_reason_code" = '' AND "confirmed_at" IS NULL AND "completed_at" IS NULL) OR
        ("status_code" = 'blocked' AND "hold_reason_code" <> '' AND "completed_at" IS NULL) OR
        ("status_code" = 'confirmed' AND "hold_reason_code" = '' AND "confirmed_by_user_id" > 0 AND "confirmed_at" IS NOT NULL AND "completed_at" IS NULL) OR
        ("status_code" = 'completed' AND "hold_reason_code" = '' AND "confirmed_by_user_id" > 0 AND "confirmed_at" IS NOT NULL AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_compliance_privacy_deletion_no UNIQUE ("task_no"),
    CONSTRAINT uk_idc_compliance_privacy_deletion_target UNIQUE ("tenant_id", "target_type_code", "target_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_deletion_queue
    ON plugin_idc_compliance_privacy_deletion_task (
        "tenant_id", "status_code", "due_at", "id"
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_action.
-- 用途：持久化 plugin_idc_compliance_privacy_action 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_action (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "action_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(40) NOT NULL,
    "target_no" VARCHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "result_reference" VARCHAR(64) NOT NULL,
    "actor_user_id" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_action_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_action_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$' AND "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_compliance_privacy_action_no UNIQUE ("action_no"),
    CONSTRAINT uk_idc_compliance_privacy_action_replay UNIQUE (
        "tenant_id", "action_code", "target_no", "idempotency_digest"
    )
);

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at")
VALUES
    (0, '隐私资料类型', 'idc_compliance_privacy_data_type', 1, 1, FALSE, '分类型留存和删除治理资料', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '隐私删除阻断原因', 'idc_compliance_privacy_hold_reason', 1, 1, FALSE, '继续保留资料的稳定依据', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_privacy_data_type', '个人实名认证材料', 'personal_verification_material', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '企业认证材料', 'enterprise_verification_material', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '税务发票证据', 'tax_invoice_evidence', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '税务留存', 'tax', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '争议处理中', 'dispute', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '审计留存', 'audit', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '法律保全', 'legal_hold', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON TABLE plugin_idc_compliance_privacy_retention_hold IS 'Exact active tax, dispute, audit, or legal holds supplied by local domain owners';
COMMENT ON TABLE plugin_idc_compliance_privacy_deletion_task IS 'Human-confirmed deletion queue with legal and business hold rechecks';
