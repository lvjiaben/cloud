-- Supplier-neutral refund-to-red-letter lifecycle. Real NuoNuo routes remain closed.

ALTER TABLE plugin_idc_compliance_tax_invoice
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_tax_invoice_status;
ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_status
    CHECK (
        "status_code" IN (
            'draft', 'pending_review', 'approved_pending_submission', 'rejected',
            'queued', 'submitting', 'issued', 'delivered', 'failed', 'result_unknown',
            'red_letter_pending', 'partially_red_lettered', 'red_lettered', 'canceled'
        )
    );

ALTER TABLE plugin_idc_compliance_red_letter_task
    ADD COLUMN IF NOT EXISTS "customer_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "currency_code" CHAR(3),
    ADD COLUMN IF NOT EXISTS "allocation_resolved" BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS "manual_reason_code" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "completion_evidence_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "customer_recheck_requested_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_compliance_red_letter_task
    ALTER COLUMN "tax_invoice_no" DROP NOT NULL,
    ALTER COLUMN "refund_operation_no" TYPE VARCHAR(192),
    ALTER COLUMN "customer_no" SET NOT NULL,
    ALTER COLUMN "currency_code" SET NOT NULL;

ALTER TABLE plugin_idc_compliance_red_letter_task
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_customer,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_currency,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_amount,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_confirmation,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_manual,
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_red_letter_task_complete;

ALTER TABLE plugin_idc_compliance_red_letter_task
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_customer
        CHECK ("customer_no" IS NULL OR length("customer_no") BETWEEN 1 AND 64),
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_currency
        CHECK ("currency_code" IS NULL OR "currency_code" ~ '^[A-Z]{3}$'),
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_amount
        CHECK (
            "gross_amount_minor" > 0 AND
            (("allocation_resolved" AND "net_amount_minor" >= 0 AND "tax_amount_minor" >= 0
              AND "gross_amount_minor" = "net_amount_minor" + "tax_amount_minor") OR
             (NOT "allocation_resolved" AND "net_amount_minor" = 0 AND "tax_amount_minor" = 0))
        ),
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_confirmation
        CHECK (
            ("customer_confirmed_at" IS NULL OR "customer_confirmation_required") AND
            ("status_code" <> 'awaiting_confirmation' OR "customer_confirmation_required")
        ),
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_manual
        CHECK (
            ("status_code" = 'manual_required' AND "manual_reason_code" IN
                ('allocation_selection_required', 'provider_automation_unavailable',
                 'provider_definitive_failure', 'provider_result_unknown')) OR
            ("status_code" <> 'manual_required' AND "manual_reason_code" = '')
        ),
    ADD CONSTRAINT ck_plugin_idc_compliance_red_letter_task_complete
        CHECK (
            ("status_code" = 'completed' AND "completed_at" IS NOT NULL
             AND "allocation_resolved" AND "completion_evidence_no" IS NOT NULL) OR
            ("status_code" <> 'completed' AND "completed_at" IS NULL)
        );

