-- Durable security context for single-use recent-authentication proofs.
-- This table is an append-only security fence, not a cache: losing a row before
-- the signed proof expires would permit replay after eviction or process restart.

-- Purpose: Append-only database replay fence for signed recent-login proofs.
-- 用途：保存近期认证消费相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_recent_auth_consumption (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "nonce_digest" CHAR(64) NOT NULL,
    "purpose_code" VARCHAR(32) NOT NULL,
    "target_digest" CHAR(64) NOT NULL,
    "session_digest" CHAR(64) NOT NULL,
    "user_id" INT NOT NULL,
    "login_at" TIMESTAMPTZ NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_reauth_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_reauth_nonce CHECK ("nonce_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_reauth_target CHECK ("target_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_reauth_session CHECK ("session_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_reauth_user CHECK ("user_id" > 0),
    CONSTRAINT ck_plugin_idc_core_reauth_purpose CHECK (
        "purpose_code" IN (
            'secret.access',
            'payment.config',
            'funds.high-value',
            'resource.bulk',
            'export.sensitive',
            'material.sensitive',
            'resource.server-operation',
            'permission.change'
        )
    ),
    CONSTRAINT ck_plugin_idc_core_reauth_time CHECK (
        "login_at" <= "issued_at" + INTERVAL '1 minute' AND
        "issued_at" <= "login_at" + INTERVAL '15 minutes' AND
        "expires_at" > "issued_at" AND
        "expires_at" <= "issued_at" + INTERVAL '5 minutes' AND
        "issued_at" <= "consumed_at" + INTERVAL '1 minute' AND
        "consumed_at" < "expires_at" + INTERVAL '1 minute'
    ),
    CONSTRAINT uk_plugin_idc_core_reauth_nonce UNIQUE ("tenant_id", "nonce_digest")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_reauth_expiry
    ON plugin_idc_core_recent_auth_consumption ("expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_reauth_actor
    ON plugin_idc_core_recent_auth_consumption (
        "tenant_id", "user_id", "consumed_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_recent_auth_consumption IS 'Append-only database replay fence for signed recent-login proofs';
COMMENT ON COLUMN plugin_idc_core_recent_auth_consumption."nonce_digest" IS 'Domain-separated SHA-256 digest of a random proof nonce; raw nonce and proof are never persisted';
COMMENT ON COLUMN plugin_idc_core_recent_auth_consumption."target_digest" IS 'Digest of the immutable dangerous-operation target or preview';
COMMENT ON COLUMN plugin_idc_core_recent_auth_consumption."session_digest" IS 'Digest binding to the host session without persisting its raw identifier';
