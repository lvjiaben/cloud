-- 062: Currency-safe marketing performance facts and staff-reviewed abuse cases.

ALTER TABLE plugin_idc_core_marketing_campaign
    ADD COLUMN IF NOT EXISTS "is_newcomer_exclusive" BOOLEAN NOT NULL DEFAULT FALSE;

-- Purpose: Stores Marketing Campaign Newcomer Request records.
-- 用途：保存营销活动新人申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_campaign_newcomer_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(64) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "enabled" BOOLEAN NOT NULL,
    "expected_version" BIGINT NOT NULL,
    "resulting_version" BIGINT NOT NULL,
    "resulting_updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "request_digest" CHAR(64) NOT NULL,
    "updated_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_campaign_newcomer_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_campaign_newcomer_request_versions CHECK (
        "expected_version" > 0 AND "resulting_version" = "expected_version" + 1
    ),
    CONSTRAINT ck_idc_marketing_campaign_newcomer_request_digest CHECK ("request_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_marketing_campaign_newcomer_request_actor CHECK ("updated_by" > 0),
    CONSTRAINT uk_idc_marketing_campaign_newcomer_request_no UNIQUE ("request_no"),
    CONSTRAINT fk_idc_marketing_campaign_newcomer_request_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_marketing_campaign_newcomer_request
    ADD COLUMN IF NOT EXISTS "resulting_updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- Purpose: Append-only authoritative marketing performance facts; currencies are never converted.
-- 用途：保存营销效果事实相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_effect_fact (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "fact_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40),
    "event_type_code" VARCHAR(16) NOT NULL,
    "currency_code" CHAR(3),
    "amount_minor" BIGINT NOT NULL DEFAULT 0,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_effect_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_effect_event CHECK (
        "event_type_code" IN (
            'exposure', 'claim', 'use', 'order', 'receipt', 'receipt_reversal',
            'refund', 'refund_reversal', 'cost', 'cost_reversal'
        )
    ),
    CONSTRAINT ck_idc_marketing_effect_currency CHECK (
        ("event_type_code" IN ('exposure', 'claim', 'use', 'order') AND
            ("currency_code" IS NULL OR "currency_code" ~ '^[A-Z]{3}$') AND "amount_minor" = 0) OR
        ("event_type_code" IN ('receipt', 'receipt_reversal', 'refund', 'refund_reversal', 'cost', 'cost_reversal') AND
            "currency_code" ~ '^[A-Z]{3}$' AND "amount_minor" > 0)
    ),
    CONSTRAINT ck_idc_marketing_effect_source CHECK (
        "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$' AND
        length(btrim("source_reference")) BETWEEN 1 AND 128 AND
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_marketing_effect_no UNIQUE ("fact_no"),
    CONSTRAINT uk_idc_marketing_effect_source UNIQUE (
        "source_type_code", "source_reference", "event_type_code", "campaign_no"
    ),
    CONSTRAINT fk_idc_marketing_effect_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_effect_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_effect_report
    ON plugin_idc_core_marketing_effect_fact (
        "tenant_id", "occurred_at", "campaign_no", "currency_code", "event_type_code"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_effect_customer
    ON plugin_idc_core_marketing_effect_fact (
        "tenant_id", "customer_no", "occurred_at", "event_type_code"
    ) WHERE "customer_no" IS NOT NULL;

-- Purpose: Redacted marketing abuse signals and permanent staff review history.
-- 用途：保存营销滥用案件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_abuse_case (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "case_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40),
    "customer_no" VARCHAR(40),
    "signal_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    "related_account_count" INT NOT NULL DEFAULT 0,
    "evidence_summary_code" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "window_started_at" TIMESTAMPTZ NOT NULL,
    "window_ended_at" TIMESTAMPTZ NOT NULL,
    "observed_count" INT NOT NULL,
    "blocked_until" TIMESTAMPTZ,
    "review_reason" VARCHAR(500),
    "reviewed_by" INT,
    "reviewed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_abuse_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_abuse_signal CHECK (
        "signal_code" IN ('duplicate_newcomer_claim', 'bulk_registration', 'coupon_bruteforce', 'self_referral', 'refund_arbitrage')
    ),
    CONSTRAINT ck_idc_marketing_abuse_status CHECK (
        "status_code" IN ('pending_review', 'confirmed_abuse', 'false_positive')
    ),
    CONSTRAINT ck_idc_marketing_abuse_counts CHECK (
        "related_account_count" >= 0 AND "observed_count" > 0
    ),
    CONSTRAINT ck_idc_marketing_abuse_evidence CHECK (
        "evidence_summary_code" ~ '^[a-z][a-z0-9_.-]{0,63}$' AND
        "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_abuse_window CHECK ("window_ended_at" >= "window_started_at"),
    CONSTRAINT ck_idc_marketing_abuse_review CHECK (
        ("status_code" = 'pending_review' AND "review_reason" IS NULL AND "reviewed_by" IS NULL AND "reviewed_at" IS NULL) OR
        ("status_code" <> 'pending_review' AND length(btrim("review_reason")) BETWEEN 2 AND 500 AND "reviewed_by" > 0 AND "reviewed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_marketing_abuse_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_abuse_no UNIQUE ("case_no"),
    CONSTRAINT fk_idc_marketing_abuse_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_abuse_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_marketing_abuse_case
    ALTER COLUMN "campaign_no" DROP NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_abuse_evidence
    ON plugin_idc_core_marketing_abuse_case (
        COALESCE("campaign_no", ''), "signal_code", "evidence_digest"
    );

CREATE INDEX IF NOT EXISTS idx_idc_marketing_abuse_queue
    ON plugin_idc_core_marketing_abuse_case (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Stores Marketing Registration Signal records.
-- 用途：保存营销注册信号相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_registration_signal (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "device_digest" CHAR(64) NOT NULL,
    "ip_digest" CHAR(64) NOT NULL,
    "registered_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_registration_signal_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_registration_signal_digests CHECK (
        "device_digest" ~ '^[0-9a-f]{64}$' AND "ip_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_marketing_registration_signal_customer UNIQUE ("customer_no"),
    CONSTRAINT fk_idc_marketing_registration_signal_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_registration_signal_device
    ON plugin_idc_core_marketing_registration_signal ("tenant_id", "device_digest", "registered_at", "customer_no");
CREATE INDEX IF NOT EXISTS idx_idc_marketing_registration_signal_ip
    ON plugin_idc_core_marketing_registration_signal ("tenant_id", "ip_digest", "registered_at", "customer_no");

-- Purpose: Privacy-safe consecutive invalid coupon counters and temporary source blocks; raw device and IP values are never stored.
-- 用途：保存营销优惠券来源状态相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_coupon_source_state (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "source_kind_code" VARCHAR(16) NOT NULL,
    "source_digest" CHAR(64) NOT NULL,
    "window_started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "failure_count" INT NOT NULL DEFAULT 0,
    "blocked_until" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_coupon_source_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_coupon_source_kind CHECK ("source_kind_code" IN ('device', 'ip')),
    CONSTRAINT ck_idc_marketing_coupon_source_digest CHECK ("source_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_marketing_coupon_source_count CHECK ("failure_count" BETWEEN 0 AND 10),
    CONSTRAINT uk_idc_marketing_coupon_source UNIQUE ("source_kind_code", "source_digest")
);

-- Purpose: Rolling thirty-minute public campaign exposure fence keyed only by a purpose-separated device digest, campaign, and currency.
-- 用途：保存营销公开曝光状态相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_public_exposure_state (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "device_digest" CHAR(64) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "window_started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_public_exposure_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_public_exposure_device CHECK ("device_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_marketing_public_exposure_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT uk_idc_marketing_public_exposure_source UNIQUE (
        "device_digest", "campaign_no", "currency_code"
    ),
    CONSTRAINT fk_idc_marketing_public_exposure_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_public_exposure_window
    ON plugin_idc_core_marketing_public_exposure_state (
        "tenant_id", "window_started_at", "campaign_no", "currency_code"
    );

CREATE INDEX IF NOT EXISTS idx_idc_marketing_coupon_source_blocked
    ON plugin_idc_core_marketing_coupon_source_state ("tenant_id", "blocked_until", "id")
    WHERE "blocked_until" IS NOT NULL;

-- Purpose: Audited coupon generation batches; only request metadata is retained and complete coupon codes are never stored.
-- 用途：保存营销优惠券批次申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_coupon_batch_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "batch_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(128) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "benefit_no" VARCHAR(40) NOT NULL,
    "code_length" INT NOT NULL,
    "generated_count" INT NOT NULL,
    "generated_by" INT NOT NULL,
    "generated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_coupon_batch_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_coupon_batch_request CHECK (
        length(btrim("request_no")) BETWEEN 8 AND 128 AND
        "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_coupon_batch_shape CHECK (
        "code_length" BETWEEN 8 AND 24 AND "generated_count" BETWEEN 1 AND 1000 AND "generated_by" > 0
    ),
    CONSTRAINT uk_idc_marketing_coupon_batch_no UNIQUE ("batch_no"),
    CONSTRAINT uk_idc_marketing_coupon_batch_request UNIQUE ("request_no"),
    CONSTRAINT fk_idc_marketing_coupon_batch_rule FOREIGN KEY ("campaign_no", "benefit_no")
        REFERENCES plugin_idc_core_marketing_benefit_rule ("campaign_no", "benefit_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_marketing_promotion_asset
    ADD COLUMN IF NOT EXISTS "coupon_batch_no" VARCHAR(40);

ALTER TABLE plugin_idc_core_marketing_promotion_asset
    DROP CONSTRAINT IF EXISTS fk_idc_marketing_asset_coupon_batch;
ALTER TABLE plugin_idc_core_marketing_promotion_asset
    ADD CONSTRAINT fk_idc_marketing_asset_coupon_batch FOREIGN KEY ("coupon_batch_no")
        REFERENCES plugin_idc_core_marketing_coupon_batch_request ("batch_no") ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_coupon_batch
    ON plugin_idc_core_marketing_promotion_asset ("tenant_id", "coupon_batch_no", "id")
    WHERE "coupon_batch_no" IS NOT NULL;

-- Purpose: Campaign-scoped future participation restrictions created only by confirmed abuse.
-- 用途：保存营销活动限制相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_campaign_restriction (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "campaign_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "case_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "released_at" TIMESTAMPTZ,
    CONSTRAINT ck_idc_marketing_restriction_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_restriction_status CHECK (
        ("status_code" = 'active' AND "released_at" IS NULL) OR
        ("status_code" = 'released' AND "released_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_marketing_restriction_case UNIQUE ("case_no"),
    CONSTRAINT fk_idc_marketing_restriction_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_restriction_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_restriction_case FOREIGN KEY ("case_no")
        REFERENCES plugin_idc_core_marketing_abuse_case ("case_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_restriction_active
    ON plugin_idc_core_marketing_campaign_restriction ("campaign_no", "customer_no")
    WHERE "status_code" = 'active';

-- Purpose: Order-time cost input frozen per order item; missing costs require an audited historical correction.
-- 用途：保存交易订单成本快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_cost_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "order_item_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "binding_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "server_quantity" INT NOT NULL,
    "purchase_cycles" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL,
    "cost_snapshot_no" VARCHAR(40),
    "unit_cost_minor" BIGINT,
    "total_cost_minor" BIGINT,
    "source_code" VARCHAR(24),
    "source_observed_at" TIMESTAMPTZ,
    "correction_request_no" VARCHAR(64),
    "correction_request_digest" CHAR(64),
    "correction_reason" VARCHAR(500),
    "corrected_by" INT,
    "corrected_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_order_cost_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_order_cost_dimensions CHECK (
        "currency_code" ~ '^[A-Z]{3}$' AND
        "period_unit_code" IN ('once', 'hour', 'day', 'month', 'year') AND
        "period_count" BETWEEN 1 AND 87600 AND "server_quantity" BETWEEN 1 AND 1000 AND
        "purchase_cycles" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_order_cost_state CHECK (
        ("status_code" = 'missing' AND "cost_snapshot_no" IS NULL AND "unit_cost_minor" IS NULL AND
            "total_cost_minor" IS NULL AND "source_code" IS NULL AND "source_observed_at" IS NULL AND
            "correction_request_no" IS NULL AND "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND
            "corrected_by" IS NULL AND "corrected_at" IS NULL) OR
        ("status_code" = 'frozen' AND "cost_snapshot_no" IS NOT NULL AND "unit_cost_minor" >= 0 AND
            "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "server_quantity" * "purchase_cycles" AND
            "source_code" IN ('manual', 'provider') AND "source_observed_at" IS NOT NULL AND
            "correction_request_no" IS NULL AND "correction_request_digest" IS NULL AND "correction_reason" IS NULL AND
            "corrected_by" IS NULL AND "corrected_at" IS NULL) OR
        ("status_code" = 'corrected' AND "cost_snapshot_no" IS NULL AND "unit_cost_minor" >= 0 AND
            "total_cost_minor"::NUMERIC = "unit_cost_minor"::NUMERIC * "server_quantity" * "purchase_cycles" AND
            "source_code" = 'historical_correction' AND "source_observed_at" IS NOT NULL AND
            length(btrim("correction_request_no")) BETWEEN 8 AND 64 AND
            "correction_request_digest" ~ '^[0-9a-f]{64}$' AND
            length(btrim("correction_reason")) BETWEEN 2 AND 500 AND "corrected_by" > 0 AND "corrected_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_order_cost_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_order_cost_item UNIQUE ("order_item_no"),
    CONSTRAINT fk_idc_order_cost_item FOREIGN KEY ("order_no", "order_item_no")
        REFERENCES plugin_idc_core_commerce_order_item ("order_no", "order_item_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_order_cost_source FOREIGN KEY ("cost_snapshot_no")
        REFERENCES plugin_idc_core_catalog_cost_snapshot ("cost_snapshot_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_commerce_order_cost_snapshot
    ALTER COLUMN "source_code" TYPE VARCHAR(24);

CREATE INDEX IF NOT EXISTS idx_idc_order_cost_order
    ON plugin_idc_core_commerce_order_cost_snapshot ("tenant_id", "order_no", "status_code", "id");

-- Purpose: Append-only evidence-backed upstream cost refunds; original order costs remain unchanged.
-- 用途：保存交易订单成本冲回相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_cost_reversal (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reversal_no" VARCHAR(40) NOT NULL,
    "order_item_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_order_cost_reversal_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_order_cost_reversal_amount CHECK ("amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_order_cost_reversal_source CHECK (
        "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$' AND
        length(btrim("source_reference")) BETWEEN 1 AND 128 AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_order_cost_reversal_no UNIQUE ("reversal_no"),
    CONSTRAINT uk_idc_order_cost_reversal_source UNIQUE ("source_type_code", "source_reference"),
    CONSTRAINT fk_idc_order_cost_reversal_item FOREIGN KEY ("order_item_no")
        REFERENCES plugin_idc_core_commerce_order_cost_snapshot ("order_item_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_order_cost_reversal_item
    ON plugin_idc_core_commerce_order_cost_reversal ("tenant_id", "order_item_no", "occurred_at", "id");

CREATE OR REPLACE FUNCTION plugin_idc_core_capture_order_cost()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    selected_cost plugin_idc_core_catalog_cost_snapshot%ROWTYPE;
    computed_total NUMERIC;
BEGIN
    SELECT * INTO selected_cost
    FROM plugin_idc_core_catalog_cost_snapshot cost
    WHERE cost."tenant_id" = 0
      AND cost."binding_no" = NEW."initial_binding_no"
      AND cost."currency_code" = NEW."currency_code"
      AND cost."period_unit_code" = NEW."period_unit_code"
      AND cost."period_count" = NEW."period_count"
      AND cost."observed_at" <= NEW."created_at"
      AND (cost."valid_until" IS NULL OR cost."valid_until" > NEW."created_at")
    ORDER BY cost."observed_at" DESC, cost."id" DESC
    LIMIT 1;

    IF selected_cost."id" IS NULL THEN
        INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (
            "tenant_id", "order_item_no", "order_no", "binding_no", "currency_code", "period_unit_code",
            "period_count", "server_quantity", "purchase_cycles", "status_code", "created_at", "updated_at"
        ) VALUES (
            0, NEW."order_item_no", NEW."order_no", NEW."initial_binding_no", NEW."currency_code",
            NEW."period_unit_code", NEW."period_count", NEW."server_quantity", NEW."purchase_cycles",
            'missing', NEW."created_at", NEW."created_at"
        );
        RETURN NEW;
    END IF;

    computed_total := selected_cost."amount_minor"::NUMERIC * NEW."server_quantity" * NEW."purchase_cycles";
    IF computed_total > 9223372036854775807 THEN
        RAISE EXCEPTION 'order cost snapshot exceeds BIGINT range';
    END IF;
    INSERT INTO plugin_idc_core_commerce_order_cost_snapshot (
        "tenant_id", "order_item_no", "order_no", "binding_no", "currency_code", "period_unit_code",
        "period_count", "server_quantity", "purchase_cycles", "status_code", "cost_snapshot_no",
        "unit_cost_minor", "total_cost_minor", "source_code", "source_observed_at", "created_at", "updated_at"
    ) VALUES (
        0, NEW."order_item_no", NEW."order_no", NEW."initial_binding_no", NEW."currency_code",
        NEW."period_unit_code", NEW."period_count", NEW."server_quantity", NEW."purchase_cycles",
        'frozen', selected_cost."cost_snapshot_no", selected_cost."amount_minor", computed_total::BIGINT,
        selected_cost."source_code", selected_cost."observed_at", NEW."created_at", NEW."created_at"
    );
    RETURN NEW;
END;
$$;

DO $legacy_order_cost_trigger_name$
DECLARE
    legacy_trigger pg_trigger%ROWTYPE;
    current_trigger pg_trigger%ROWTYPE;
BEGIN
    SELECT * INTO legacy_trigger
      FROM pg_trigger
     WHERE tgrelid = 'plugin_idc_core_commerce_order_item'::REGCLASS
       AND tgname = 'trg_linapro_idc_core_capture_order_cost';
    IF FOUND THEN
        IF legacy_trigger.tgisinternal
           OR legacy_trigger.tgconstraint <> 0
           OR legacy_trigger.tgfoid <> 'plugin_idc_core_capture_order_cost()'::REGPROCEDURE THEN
            RAISE EXCEPTION 'legacy order cost trigger catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;

        SELECT * INTO current_trigger
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_commerce_order_item'::REGCLASS
           AND tgname = 'trg_idc_core_capture_order_cost';
        IF FOUND AND (
            current_trigger.tgisinternal
            OR current_trigger.tgconstraint <> 0
            OR current_trigger.tgfoid <> 'plugin_idc_core_capture_order_cost()'::REGPROCEDURE
            OR ROW(current_trigger.tgtype, current_trigger.tgnargs,
                   current_trigger.tgargs, current_trigger.tgattr::TEXT,
                   current_trigger.tgqual::TEXT, current_trigger.tgoldtable,
                   current_trigger.tgnewtable)
               IS DISTINCT FROM
               ROW(legacy_trigger.tgtype, legacy_trigger.tgnargs,
                   legacy_trigger.tgargs, legacy_trigger.tgattr::TEXT,
                   legacy_trigger.tgqual::TEXT, legacy_trigger.tgoldtable,
                   legacy_trigger.tgnewtable)
        ) THEN
            RAISE EXCEPTION 'legacy and current order cost triggers are not equivalent'
                USING ERRCODE = '23514';
        END IF;

        DROP TRIGGER trg_linapro_idc_core_capture_order_cost
            ON plugin_idc_core_commerce_order_item;
    END IF;
END;
$legacy_order_cost_trigger_name$;

DROP TRIGGER IF EXISTS trg_idc_core_capture_order_cost
    ON plugin_idc_core_commerce_order_item;
CREATE TRIGGER trg_idc_core_capture_order_cost
AFTER INSERT ON plugin_idc_core_commerce_order_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_capture_order_cost();

COMMENT ON TABLE plugin_idc_core_marketing_effect_fact
    IS 'Append-only authoritative marketing performance facts; currencies are never converted.';
COMMENT ON TABLE plugin_idc_core_marketing_abuse_case
    IS 'Redacted marketing abuse signals and permanent staff review history.';
COMMENT ON TABLE plugin_idc_core_marketing_coupon_source_state
    IS 'Privacy-safe consecutive invalid coupon counters and temporary source blocks; raw device and IP values are never stored.';
COMMENT ON TABLE plugin_idc_core_marketing_public_exposure_state
    IS 'Rolling thirty-minute public campaign exposure fence keyed only by a purpose-separated device digest, campaign, and currency.';
COMMENT ON TABLE plugin_idc_core_marketing_coupon_batch_request
    IS 'Audited coupon generation batches; only request metadata is retained and complete coupon codes are never stored.';
COMMENT ON TABLE plugin_idc_core_marketing_campaign_restriction
    IS 'Campaign-scoped future participation restrictions created only by confirmed abuse.';
COMMENT ON TABLE plugin_idc_core_commerce_order_cost_snapshot
    IS 'Order-time cost input frozen per order item; missing costs require an audited historical correction.';
COMMENT ON TABLE plugin_idc_core_commerce_order_cost_reversal
    IS 'Append-only evidence-backed upstream cost refunds; original order costs remain unchanged.';
