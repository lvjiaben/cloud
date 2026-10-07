-- Customer account workflow foundation. Legal-change, closure, export and
-- status-history records are retained as workflow evidence and never use soft
-- deletion. Approved closure anonymizes ordinary customer PII in place while
-- preserving records that remain subject to legal, accounting or dispute
-- retention; it never erases immutable business evidence indiscriminately.

-- Purpose: Review workflow for changing verified legal projections; raw identifiers, documents and certification materials remain compliance-owned and are never stored here.
-- 用途：保存客户法定变更申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_legal_change_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'submitted',
    "reason_code" VARCHAR(48) NOT NULL,
    "change_legal_name" BOOLEAN NOT NULL DEFAULT FALSE,
    "change_legal_identifier" BOOLEAN NOT NULL DEFAULT FALSE,
    "current_verification_status_code" VARCHAR(24) NOT NULL,
    "current_verification_case_no" VARCHAR(40) NOT NULL,
    "current_profile_version" BIGINT NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "submitted_by_user_id" INT NOT NULL,
    "submitted_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "reviewed_by_user_id" INT,
    "reviewed_at" TIMESTAMPTZ,
    "resolution_code" VARCHAR(64) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_legal_change_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_legal_change_no CHECK (
        "request_no" ~ '^LCR_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_type CHECK (
        "customer_type_code" IN ('individual', 'enterprise')
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_status CHECK (
        "status_code" IN ('submitted', 'under_review', 'approved', 'rejected', 'cancelled')
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_reason CHECK (
        "reason_code" IN ('legal_name_change', 'identifier_correction', 'enterprise_registration_change', 'other')
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_fields CHECK (
        "change_legal_name" OR "change_legal_identifier"
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_verified CHECK (
        "current_verification_status_code" = 'verified' AND
        "current_verification_case_no" <> '' AND
        "current_profile_version" > 0
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_digests CHECK (
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_core_legal_change_actor CHECK ("submitted_by_user_id" > 0),
    CONSTRAINT ck_plugin_idc_core_legal_change_review_state CHECK (
        ("status_code" IN ('submitted', 'under_review') AND
            "reviewed_by_user_id" IS NULL AND "reviewed_at" IS NULL AND "resolution_code" = '') OR
        ("status_code" IN ('approved', 'rejected') AND
            "reviewed_by_user_id" > 0 AND "reviewed_at" >= "submitted_at" AND "resolution_code" <> '') OR
        ("status_code" = 'cancelled' AND "reviewed_at" >= "submitted_at" AND "resolution_code" <> '')
    ),
    CONSTRAINT uk_plugin_idc_core_legal_change_no UNIQUE ("request_no"),
    CONSTRAINT uk_plugin_idc_core_legal_change_request UNIQUE (
        "tenant_id", "customer_no", "idempotency_key_digest"
    ),
    CONSTRAINT fk_plugin_idc_core_legal_change_customer FOREIGN KEY (
        "customer_no", "customer_type_code"
    ) REFERENCES plugin_idc_core_customer_subject (
        "customer_no", "customer_type_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_legal_change_customer
    ON plugin_idc_core_customer_legal_change_request (
        "tenant_id", "customer_no", "status_code", "submitted_at" DESC, "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_legal_change_review
    ON plugin_idc_core_customer_legal_change_request (
        "tenant_id", "status_code", "submitted_at", "id"
    ) WHERE "status_code" IN ('submitted', 'under_review');
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_legal_change_active
    ON plugin_idc_core_customer_legal_change_request ("tenant_id", "customer_no")
    WHERE "status_code" IN ('submitted', 'under_review');

COMMENT ON TABLE plugin_idc_core_customer_legal_change_request IS
    'Review workflow for changing verified legal projections; raw identifiers, documents and certification materials remain compliance-owned and are never stored here';
COMMENT ON COLUMN plugin_idc_core_customer_legal_change_request."current_verification_case_no" IS
    'Opaque reference to the verification case whose result remains unchanged while this request is reviewed';
COMMENT ON COLUMN plugin_idc_core_customer_legal_change_request."change_legal_name" IS
    'Declares a requested field category without storing the proposed raw legal value';
COMMENT ON COLUMN plugin_idc_core_customer_legal_change_request."change_legal_identifier" IS
    'Declares a requested identifier category without storing a raw identifier or document';

-- Purpose: Append-only account lifecycle evidence; rows are retained through restriction, disablement and approved anonymization and are never soft-deleted.
-- 用途：保存客户账户状态历史相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_account_state_history (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transition_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "from_status_code" VARCHAR(32) NOT NULL,
    "to_status_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(48) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL,
    "source_no" VARCHAR(64) NOT NULL DEFAULT '',
    "actor_user_id" INT NOT NULL DEFAULT 0,
    "subject_version" BIGINT NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_account_history_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_account_history_no CHECK (
        "transition_no" ~ '^CAT_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_from CHECK (
        "from_status_code" IN ('pending_activation', 'active', 'restricted', 'disabled', 'anonymized')
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_to CHECK (
        "to_status_code" IN ('pending_activation', 'active', 'restricted', 'disabled', 'anonymized')
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_transition CHECK (
        "from_status_code" <> "to_status_code"
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_reason CHECK (
        "reason_code" IN (
            'activation_completed', 'verification_required', 'risk_restriction',
            'restriction_resolved', 'manual_disable', 'account_reactivated',
            'approved_closure'
        )
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_source CHECK (
        "source_code" IN ('staff', 'closure')
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_actor CHECK (
        "source_code" <> 'staff' OR "actor_user_id" > 0
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_version CHECK ("subject_version" > 1),
    CONSTRAINT ck_plugin_idc_core_account_history_digests CHECK (
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_core_account_history_matrix CHECK (
        ("source_code" = 'staff' AND (
            ("from_status_code" = 'pending_activation' AND "to_status_code" = 'active' AND "reason_code" = 'activation_completed') OR
            ("from_status_code" = 'pending_activation' AND "to_status_code" = 'restricted' AND "reason_code" = 'verification_required') OR
            ("from_status_code" = 'pending_activation' AND "to_status_code" = 'disabled' AND "reason_code" = 'manual_disable') OR
            ("from_status_code" = 'active' AND "to_status_code" = 'restricted' AND "reason_code" IN ('risk_restriction', 'verification_required')) OR
            ("from_status_code" = 'active' AND "to_status_code" = 'disabled' AND "reason_code" = 'manual_disable') OR
            ("from_status_code" = 'restricted' AND "to_status_code" = 'active' AND "reason_code" = 'restriction_resolved') OR
            ("from_status_code" = 'restricted' AND "to_status_code" = 'disabled' AND "reason_code" = 'manual_disable') OR
            ("from_status_code" = 'disabled' AND "to_status_code" = 'active' AND "reason_code" = 'account_reactivated')
        )) OR
        ("source_code" = 'closure' AND "from_status_code" IN ('active', 'restricted', 'disabled') AND
            "to_status_code" = 'anonymized' AND "reason_code" = 'approved_closure')
    ),
    CONSTRAINT uk_plugin_idc_core_account_history_no UNIQUE ("transition_no"),
    CONSTRAINT uk_plugin_idc_core_account_history_request UNIQUE (
        "tenant_id", "customer_no", "idempotency_key_digest"
    ),
    CONSTRAINT fk_plugin_idc_core_account_history_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_account_history_customer
    ON plugin_idc_core_customer_account_state_history (
        "tenant_id", "customer_no", "occurred_at" DESC, "id"
    );

COMMENT ON TABLE plugin_idc_core_customer_account_state_history IS
    'Append-only account lifecycle evidence; rows are retained through restriction, disablement and approved anonymization and are never soft-deleted';

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_consent_customer
    ON plugin_idc_core_customer_consent ("consent_no", "customer_no");

-- Purpose: Controlled closure requests with a bounded cross-domain precheck projection; completion later rechecks blockers and anonymizes only data permitted by the frozen retention policy.
-- 用途：保存客户注销申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_closure_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "consent_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "subject_version" BIGINT NOT NULL,
    "unsettled_funds_count" INT NOT NULL DEFAULT 0,
    "active_service_count" INT NOT NULL DEFAULT 0,
    "unsettled_invoice_count" INT NOT NULL DEFAULT 0,
    "active_contract_count" INT NOT NULL DEFAULT 0,
    "in_progress_operation_count" INT NOT NULL DEFAULT 0,
    "legal_retention_count" INT NOT NULL DEFAULT 0,
    "precheck_contract_version" INT NOT NULL DEFAULT 1,
    "prechecked_at" TIMESTAMPTZ NOT NULL,
    "retention_policy_code" VARCHAR(64) NOT NULL,
    "anonymization_policy_code" VARCHAR(64) NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "requested_by_user_id" INT NOT NULL,
    "confirmed_at" TIMESTAMPTZ NOT NULL,
    "requested_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "reviewed_by_user_id" INT,
    "reviewed_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_closure_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_closure_no CHECK (
        "request_no" ~ '^CLR_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_core_closure_status CHECK (
        "status_code" IN (
            'pending_review', 'approved', 'rejected',
            'anonymizing', 'completed', 'cancelled'
        )
    ),
    CONSTRAINT ck_plugin_idc_core_closure_version CHECK (
        "subject_version" > 0 AND "precheck_contract_version" = 1
    ),
    CONSTRAINT ck_plugin_idc_core_closure_counts CHECK (
        "unsettled_funds_count" >= 0 AND
        "active_service_count" >= 0 AND
        "unsettled_invoice_count" >= 0 AND
        "active_contract_count" >= 0 AND
        "in_progress_operation_count" >= 0 AND
        "legal_retention_count" >= 0 AND
        "unsettled_funds_count" <= 1000000000 AND
        "active_service_count" <= 1000000000 AND
        "unsettled_invoice_count" <= 1000000000 AND
        "active_contract_count" <= 1000000000 AND
        "in_progress_operation_count" <= 1000000000 AND
        "legal_retention_count" <= 1000000000
    ),
    CONSTRAINT ck_plugin_idc_core_closure_clear CHECK (
        "unsettled_funds_count" = 0 AND "active_service_count" = 0 AND
        "unsettled_invoice_count" = 0 AND "active_contract_count" = 0 AND
        "in_progress_operation_count" = 0
    ),
    CONSTRAINT ck_plugin_idc_core_closure_policies CHECK (
        "retention_policy_code" = 'customer_retention_v1' AND
        "anonymization_policy_code" = 'customer_anonymization_v1'
    ),
    CONSTRAINT ck_plugin_idc_core_closure_actor CHECK ("requested_by_user_id" > 0),
    CONSTRAINT ck_plugin_idc_core_closure_time CHECK (
        "prechecked_at" <= "confirmed_at" AND "confirmed_at" <= "requested_at"
    ),
    CONSTRAINT ck_plugin_idc_core_closure_review_state CHECK (
        ("status_code" = 'pending_review' AND "reviewed_by_user_id" IS NULL AND
            "reviewed_at" IS NULL AND "completed_at" IS NULL) OR
        ("status_code" IN ('approved', 'rejected') AND "reviewed_by_user_id" > 0 AND
            "reviewed_at" IS NOT NULL AND "completed_at" IS NULL) OR
        ("status_code" = 'anonymizing' AND "reviewed_by_user_id" > 0 AND
            "reviewed_at" IS NOT NULL AND "completed_at" IS NULL) OR
        ("status_code" = 'completed' AND "reviewed_by_user_id" > 0 AND
            "reviewed_at" IS NOT NULL AND "completed_at" IS NOT NULL) OR
        ("status_code" = 'cancelled' AND "completed_at" IS NULL)
    ),
    CONSTRAINT ck_plugin_idc_core_closure_review_time CHECK (
        ("reviewed_at" IS NULL OR "reviewed_at" >= "requested_at") AND
        ("completed_at" IS NULL OR ("reviewed_at" IS NOT NULL AND "completed_at" >= "reviewed_at"))
    ),
    CONSTRAINT ck_plugin_idc_core_closure_digests CHECK (
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_closure_no UNIQUE ("request_no"),
    CONSTRAINT uk_plugin_idc_core_closure_request UNIQUE (
        "tenant_id", "customer_no", "idempotency_key_digest"
    ),
    CONSTRAINT fk_plugin_idc_core_closure_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_closure_consent FOREIGN KEY ("consent_no", "customer_no")
        REFERENCES plugin_idc_core_customer_consent ("consent_no", "customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_closure_active
    ON plugin_idc_core_customer_closure_request ("tenant_id", "customer_no")
    WHERE "status_code" IN ('pending_review', 'approved', 'anonymizing');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_closure_customer
    ON plugin_idc_core_customer_closure_request (
        "tenant_id", "customer_no", "status_code", "requested_at" DESC, "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_closure_review
    ON plugin_idc_core_customer_closure_request (
        "tenant_id", "status_code", "requested_at", "id"
    ) WHERE "status_code" IN ('pending_review', 'approved', 'anonymizing');

COMMENT ON TABLE plugin_idc_core_customer_closure_request IS
    'Controlled closure requests with a bounded cross-domain precheck projection; completion later rechecks blockers and anonymizes only data permitted by the frozen retention policy';
COMMENT ON COLUMN plugin_idc_core_customer_closure_request."legal_retention_count" IS
    'Retention findings do not by themselves authorize deletion or bypass closure review; legally retained evidence survives ordinary-profile anonymization';

-- Purpose: Recently reauthenticated self-service export request linked by opaque task number to the durable asynchronous task service; generated artifacts remain private and time-limited.
-- 用途：保存客户数据导出申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_data_export_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "consent_no" VARCHAR(40) NOT NULL,
    "task_no" VARCHAR(40) NOT NULL,
    "scope_code" VARCHAR(32) NOT NULL,
    "format_code" VARCHAR(24) NOT NULL,
    "subject_version" BIGINT NOT NULL,
    "idempotency_key_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "reauth_issued_at" TIMESTAMPTZ NOT NULL,
    "requested_by_user_id" INT NOT NULL,
    "requested_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_export_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_export_no CHECK (
        "request_no" ~ '^DEX_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_core_export_task_no CHECK (
        "task_no" ~ '^TSK_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_core_export_scope CHECK (
        "scope_code" = 'own_deliverable_data'
    ),
    CONSTRAINT ck_plugin_idc_core_export_format CHECK (
        "format_code" = 'json_zip'
    ),
    CONSTRAINT ck_plugin_idc_core_export_version CHECK ("subject_version" > 0),
    CONSTRAINT ck_plugin_idc_core_export_digests CHECK (
        "idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_core_export_reauth_time CHECK (
        "reauth_issued_at" <= "requested_at"
    ),
    CONSTRAINT ck_plugin_idc_core_export_actor CHECK ("requested_by_user_id" > 0),
    CONSTRAINT uk_plugin_idc_core_export_no UNIQUE ("request_no"),
    CONSTRAINT uk_plugin_idc_core_export_task UNIQUE ("task_no"),
    CONSTRAINT uk_plugin_idc_core_export_request UNIQUE (
        "tenant_id", "customer_no", "idempotency_key_digest"
    ),
    CONSTRAINT fk_plugin_idc_core_export_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_export_consent FOREIGN KEY ("consent_no", "customer_no")
        REFERENCES plugin_idc_core_customer_consent ("consent_no", "customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_export_customer
    ON plugin_idc_core_customer_data_export_request (
        "tenant_id", "customer_no", "requested_at" DESC, "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_export_timeline
    ON plugin_idc_core_customer_data_export_request (
        "tenant_id", "requested_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_customer_data_export_request IS
    'Recently reauthenticated self-service export request linked by opaque task number to the durable asynchronous task service; generated artifacts remain private and time-limited';
COMMENT ON COLUMN plugin_idc_core_customer_data_export_request."task_no" IS
    'Application-validated asynchronous task reference without a cross-domain database foreign key';

INSERT INTO sys_dict_type (
    "tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at"
)
VALUES
    (0, 'IDC 客户账户状态原因', 'idc_customer_account_status_reason', 1, 1, FALSE, '客户账户状态转换的稳定原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户账户转换来源', 'idc_customer_account_transition_source', 1, 1, FALSE, '客户账户状态转换来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户注销阻碍', 'idc_customer_closure_finding', 1, 1, FALSE, '客户注销前置检查结果', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户注销状态', 'idc_customer_closure_status', 1, 1, FALSE, '客户注销申请生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户数据导出格式', 'idc_customer_data_export_format', 1, 1, FALSE, '客户自有数据导出格式', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户数据导出范围', 'idc_customer_data_export_scope', 1, 1, FALSE, '客户自有数据导出范围', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 法定资料变更原因', 'idc_customer_legal_change_reason', 1, 1, FALSE, '法定资料变更复核原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 法定资料变更状态', 'idc_customer_legal_change_status', 1, 1, FALSE, '法定资料变更复核生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at"
)
VALUES
    (0, 'idc_customer_account_status_reason', '完成激活', 'activation_completed', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '需要认证', 'verification_required', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '风险限制', 'risk_restriction', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '限制已解除', 'restriction_resolved', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '人工停用', 'manual_disable', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '账户已恢复', 'account_reactivated', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_status_reason', '批准注销', 'approved_closure', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_transition_source', '后台员工', 'staff', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_account_transition_source', '注销流程', 'closure', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在未结资金', 'unsettled_funds', 1, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在有效服务', 'active_services', 2, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在未结账单', 'unsettled_invoices', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在进行中合同', 'active_contracts', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在进行中业务操作', 'in_progress_operations', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_finding', '存在法定留存义务', 'legal_retention', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '存在阻碍', 'blocked', 1, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '待复核', 'pending_review', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '已批准', 'approved', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '已驳回', 'rejected', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '匿名化处理中', 'anonymizing', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '已完成', 'completed', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_closure_status', '已取消', 'cancelled', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_data_export_format', 'JSON 压缩包', 'json_zip', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_data_export_scope', '本人可交付数据', 'own_deliverable_data', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_reason', '法定名称变更', 'legal_name_change', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_reason', '标识信息更正', 'identifier_correction', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_reason', '企业登记变更', 'enterprise_registration_change', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_reason', '其他', 'other', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_status', '已提交', 'submitted', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_status', '复核中', 'under_review', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_status', '已批准', 'approved', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_status', '已驳回', 'rejected', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_legal_change_status', '已取消', 'cancelled', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
