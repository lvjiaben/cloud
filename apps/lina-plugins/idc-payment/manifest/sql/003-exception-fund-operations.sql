-- 003: Audited late-payment allocation and original-source refund handling.
-- Purpose: Persists business records for plugin_idc_payment_exception_fund_action.
-- 用途：持久化 plugin_idc_payment_exception_fund_action 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_exception_fund_action (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "action_no" VARCHAR(64) NOT NULL,
    "exception_no" VARCHAR(64) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "target_order_no" VARCHAR(64) NOT NULL DEFAULT '',
    "target_invoice_no" VARCHAR(64) NOT NULL DEFAULT '',
    "allocation_no" VARCHAR(64) NOT NULL DEFAULT '',
    "settlement_no" VARCHAR(64) NOT NULL DEFAULT '',
    "refund_attempt_no" VARCHAR(64) NOT NULL DEFAULT '',
    "proof_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "proof_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    "requested_by" INT NOT NULL,
    "completed_by" INT NOT NULL DEFAULT 0,
    "request_digest" CHAR(64) NOT NULL,
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_payment_exception_action_no UNIQUE ("action_no"),
    CONSTRAINT uk_idc_payment_exception_action_live UNIQUE ("exception_no"),
    CONSTRAINT fk_idc_payment_exception_action_fund FOREIGN KEY ("exception_no")
        REFERENCES plugin_idc_payment_exception_fund("exception_no") ON DELETE RESTRICT,
    CONSTRAINT ck_idc_payment_exception_action_code CHECK ("action_code" IN ('allocate', 'original_refund')),
    CONSTRAINT ck_idc_payment_exception_action_status CHECK ("status_code" IN ('pending', 'automatic_pending', 'automatic_running', 'manual_pending', 'completed', 'result_unknown', 'failed')),
    CONSTRAINT ck_idc_payment_exception_action_actor CHECK ("requested_by" > 0 AND "completed_by" >= 0),
    CONSTRAINT ck_idc_payment_exception_action_digest CHECK ("request_digest" ~ '^[0-9a-f]{64}$' AND "proof_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_payment_exception_action_version CHECK ("version" > 0)
);

-- Purpose: Persists business records for plugin_idc_payment_closure_operation.
-- 用途：持久化 plugin_idc_payment_closure_operation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_payment_closure_operation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "operation_no" VARCHAR(64) NOT NULL,
    "order_no" VARCHAR(64) NOT NULL,
    "attempt_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'prepared',
    "result_evidence_digest" CHAR(64) NOT NULL DEFAULT repeat('0', 64),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uk_idc_payment_closure_operation_no UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_payment_closure_attempt UNIQUE ("attempt_no"),
    CONSTRAINT fk_idc_payment_closure_attempt FOREIGN KEY ("attempt_no")
        REFERENCES plugin_idc_payment_attempt("attempt_no") ON DELETE RESTRICT,
    CONSTRAINT ck_idc_payment_closure_status CHECK ("status_code" IN ('prepared', 'calling', 'closed', 'result_unknown')),
    CONSTRAINT ck_idc_payment_closure_digest CHECK ("result_evidence_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_payment_closure_version CHECK ("version" > 0)
);

CREATE INDEX IF NOT EXISTS idx_idc_payment_exception_fund_queue
    ON plugin_idc_payment_exception_fund("status_code", created_at, "id")
    WHERE "status_code" IN ('pending', 'disputed');

CREATE INDEX IF NOT EXISTS idx_idc_payment_exception_action_queue
    ON plugin_idc_payment_exception_fund_action("status_code", requested_at, "id")
    WHERE "status_code" IN ('pending', 'automatic_pending', 'manual_pending', 'result_unknown');

CREATE INDEX IF NOT EXISTS idx_idc_payment_closure_operation_state
    ON plugin_idc_payment_closure_operation("status_code", updated_at, "id")
    WHERE "status_code" IN ('prepared', 'calling', 'result_unknown');
