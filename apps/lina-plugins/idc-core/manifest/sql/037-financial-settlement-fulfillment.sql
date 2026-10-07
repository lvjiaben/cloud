-- 037: Materialize financially settled new purchases into core-owned service
-- and business-operation intents before any Provider task may be submitted.

ALTER TABLE plugin_idc_core_business_operation
    ADD COLUMN IF NOT EXISTS "source_fulfillment_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "settlement_no" VARCHAR(40);

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_execution;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_execution CHECK (
        "execution_mode_code" IN (
            'unresolved', 'provider_confirmed', 'local_entitlement', 'manual_review'
        )
        AND ("execution_mode_code" <> 'local_entitlement'
            OR "operation_type_code" = 'renewal')
        AND ("execution_mode_code" <> 'unresolved'
            OR "operation_type_code" = 'new_purchase')
        AND (
            ("execution_mode_code" = 'unresolved'
                AND "channel_no" IS NULL
                AND "provider_task_no" IS NULL
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL)
            OR ("execution_mode_code" = 'manual_review'
                AND "channel_no" IS NULL
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL)
            OR ("execution_mode_code" IN ('provider_confirmed', 'local_entitlement')
                AND "channel_no" IS NOT NULL
                AND "capability_evidence_no" IS NOT NULL
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$')
        )
        AND ("provider_task_no" IS NULL
            OR "execution_mode_code" = 'provider_confirmed')
    );

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_new_purchase_source;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_new_purchase_source CHECK (
        (
            "operation_type_code" = 'new_purchase'
            AND "source_fulfillment_no" IS NOT NULL
            AND "settlement_no" IS NOT NULL
            AND "quote_no" IS NOT NULL
            AND "order_no" IS NOT NULL
            AND "invoice_no" IS NOT NULL
        ) OR (
            "operation_type_code" <> 'new_purchase'
            AND "source_fulfillment_no" IS NULL
        )
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_service_operation_source'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT uk_idc_service_operation_source UNIQUE (
                "service_no", "source_fulfillment_no", "order_no", "customer_no"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_business_operation_fulfillment_link'
          AND conrelid = 'plugin_idc_core_business_operation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT uk_idc_business_operation_fulfillment_link UNIQUE (
                "operation_no", "service_no", "source_fulfillment_no", "order_no"
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_business_operation_new_purchase_service'
          AND conrelid = 'plugin_idc_core_business_operation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT fk_idc_business_operation_new_purchase_service FOREIGN KEY (
                "service_no", "source_fulfillment_no", "order_no", "customer_no"
            ) REFERENCES plugin_idc_core_service_instance (
                "service_no", "source_fulfillment_no", "order_no", "customer_no"
            ) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_business_operation_settlement'
          AND conrelid = 'plugin_idc_core_business_operation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT fk_idc_business_operation_settlement FOREIGN KEY (
                "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
            ) REFERENCES plugin_idc_core_financial_settlement (
                "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
            ) ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_com_fulfillment_operation_owner'
          AND conrelid = 'plugin_idc_core_commerce_fulfillment_unit'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
            ADD CONSTRAINT fk_idc_com_fulfillment_operation_owner FOREIGN KEY (
                "business_operation_no", "service_no", "fulfillment_no", "order_no"
            ) REFERENCES plugin_idc_core_business_operation (
                "operation_no", "service_no", "source_fulfillment_no", "order_no"
            ) ON DELETE RESTRICT;
    END IF;
END;
$constraint$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_new_purchase_service
    ON plugin_idc_core_business_operation (
        "service_no", "operation_type_code"
    ) WHERE "operation_type_code" = 'new_purchase';

CREATE INDEX IF NOT EXISTS idx_idc_business_operation_settlement
    ON plugin_idc_core_business_operation (
        "tenant_id", "settlement_no", "source_fulfillment_no", "id"
    ) WHERE "settlement_no" IS NOT NULL;

ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
    DROP CONSTRAINT IF EXISTS ck_idc_com_fulfillment_status;
ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
    ADD CONSTRAINT ck_idc_com_fulfillment_status CHECK (
        "status_code" IN (
            'pending_payment', 'waiting_gate', 'ready', 'executing',
            'succeeded', 'failed', 'result_unknown', 'manual_review', 'cancelled'
        )
    );

ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
    DROP CONSTRAINT IF EXISTS ck_idc_com_fulfillment_lifecycle;
