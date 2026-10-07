-- Purpose: Persists business records for plugin_idc_compliance_provider_configuration.
-- 用途：持久化 plugin_idc_compliance_provider_configuration 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_provider_configuration (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "configuration_no" VARCHAR(64) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'draft',
    "scene_id" VARCHAR(64) NOT NULL DEFAULT '',
    "credential_secret_ref" VARCHAR(256),
    "credential_generation" BIGINT NOT NULL DEFAULT 0,
    "last_mutation_digest" CHAR(64),
    "mutation_operation_key" CHAR(64),
    "mutation_secret_updated_at" TIMESTAMPTZ,
    "last_validation_code" VARCHAR(64) NOT NULL DEFAULT '',
    "validated_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_pcfg_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_provider
        CHECK ("provider_code" = 'aliyun_face'),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_status
        CHECK ("status_code" IN ('draft', 'updating', 'configured', 'disabled')),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_scene
        CHECK (
            "scene_id" = '' OR
            (length("scene_id") BETWEEN 1 AND 64 AND "scene_id" ~ '^[0-9]+$')
        ),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_ready
        CHECK (
            "status_code" <> 'configured' OR
            (
                "scene_id" <> '' AND
                "credential_secret_ref" IS NOT NULL AND
                "credential_generation" > 0
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_generation
        CHECK ("credential_generation" >= 0),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_validation
        CHECK ("last_validation_code" IN ('', 'reachable', 'rejected', 'unavailable')),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_mutation_digest
        CHECK (
            "last_mutation_digest" IS NULL OR
            "last_mutation_digest" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_mutation_fence
        CHECK (
            "status_code" <> 'updating' OR
            (
                "last_mutation_digest" IS NOT NULL AND
                "mutation_operation_key" IS NOT NULL AND
                "mutation_operation_key" ~ '^[0-9a-f]{64}$'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_pcfg_version
        CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_pcfg_no
        UNIQUE ("configuration_no"),
    CONSTRAINT uk_plugin_idc_compliance_pcfg_provider
        UNIQUE ("tenant_id", "provider_code")
);

COMMENT ON TABLE plugin_idc_compliance_provider_configuration IS
    'Compliance-owned provider configuration; secretcap routing references are never exposed through API, logs, or audit payloads';
COMMENT ON COLUMN plugin_idc_compliance_provider_configuration."scene_id" IS
    'Merchant-owned Aliyun financial-grade identity verification scene ID; endpoint, service, version, FACE code, and identity type are fixed in code';
COMMENT ON COLUMN plugin_idc_compliance_provider_configuration."credential_secret_ref" IS
    'Sensitive secretcap routing reference for one atomic RAM AccessKey credential envelope; plaintext and ciphertext are forbidden';
COMMENT ON COLUMN plugin_idc_compliance_provider_configuration."last_mutation_digest" IS
    'SHA-256 digest of the bounded operator idempotency key; credentials are excluded';
COMMENT ON COLUMN plugin_idc_compliance_provider_configuration."mutation_operation_key" IS
    'Internal random secretcap idempotency key retained only for audited recovery; never exposed by API, logs, or audit payloads';
COMMENT ON COLUMN plugin_idc_compliance_provider_configuration."mutation_secret_updated_at" IS
    'Safe secretcap optimistic fence retained to resume or diagnose the last idempotent credential replacement';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 合规服务商配置状态', 'idc_compliance_provider_configuration_status', 1, 1, FALSE,
     '安装实例级合规服务商配置生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 合规服务商验证状态', 'idc_compliance_provider_validation_status', 1, 1, FALSE,
     '不含供应商原始消息的只读连通性结果', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_provider_configuration_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_configuration_status', '更新中', 'updating', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_configuration_status', '已配置', 'configured', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_configuration_status', '已停用', 'disabled', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_validation_status', '未测试', 'untested', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_validation_status', '可连接', 'reachable', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_validation_status', '被拒绝', 'rejected', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_provider_validation_status', '暂不可用', 'unavailable', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

-- Purpose: Persists business records for plugin_idc_compliance_personal_verification_attempt.
-- 用途：持久化 plugin_idc_compliance_personal_verification_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_personal_verification_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "attempt_no" VARCHAR(64) NOT NULL,
    "case_no" VARCHAR(64) NOT NULL,
    "subject_type_code" VARCHAR(16) NOT NULL DEFAULT 'personal',
    "configuration_no" VARCHAR(64) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "configuration_version" BIGINT NOT NULL,
    "outer_order_no" CHAR(32) NOT NULL,
    "certify_id" VARCHAR(128),
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'preparing',
    "last_provider_result_code" VARCHAR(64) NOT NULL DEFAULT '',
    "not_passed_observation_count" INT NOT NULL DEFAULT 0,
    "initialization_started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "issued_at" TIMESTAMPTZ,
    "certify_expires_at" TIMESTAMPTZ,
    "last_queried_at" TIMESTAMPTZ,
    "passed_at" TIMESTAMPTZ,
    "expired_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_pva_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_pva_subject
        CHECK ("subject_type_code" = 'personal'),
    CONSTRAINT ck_plugin_idc_compliance_pva_provider
        CHECK ("provider_code" = 'aliyun_face'),
    CONSTRAINT ck_plugin_idc_compliance_pva_config_version
        CHECK ("configuration_version" > 0),
    CONSTRAINT ck_plugin_idc_compliance_pva_outer_order
        CHECK ("outer_order_no" ~ '^[A-Za-z0-9]{32}$'),
    CONSTRAINT ck_plugin_idc_compliance_pva_status
        CHECK (
            "status_code" IN (
                'preparing', 'initialization_unknown', 'ready', 'pending',
                'passed', 'expired', 'canceled'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_pva_count
        CHECK ("not_passed_observation_count" >= 0),
    CONSTRAINT ck_plugin_idc_compliance_pva_issued
        CHECK (
            ("certify_id" IS NULL AND "issued_at" IS NULL AND "certify_expires_at" IS NULL) OR
            (
                "certify_id" IS NOT NULL AND
                "issued_at" IS NOT NULL AND
                "certify_expires_at" > "issued_at"
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_pva_terminal
        CHECK (
            ("status_code" = 'passed' AND "passed_at" IS NOT NULL AND "expired_at" IS NULL) OR
            ("status_code" = 'expired' AND "expired_at" IS NOT NULL AND "passed_at" IS NULL) OR
            ("status_code" NOT IN ('passed', 'expired') AND "passed_at" IS NULL AND "expired_at" IS NULL)
        ),
    CONSTRAINT ck_plugin_idc_compliance_pva_version
        CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_pva_no
        UNIQUE ("attempt_no"),
    CONSTRAINT uk_plugin_idc_compliance_pva_outer_order
        UNIQUE ("outer_order_no"),
    CONSTRAINT fk_plugin_idc_compliance_pva_case
        FOREIGN KEY ("case_no", "subject_type_code")
        REFERENCES plugin_idc_compliance_verification_case ("case_no", "subject_type_code"),
    CONSTRAINT fk_plugin_idc_compliance_pva_config
        FOREIGN KEY ("configuration_no")
        REFERENCES plugin_idc_compliance_provider_configuration ("configuration_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_pva_certify
    ON plugin_idc_compliance_personal_verification_attempt ("certify_id")
    WHERE "certify_id" IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_pva_active
    ON plugin_idc_compliance_personal_verification_attempt ("tenant_id", "case_no")
    WHERE "status_code" IN ('preparing', 'initialization_unknown', 'ready', 'pending');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_pva_recovery
    ON plugin_idc_compliance_personal_verification_attempt (
        "tenant_id", "status_code", "initialization_started_at", "certify_expires_at", "id"
    ) WHERE "status_code" IN ('preparing', 'initialization_unknown', 'ready', 'pending');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_pva_case
    ON plugin_idc_compliance_personal_verification_attempt (
        "tenant_id", "case_no", "created_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_compliance_personal_verification_attempt IS
    'Aliyun Alipay H5 FACE attempt state; browser return is never authoritative and passed=F remains non-terminal until the 30-minute window expires';
COMMENT ON COLUMN plugin_idc_compliance_personal_verification_attempt."certify_id" IS
    'Aliyun billing and reconciliation identifier; certify URL, face images, face videos, identity plaintext, and raw provider payloads are forbidden';

COMMENT ON COLUMN plugin_idc_compliance_verification_case."legal_profile_sha256" IS
    'SHA-256 of a random challenge stored only inside the encrypted legal-profile snapshot; never derived from identity plaintext';
