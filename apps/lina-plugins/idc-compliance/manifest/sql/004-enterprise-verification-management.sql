-- Staff review metadata and keyset performance for enterprise verification.
-- Raw legal values, original material bytes, access contexts, and session
-- identifiers are forbidden from these columns.

ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "review_idempotency_key_digest" CHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "review_fingerprint" CHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "decision_idempotency_key_digest" CHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "decision_fingerprint" CHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_compliance_verification_case
    ADD COLUMN IF NOT EXISTS "decision_action_code" VARCHAR(24) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_compliance_verification_case
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_ent_review_metadata;
ALTER TABLE plugin_idc_compliance_verification_case
    ADD CONSTRAINT ck_plugin_idc_compliance_ent_review_metadata CHECK (
        (
            "subject_type_code" = 'personal' AND
            "review_idempotency_key_digest" = '' AND
            "review_fingerprint" = '' AND
            "decision_idempotency_key_digest" = '' AND
            "decision_fingerprint" = '' AND
            "decision_action_code" = ''
        ) OR
        (
            "subject_type_code" = 'enterprise' AND
            (
                ("review_idempotency_key_digest" = '' AND "review_fingerprint" = '') OR
                (
                    "review_idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
                    "review_fingerprint" ~ '^[0-9a-f]{64}$'
                )
            ) AND
            (
                (
                    "decision_idempotency_key_digest" = '' AND
                    "decision_fingerprint" = '' AND
                    "decision_action_code" = ''
                ) OR
                (
                    "decision_idempotency_key_digest" ~ '^[0-9a-f]{64}$' AND
                    "decision_fingerprint" ~ '^[0-9a-f]{64}$' AND
                    "decision_action_code" IN ('approved', 'rejected', 'duplicate_flagged')
                )
            )
        )
    );

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ent_case_dir
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "subject_type_code", "submitted_at" DESC, "case_no" DESC
    ) WHERE "subject_type_code" = 'enterprise' AND "submitted_at" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ent_status_dir
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "subject_type_code", "status_code", "submitted_at" DESC, "case_no" DESC
    ) WHERE "subject_type_code" = 'enterprise' AND "submitted_at" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_ent_customer_dir
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "subject_type_code", "customer_no", "submitted_at" DESC, "case_no" DESC
    ) WHERE "subject_type_code" = 'enterprise' AND "submitted_at" IS NOT NULL;

COMMENT ON COLUMN plugin_idc_compliance_verification_case."legal_profile_sha256" IS
    'SHA-256 of the authenticated encrypted legal-profile envelope; never a digest of legal plaintext';
COMMENT ON COLUMN plugin_idc_compliance_verification_case."review_idempotency_key_digest" IS
    'SHA-256 of the bounded review-start idempotency key; the raw key is never persisted';
COMMENT ON COLUMN plugin_idc_compliance_verification_case."decision_idempotency_key_digest" IS
    'SHA-256 of the bounded decision idempotency key; the raw key is never persisted';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, '企业认证审核原因', 'idc_compliance_enterprise_decision_reason', 1, 1, FALSE,
     '企业人工认证批准、驳回及重复登记风险的稳定原因码', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_enterprise_decision_reason', '人工审核通过', 'manual_review_passed', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '提交材料不完整', 'materials_incomplete', 2, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '提交材料无法辨认', 'materials_unreadable', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '法定资料与证明不一致', 'legal_information_mismatch', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '企业登记标识无效', 'registration_identifier_invalid', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '法定代表人身份不一致', 'representative_identity_mismatch', 6, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '经办人授权资料无效', 'agent_authorization_invalid', 7, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '企业登记状态暂无法核验', 'registration_status_unverifiable', 8, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '业务范围暂不支持', 'business_scope_unsupported', 9, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_enterprise_decision_reason', '企业登记标识存在重复风险', 'duplicate_registration_risk', 10, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
