-- Purpose: Persists business records for plugin_shared_services_challenge.
-- 用途：持久化 plugin_shared_services_challenge 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_shared_services_challenge (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "challenge_no" VARCHAR(64) NOT NULL,
    "caller_plugin_id" VARCHAR(128) NOT NULL,
    "purpose" VARCHAR(128) NOT NULL,
    "binding_digest" CHAR(64) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL CHECK ("provider_code" IN ('local', 'turnstile')),
    "presentation_kind" VARCHAR(32) NOT NULL CHECK ("presentation_kind" IN ('image', 'accessible', 'turnstile')),
    "answer_salt" CHAR(64),
    "answer_digest" CHAR(64),
    "proof_digest" CHAR(64),
    "status" VARCHAR(16) NOT NULL CHECK ("status" IN ('pending', 'verified', 'consumed', 'expired', 'failed')),
    "attempt_count" SMALLINT NOT NULL DEFAULT 0 CHECK ("attempt_count" >= 0 AND "attempt_count" <= 5),
    "max_attempts" SMALLINT NOT NULL DEFAULT 5 CHECK ("max_attempts" = 5),
    expires_at TIMESTAMPTZ NOT NULL,
    proof_expires_at TIMESTAMPTZ,
    verified_at TIMESTAMPTZ,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_shared_services_challenge_no UNIQUE ("tenant_id", "challenge_no"),
    CONSTRAINT uq_shared_services_challenge_proof UNIQUE ("tenant_id", "proof_digest"),
    CONSTRAINT ck_shared_services_challenge_answer CHECK (
        ("provider_code" = 'local' AND "answer_salt" IS NOT NULL AND "answer_digest" IS NOT NULL)
        OR ("provider_code" = 'turnstile' AND "answer_salt" IS NULL AND "answer_digest" IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_shared_services_challenge_expiry
    ON plugin_shared_services_challenge ("status", expires_at, "id");
CREATE INDEX IF NOT EXISTS idx_shared_services_challenge_caller
    ON plugin_shared_services_challenge ("caller_plugin_id", "purpose", created_at DESC);

-- Purpose: Persists business records for plugin_shared_services_inbox_message.
-- 用途：持久化 plugin_shared_services_inbox_message 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_shared_services_inbox_message (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "message_no" VARCHAR(64) NOT NULL,
    "recipient_user_id" BIGINT NOT NULL CHECK ("recipient_user_id" > 0),
    "caller_plugin_id" VARCHAR(128) NOT NULL,
    "source_type" VARCHAR(128) NOT NULL,
    "source_no" VARCHAR(128) NOT NULL,
    "idempotency_key" VARCHAR(128) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "title" VARCHAR(200) NOT NULL,
    "body" VARCHAR(4000) NOT NULL,
    "category" VARCHAR(64) NOT NULL,
    "action_url" VARCHAR(2048),
    "status" VARCHAR(16) NOT NULL DEFAULT 'published' CHECK ("status" IN ('published', 'read')),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_shared_services_inbox_message_no UNIQUE ("tenant_id", "message_no"),
    CONSTRAINT uq_shared_services_inbox_idempotency UNIQUE (
        "tenant_id", "caller_plugin_id", "recipient_user_id", "source_type", "source_no", "idempotency_key"
    )
);

CREATE INDEX IF NOT EXISTS idx_shared_services_inbox_recipient
    ON plugin_shared_services_inbox_message ("recipient_user_id", created_at DESC, "id" DESC);

-- Purpose: Persists business records for plugin_shared_services_inbox_delivery_evidence.
-- 用途：持久化 plugin_shared_services_inbox_delivery_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_shared_services_inbox_delivery_evidence (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "evidence_no" VARCHAR(64) NOT NULL,
    "message_no" VARCHAR(64) NOT NULL,
    "caller_plugin_id" VARCHAR(128) NOT NULL,
    "evidence_type" VARCHAR(32) NOT NULL CHECK ("evidence_type" IN ('published', 'idempotent_replay')),
    "content_digest" CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_shared_services_inbox_evidence_no UNIQUE ("tenant_id", "evidence_no"),
    CONSTRAINT fk_shared_services_inbox_evidence_message FOREIGN KEY ("tenant_id", "message_no")
        REFERENCES plugin_shared_services_inbox_message ("tenant_id", "message_no")
);

CREATE INDEX IF NOT EXISTS idx_shared_services_inbox_evidence_message
    ON plugin_shared_services_inbox_delivery_evidence ("message_no", created_at DESC, "id" DESC);
