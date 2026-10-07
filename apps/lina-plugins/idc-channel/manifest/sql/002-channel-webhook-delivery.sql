-- Versioned webhook delivery queue: immutable event identity plus leased,
-- bounded retries. Every statement is repeatable for upgrade/reinstall.
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "event_type" VARCHAR(80) NOT NULL DEFAULT 'unknown';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "subject_ref" VARCHAR(160) NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "sequence_no" BIGINT NOT NULL DEFAULT 0;
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "payload_hash" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "payload_text" TEXT NOT NULL DEFAULT '{}';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "lease_token" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS "last_error" VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE IF EXISTS plugin_idc_channel_webhook
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Preserve the old structured payload for events created before the raw
-- signed bytes column existed; new events always write both columns.
UPDATE plugin_idc_channel_webhook
SET "payload_text" = payload::text
WHERE "payload_text" = '{}' AND payload <> '{}'::jsonb;

CREATE INDEX IF NOT EXISTS idx_idc_channel_webhook_due
    ON plugin_idc_channel_webhook("status", next_attempt_at, lease_expires_at, "id");
CREATE INDEX IF NOT EXISTS idx_idc_channel_webhook_subject
    ON plugin_idc_channel_webhook("channel_id", "subject_ref", "sequence_no");
