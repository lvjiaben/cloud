-- Downstream channel tokens, sanitized publication, idempotent requests and signed event evidence.
-- Purpose: Stores hashed downstream channel access tokens and bounded access policies.
-- 用途：保存下游渠道访问令牌哈希和受限访问策略。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_token (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "token_hash" VARCHAR(128) NOT NULL,
    scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
    source_cidrs JSONB NOT NULL DEFAULT '[]'::jsonb,
    "rate_limit" INT NOT NULL DEFAULT 60,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("channel_id", "token_hash")
);
-- Purpose: Stores product versions and prices published to each downstream channel.
-- 用途：保存向各下游渠道发布的产品版本和价格。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_publication (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "product_version_id" BIGINT NOT NULL,
    "currency" VARCHAR(16) NOT NULL,
    "price" NUMERIC(24,8) NOT NULL DEFAULT 0,
    "stock" INT NOT NULL DEFAULT 0,
    "status" VARCHAR(20) NOT NULL DEFAULT 'published',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("channel_id", "product_version_id", "currency")
);
-- Purpose: Stores idempotent downstream channel requests and sanitized results.
-- 用途：保存下游渠道幂等请求及脱敏结果。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_request (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "request_id" VARCHAR(120) NOT NULL,
    "operation" VARCHAR(80) NOT NULL,
    "request_hash" VARCHAR(128) NOT NULL,
    response JSONB NOT NULL DEFAULT '{}'::jsonb,
    "status" VARCHAR(30) NOT NULL DEFAULT 'accepted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("channel_id", "request_id")
);
-- Purpose: Stores channel order attribution and settlement progress.
-- 用途：保存渠道订单归属和结算进度。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_order (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "external_customer_id" VARCHAR(160) NOT NULL DEFAULT '',
    "ownership_mode" VARCHAR(30) NOT NULL DEFAULT 'channel_owned',
    "product_version_id" BIGINT NOT NULL,
    "currency" VARCHAR(16) NOT NULL,
    "amount" NUMERIC(24,8) NOT NULL DEFAULT 0,
    "status" VARCHAR(30) NOT NULL DEFAULT 'pending_identity',
    financially_settled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
-- Purpose: Stores signed channel webhook events and bounded delivery state.
-- 用途：保存已签名的渠道回调事件和受限投递状态。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_webhook (
    "id" BIGSERIAL PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "event_id" VARCHAR(120) NOT NULL,
    "event_version" VARCHAR(20) NOT NULL DEFAULT 'v1',
    "event_type" VARCHAR(80) NOT NULL DEFAULT 'unknown',
    "subject_ref" VARCHAR(160) NOT NULL DEFAULT '',
    "sequence_no" BIGINT NOT NULL DEFAULT 0,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    "payload_text" TEXT NOT NULL DEFAULT '{}',
    "payload_hash" VARCHAR(64) NOT NULL DEFAULT '',
    "signature" VARCHAR(128) NOT NULL DEFAULT '',
    "status" VARCHAR(30) NOT NULL DEFAULT 'pending',
    "attempts" INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    "lease_token" VARCHAR(64) NOT NULL DEFAULT '',
    lease_expires_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    "last_error" VARCHAR(200) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("channel_id", "event_id")
);
CREATE INDEX IF NOT EXISTS idx_idc_channel_token_active ON plugin_idc_channel_token("channel_id", revoked_at, expires_at);
CREATE INDEX IF NOT EXISTS idx_idc_channel_request_operation ON plugin_idc_channel_request("channel_id", "operation", created_at);