ALTER TABLE plugin_idc_core_commerce_fulfillment_unit
    ADD CONSTRAINT ck_idc_com_fulfillment_lifecycle CHECK (
        ("status_code" IN ('pending_payment', 'waiting_gate', 'ready')
            AND "started_at" IS NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('executing', 'result_unknown', 'manual_review')
            AND "started_at" IS NOT NULL AND "completed_at" IS NULL)
        OR ("status_code" IN ('succeeded', 'failed')
            AND "started_at" IS NOT NULL AND "completed_at" IS NOT NULL)
        OR ("status_code" = 'cancelled' AND "completed_at" IS NOT NULL)
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_outbox_delivery_owner'
          AND conrelid = 'plugin_idc_core_outbox_event'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_outbox_event
            ADD CONSTRAINT uk_idc_outbox_delivery_owner UNIQUE (
                "event_no", "event_type", "aggregate_no", "correlation_no"
            );
    END IF;
END;
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_outbox_event_type_claim
    ON plugin_idc_core_outbox_event (
        "tenant_id", "event_type", "status_code", "available_at", "id"
    );

-- Purpose: Append-only receipt for idempotent local consumption of one financially settled order event.
-- 用途：保存财务结算交付相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_settlement_delivery (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "delivery_no" VARCHAR(40) NOT NULL,
    "event_no" VARCHAR(40) NOT NULL,
    "event_type" VARCHAR(128) NOT NULL,
    "settlement_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "fulfillment_count" INT NOT NULL,
    "ready_count" INT NOT NULL,
    "waiting_gate_count" INT NOT NULL,
    "manual_review_count" INT NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_fin_delivery_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_fin_delivery_event CHECK (
        "event_type" = 'commerce.order.financially_settled'
    ),
    CONSTRAINT ck_idc_fin_delivery_counts CHECK (
        "fulfillment_count" BETWEEN 1 AND 1000
        AND "ready_count" BETWEEN 0 AND "fulfillment_count"
        AND "waiting_gate_count" BETWEEN 0 AND "fulfillment_count"
        AND "manual_review_count" BETWEEN 0 AND "fulfillment_count"
        AND "ready_count" + "waiting_gate_count" + "manual_review_count"
            = "fulfillment_count"
    ),
    CONSTRAINT ck_idc_fin_delivery_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_fin_delivery_no UNIQUE ("delivery_no"),
    CONSTRAINT uk_idc_fin_delivery_event UNIQUE ("event_no"),
    CONSTRAINT uk_idc_fin_delivery_settlement UNIQUE ("settlement_no"),
    CONSTRAINT fk_idc_fin_delivery_event FOREIGN KEY (
        "event_no", "event_type", "order_no", "settlement_no"
    ) REFERENCES plugin_idc_core_outbox_event (
        "event_no", "event_type", "aggregate_no", "correlation_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_fin_delivery_settlement FOREIGN KEY (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_financial_settlement (
        "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_fin_delivery_order
    ON plugin_idc_core_financial_settlement_delivery (
        "tenant_id", "order_no", "created_at", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_financial_delivery_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'financial settlement deliveries are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_fin_delivery_immutable
    ON plugin_idc_core_financial_settlement_delivery;
CREATE TRIGGER trg_idc_fin_delivery_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_financial_settlement_delivery
FOR EACH ROW EXECUTE FUNCTION
    plugin_idc_core_reject_financial_delivery_mutation();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."operation_no", OLD."operation_type_code",
        OLD."customer_no", OLD."service_no", OLD."currency_code",
        OLD."price_snapshot_no", OLD."quote_no",
        OLD."order_no", OLD."invoice_no", OLD."financial_settlement_required",
        OLD."execution_mode_code", OLD."resource_owner_code", OLD."channel_no",
        OLD."pool_no",
        OLD."capability_evidence_no", OLD."capability_evidence_digest",
        OLD."idempotency_digest", OLD."request_digest", OLD."content_digest",
        OLD."expected_service_version", OLD."source_fulfillment_no",
        OLD."settlement_no", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."operation_no", NEW."operation_type_code",
        NEW."customer_no", NEW."service_no", NEW."currency_code",
        NEW."price_snapshot_no", NEW."quote_no",
        NEW."order_no", NEW."invoice_no", NEW."financial_settlement_required",
        NEW."execution_mode_code", NEW."resource_owner_code", NEW."channel_no",
        NEW."pool_no",
        NEW."capability_evidence_no", NEW."capability_evidence_digest",
        NEW."idempotency_digest", NEW."request_digest", NEW."content_digest",
        NEW."expected_service_version", NEW."source_fulfillment_no",
        NEW."settlement_no", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

COMMENT ON COLUMN plugin_idc_core_business_operation."source_fulfillment_no"
    IS 'Immutable new-purchase server fulfillment identity';
COMMENT ON COLUMN plugin_idc_core_business_operation."settlement_no"
    IS 'Unified financial settlement authority for a paid new-purchase operation';
COMMENT ON TABLE plugin_idc_core_financial_settlement_delivery
    IS 'Append-only receipt for idempotent local consumption of one financially settled order event';
