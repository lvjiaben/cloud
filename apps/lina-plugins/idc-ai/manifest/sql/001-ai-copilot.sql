-- Purpose: Stores tenant-scoped AI copilot connection, budget, rate and retention settings.
-- 用途：保存租户级 AI 助手连接、预算、限流和留存设置。
CREATE TABLE IF NOT EXISTS plugin_idc_ai_config (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "base_url" TEXT NOT NULL,
    "model_name" VARCHAR(128) NOT NULL,
    "encrypted_api_key" BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    "budget_minor" BIGINT NOT NULL DEFAULT 0,
    "rate_limit_per_minute" INT NOT NULL DEFAULT 30,
    "retention_days" INT NOT NULL DEFAULT 30,
    "revision" BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("tenant_id")
);
-- Purpose: Stores redacted AI conversation evidence for cost, safety and audit review.
-- 用途：保存已脱敏的 AI 会话证据，用于成本、安全和审计复核。
CREATE TABLE IF NOT EXISTS plugin_idc_ai_conversation (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "actor_type" VARCHAR(16) NOT NULL,
    "actor_id" VARCHAR(64) NOT NULL,
    "prompt_digest" CHAR(64) NOT NULL,
    "response_text" TEXT NOT NULL,
    "model_name" VARCHAR(128),
    "token_count" INT NOT NULL DEFAULT 0,
    "estimated_cost_minor" BIGINT NOT NULL DEFAULT 0,
    evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    "safety_decision" VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Purpose: Stores AI tool authorization decisions and their governed evidence.
-- 用途：保存 AI 工具授权决策及其受治理证据。
CREATE TABLE IF NOT EXISTS plugin_idc_ai_tool_audit (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "actor_type" VARCHAR(16) NOT NULL,
    "actor_id" VARCHAR(64) NOT NULL,
    "tool_name" VARCHAR(96) NOT NULL,
    "decision" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64),
    evidence_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Purpose: Stores user ratings and comments for completed AI conversations.
-- 用途：保存用户对已完成 AI 会话的评分和意见。
CREATE TABLE IF NOT EXISTS plugin_idc_ai_feedback (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "conversation_id" BIGINT,
    "actor_id" VARCHAR(64) NOT NULL,
    "rating" SMALLINT NOT NULL,
    comment TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS plugin_idc_ai_tool_audit_tenant_created_idx ON plugin_idc_ai_tool_audit ("tenant_id", created_at DESC);
CREATE INDEX IF NOT EXISTS plugin_idc_ai_conversation_tenant_created_idx ON plugin_idc_ai_conversation ("tenant_id", created_at DESC);
