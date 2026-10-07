-- 031: Bind idempotent renewal quote requests and append-only automatic
-- renewal attempts to the immutable quote, invoice, operation, and ledger facts.

ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_quote_item_inventory;
ALTER TABLE plugin_idc_core_commerce_quote_item_snapshot
    ADD CONSTRAINT ck_idc_com_quote_item_inventory CHECK (
        "inventory_decision_code" IN (
            'available', 'manual_confirmation', 'not_required'
        )
        AND (
            ("inventory_decision_code" = 'available'
                AND "inventory_snapshot_no" IS NOT NULL)
            OR "inventory_decision_code" = 'manual_confirmation'
            OR ("inventory_decision_code" = 'not_required'
                AND "inventory_snapshot_no" IS NULL)
        )
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_service_price_renewal_quote
    ON plugin_idc_core_service_price_snapshot (
        "tenant_id", "source_quote_no"
    ) WHERE "phase_code" = 'renewal' AND "source_quote_no" IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_renewal_quote
    ON plugin_idc_core_business_operation (
        "tenant_id", "quote_no"
    ) WHERE "operation_type_code" = 'renewal' AND "quote_no" IS NOT NULL;

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_business_operation_renewal_attempt_owner'
          AND conrelid = 'plugin_idc_core_business_operation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_business_operation
            ADD CONSTRAINT uk_idc_business_operation_renewal_attempt_owner UNIQUE (
                "operation_no", "service_no", "customer_no", "currency_code", "invoice_no"
            );
    END IF;
END;
$constraint$;

-- Purpose: Immutable idempotent request binding one service to one renewal quote and price snapshot.
-- 用途：保存服务续费报价申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_renewal_quote_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "request_source_code" VARCHAR(16) NOT NULL,
    "renewal_cycles" INT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "price_snapshot_no" VARCHAR(40) NOT NULL,
    "expected_entitlement_version" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_renewal_quote_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_renewal_quote_request_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "request_source_code" IN ('customer', 'staff', 'system', 'channel')
        AND "renewal_cycles" BETWEEN 1 AND 120
        AND "expected_entitlement_version" > 0
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_renewal_quote_request_no UNIQUE ("request_no"),
    CONSTRAINT uk_idc_service_renewal_quote_request_idempotency UNIQUE (
        "customer_no", "request_source_code", "idempotency_digest"
    ),
    CONSTRAINT uk_idc_service_renewal_quote_request_quote UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_service_renewal_quote_request_price UNIQUE ("price_snapshot_no"),
    CONSTRAINT fk_idc_service_renewal_quote_request_quote FOREIGN KEY (
        "quote_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_snapshot (
        "quote_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_renewal_quote_request_price FOREIGN KEY ("price_snapshot_no")
        REFERENCES plugin_idc_core_service_price_snapshot ("price_snapshot_no") ON DELETE RESTRICT
);

DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_quote_request'
           AND column_name = 'service_no'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_quote_request'
           AND column_name = 'entitlement_no'
    ) THEN
        ALTER TABLE plugin_idc_core_service_renewal_quote_request
            RENAME COLUMN "service_no" TO "entitlement_no";
    END IF;
END;
$migration$;

