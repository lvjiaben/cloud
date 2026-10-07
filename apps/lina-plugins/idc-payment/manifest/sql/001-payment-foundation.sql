-- 001: IDC payment channel, attempt, callback, query, refund, reconciliation and evidence foundation.
-- Money is stored as integer minor units. Channel secrets stay in core secretcap and only opaque refs are stored here.

-- Purpose: Persists business records for plugin_idc_payment_channel.
-- 用途：持久化 plugin_idc_payment_channel 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_channel (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "channel_no" VARCHAR(64) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL,
    "endpoint_origin" VARCHAR(2048) NOT NULL,
    "merchant_reference" VARCHAR(256) NOT NULL,
    "secret_ref" VARCHAR(256) NOT NULL,
    supported_modes JSONB NOT NULL DEFAULT '[]'::JSONB,
    supported_currencies JSONB NOT NULL DEFAULT '[]'::JSONB,
    capability_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'disabled',
    verified_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_channel_no UNIQUE ("channel_no"),
    CONSTRAINT ck_plugin_idc_payment_channel_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_payment_channel_provider CHECK ("provider_code" IN ('alipay', 'wechatpay', 'epay')),
    CONSTRAINT ck_plugin_idc_payment_channel_endpoint CHECK (left(lower("endpoint_origin"), 8) = 'https://' AND "endpoint_origin" = btrim("endpoint_origin")),
    CONSTRAINT ck_plugin_idc_payment_channel_json CHECK (jsonb_typeof(supported_modes) = 'array' AND jsonb_typeof(supported_currencies) = 'array' AND jsonb_typeof(capability_snapshot) = 'object'),
    CONSTRAINT ck_plugin_idc_payment_channel_status CHECK ("status_code" IN ('disabled', 'verifying', 'enabled', 'degraded', 'maintenance', 'deleted')),
    CONSTRAINT ck_plugin_idc_payment_channel_enabled CHECK ("status_code" <> 'enabled' OR verified_at IS NOT NULL),
    CONSTRAINT ck_plugin_idc_payment_channel_deleted CHECK (("status_code" = 'deleted' AND deleted_at IS NOT NULL) OR ("status_code" <> 'deleted' AND deleted_at IS NULL)),
    CONSTRAINT ck_plugin_idc_payment_channel_version CHECK ("version" > 0)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_channel_available
    ON plugin_idc_payment_channel ("tenant_id", "status_code", "provider_code", "id")
    WHERE deleted_at IS NULL;

-- Purpose: Persists business records for plugin_idc_payment_attempt.
-- 用途：持久化 plugin_idc_payment_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "attempt_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "invoice_no" VARCHAR(64) NOT NULL,
    "order_no" VARCHAR(64) NOT NULL,
    "purpose_code" VARCHAR(24) NOT NULL DEFAULT 'invoice',
    "purpose_reference" VARCHAR(64) NOT NULL DEFAULT '',
    "customer_ref_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "balance_lock_reference" VARCHAR(64) NOT NULL DEFAULT '',
    "balance_locked_minor" BIGINT NOT NULL DEFAULT 0,
    "mode_code" VARCHAR(32) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "external_trade_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'created',
    expires_at TIMESTAMPTZ NOT NULL,
    submitted_at TIMESTAMPTZ,
    confirmed_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    result_unknown BOOLEAN NOT NULL DEFAULT FALSE,
    "funding_result_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    "version" BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_payment_attempt_no UNIQUE ("attempt_no"),
    CONSTRAINT fk_plugin_idc_payment_attempt_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_attempt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_payment_attempt_money CHECK ("amount_minor" > 0 AND "balance_locked_minor" >= 0 AND "balance_locked_minor" < "amount_minor"),
    CONSTRAINT ck_plugin_idc_payment_attempt_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_attempt_digest CHECK ("customer_ref_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$' AND "funding_result_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_payment_attempt_status CHECK ("status_code" IN ('created', 'submitted', 'confirming', 'succeeded', 'failed', 'expired', 'closed', 'late_success', 'manual_review', 'disputed')),
    CONSTRAINT ck_plugin_idc_payment_attempt_purpose CHECK (("purpose_code" = 'invoice' AND "purpose_reference" = '' AND "invoice_no" <> '' AND "order_no" <> '') OR ("purpose_code" IN ('credit_statement', 'balance_recharge') AND "purpose_reference" <> '' AND "invoice_no" = '' AND "order_no" = '')),
    CONSTRAINT ck_plugin_idc_payment_attempt_unknown CHECK (NOT result_unknown OR "status_code" IN ('confirming', 'manual_review')),
    CONSTRAINT ck_plugin_idc_payment_attempt_version CHECK ("version" > 0)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_attempt_customer
    ON plugin_idc_payment_attempt ("tenant_id", "customer_ref_digest", created_at DESC, "id" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_attempt_work
    ON plugin_idc_payment_attempt ("status_code", expires_at, "id")
    WHERE "status_code" IN ('created', 'submitted', 'confirming');
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_payment_external_trade
    ON plugin_idc_payment_attempt ("channel_no", "external_trade_reference")
    WHERE "external_trade_reference" <> '';

-- Purpose: Persists business records for plugin_idc_payment_callback_receipt.
-- 用途：持久化 plugin_idc_payment_callback_receipt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_callback_receipt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "receipt_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL DEFAULT '',
    "provider_event_reference" VARCHAR(256) NOT NULL,
    "payload_digest" CHAR(64) NOT NULL,
    signature_valid BOOLEAN NOT NULL,
    merchant_match BOOLEAN NOT NULL,
    amount_match BOOLEAN NOT NULL,
    currency_match BOOLEAN NOT NULL,
    "processing_code" VARCHAR(32) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_callback_no UNIQUE ("receipt_no"),
    CONSTRAINT uk_plugin_idc_payment_callback_event UNIQUE ("channel_no", "provider_event_reference", "payload_digest"),
    CONSTRAINT fk_plugin_idc_payment_callback_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_callback_digest CHECK ("payload_digest" ~ '^[0-9a-f]{64}$' AND "evidence_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_payment_callback_processing CHECK ("processing_code" IN ('received', 'rejected', 'duplicate', 'queued', 'applied', 'late_success', 'manual_review'))
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_callback_work
    ON plugin_idc_payment_callback_receipt ("processing_code", received_at, "id")
    WHERE "processing_code" IN ('received', 'queued');

-- Purpose: Persists business records for plugin_idc_payment_work_item.
-- 用途：持久化 plugin_idc_payment_work_item 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_work_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "work_no" VARCHAR(64) NOT NULL,
    "work_type" VARCHAR(32) NOT NULL,
    "subject_no" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "max_attempts" INT NOT NULL DEFAULT 8,
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    lease_expires_at TIMESTAMPTZ,
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_work_no UNIQUE ("work_no"),
    CONSTRAINT uk_plugin_idc_payment_work_subject UNIQUE ("work_type", "subject_no"),
    CONSTRAINT ck_plugin_idc_payment_work_type CHECK ("work_type" IN ('confirm_payment', 'release_balance', 'query_payment', 'refund', 'reconcile', 'manual_review')),
    CONSTRAINT ck_plugin_idc_payment_work_status CHECK ("status_code" IN ('pending', 'leased', 'succeeded', 'failed', 'manual_review')),
    CONSTRAINT ck_plugin_idc_payment_work_attempt CHECK ("attempt_count" >= 0 AND "max_attempts" BETWEEN 1 AND 100 AND "attempt_count" <= "max_attempts"),
    CONSTRAINT ck_plugin_idc_payment_work_lease CHECK (("status_code" = 'leased' AND lease_expires_at IS NOT NULL) OR ("status_code" <> 'leased' AND lease_expires_at IS NULL)),
    CONSTRAINT ck_plugin_idc_payment_work_digest CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$' AND "lease_token_digest" ~ '^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_work_claim
    ON plugin_idc_payment_work_item (available_at, "id")
    WHERE "status_code" = 'pending';

-- Purpose: Persists business records for plugin_idc_payment_query_observation.
-- 用途：持久化 plugin_idc_payment_query_observation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_query_observation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "observation_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL,
    "query_reference" VARCHAR(128) NOT NULL,
    "authority_code" VARCHAR(24) NOT NULL,
    "observed_status_code" VARCHAR(32) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_payment_query_no UNIQUE ("observation_no"),
    CONSTRAINT uk_plugin_idc_payment_query_reference UNIQUE ("attempt_no", "query_reference"),
    CONSTRAINT fk_plugin_idc_payment_query_attempt FOREIGN KEY ("attempt_no") REFERENCES plugin_idc_payment_attempt("attempt_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_query_authority CHECK ("authority_code" IN ('authoritative', 'advisory')),
    CONSTRAINT ck_plugin_idc_payment_query_digest CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$')
);

-- Purpose: Persists business records for plugin_idc_payment_refund_attempt.
-- 用途：持久化 plugin_idc_payment_refund_attempt 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_refund_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "refund_attempt_no" VARCHAR(64) NOT NULL,
    "refund_request_no" VARCHAR(64) NOT NULL,
    "payment_attempt_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "external_refund_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "capability_code" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'pending',
    result_unknown BOOLEAN NOT NULL DEFAULT FALSE,
    "result_code" VARCHAR(48) NOT NULL DEFAULT '',
    "result_evidence_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    "manual_evidence_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uk_plugin_idc_payment_refund_attempt_no UNIQUE ("refund_attempt_no"),
    CONSTRAINT uk_plugin_idc_payment_refund_request UNIQUE ("refund_request_no", "payment_attempt_no"),
    CONSTRAINT fk_plugin_idc_payment_refund_payment FOREIGN KEY ("payment_attempt_no") REFERENCES plugin_idc_payment_attempt("attempt_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_payment_refund_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_refund_money CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_refund_capability CHECK ("capability_code" IN ('automatic', 'manual')),
    CONSTRAINT ck_plugin_idc_payment_refund_status CHECK ("status_code" IN ('pending', 'submitted', 'succeeded', 'failed', 'result_unknown', 'manual_required', 'manual_confirmed', 'disputed')),
    CONSTRAINT ck_plugin_idc_payment_refund_unknown CHECK (result_unknown = ("status_code" = 'result_unknown')),
    CONSTRAINT ck_plugin_idc_payment_refund_digest CHECK ("request_digest" ~ '^[0-9a-f]{64}$' AND "result_evidence_digest" ~ '^[0-9a-f]{64}$' AND "manual_evidence_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_payment_refund_version CHECK ("version" > 0)
);
ALTER TABLE plugin_idc_payment_refund_attempt
    ADD COLUMN IF NOT EXISTS "result_code" VARCHAR(48) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "result_evidence_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_refund_work
    ON plugin_idc_payment_refund_attempt ("status_code", requested_at, "id")
    WHERE "status_code" IN ('pending', 'submitted', 'result_unknown', 'manual_required');
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_payment_external_refund
    ON plugin_idc_payment_refund_attempt ("channel_no", "external_refund_reference")
    WHERE "external_refund_reference" <> '';

-- Purpose: Persists business records for plugin_idc_payment_reconciliation_batch.
-- 用途：持久化 plugin_idc_payment_reconciliation_batch 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_reconciliation_batch (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "batch_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    business_date DATE NOT NULL,
    "source_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "total_count" INT NOT NULL DEFAULT 0,
    "difference_count" INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_reconciliation_batch_no UNIQUE ("batch_no"),
    CONSTRAINT uk_plugin_idc_payment_reconciliation_scope UNIQUE ("channel_no", business_date, "source_digest"),
    CONSTRAINT fk_plugin_idc_payment_reconciliation_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_reconciliation_status CHECK ("status_code" IN ('pending', 'processing', 'matched', 'differences', 'failed')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_count CHECK ("total_count" >= 0 AND "difference_count" >= 0 AND "difference_count" <= "total_count"),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_digest CHECK ("source_digest" ~ '^[0-9a-f]{64}$')
);

-- Purpose: Persists business records for plugin_idc_payment_reconciliation_line.
-- 用途：持久化 plugin_idc_payment_reconciliation_line 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_reconciliation_line (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "line_no" VARCHAR(64) NOT NULL,
    "batch_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL DEFAULT '',
    "refund_attempt_no" VARCHAR(64) NOT NULL DEFAULT '',
    "external_reference" VARCHAR(256) NOT NULL,
    "difference_code" VARCHAR(32) NOT NULL,
    "local_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "channel_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "currency_code" CHAR(3) NOT NULL,
    "resolution_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "evidence_digest" CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_reconciliation_line_no UNIQUE ("line_no"),
    CONSTRAINT uk_plugin_idc_payment_reconciliation_external UNIQUE ("batch_no", "external_reference"),
    CONSTRAINT fk_plugin_idc_payment_reconciliation_line_batch FOREIGN KEY ("batch_no") REFERENCES plugin_idc_payment_reconciliation_batch("batch_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_reconciliation_difference CHECK ("difference_code" IN ('none', 'local_missing', 'channel_missing', 'amount_mismatch', 'status_mismatch', 'duplicate')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_resolution CHECK ("resolution_code" IN ('pending', 'accepted', 'supplemented', 'refunded', 'ignored')),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_line_money CHECK ("local_amount_minor" >= 0 AND "channel_amount_minor" >= 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_reconciliation_line_digest CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_reconciliation_pending
    ON plugin_idc_payment_reconciliation_line ("batch_no", "resolution_code", "difference_code", "id")
    WHERE "resolution_code" = 'pending';

-- Purpose: Persists business records for plugin_idc_payment_exception_fund.
-- 用途：持久化 plugin_idc_payment_exception_fund 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_exception_fund (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "exception_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "reason_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "evidence_digest" CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    CONSTRAINT uk_plugin_idc_payment_exception_no UNIQUE ("exception_no"),
    CONSTRAINT uk_plugin_idc_payment_exception_attempt UNIQUE ("attempt_no"),
    CONSTRAINT fk_plugin_idc_payment_exception_attempt FOREIGN KEY ("attempt_no") REFERENCES plugin_idc_payment_attempt("attempt_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_payment_exception_channel FOREIGN KEY ("channel_no") REFERENCES plugin_idc_payment_channel("channel_no") ON DELETE RESTRICT,
    CONSTRAINT ck_plugin_idc_payment_exception_money CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_plugin_idc_payment_exception_reason CHECK ("reason_code" IN ('late_success', 'unmatched_success', 'reversal', 'chargeback', 'settlement_reversal')),
    CONSTRAINT ck_plugin_idc_payment_exception_status CHECK ("status_code" IN ('pending', 'allocated', 'refunded', 'disputed', 'closed')),
    CONSTRAINT ck_plugin_idc_payment_exception_digest CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$')
);

-- Purpose: Persists business records for plugin_idc_payment_evidence.
-- 用途：持久化 plugin_idc_payment_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_evidence (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "evidence_no" VARCHAR(64) NOT NULL,
    "subject_type" VARCHAR(32) NOT NULL,
    "subject_no" VARCHAR(64) NOT NULL,
    "evidence_type" VARCHAR(32) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "summary_code" VARCHAR(64) NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_plugin_idc_payment_evidence_no UNIQUE ("evidence_no"),
    CONSTRAINT uk_plugin_idc_payment_evidence_subject UNIQUE ("subject_type", "subject_no", "evidence_type", "evidence_digest"),
    CONSTRAINT ck_plugin_idc_payment_evidence_subject CHECK ("subject_type" IN ('payment_attempt', 'callback', 'query', 'refund_attempt', 'reconciliation', 'exception_fund')),
    CONSTRAINT ck_plugin_idc_payment_evidence_digest CHECK ("evidence_digest" ~ '^[0-9a-f]{64}$')
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_payment_evidence_subject
    ON plugin_idc_payment_evidence ("subject_type", "subject_no", observed_at DESC, "id" DESC);

CREATE OR REPLACE FUNCTION plugin_idc_payment_reject_history_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'payment history is append-only';
END;
$$;

DO $$
DECLARE table_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'plugin_idc_payment_callback_receipt',
        'plugin_idc_payment_query_observation',
        'plugin_idc_payment_reconciliation_line',
        'plugin_idc_payment_evidence'
    ] LOOP
        EXECUTE format('DROP TRIGGER IF EXISTS trg_payment_history_immutable ON %I', table_name);
        EXECUTE format('CREATE TRIGGER trg_payment_history_immutable BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION plugin_idc_payment_reject_history_mutation()', table_name);
    END LOOP;
END;
$$;
