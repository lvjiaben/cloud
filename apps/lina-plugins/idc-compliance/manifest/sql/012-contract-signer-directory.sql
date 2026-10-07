-- Purpose: Persists business records for plugin_idc_compliance_contract_signer_profile.
-- 用途：持久化 plugin_idc_compliance_contract_signer_profile 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_signer_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "signer_no" VARCHAR(64) NOT NULL,
    "customer_no" VARCHAR(64) NOT NULL,
    "profile_snapshot_ref" VARCHAR(512) NOT NULL,
    "profile_snapshot_sha256" CHAR(64) NOT NULL CHECK ("profile_snapshot_sha256" ~ '^[0-9a-f]{64}$'),
    "masked_name" VARCHAR(128) NOT NULL,
    "masked_identity_no" VARCHAR(128) NOT NULL,
    "masked_phone" VARCHAR(64) NOT NULL,
    "consent_policy_version" VARCHAR(16) NOT NULL,
    consent_occurred_at TIMESTAMPTZ NOT NULL,
    "identity_status_code" VARCHAR(24) NOT NULL DEFAULT 'unverified'
        CHECK ("identity_status_code" IN ('unverified','verifying','verified','review_required','expired')),
    "identity_attempt_no" VARCHAR(64),
    "identity_verification_version" BIGINT NOT NULL DEFAULT 0 CHECK ("identity_verification_version" >= 0),
    identity_verified_at TIMESTAMPTZ,
    "phone_status_code" VARCHAR(24) NOT NULL DEFAULT 'unverified'
        CHECK ("phone_status_code" IN ('unverified','verifying','verified')),
    "phone_verification_no" VARCHAR(48),
    "phone_verification_version" BIGINT NOT NULL DEFAULT 0 CHECK ("phone_verification_version" >= 0),
    phone_verified_at TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active' CHECK ("status_code" IN ('active','disabled')),
    "authorization_version" BIGINT NOT NULL DEFAULT 1 CHECK ("authorization_version" > 0),
    "creation_digest" CHAR(64) NOT NULL CHECK ("creation_digest" ~ '^[0-9a-f]{64}$'),
    disabled_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("signer_no"),
    UNIQUE ("tenant_id", "customer_no", "creation_digest"),
    CHECK (("identity_status_code" = 'verified') = ("identity_verification_version" > 0 AND identity_verified_at IS NOT NULL)),
    CHECK (("phone_status_code" = 'verified') = ("phone_verification_version" > 0 AND phone_verified_at IS NOT NULL)),
    CHECK (char_length("consent_policy_version") BETWEEN 1 AND 16),
    CHECK (("status_code" = 'disabled') = (disabled_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_signer_customer
    ON plugin_idc_compliance_contract_signer_profile
    ("tenant_id", "customer_no", "status_code", created_at DESC, "id" DESC);
CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_signer_candidates
    ON plugin_idc_compliance_contract_signer_profile
    ("tenant_id", "customer_no", "identity_status_code", "phone_status_code", "status_code", updated_at DESC)
    WHERE "identity_status_code"='verified' AND "phone_status_code"='verified' AND "status_code"='active';

-- Purpose: Persists business records for plugin_idc_compliance_contract_signer_identity_attempt.
-- 用途：持久化 plugin_idc_compliance_contract_signer_identity_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_signer_identity_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INTEGER NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "attempt_no" VARCHAR(64) NOT NULL,
    "signer_no" VARCHAR(64) NOT NULL,
    "outer_order_no" VARCHAR(64) NOT NULL,
    "configuration_no" VARCHAR(64) NOT NULL,
    "configuration_version" BIGINT NOT NULL CHECK ("configuration_version" > 0),
    "credential_generation" BIGINT NOT NULL CHECK ("credential_generation" > 0),
    "certify_id" VARCHAR(256),
    "status_code" VARCHAR(24) NOT NULL CHECK ("status_code" IN ('preparing','pending','passed','review_required','expired')),
    "reason_code" VARCHAR(64) NOT NULL DEFAULT '',
    initialization_started_at TIMESTAMPTZ NOT NULL,
    certify_expires_at TIMESTAMPTZ,
    last_queried_at TIMESTAMPTZ,
    passed_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("attempt_no"),
    UNIQUE ("outer_order_no"),
    FOREIGN KEY ("signer_no") REFERENCES plugin_idc_compliance_contract_signer_profile ("signer_no") ON DELETE RESTRICT,
    CHECK (
        ("status_code" = 'preparing' AND "certify_id" IS NULL) OR
        ("status_code" IN ('pending','passed','expired') AND "certify_id" IS NOT NULL) OR
        "status_code" = 'review_required'
    ),
    CHECK (("status_code" = 'passed') = (passed_at IS NOT NULL)),
    CHECK (("status_code" = 'expired') = (expired_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_signer_attempt
    ON plugin_idc_compliance_contract_signer_identity_attempt
    ("tenant_id", "signer_no", created_at DESC, "id" DESC);
