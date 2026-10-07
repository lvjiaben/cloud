-- Installation-local enterprise auto-sign authorization and accepted-use evidence.
-- Rows are immutable or explicitly revoked; legal and security history is never soft-deleted.

-- Purpose: Persists business records for plugin_idc_compliance_auto_sign_auth.
-- 用途：持久化 plugin_idc_compliance_auto_sign_auth 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_auto_sign_auth (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "authorization_no" VARCHAR(64) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL DEFAULT 'tencent_essbasic',
    "currency_code" CHAR(3) NOT NULL,
    "allow_required_order" BOOLEAN NOT NULL DEFAULT TRUE,
    "allow_manual_optional" BOOLEAN NOT NULL DEFAULT FALSE,
    "max_amount_minor" BIGINT NOT NULL,
    "success_limit" BIGINT NOT NULL,
    "success_count" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_by_user_id" INT NOT NULL,
    "revoked_by_user_id" INT,
    "revoked_at" TIMESTAMPTZ,
    "creation_idempotency_digest" CHAR(64) NOT NULL,
    "creation_fingerprint" CHAR(64) NOT NULL,
    "revocation_idempotency_digest" CHAR(64),
    "revocation_fingerprint" CHAR(64),
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_casa_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_casa_provider
        CHECK ("provider_code" = 'tencent_essbasic'),
    CONSTRAINT ck_plugin_idc_compliance_casa_currency
        CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_compliance_casa_scope
        CHECK ("allow_required_order" OR "allow_manual_optional"),
    CONSTRAINT ck_plugin_idc_compliance_casa_amount CHECK ("max_amount_minor" > 0),
    CONSTRAINT ck_plugin_idc_compliance_casa_count
        CHECK ("success_limit" > 0 AND "success_count" >= 0 AND "success_count" <= "success_limit"),
    CONSTRAINT ck_plugin_idc_compliance_casa_status
        CHECK ("status_code" IN ('active', 'expired', 'revoked')),
    CONSTRAINT ck_plugin_idc_compliance_casa_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT ck_plugin_idc_compliance_casa_creation
        CHECK (
            "creation_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
            "creation_fingerprint" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_casa_revocation
        CHECK (
            (
                "status_code" = 'revoked' AND "revoked_by_user_id" IS NOT NULL AND
                "revoked_at" IS NOT NULL AND
                "revocation_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
                "revocation_fingerprint" ~ '^[0-9a-f]{64}$'
            ) OR
            (
                "status_code" <> 'revoked' AND "revoked_by_user_id" IS NULL AND
                "revoked_at" IS NULL AND "revocation_idempotency_digest" IS NULL AND
                "revocation_fingerprint" IS NULL
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_casa_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_casa_no UNIQUE ("authorization_no"),
    CONSTRAINT uk_plugin_idc_compliance_casa_create_key
        UNIQUE ("tenant_id", "creation_idempotency_digest")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_casa_active_currency
    ON plugin_idc_compliance_auto_sign_auth ("tenant_id", "currency_code")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_casa_list
    ON plugin_idc_compliance_auto_sign_auth (
        "tenant_id", "created_at" DESC, "id" DESC
    );

-- Purpose: Persists business records for plugin_idc_compliance_auto_sign_auth_template.
-- 用途：持久化 plugin_idc_compliance_auto_sign_auth_template 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_auto_sign_auth_template (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "authorization_no" VARCHAR(64) NOT NULL,
    "template_no" VARCHAR(64) NOT NULL,
    "version_no" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_casat_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_casat_version CHECK ("version_no" > 0),
    CONSTRAINT uk_plugin_idc_compliance_casat_ref
        UNIQUE ("authorization_no", "template_no", "version_no"),
    CONSTRAINT fk_plugin_idc_compliance_casat_authorization
        FOREIGN KEY ("authorization_no")
        REFERENCES plugin_idc_compliance_auto_sign_auth ("authorization_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_casat_template
        FOREIGN KEY ("template_no", "version_no")
        REFERENCES plugin_idc_compliance_contract_template_version ("template_no", "version_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_casat_template
    ON plugin_idc_compliance_auto_sign_auth_template (
        "tenant_id", "template_no", "version_no", "authorization_no"
    );

-- Purpose: Persists business records for plugin_idc_compliance_contract_auto_sign_usage.
-- 用途：持久化 plugin_idc_compliance_contract_auto_sign_usage 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_auto_sign_usage (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "usage_no" VARCHAR(64) NOT NULL,
    "authorization_no" VARCHAR(64) NOT NULL,
    "contract_no" VARCHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "command_idempotency_digest" CHAR(64) NOT NULL,
    "command_fingerprint" CHAR(64) NOT NULL,
    "provider_accepted_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_casu_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_casu_currency
        CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_compliance_casu_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_plugin_idc_compliance_casu_command
        CHECK (
            "command_idempotency_digest" ~ '^[0-9a-f]{64}$' AND
            "command_fingerprint" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT uk_plugin_idc_compliance_casu_no UNIQUE ("usage_no"),
    CONSTRAINT uk_plugin_idc_compliance_casu_contract
        UNIQUE ("authorization_no", "contract_no"),
    CONSTRAINT uk_plugin_idc_compliance_casu_command
        UNIQUE ("tenant_id", "command_idempotency_digest"),
    CONSTRAINT fk_plugin_idc_compliance_casu_authorization
        FOREIGN KEY ("authorization_no")
        REFERENCES plugin_idc_compliance_auto_sign_auth ("authorization_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_casu_authorization
    ON plugin_idc_compliance_contract_auto_sign_usage (
        "tenant_id", "authorization_no", "provider_accepted_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_compliance_auto_sign_auth IS
    'Immutable installation-local enterprise auto-sign authorization; changes require revoke then create';
COMMENT ON TABLE plugin_idc_compliance_auto_sign_auth_template IS
    'Exact immutable contract-template versions covered by one local auto-sign authorization';
COMMENT ON TABLE plugin_idc_compliance_contract_auto_sign_usage IS
    'Append-only evidence for supplier-accepted auto-sign starts; safe replay never increments twice';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, '企业自动签授权状态', 'idc_compliance_contract_auto_sign_status', 1, 1, FALSE,
     '安装实例内企业自动签授权的不可编辑生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '企业自动签合同来源', 'idc_compliance_contract_auto_sign_source', 1, 1, FALSE,
     '企业自动签授权明确允许的新合同来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_contract_auto_sign_status', '生效中', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_contract_auto_sign_status', '已到期', 'expired', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_contract_auto_sign_status', '已撤销', 'revoked', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_contract_auto_sign_source', '必须签署的订单合同', 'required_order', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_contract_auto_sign_source', '后台人工发起的可选合同', 'manual_optional', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
