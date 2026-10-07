-- Server ownership transfer policy, short-lived bilateral confirmation, and
-- ownership history. Historical business documents keep their original
-- customer_no; only the service row moves after the recipient confirms.

ALTER TABLE plugin_idc_core_server_operation_policy
    ADD COLUMN IF NOT EXISTS "allow_customer_transfer" BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE plugin_idc_core_service_instance
    ADD COLUMN IF NOT EXISTS "ownership_started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_type;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_type CHECK (
        "operation_type_code" IN (
            'new_purchase', 'renewal', 'change', 'ip_change', 'addon',
            'refund', 'suspend', 'resume', 'terminate', 'start', 'stop',
            'reboot', 'reset_password', 'console', 'reinstall', 'resize'
        ) AND "currency_code" ~ '^[A-Z]{3}$'
    );
ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_financial;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_financial CHECK (
        (("financial_settlement_required" AND "operation_type_code" IN (
                'new_purchase', 'renewal', 'change', 'ip_change', 'addon', 'resize'
            )) OR (NOT "financial_settlement_required"))
        AND ("status_code" NOT IN (
                'financially_settled', 'ready', 'executing', 'succeeded',
                'failed', 'compensating', 'refunding'
            ) OR NOT "financial_settlement_required"
            OR "financially_settled_at" IS NOT NULL)
    );
ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_reference;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_reference CHECK (
        ("operation_type_code" IN ('new_purchase', 'renewal', 'change')
            AND "price_snapshot_no" IS NOT NULL)
        OR ("operation_type_code" IN ('addon', 'ip_change', 'resize')
            AND (("financial_settlement_required" AND "price_snapshot_no" IS NOT NULL)
                OR (NOT "financial_settlement_required" AND "price_snapshot_no" IS NULL)))
        OR ("operation_type_code" IN (
                'refund', 'suspend', 'resume', 'terminate', 'start', 'stop',
                'reboot', 'reset_password', 'console', 'reinstall'
            )
            AND "price_snapshot_no" IS NULL)
    );

-- Older tables froze the then-current customer in historical rows and also
-- used that value as part of their service foreign key. That made a legitimate
-- ownership change impossible. Keep each historical customer_no unchanged and
-- retain only the stable service_no reference.
ALTER TABLE plugin_idc_core_service_price_snapshot DROP CONSTRAINT IF EXISTS fk_idc_service_price_service;
ALTER TABLE plugin_idc_core_service_price_snapshot DROP CONSTRAINT IF EXISTS fk_idc_service_price_service_identity;
ALTER TABLE plugin_idc_core_service_price_snapshot ADD CONSTRAINT fk_idc_service_price_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_business_operation DROP CONSTRAINT IF EXISTS fk_idc_business_operation_service;
ALTER TABLE plugin_idc_core_business_operation DROP CONSTRAINT IF EXISTS fk_idc_business_operation_new_purchase_service;
ALTER TABLE plugin_idc_core_business_operation DROP CONSTRAINT IF EXISTS fk_idc_business_operation_service_identity;
ALTER TABLE plugin_idc_core_business_operation ADD CONSTRAINT fk_idc_business_operation_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_service_lifecycle_task DROP CONSTRAINT IF EXISTS fk_idc_service_lifecycle_task_service;
ALTER TABLE plugin_idc_core_service_lifecycle_task DROP CONSTRAINT IF EXISTS fk_idc_service_lifecycle_task_service_identity;
ALTER TABLE plugin_idc_core_service_lifecycle_task ADD CONSTRAINT fk_idc_service_lifecycle_task_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_refund_request_item DROP CONSTRAINT IF EXISTS fk_idc_refund_item_service;
ALTER TABLE plugin_idc_core_refund_request_item DROP CONSTRAINT IF EXISTS fk_idc_refund_item_service_identity;
ALTER TABLE plugin_idc_core_refund_request_item ADD CONSTRAINT fk_idc_refund_item_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_refund_service_impact DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_service;
ALTER TABLE plugin_idc_core_refund_service_impact DROP CONSTRAINT IF EXISTS fk_idc_refund_impact_service_identity;
ALTER TABLE plugin_idc_core_refund_service_impact ADD CONSTRAINT fk_idc_refund_impact_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_accessory_quote DROP CONSTRAINT IF EXISTS fk_idc_server_accessory_quote_service;
ALTER TABLE plugin_idc_core_server_accessory_quote DROP CONSTRAINT IF EXISTS fk_idc_server_accessory_quote_service_identity;
ALTER TABLE plugin_idc_core_server_accessory_quote ADD CONSTRAINT fk_idc_server_accessory_quote_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_accessory_entitlement DROP CONSTRAINT IF EXISTS fk_idc_server_accessory_entitlement_service;
ALTER TABLE plugin_idc_core_server_accessory_entitlement DROP CONSTRAINT IF EXISTS fk_idc_server_accessory_entitlement_service_identity;
ALTER TABLE plugin_idc_core_server_accessory_entitlement ADD CONSTRAINT fk_idc_server_accessory_entitlement_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_ip_change_quote DROP CONSTRAINT IF EXISTS fk_idc_server_ip_change_quote_service;
ALTER TABLE plugin_idc_core_server_ip_change_quote DROP CONSTRAINT IF EXISTS fk_idc_server_ip_change_quote_service_identity;
ALTER TABLE plugin_idc_core_server_ip_change_quote ADD CONSTRAINT fk_idc_server_ip_change_quote_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_service_tag DROP CONSTRAINT IF EXISTS fk_idc_server_service_tag_service;
ALTER TABLE plugin_idc_core_server_service_tag DROP CONSTRAINT IF EXISTS fk_idc_server_service_tag_service_identity;
ALTER TABLE plugin_idc_core_server_service_tag ADD CONSTRAINT fk_idc_server_service_tag_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_service_note DROP CONSTRAINT IF EXISTS fk_idc_server_service_note_service;
ALTER TABLE plugin_idc_core_server_service_note DROP CONSTRAINT IF EXISTS fk_idc_server_service_note_service_identity;
ALTER TABLE plugin_idc_core_server_service_note ADD CONSTRAINT fk_idc_server_service_note_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_server_service_batch_item DROP CONSTRAINT IF EXISTS fk_idc_server_service_batch_item_service;
ALTER TABLE plugin_idc_core_server_service_batch_item DROP CONSTRAINT IF EXISTS fk_idc_server_service_batch_item_service_identity;
ALTER TABLE plugin_idc_core_server_service_batch_item ADD CONSTRAINT fk_idc_server_service_batch_item_service_identity
    FOREIGN KEY ("service_no") REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT;

