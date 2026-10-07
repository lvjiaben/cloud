-- Core-owned compensation fence for a paid order whose required contract
-- reached a supplier-authoritative terminal state before fulfillment started.

ALTER TABLE plugin_idc_core_commerce_order
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_status;
ALTER TABLE plugin_idc_core_commerce_order
    ADD CONSTRAINT ck_idc_com_order_status CHECK (
        "status_code" IN (
            'pending_payment', 'payment_processing', 'paid', 'fulfilling',
            'partially_fulfilled', 'fulfilled', 'cancelled', 'refund_pending',
            'partially_refunded', 'refunded', 'manual_review'
        )
    );

ALTER TABLE plugin_idc_core_commerce_order
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_lifecycle;
ALTER TABLE plugin_idc_core_commerce_order
    ADD CONSTRAINT ck_idc_com_order_lifecycle CHECK (
        ("status_code" IN ('pending_payment', 'payment_processing')
            AND "paid_at" IS NULL AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
        OR ("status_code" IN ('paid', 'fulfilling', 'partially_fulfilled', 'refund_pending')
            AND "paid_at" IS NOT NULL AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'fulfilled' AND "paid_at" IS NOT NULL
            AND "completed_at" IS NOT NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'cancelled' AND "paid_at" IS NULL
            AND "completed_at" IS NULL AND "cancelled_at" IS NOT NULL)
        OR ("status_code" IN ('partially_refunded', 'refunded')
            AND "paid_at" IS NOT NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'manual_review'
            AND "completed_at" IS NULL AND "cancelled_at" IS NULL)
    );

ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_com_inventory_res_lifecycle;
ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    ADD CONSTRAINT ck_idc_com_inventory_res_lifecycle CHECK (
        ("status_code" = 'held' AND "committed_at" IS NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = 0 AND "released_quantity" = 0)
        OR ("status_code" = 'committed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = 0 AND "released_quantity" = 0)
        OR ("status_code" = 'consumed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NOT NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = "reserved_quantity"
            AND "released_quantity" = 0)
        OR ("status_code" = 'partial' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NOT NULL AND "released_at" IS NOT NULL
            AND "release_reason_code" = 'fulfillment_failed'
            AND "consumed_quantity" > 0 AND "released_quantity" > 0
            AND "consumed_quantity" + "released_quantity" = "reserved_quantity")
        OR ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL
            AND "released_at" IS NOT NULL
            AND "release_reason_code" IN (
                'order_cancelled', 'payment_expired', 'rerouted',
                'fulfillment_failed', 'manual_release', 'contract_failed'
            )
            AND "consumed_quantity" = 0
            AND "released_quantity" = "reserved_quantity")
    );

-- Purpose: Idempotent core-owned stop-and-refund handoff for one failed required contract.
-- 用途：保存合同失败补偿相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_contract_failure_compensation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "compensation_no" VARCHAR(40) NOT NULL,
    "contract_no" VARCHAR(64) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "terminal_event_no" VARCHAR(64) NOT NULL,
    "terminal_status_code" VARCHAR(16) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "handling_deadline" TIMESTAMPTZ NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(128) NOT NULL,
    "refund_request_no" VARCHAR(40),
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_contract_compensation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_contract_compensation_terminal CHECK (
        "terminal_status_code" IN ('rejected', 'expired', 'revoked', 'failed')
    ),
    CONSTRAINT ck_idc_contract_compensation_policy CHECK (
        "policy_version" > 0 AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_contract_compensation_status CHECK (
        "status_code" IN ('refund_pending', 'completed', 'manual_review')
        AND (("status_code" = 'refund_pending' AND "refund_request_no" IS NOT NULL
                AND "completed_at" IS NULL)
            OR ("status_code" = 'completed' AND "completed_at" IS NOT NULL)
            OR ("status_code" = 'manual_review' AND "completed_at" IS NULL))
    ),
    CONSTRAINT uk_idc_contract_compensation_no UNIQUE ("compensation_no"),
    CONSTRAINT uk_idc_contract_compensation_contract UNIQUE ("contract_no"),
    CONSTRAINT uk_idc_contract_compensation_request UNIQUE ("request_digest"),
    CONSTRAINT fk_idc_contract_compensation_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_contract_compensation_invoice FOREIGN KEY ("invoice_no")
        REFERENCES plugin_idc_core_billing_invoice ("invoice_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_contract_compensation_status
    ON plugin_idc_core_contract_failure_compensation (
        "tenant_id", "status_code", "created_at", "id"
    );

-- Purpose: Unique payment-owner handoff; ready never means the customer has received money.
-- 用途：保存账务退款申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_refund_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "refund_request_no" VARCHAR(40) NOT NULL,
    "compensation_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'ready',
    "idempotency_digest" CHAR(64) NOT NULL,
    "result_reference" VARCHAR(128) NOT NULL DEFAULT '',
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_billing_refund_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_billing_refund_request_value CHECK (
        "currency_code" ~ '^[A-Z]{3}$' AND "amount_minor" > 0
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_billing_refund_request_status CHECK (
        "status_code" IN (
            'ready', 'processing', 'result_unknown', 'succeeded', 'failed', 'manual_review'
        )
    ),
    CONSTRAINT uk_idc_billing_refund_request_no UNIQUE ("refund_request_no"),
    CONSTRAINT uk_idc_billing_refund_request_compensation UNIQUE ("compensation_no"),
    CONSTRAINT uk_idc_billing_refund_request_idempotency UNIQUE ("idempotency_digest"),
    CONSTRAINT fk_idc_billing_refund_request_compensation FOREIGN KEY ("compensation_no")
        REFERENCES plugin_idc_core_contract_failure_compensation ("compensation_no")
        ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT fk_idc_billing_refund_request_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_billing_refund_request_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_billing_refund_request_work
    ON plugin_idc_core_billing_refund_request (
        "tenant_id", "status_code", "created_at", "id"
    ) WHERE "status_code" IN ('ready', 'processing', 'result_unknown', 'manual_review');

COMMENT ON TABLE plugin_idc_core_contract_failure_compensation IS
    'Idempotent core-owned stop-and-refund handoff for one failed required contract';
COMMENT ON TABLE plugin_idc_core_billing_refund_request IS
    'Unique payment-owner handoff; ready never means the customer has received money';
