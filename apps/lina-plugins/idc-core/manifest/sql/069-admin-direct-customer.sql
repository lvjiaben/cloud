-- Phase 11 feedback: direct active-customer creation and first-login password change.
ALTER TABLE plugin_idc_core_customer_identity_binding
    ADD COLUMN IF NOT EXISTS "must_change_password" BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS "idempotency_key_digest" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "request_fingerprint" VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_core_customer_identity_binding
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_binding_source;

ALTER TABLE plugin_idc_core_customer_identity_binding
    ADD CONSTRAINT ck_plugin_idc_core_binding_source
    CHECK ("source_code" IN ('onboarding', 'admin_invitation', 'admin_direct', 'migration'));

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_binding_direct_request
    ON plugin_idc_core_customer_identity_binding ("tenant_id", "created_by", "idempotency_key_digest")
    WHERE "idempotency_key_digest" <> '';

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_binding_password_change
    ON plugin_idc_core_customer_identity_binding ("tenant_id", "sys_user_id", "status_code", "must_change_password");

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_binding_source', '后台直接创建', 'admin_direct', 4, 'success',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."must_change_password" IS 'Direct-created customer must replace the administrator-provided initial password before using account APIs';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."idempotency_key_digest" IS 'Irreversible digest of the administrator direct-create retry key';
COMMENT ON COLUMN plugin_idc_core_customer_identity_binding."request_fingerprint" IS 'Normalized direct-create request fingerprint excluding plaintext password';
