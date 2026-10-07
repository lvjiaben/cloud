-- Installation-local privacy governance introduced by task 10.13.
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

-- Purpose: Persists business records for plugin_idc_compliance_privacy_assessment.
-- 用途：持久化 plugin_idc_compliance_privacy_assessment 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_assessment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "assessment_no" VARCHAR(40) NOT NULL,
    "assessment_type_code" VARCHAR(40) NOT NULL,
    "version" BIGINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "purpose" VARCHAR(2000) NOT NULL,
    "necessity" VARCHAR(2000) NOT NULL,
    "impact" VARCHAR(2000) NOT NULL,
    "recipient_description" VARCHAR(2000) NOT NULL,
    "storage_location" VARCHAR(1000) NOT NULL,
    "retention_description" VARCHAR(1000) NOT NULL,
    "rights_entry" VARCHAR(1000) NOT NULL,
    "merchant_role" VARCHAR(1000) NOT NULL,
    "third_party_role" VARCHAR(1000) NOT NULL,
    "processing_instruction" VARCHAR(2000) NOT NULL,
    "subcontracting_boundary" VARCHAR(2000) NOT NULL,
    "audit_evidence_description" VARCHAR(2000) NOT NULL,
    "cross_border_transfer" BOOLEAN NOT NULL DEFAULT FALSE,
    "confirmed_by_user_id" INT NOT NULL DEFAULT 0,
    "confirmed_at" TIMESTAMPTZ,
    "superseded_at" TIMESTAMPTZ,
    "created_by_user_id" INT NOT NULL,
    "updated_by_user_id" INT NOT NULL,
    "row_version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_assessment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_assessment_type CHECK (
        "assessment_type_code" IN (
            'personal_verification', 'enterprise_verification', 'electronic_contract',
            'tax_invoice', 'local_sensitive_export', 'ai_safe_projection'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_assessment_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_compliance_privacy_assessment_row_version CHECK ("row_version" > 0),
    CONSTRAINT ck_idc_compliance_privacy_assessment_status CHECK (
        "status_code" IN ('draft', 'published', 'superseded')
    ),
    CONSTRAINT ck_idc_compliance_privacy_assessment_text CHECK (
        length(btrim("purpose")) BETWEEN 1 AND 2000 AND
        length(btrim("necessity")) BETWEEN 1 AND 2000 AND
        length(btrim("impact")) BETWEEN 1 AND 2000 AND
        length(btrim("recipient_description")) BETWEEN 1 AND 2000 AND
        length(btrim("storage_location")) BETWEEN 1 AND 1000 AND
        length(btrim("retention_description")) BETWEEN 1 AND 1000 AND
        length(btrim("rights_entry")) BETWEEN 1 AND 1000 AND
        length(btrim("merchant_role")) BETWEEN 1 AND 1000 AND
        length(btrim("third_party_role")) BETWEEN 1 AND 1000 AND
        length(btrim("processing_instruction")) BETWEEN 1 AND 2000 AND
        length(btrim("subcontracting_boundary")) BETWEEN 1 AND 2000 AND
        length(btrim("audit_evidence_description")) BETWEEN 1 AND 2000
    ),
    CONSTRAINT ck_idc_compliance_privacy_assessment_cross_border CHECK (NOT "cross_border_transfer"),
    CONSTRAINT ck_idc_compliance_privacy_assessment_confirmation CHECK (
        ("status_code" = 'draft' AND "confirmed_by_user_id" = 0 AND "confirmed_at" IS NULL AND "superseded_at" IS NULL) OR
        ("status_code" = 'published' AND "confirmed_by_user_id" > 0 AND "confirmed_at" IS NOT NULL AND "superseded_at" IS NULL) OR
        ("status_code" = 'superseded' AND "confirmed_by_user_id" > 0 AND "confirmed_at" IS NOT NULL AND "superseded_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_compliance_privacy_assessment_no UNIQUE ("assessment_no"),
    CONSTRAINT uk_idc_compliance_privacy_assessment_version UNIQUE ("tenant_id", "assessment_type_code", "version")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_privacy_assessment_draft
    ON plugin_idc_compliance_privacy_assessment ("tenant_id", "assessment_type_code")
    WHERE "status_code" = 'draft';
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_privacy_assessment_published
    ON plugin_idc_compliance_privacy_assessment ("tenant_id", "assessment_type_code")
    WHERE "status_code" = 'published';
CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_assessment_directory
    ON plugin_idc_compliance_privacy_assessment (
        "tenant_id", "assessment_type_code", "version" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_ai_field.
-- 用途：持久化 plugin_idc_compliance_privacy_ai_field 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_ai_field (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "assessment_no" VARCHAR(40) NOT NULL,
    "field_code" VARCHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_ai_field_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_ai_field_code CHECK (
        "field_code" IN (
            'personal_verification_status', 'enterprise_verification_status',
            'contract_status', 'tax_invoice_status', 'aggregate_case_count',
            'aggregate_contract_count', 'aggregate_invoice_count'
        )
    ),
    CONSTRAINT uk_idc_compliance_privacy_ai_field UNIQUE ("assessment_no", "field_code"),
    CONSTRAINT fk_idc_compliance_privacy_ai_field_assessment FOREIGN KEY ("assessment_no")
        REFERENCES plugin_idc_compliance_privacy_assessment ("assessment_no") ON DELETE RESTRICT
);

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
            'electronic_contract_evidence', 'tax_invoice_evidence',
            'sensitive_access_audit', 'local_sensitive_export'
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
            'personal_verification_case', 'enterprise_verification_material',
            'electronic_contract_file', 'tax_invoice_file', 'sensitive_access_audit',
            'local_sensitive_export'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_hold_reason CHECK (
        "reason_code" IN ('contract', 'tax', 'dispute', 'audit', 'legal_hold')
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
            'electronic_contract_evidence', 'tax_invoice_evidence',
            'sensitive_access_audit', 'local_sensitive_export'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_target_type CHECK (
        "target_type_code" IN (
            'personal_verification_case', 'enterprise_verification_material',
            'electronic_contract_file', 'tax_invoice_file', 'sensitive_access_audit',
            'local_sensitive_export'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_status CHECK (
        "status_code" IN ('pending', 'blocked', 'completed')
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_hold CHECK (
        "hold_reason_code" IN ('', 'contract', 'tax', 'dispute', 'audit', 'legal_hold')
    ),
    CONSTRAINT ck_idc_compliance_privacy_deletion_state CHECK (
        ("status_code" = 'pending' AND "hold_reason_code" = '' AND "confirmed_at" IS NULL AND "completed_at" IS NULL) OR
        ("status_code" = 'blocked' AND "hold_reason_code" <> '' AND "completed_at" IS NULL) OR
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

-- Purpose: Persists business records for plugin_idc_compliance_privacy_incident.
-- 用途：持久化 plugin_idc_compliance_privacy_incident 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_incident (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "incident_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'discovered',
    "severity_code" VARCHAR(16) NOT NULL,
    "source_code" VARCHAR(32) NOT NULL,
    "safe_summary" VARCHAR(2000) NOT NULL,
    "ticket_generation" BIGINT NOT NULL,
    "export_blocked" BOOLEAN NOT NULL DEFAULT TRUE,
    "external_notification_confirmed" BOOLEAN NOT NULL DEFAULT FALSE,
    "external_notification_confirmed_by" INT NOT NULL DEFAULT 0,
    "external_notification_confirmed_at" TIMESTAMPTZ,
    "resolved_at" TIMESTAMPTZ,
    "created_by_user_id" INT NOT NULL,
    "updated_by_user_id" INT NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_incident_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_incident_status CHECK (
        "status_code" IN (
            'discovered', 'contained', 'assessed', 'notification_pending',
            'notification_confirmed', 'remediating', 'resolved'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_severity CHECK (
        "severity_code" IN ('low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_source CHECK (
        "source_code" IN ('unauthorized_attempt', 'bulk_download', 'data_loss', 'manual_report', 'system_detection')
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_summary CHECK (
        length(btrim("safe_summary")) BETWEEN 1 AND 2000
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_generation CHECK ("ticket_generation" > 0),
    CONSTRAINT ck_idc_compliance_privacy_incident_notification CHECK (
        (NOT "external_notification_confirmed" AND "external_notification_confirmed_by" = 0 AND "external_notification_confirmed_at" IS NULL) OR
        ("external_notification_confirmed" AND "external_notification_confirmed_by" > 0 AND "external_notification_confirmed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_resolved CHECK (
        ("status_code" = 'resolved' AND "resolved_at" IS NOT NULL) OR
        ("status_code" <> 'resolved' AND "resolved_at" IS NULL)
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_compliance_privacy_incident_no UNIQUE ("incident_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_incident_directory
    ON plugin_idc_compliance_privacy_incident (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_incident_event.
-- 用途：持久化 plugin_idc_compliance_privacy_incident_event 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_incident_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "incident_no" VARCHAR(40) NOT NULL,
    "stage_code" VARCHAR(24) NOT NULL,
    "safe_note" VARCHAR(2000) NOT NULL,
    "actor_user_id" INT NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_incident_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_incident_event_stage CHECK (
        "stage_code" IN (
            'discovered', 'contained', 'assessed', 'notification_pending',
            'notification_confirmed', 'remediating', 'resolved'
        )
    ),
    CONSTRAINT ck_idc_compliance_privacy_incident_event_note CHECK (
        length(btrim("safe_note")) BETWEEN 1 AND 2000
    ),
    CONSTRAINT uk_idc_compliance_privacy_incident_event_no UNIQUE ("event_no"),
    CONSTRAINT fk_idc_compliance_privacy_incident_event FOREIGN KEY ("incident_no")
        REFERENCES plugin_idc_compliance_privacy_incident ("incident_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_incident_timeline
    ON plugin_idc_compliance_privacy_incident_event (
        "tenant_id", "incident_no", "occurred_at", "id"
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_customer_request.
-- 用途：持久化 plugin_idc_compliance_privacy_customer_request 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_customer_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "request_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'submitted',
    "hold_reason_code" VARCHAR(32) NOT NULL DEFAULT '',
    "submitted_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_customer_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_customer_request_type CHECK (
        "request_type_code" IN ('export', 'delete')
    ),
    CONSTRAINT ck_idc_compliance_privacy_customer_request_status CHECK (
        "status_code" IN ('submitted', 'processing', 'retention_blocked', 'completed', 'rejected')
    ),
    CONSTRAINT ck_idc_compliance_privacy_customer_request_hold CHECK (
        "hold_reason_code" IN ('', 'contract', 'tax', 'dispute', 'audit', 'legal_hold')
    ),
    CONSTRAINT ck_idc_compliance_privacy_customer_request_completion CHECK (
        ("status_code" = 'completed' AND "completed_at" IS NOT NULL) OR
        ("status_code" <> 'completed' AND "completed_at" IS NULL)
    ),
    CONSTRAINT ck_idc_compliance_privacy_customer_request_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_compliance_privacy_customer_request_no UNIQUE ("request_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_customer_request_owner
    ON plugin_idc_compliance_privacy_customer_request (
        "tenant_id", "customer_no", "submitted_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_compliance_privacy_support_export.
-- 用途：持久化 plugin_idc_compliance_privacy_support_export 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_privacy_support_export (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "export_no" VARCHAR(40) NOT NULL,
    "purpose_code" VARCHAR(32) NOT NULL DEFAULT 'support_diagnosis',
    "assessment_no" VARCHAR(40) NOT NULL,
    "field_count" INT NOT NULL,
    "record_count" INT NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "content_bytes" BYTEA NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "retention_until" TIMESTAMPTZ NOT NULL,
    "created_by_user_id" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_privacy_support_export_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_privacy_support_export_purpose CHECK ("purpose_code" = 'support_diagnosis'),
    CONSTRAINT ck_idc_compliance_privacy_support_export_count CHECK (
        "field_count" > 0 AND "field_count" <= 32 AND "record_count" >= 0 AND "record_count" <= 1000
    ),
    CONSTRAINT ck_idc_compliance_privacy_support_export_digest CHECK (
        "content_sha256" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_compliance_privacy_support_export_content CHECK (
        octet_length("content_bytes") BETWEEN 2 AND 65536
    ),
    CONSTRAINT ck_idc_compliance_privacy_support_export_status CHECK (
        "status_code" IN ('active', 'deleted')
    ),
    CONSTRAINT uk_idc_compliance_privacy_support_export_no UNIQUE ("export_no"),
    CONSTRAINT fk_idc_compliance_privacy_support_export_assessment FOREIGN KEY ("assessment_no")
        REFERENCES plugin_idc_compliance_privacy_assessment ("assessment_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_privacy_support_export_retention
    ON plugin_idc_compliance_privacy_support_export (
        "tenant_id", "status_code", "retention_until", "id"
    ) WHERE "status_code" = 'active';

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
    (0, '隐私影响评估类型', 'idc_compliance_privacy_assessment_type', 1, 1, FALSE, '六类独立个人信息保护影响评估', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '隐私资料类型', 'idc_compliance_privacy_data_type', 1, 1, FALSE, '分类型留存和删除治理资料', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '隐私删除阻断原因', 'idc_compliance_privacy_hold_reason', 1, 1, FALSE, '继续保留资料的稳定依据', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '隐私事件阶段', 'idc_compliance_privacy_incident_stage', 1, 1, FALSE, '泄露响应闭环阶段', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '隐私申请状态', 'idc_compliance_privacy_request_status', 1, 1, FALSE, '客户导出和删除申请状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_privacy_assessment_type', '个人实名认证', 'personal_verification', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_assessment_type', '企业认证', 'enterprise_verification', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_assessment_type', '电子合同', 'electronic_contract', 3, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_assessment_type', '税务发票', 'tax_invoice', 4, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_assessment_type', '本地敏感资料导出', 'local_sensitive_export', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_assessment_type', 'AI安全投影', 'ai_safe_projection', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '个人实名认证材料', 'personal_verification_material', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '企业认证材料', 'enterprise_verification_material', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '电子合同证据', 'electronic_contract_evidence', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '税务发票证据', 'tax_invoice_evidence', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '敏感访问审计', 'sensitive_access_audit', 5, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_data_type', '本地敏感资料导出', 'local_sensitive_export', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '合同留存', 'contract', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '税务留存', 'tax', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '争议处理中', 'dispute', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '审计留存', 'audit', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_hold_reason', '法律保全', 'legal_hold', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '已发现', 'discovered', 1, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '已遏制', 'contained', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '已评估', 'assessed', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '待确认通知', 'notification_pending', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '已确认通知', 'notification_confirmed', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '修复中', 'remediating', 6, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_incident_stage', '已解决', 'resolved', 7, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_request_status', '已提交', 'submitted', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_request_status', '处理中', 'processing', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_request_status', '因留存要求暂缓', 'retention_blocked', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_request_status', '已完成', 'completed', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_privacy_request_status', '已拒绝', 'rejected', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON TABLE plugin_idc_compliance_privacy_assessment IS 'Versioned installation-local privacy impact assessments; raw personal data is forbidden';
COMMENT ON TABLE plugin_idc_compliance_privacy_retention_hold IS 'Exact active contract, tax, dispute, audit, or legal holds supplied by local domain owners';
COMMENT ON TABLE plugin_idc_compliance_privacy_deletion_task IS 'Human-confirmed deletion queue with legal and business hold rechecks';
COMMENT ON TABLE plugin_idc_compliance_privacy_incident IS 'Installation-local privacy incident response state; no automatic external notification';
COMMENT ON TABLE plugin_idc_compliance_privacy_support_export IS 'Metadata for locally generated minimal redacted support exports; no remote support access';
