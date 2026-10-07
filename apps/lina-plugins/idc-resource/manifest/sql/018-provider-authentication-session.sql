-- 018: cluster-safe Provider authentication sessions
--
-- PostgreSQL is the authority for token generation, expiry, refresh leases,
-- and invalidation. Token plaintext is encrypted by core secretcap; this table
-- stores only its opaque reference. Rows are lifecycle state and are never
-- soft-deleted.

-- Purpose: Persists business records for plugin_idc_resource_provider_auth_session.
-- 用途：持久化 plugin_idc_resource_provider_auth_session 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provider_auth_session (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(64) PRIMARY KEY,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'idle',
    "generation" BIGINT NOT NULL DEFAULT 0,
    "secret_ref" VARCHAR(256),
    "expires_at" TIMESTAMPTZ,
    "lease_token_digest" VARCHAR(64),
    "lease_expires_at" TIMESTAMPTZ,
    "invalidated_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_plugin_idc_resource_auth_session_channel
        FOREIGN KEY ("channel_no") REFERENCES plugin_idc_resource_channel ("channel_no")
        ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_resource_auth_session_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_auth_session_status CHECK (
        "status_code" IN ('idle', 'refreshing', 'ready')
    ),
    CONSTRAINT ck_plugin_idc_resource_auth_session_generation CHECK ("generation" >= 0),
    CONSTRAINT ck_plugin_idc_resource_auth_session_version CHECK ("version" > 0),
    CONSTRAINT ck_plugin_idc_resource_auth_session_shape CHECK (
        ("status_code" = 'idle' AND "expires_at" IS NULL
            AND "lease_token_digest" IS NULL AND "lease_expires_at" IS NULL) OR
        ("status_code" = 'refreshing' AND "expires_at" IS NULL
            AND "lease_token_digest" IS NOT NULL AND "lease_expires_at" IS NOT NULL) OR
        ("status_code" = 'ready' AND "secret_ref" IS NOT NULL AND "expires_at" IS NOT NULL
            AND "lease_token_digest" IS NULL AND "lease_expires_at" IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_auth_session_refresh
    ON plugin_idc_resource_provider_auth_session (
        "tenant_id", "status_code", "lease_expires_at", "channel_no"
    ) WHERE "status_code" = 'refreshing';

COMMENT ON TABLE plugin_idc_resource_provider_auth_session
    IS 'Database-authoritative encrypted Provider login session and refresh lease';
COMMENT ON COLUMN plugin_idc_resource_provider_auth_session."secret_ref"
    IS 'Opaque core secretcap handle; never returned, logged, audited, or used as plaintext';
COMMENT ON COLUMN plugin_idc_resource_provider_auth_session."generation"
    IS 'Monotonic fence preventing a stale 401 from invalidating a newer token';
COMMENT ON COLUMN plugin_idc_resource_provider_auth_session."lease_token_digest"
    IS 'SHA-256 digest of an ephemeral refresh lease token; plaintext is never persisted';