CREATE INDEX IF NOT EXISTS idx_idc_service_renewal_quote_request_entitlement
    ON plugin_idc_core_service_renewal_quote_request (
        "tenant_id", "entitlement_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only same-currency automatic renewal attempt evidence.
-- 用途：保存服务续费尝试相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_renewal_attempt (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "attempt_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "attempt_sequence" INT NOT NULL,
    "outcome_code" VARCHAR(24) NOT NULL,
    "ledger_transaction_no" VARCHAR(40),
    "ledger_transaction_type_code" VARCHAR(24),
    "failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "next_attempt_at" TIMESTAMPTZ,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_renewal_attempt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_renewal_attempt_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "attempt_sequence" BETWEEN 1 AND 32
        AND "outcome_code" IN (
            'balance_settled', 'zero_settled', 'insufficient_balance',
            'retryable_failure', 'exhausted'
        )
        AND ("failure_code" = ''
            OR "failure_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (
            ("outcome_code" = 'balance_settled'
                AND "ledger_transaction_no" IS NOT NULL
                AND "ledger_transaction_type_code" = 'balance_payment'
                AND "failure_code" = '' AND "next_attempt_at" IS NULL)
            OR ("outcome_code" = 'zero_settled'
                AND "ledger_transaction_no" IS NULL
                AND "ledger_transaction_type_code" IS NULL
                AND "failure_code" = '' AND "next_attempt_at" IS NULL)
            OR ("outcome_code" IN ('insufficient_balance', 'retryable_failure')
                AND "ledger_transaction_no" IS NULL
                AND "ledger_transaction_type_code" IS NULL
                AND "failure_code" <> '' AND "next_attempt_at" > "created_at")
            OR ("outcome_code" = 'exhausted'
                AND "ledger_transaction_no" IS NULL
                AND "ledger_transaction_type_code" IS NULL
                AND "failure_code" <> '' AND "next_attempt_at" IS NULL)
        )
    ),
    CONSTRAINT uk_idc_service_renewal_attempt_no UNIQUE ("attempt_no"),
    CONSTRAINT uk_idc_service_renewal_attempt_sequence UNIQUE (
        "operation_no", "attempt_sequence"
    ),
    CONSTRAINT fk_idc_service_renewal_attempt_operation FOREIGN KEY ("operation_no")
        REFERENCES plugin_idc_core_business_operation ("operation_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_renewal_attempt_policy FOREIGN KEY (
        "policy_no", "policy_version", "entitlement_no"
    ) REFERENCES plugin_idc_core_service_renewal_policy (
        "policy_no", "policy_version", "entitlement_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_renewal_attempt_ledger FOREIGN KEY (
        "ledger_transaction_no", "currency_code", "ledger_transaction_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code", "transaction_type_code"
    ) ON DELETE RESTRICT
);

DO $migration$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_attempt'
           AND column_name = 'service_no'
    ) AND NOT EXISTS (
        SELECT 1 FROM information_schema.columns
         WHERE table_schema = current_schema()
           AND table_name = 'plugin_idc_core_service_renewal_attempt'
           AND column_name = 'entitlement_no'
    ) THEN
        ALTER TABLE plugin_idc_core_service_renewal_attempt
            RENAME COLUMN "service_no" TO "entitlement_no";
    END IF;
END;
$migration$;

CREATE INDEX IF NOT EXISTS idx_idc_service_renewal_attempt_history
    ON plugin_idc_core_service_renewal_attempt (
        "tenant_id", "operation_no", "attempt_sequence" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_renewal_attempt_retry
    ON plugin_idc_core_service_renewal_attempt (
        "tenant_id", "next_attempt_at", "id"
    ) WHERE "outcome_code" IN ('insufficient_balance', 'retryable_failure');

DROP TRIGGER IF EXISTS trg_idc_service_renewal_quote_request_immutable
    ON plugin_idc_core_service_renewal_quote_request;
CREATE TRIGGER trg_idc_service_renewal_quote_request_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_renewal_quote_request
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_renewal_attempt_immutable
    ON plugin_idc_core_service_renewal_attempt;
CREATE TRIGGER trg_idc_service_renewal_attempt_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_renewal_attempt
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

COMMENT ON TABLE plugin_idc_core_service_renewal_quote_request
    IS 'Immutable idempotent request binding one service to one renewal quote and price snapshot';
COMMENT ON TABLE plugin_idc_core_service_renewal_attempt
    IS 'Append-only same-currency automatic renewal attempt evidence';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 自动续费尝试结果', 'idc_service_renewal_attempt_outcome', 1, 1, FALSE,
     '自动续费同币种余额扣款及重试结果', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_service_renewal_attempt_outcome', '余额结算成功', 'balance_settled', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_attempt_outcome', '零金额结算成功', 'zero_settled', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_attempt_outcome', '余额不足', 'insufficient_balance', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_attempt_outcome', '等待重试', 'retryable_failure', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_renewal_attempt_outcome', '尝试已耗尽', 'exhausted', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