DROP INDEX IF EXISTS uk_plugin_idc_compliance_red_letter_task_active;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_red_letter_task_refund
    ON plugin_idc_compliance_red_letter_task (
        "tenant_id", "refund_operation_no", "request_sha256"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_task_customer
    ON plugin_idc_compliance_red_letter_task (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

ALTER TABLE plugin_idc_compliance_red_letter_allocation
    ADD COLUMN IF NOT EXISTS "tax_invoice_item_no" VARCHAR(64);
ALTER TABLE plugin_idc_compliance_red_letter_allocation
    ALTER COLUMN "tax_invoice_item_no" SET NOT NULL;
ALTER TABLE plugin_idc_compliance_red_letter_allocation
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_compliance_red_letter_allocation_task,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_compliance_red_letter_allocation_item;
ALTER TABLE plugin_idc_compliance_red_letter_allocation
    ADD CONSTRAINT fk_plugin_idc_compliance_red_letter_allocation_task
        FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_compliance_red_letter_task ("task_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_compliance_red_letter_allocation
    ADD CONSTRAINT fk_plugin_idc_compliance_red_letter_allocation_item
        FOREIGN KEY ("tax_invoice_item_no")
        REFERENCES plugin_idc_compliance_tax_invoice_item ("item_no") ON DELETE RESTRICT;

-- Purpose: Persists business records for plugin_idc_compliance_red_letter_candidate.
-- 用途：持久化 plugin_idc_compliance_red_letter_candidate 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_red_letter_candidate (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "candidate_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "refund_allocation_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "tax_invoice_allocation_no" VARCHAR(64) NOT NULL,
    "tax_invoice_item_no" VARCHAR(64) NOT NULL,
    "requested_gross_minor" BIGINT NOT NULL,
    "available_gross_minor" BIGINT NOT NULL,
    "available_net_minor" BIGINT NOT NULL,
    "available_tax_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_red_letter_candidate_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_candidate_amount CHECK (
        "requested_gross_minor" > 0 AND "available_gross_minor" > 0
        AND "available_net_minor" >= 0 AND "available_tax_minor" >= 0
        AND "available_gross_minor" = "available_net_minor" + "available_tax_minor"
    ),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_candidate_no UNIQUE ("candidate_no"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_candidate_source UNIQUE (
        "task_no", "refund_allocation_no", "tax_invoice_allocation_no"
    ),
    CONSTRAINT fk_plugin_idc_compliance_red_letter_candidate_task
        FOREIGN KEY ("task_no") REFERENCES plugin_idc_compliance_red_letter_task ("task_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_compliance_rl_candidate_invoice_allocation
        FOREIGN KEY ("tax_invoice_allocation_no", "tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice_allocation ("allocation_no", "tax_invoice_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_red_letter_candidate_item
        FOREIGN KEY ("tax_invoice_item_no")
        REFERENCES plugin_idc_compliance_tax_invoice_item ("item_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_candidate_task
    ON plugin_idc_compliance_red_letter_candidate (
        "tenant_id", "task_no", "refund_allocation_no", "tax_invoice_allocation_no", "id"
    );

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_red_letter_candidate_append
    ON plugin_idc_compliance_red_letter_candidate;
CREATE TRIGGER trg_plugin_idc_compliance_red_letter_candidate_append
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_red_letter_candidate
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

-- Purpose: Persists business records for plugin_idc_compliance_red_letter_action.
-- 用途：持久化 plugin_idc_compliance_red_letter_action 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_red_letter_action (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "action_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "actor_type_code" VARCHAR(16) NOT NULL,
    "actor_user_id" INT,
    "before_status_code" VARCHAR(24) NOT NULL,
    "after_status_code" VARCHAR(24) NOT NULL,
    "before_version" BIGINT NOT NULL,
    "after_version" BIGINT NOT NULL,
    "evidence_no" VARCHAR(64),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_red_letter_action_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_action_code
        CHECK ("action_code" IN ('record_refund', 'resolve_allocation', 'customer_recheck', 'manual_complete')),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_action_actor
        CHECK (
            ("actor_type_code" = 'system' AND "actor_user_id" IS NULL) OR
            ("actor_type_code" IN ('staff', 'customer') AND "actor_user_id" > 0)
        ),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_action_digest
        CHECK ("idempotency_digest" ~ '^[0-9a-f]{64}$' AND "request_fingerprint" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_action_version
        CHECK ("before_version" > 0 AND "after_version" >= "before_version"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_action_no UNIQUE ("action_no"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_action_replay
        UNIQUE ("tenant_id", "task_no", "action_code", "idempotency_digest"),
    CONSTRAINT fk_plugin_idc_compliance_red_letter_action_task
        FOREIGN KEY ("task_no") REFERENCES plugin_idc_compliance_red_letter_task ("task_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_red_letter_action_evidence
        FOREIGN KEY ("evidence_no") REFERENCES plugin_idc_compliance_external_evidence ("evidence_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_action_task
    ON plugin_idc_compliance_red_letter_action ("tenant_id", "task_no", "created_at", "id");

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_red_letter_action_append
    ON plugin_idc_compliance_red_letter_action;
CREATE TRIGGER trg_plugin_idc_compliance_red_letter_action_append
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_red_letter_action
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

-- Purpose: Persists business records for plugin_idc_compliance_red_letter_intake_cursor.
-- 用途：持久化 plugin_idc_compliance_red_letter_intake_cursor 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_red_letter_intake_cursor (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "feed_code" VARCHAR(32) NOT NULL,
    "source_cursor" VARCHAR(64) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_rl_intake_cursor_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_rl_intake_cursor_feed CHECK ("feed_code" = 'posted_refund'),
    CONSTRAINT ck_idc_compliance_rl_intake_cursor_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_compliance_rl_intake_cursor_feed UNIQUE ("tenant_id", "feed_code")
);

COMMENT ON TABLE plugin_idc_compliance_red_letter_intake_cursor IS
    'Durable source cursor for the bounded posted-refund red-letter intake worker';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, '红字任务状态', 'idc_compliance_red_letter_status', 1, 1, FALSE,
     '合规插件稳定红字状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '红字人工处理原因', 'idc_compliance_red_letter_manual_reason', 1, 1, FALSE,
     '合规插件稳定红字人工原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_red_letter_status', '待处理', 'pending_review', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '需要你确认', 'awaiting_confirmation', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '待提交', 'queued', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '提交中', 'submitting', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '处理中', 'processing', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '已完成', 'completed', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '处理失败', 'failed', 7, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '结果待核实', 'result_unknown', 8, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '需要人工处理', 'manual_required', 9, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_status', '已取消', 'canceled', 10, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_manual_reason', '请选择原发票', 'allocation_selection_required', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_manual_reason', '当前需要人工红冲', 'provider_automation_unavailable', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_manual_reason', '供应商明确处理失败', 'provider_definitive_failure', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_red_letter_manual_reason', '供应商结果需要核实', 'provider_result_unknown', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON TABLE plugin_idc_compliance_red_letter_action IS
    'Append-only idempotency and transition evidence for refund red-letter workflows';
COMMENT ON TABLE plugin_idc_compliance_red_letter_candidate IS
    'Append-only eligible original-invoice choices used only when refund allocation is not unique';