-- Purpose: Bilateral customer service transfer request with a fixed 24-hour confirmation window.
-- 用途：保存服务转移申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_transfer_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transfer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "source_customer_no" VARCHAR(40) NOT NULL,
    "target_customer_no" VARCHAR(40),
    "target_email_digest" CHAR(64) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "confirmed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_transfer_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_transfer_digest CHECK (
        "target_email_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_service_transfer_status CHECK (
        "status_code" IN ('pending','unmatched','confirmed','expired','cancelled','blocked')
    ),
    CONSTRAINT ck_idc_service_transfer_target CHECK (
        ("status_code" = 'unmatched' AND "target_customer_no" IS NULL)
        OR ("status_code" <> 'unmatched' AND "target_customer_no" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_service_transfer_confirmation CHECK (
        ("status_code" = 'confirmed' AND "confirmed_at" IS NOT NULL)
        OR ("status_code" <> 'confirmed' AND "confirmed_at" IS NULL)
    ),
    CONSTRAINT ck_idc_service_transfer_time CHECK (
        "expires_at" > "created_at" AND "expected_service_version" > 0 AND "version" > 0
    ),
    CONSTRAINT uk_idc_service_transfer_no UNIQUE ("transfer_no"),
    CONSTRAINT uk_idc_service_transfer_idempotency UNIQUE (
        "tenant_id", "source_customer_no", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_service_transfer_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_transfer_source FOREIGN KEY ("source_customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_transfer_target FOREIGN KEY ("target_customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_service_transfer_active
    ON plugin_idc_core_service_transfer_request ("tenant_id", "service_no")
    WHERE "status_code" = 'pending';
CREATE INDEX IF NOT EXISTS idx_idc_service_transfer_recipient
    ON plugin_idc_core_service_transfer_request (
        "tenant_id", "target_customer_no", "status_code", "expires_at", "id"
    );

-- Purpose: Append-only service ownership boundary; historical documents remain with their original customer.
-- 用途：保存服务归属事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_ownership_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "transfer_no" VARCHAR(40) NOT NULL,
    "source_customer_no" VARCHAR(40) NOT NULL,
    "target_customer_no" VARCHAR(40) NOT NULL,
    "effective_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_owner_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_idc_service_owner_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_idc_service_owner_event_transfer UNIQUE ("transfer_no"),
    CONSTRAINT fk_idc_service_owner_event_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_owner_event_transfer FOREIGN KEY ("transfer_no")
        REFERENCES plugin_idc_core_service_transfer_request ("transfer_no") ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_service_ownership_event()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'service ownership events are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_service_ownership_event_immutable
    ON plugin_idc_core_service_ownership_event;
CREATE TRIGGER trg_idc_service_ownership_event_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_ownership_event
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_service_ownership_event();

COMMENT ON TABLE plugin_idc_core_service_transfer_request
    IS 'Bilateral customer service transfer request with a fixed 24-hour confirmation window';
COMMENT ON TABLE plugin_idc_core_service_ownership_event
    IS 'Append-only service ownership boundary; historical documents remain with their original customer';

-- Dedicated configuration-change quote and execution intent. The quote freezes
-- both the one-time remaining-period charge and the target recurring price.
-- Downgrades carry no order and remain scheduled until the current entitlement
-- expires; upgrades are released only by the financial-settlement consumer.
-- Purpose: Immutable customer configuration-change quote with remaining-period charge and target renewal baseline.
-- 用途：保存服务器变配报价相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_resize_quote (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "public_slug" VARCHAR(128) NOT NULL,
    "source_configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "target_configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "target_version_no" VARCHAR(40) NOT NULL,
    "target_billing_rule_no" VARCHAR(40) NOT NULL,
    "direction_code" VARCHAR(16) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "current_recurring_amount_minor" BIGINT NOT NULL,
    "target_recurring_amount_minor" BIGINT NOT NULL,
    "target_subtotal_minor" BIGINT NOT NULL,
    "target_net_amount_minor" BIGINT NOT NULL,
    "target_tax_amount_minor" BIGINT NOT NULL,
    "charge_amount_minor" BIGINT NOT NULL,
    "charge_subtotal_minor" BIGINT NOT NULL,
    "charge_net_amount_minor" BIGINT NOT NULL,
    "charge_tax_amount_minor" BIGINT NOT NULL,
    "period_started_at" TIMESTAMPTZ NOT NULL,
    "period_expires_at" TIMESTAMPTZ NOT NULL,
    "effective_at" TIMESTAMPTZ NOT NULL,
    "provider_parameters" JSONB NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "customer_subject_version" BIGINT NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_resize_quote_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_resize_quote_direction CHECK (
        ("direction_code" = 'upgrade' AND "target_recurring_amount_minor" >= "current_recurring_amount_minor"
            AND "effective_at" = "issued_at")
        OR ("direction_code" = 'downgrade' AND "target_recurring_amount_minor" < "current_recurring_amount_minor"
            AND "charge_amount_minor" = 0 AND "effective_at" = "period_expires_at")
    ),
    CONSTRAINT ck_idc_server_resize_quote_amount CHECK (
        "current_recurring_amount_minor" >= 0 AND "target_recurring_amount_minor" >= 0
        AND "target_subtotal_minor" >= 0 AND "target_net_amount_minor" >= 0 AND "target_tax_amount_minor" >= 0
        AND "target_recurring_amount_minor" = "target_net_amount_minor" + "target_tax_amount_minor"
        AND "charge_amount_minor" >= 0 AND "charge_subtotal_minor" >= 0
        AND "charge_net_amount_minor" >= 0 AND "charge_tax_amount_minor" >= 0
        AND "charge_amount_minor" = "charge_net_amount_minor" + "charge_tax_amount_minor"
        AND "expected_service_version" > 0
        AND "customer_subject_version" > 0
    ),
    CONSTRAINT ck_idc_server_resize_quote_time CHECK (
        "period_expires_at" > "period_started_at" AND "expires_at" > "issued_at"
        AND "effective_at" >= "issued_at"
    ),
    CONSTRAINT ck_idc_server_resize_quote_payload CHECK (
        jsonb_typeof("provider_parameters") = 'object'
        AND octet_length("provider_parameters"::TEXT) <= 16384
        AND "request_digest" ~ '^[0-9a-f]{64}$' AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_server_resize_quote_no UNIQUE ("quote_no"),
    CONSTRAINT fk_idc_server_resize_quote_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_quote_source_config FOREIGN KEY ("source_configuration_snapshot_no")
        REFERENCES plugin_idc_core_commerce_configuration_snapshot ("configuration_snapshot_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_quote_target_config FOREIGN KEY ("target_configuration_snapshot_no", "target_version_no")
        REFERENCES plugin_idc_core_commerce_configuration_snapshot ("configuration_snapshot_no", "version_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_resize_quote_customer
    ON plugin_idc_core_server_resize_quote ("tenant_id", "customer_no", "service_no", "expires_at" DESC);

-- Purpose: Upgrade execution or next-renewal downgrade schedule; unknown outcomes remain locked for manual review.
-- 用途：保存服务器变配意图相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_resize_intent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_no" VARCHAR(40) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "direction_code" VARCHAR(16) NOT NULL,
    "target_configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "target_version_no" VARCHAR(40) NOT NULL,
    "target_billing_rule_no" VARCHAR(40) NOT NULL,
    "target_price_snapshot_no" VARCHAR(40) NOT NULL,
    "target_recurring_amount_minor" BIGINT NOT NULL,
    "effective_at" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "refund_handoff_no" VARCHAR(40),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_resize_intent_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_resize_intent_direction CHECK ("direction_code" IN ('upgrade','downgrade')),
    CONSTRAINT ck_idc_server_resize_intent_status CHECK (
        "status_code" IN ('awaiting_payment','scheduled','ready','executing','succeeded','failed','result_unknown','manual_review','refund_pending')
    ),
    CONSTRAINT ck_idc_server_resize_intent_amount CHECK ("target_recurring_amount_minor" >= 0),
    CONSTRAINT uk_idc_server_resize_intent_operation UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_server_resize_intent_quote UNIQUE ("quote_no"),
    CONSTRAINT fk_idc_server_resize_intent_operation FOREIGN KEY ("operation_no", "service_no")
        REFERENCES plugin_idc_core_business_operation ("operation_no", "service_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_intent_quote FOREIGN KEY ("quote_no")
        REFERENCES plugin_idc_core_server_resize_quote ("quote_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_intent_target_config FOREIGN KEY ("target_configuration_snapshot_no", "target_version_no")
        REFERENCES plugin_idc_core_commerce_configuration_snapshot ("configuration_snapshot_no", "version_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_server_resize_active
    ON plugin_idc_core_server_resize_intent ("tenant_id", "service_no")
    WHERE "status_code" IN ('awaiting_payment','scheduled','ready','executing','result_unknown','manual_review','refund_pending');
CREATE INDEX IF NOT EXISTS idx_idc_server_resize_due
    ON plugin_idc_core_server_resize_intent ("tenant_id", "status_code", "effective_at", "id")
    WHERE "status_code" = 'scheduled';

-- Purpose: Stores Server Resize Refund Handoff records.
-- 用途：保存服务器变配退款交接相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_resize_refund_handoff (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "handoff_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'ready',
    "reason_code" VARCHAR(128) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "refund_no" VARCHAR(40),
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_resize_refund_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_resize_refund_value CHECK (
        "amount_minor" > 0 AND "currency_code" ~ '^[A-Z]{3}$'
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_server_resize_refund_status CHECK (
        ("status_code" = 'ready' AND "refund_no" IS NULL)
        OR ("status_code" IN ('processing','succeeded','failed','result_unknown','manual_review')
            AND "refund_no" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_server_resize_refund_no UNIQUE ("handoff_no"),
    CONSTRAINT uk_idc_server_resize_refund_operation UNIQUE ("operation_no"),
    CONSTRAINT fk_idc_server_resize_refund_operation FOREIGN KEY ("operation_no")
        REFERENCES plugin_idc_core_business_operation ("operation_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_refund_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_refund_invoice FOREIGN KEY ("invoice_no", "customer_no", "currency_code")
        REFERENCES plugin_idc_core_billing_invoice ("invoice_no", "customer_no", "currency_code") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_resize_refund_request FOREIGN KEY ("refund_no")
        REFERENCES plugin_idc_core_refund_request ("refund_no") ON DELETE RESTRICT
);
ALTER TABLE plugin_idc_core_server_resize_refund_handoff
    ADD COLUMN IF NOT EXISTS "refund_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "last_error_code" VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_server_resize_refund_handoff
    DROP CONSTRAINT IF EXISTS ck_idc_server_resize_refund_status;
ALTER TABLE plugin_idc_core_server_resize_refund_handoff
    ADD CONSTRAINT ck_idc_server_resize_refund_status CHECK (
        ("status_code" = 'ready' AND "refund_no" IS NULL)
        OR ("status_code" IN ('processing','succeeded','failed','result_unknown','manual_review')
            AND "refund_no" IS NOT NULL)
    );
ALTER TABLE plugin_idc_core_server_resize_refund_handoff
    DROP CONSTRAINT IF EXISTS fk_idc_server_resize_refund_request;
ALTER TABLE plugin_idc_core_server_resize_refund_handoff
    ADD CONSTRAINT fk_idc_server_resize_refund_request FOREIGN KEY ("refund_no")
        REFERENCES plugin_idc_core_refund_request ("refund_no") ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_idc_server_resize_refund_work
    ON plugin_idc_core_server_resize_refund_handoff ("tenant_id", "status_code", "created_at", "id")
    WHERE "status_code" IN ('ready','processing','result_unknown','manual_review');

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_resize_quote
    ON plugin_idc_core_business_operation ("tenant_id", "quote_no")
    WHERE "operation_type_code" = 'resize' AND "quote_no" IS NOT NULL;

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_phase;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT ck_idc_service_price_phase CHECK (
        "phase_code" IN ('new_purchase', 'renewal', 'addon', 'ip_change', 'resize_charge', 'resize_target')
        AND (("source_quote_no" IS NULL AND "source_quote_item_no" IS NULL)
            OR ("source_quote_no" IS NOT NULL AND "source_quote_item_no" IS NOT NULL))
    );

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_renewal_mode;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT ck_idc_service_price_renewal_mode CHECK (
        "renewal_mode_code" IN ('unresolved', 'provider_confirmed', 'local_entitlement')
        AND (
            ("phase_code" = 'new_purchase' AND "renewal_mode_code" = 'unresolved'
                AND "capability_evidence_no" IS NULL AND "capability_evidence_digest" IS NULL
                AND "capability_evidence_observed_at" IS NULL AND "capability_evidence_expires_at" IS NULL
                AND "original_expires_at" IS NULL)
            OR ("phase_code" = 'renewal' AND "renewal_mode_code" <> 'unresolved'
                AND "capability_evidence_no" IS NOT NULL AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NOT NULL AND "projected_expires_at" > "original_expires_at")
            OR ("phase_code" IN ('addon','ip_change','resize_charge','resize_target')
                AND "renewal_mode_code" = 'provider_confirmed'
                AND "capability_evidence_no" IS NOT NULL AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NULL AND "projected_expires_at" IS NULL)
        )
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'business operations cannot be deleted'; END IF;
    IF ROW(OLD."tenant_id",OLD."operation_no",OLD."operation_type_code",OLD."customer_no",OLD."service_no",
        OLD."currency_code",OLD."price_snapshot_no",OLD."quote_no",OLD."order_no",OLD."invoice_no",
        OLD."financial_settlement_required",OLD."execution_mode_code",OLD."resource_owner_code",OLD."channel_no",
        OLD."pool_no",OLD."capability_evidence_no",OLD."capability_evidence_digest",OLD."idempotency_digest",
        OLD."request_digest",OLD."content_digest",OLD."expected_service_version",OLD."source_fulfillment_no",OLD."created_at")
       IS DISTINCT FROM
       ROW(NEW."tenant_id",NEW."operation_no",NEW."operation_type_code",NEW."customer_no",NEW."service_no",
        NEW."currency_code",NEW."price_snapshot_no",NEW."quote_no",NEW."order_no",NEW."invoice_no",
        NEW."financial_settlement_required",NEW."execution_mode_code",NEW."resource_owner_code",NEW."channel_no",
        NEW."pool_no",NEW."capability_evidence_no",NEW."capability_evidence_digest",NEW."idempotency_digest",
        NEW."request_digest",NEW."content_digest",NEW."expected_service_version",NEW."source_fulfillment_no",NEW."created_at")
    THEN RAISE EXCEPTION 'business operation intent is immutable'; END IF;
    IF OLD."settlement_no" IS DISTINCT FROM NEW."settlement_no" AND NOT (
        OLD."settlement_no" IS NULL AND NEW."settlement_no" IS NOT NULL
        AND OLD."operation_type_code" IN ('addon','ip_change','resize')
        AND OLD."status_code" = 'awaiting_financial_settlement'
        AND NEW."status_code" IN ('financially_settled','ready'))
    THEN RAISE EXCEPTION 'business operation settlement link is immutable'; END IF;
    RETURN NEW;
END;
$$;

COMMENT ON TABLE plugin_idc_core_server_resize_quote
    IS 'Immutable customer configuration-change quote with remaining-period charge and target renewal baseline';
COMMENT ON TABLE plugin_idc_core_server_resize_intent
    IS 'Upgrade execution or next-renewal downgrade schedule; unknown outcomes remain locked for manual review';
