-- 002: Bounded daily reconciliation, immutable provider evidence, channel health,
-- and two-person controlled payment supplementation.

ALTER TABLE plugin_idc_payment_reconciliation_batch
    ADD COLUMN IF NOT EXISTS daily_scope_key VARCHAR(96) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS merchant_timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    ADD COLUMN IF NOT EXISTS "source_type_code" VARCHAR(32) NOT NULL DEFAULT 'provider_statement',
    ADD COLUMN IF NOT EXISTS "capability_code" VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    ADD COLUMN IF NOT EXISTS payment_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refund_count INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS result_unknown BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS "version" BIGINT NOT NULL DEFAULT 1;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_payment_reconciliation_daily
    ON plugin_idc_payment_reconciliation_batch (daily_scope_key)
    WHERE daily_scope_key <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_reconciliation_batch_list
    ON plugin_idc_payment_reconciliation_batch (business_date DESC, "channel_no", "id" DESC);

-- Purpose: Persists business records for plugin_idc_payment_reconciliation_source.
-- 用途：持久化 plugin_idc_payment_reconciliation_source 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_reconciliation_source (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "source_no" VARCHAR(64) NOT NULL,
    "batch_no" VARCHAR(64) NOT NULL,
    "record_type_code" VARCHAR(16) NOT NULL,
    "local_subject_no" VARCHAR(64) NOT NULL DEFAULT '',
    "provider_event_reference" VARCHAR(256) NOT NULL,
    "external_reference" VARCHAR(256) NOT NULL,
    "customer_ref_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "observed_status_code" VARCHAR(32) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_payment_reconciliation_source_no UNIQUE ("source_no"),
    CONSTRAINT uk_plugin_idc_payment_reconciliation_source_event UNIQUE ("batch_no", "record_type_code", "provider_event_reference"),
    CONSTRAINT fk_plugin_idc_payment_reconciliation_source_batch FOREIGN KEY ("batch_no") REFERENCES plugin_idc_payment_reconciliation_batch("batch_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_reconciliation_source_type CHECK ("record_type_code" IN ('payment', 'refund')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_source_money CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_source_digest CHECK ("customer_ref_digest" ~ '^[0-9a-f]{64}$' AND "evidence_digest" ~ '^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_reconciliation_source_match
    ON plugin_idc_payment_reconciliation_source ("batch_no", "record_type_code", "local_subject_no", "external_reference", "id");

ALTER TABLE plugin_idc_payment_reconciliation_line
    ADD COLUMN IF NOT EXISTS "record_type_code" VARCHAR(16) NOT NULL DEFAULT 'payment',
    ADD COLUMN IF NOT EXISTS "customer_ref_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    ADD COLUMN IF NOT EXISTS local_status_code VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS channel_status_code VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "version" BIGINT NOT NULL DEFAULT 1;

ALTER TABLE plugin_idc_payment_reconciliation_line
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_payment_reconciliation_difference;
ALTER TABLE plugin_idc_payment_reconciliation_line
    ADD CONSTRAINT ck_plugin_idc_payment_reconciliation_difference
    CHECK ("difference_code" IN ('none', 'local_missing', 'provider_missing', 'channel_missing', 'amount_mismatch', 'status_mismatch', 'duplicate_transaction', 'duplicate'));

-- Purpose: Persists business records for plugin_idc_payment_reconciliation_channel_health.
-- 用途：持久化 plugin_idc_payment_reconciliation_channel_health 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_reconciliation_channel_health (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "channel_no" VARCHAR(64) NOT NULL,
    "capability_code" VARCHAR(32) NOT NULL,
    "health_code" VARCHAR(32) NOT NULL,
    "latest_batch_no" VARCHAR(64) NOT NULL DEFAULT '',
    latest_business_date DATE,
    "unresolved_count" INT NOT NULL DEFAULT 0,
    "consecutive_failure_count" INT NOT NULL DEFAULT 0,
    result_unknown BOOLEAN NOT NULL DEFAULT FALSE,
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_payment_reconciliation_health_channel UNIQUE ("channel_no"),
    CONSTRAINT fk_plugin_idc_payment_reconciliation_health_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_reconciliation_health_capability CHECK ("capability_code" IN ('statement', 'authoritative_query', 'unavailable')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_health_code CHECK ("health_code" IN ('healthy', 'differences', 'failed', 'unavailable', 'never_run')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_health_count CHECK ("unresolved_count" >= 0 AND "consecutive_failure_count" >= 0 AND "version" > 0)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_reconciliation_health_list
    ON plugin_idc_payment_reconciliation_channel_health ("health_code", "channel_no", "id");

-- Purpose: Persists business records for plugin_idc_payment_reconciliation_supplement.
-- 用途：持久化 plugin_idc_payment_reconciliation_supplement 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_reconciliation_supplement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "request_no" VARCHAR(64) NOT NULL,
    "line_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "customer_ref_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "external_reference" VARCHAR(256) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    "submitter_user_id" INT NOT NULL,
    "reviewer_user_id" INT NOT NULL DEFAULT 0,
    "rejection_reason_code" VARCHAR(48) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1,
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reviewed_at TIMESTAMPTZ,
    applied_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_reconciliation_supplement_no UNIQUE ("request_no"),
    CONSTRAINT uk_plugin_idc_payment_reconciliation_supplement_line UNIQUE ("line_no"),
    CONSTRAINT fk_plugin_idc_payment_reconciliation_supplement_line FOREIGN KEY ("line_no") REFERENCES plugin_idc_payment_reconciliation_line("line_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_payment_reconciliation_supplement_attempt FOREIGN KEY ("attempt_no") REFERENCES plugin_idc_payment_attempt("attempt_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_status CHECK ("status_code" IN ('pending_review', 'approved', 'applied', 'rejected')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_money CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_actor CHECK ("submitter_user_id" > 0 AND "reviewer_user_id" >= 0 AND ("reviewer_user_id" = 0 OR "reviewer_user_id" <> "submitter_user_id")),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_digest CHECK ("customer_ref_digest" ~ '^[0-9a-f]{64}$' AND "evidence_digest" ~ '^[0-9a-f]{64}$' AND "content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_rejection CHECK (("status_code" = 'rejected' AND "rejection_reason_code" IN ('evidence_insufficient', 'ownership_mismatch', 'amount_or_currency_mismatch', 'provider_reference_conflict', 'already_confirmed')) OR ("status_code" <> 'rejected' AND "rejection_reason_code" = '')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_supplement_version CHECK ("version" > 0)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_reconciliation_supplement_review
    ON plugin_idc_payment_reconciliation_supplement ("status_code", submitted_at, "id")
    WHERE "status_code" = 'pending_review';

DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'plugin_idc_payment_reconciliation_source'
    ] LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS trg_payment_history_immutable ON %I', table_name);
        EXECUTE format('CREATE TRIGGER trg_payment_history_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION plugin_idc_payment_reject_history_mutation()', table_name);
    END LOOP;
END;
$$;
