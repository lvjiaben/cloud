-- Phase 10 supplier-neutral tax-invoice request persistence.
-- The migration keeps customer input immutable, makes source-item ownership
-- queryable for capacity checks, and leaves provider-specific fields for the
-- separately gated review and submission tasks.

ALTER TABLE plugin_idc_compliance_tax_invoice
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_tax_invoice_type;

ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_type
    CHECK (
        "invoice_type_code" IN (
            'pending_confirmation',
            'vat_electronic_normal', 'vat_electronic_special',
            'digital_electronic_normal', 'digital_electronic_special'
        )
    );

ALTER TABLE plugin_idc_compliance_tax_invoice
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_tax_invoice_status;

ALTER TABLE plugin_idc_compliance_tax_invoice
    ALTER COLUMN "status_code" TYPE VARCHAR(32);

ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_status
    CHECK (
        "status_code" IN (
            'draft', 'pending_review', 'approved_pending_submission', 'rejected',
            'queued', 'submitting',
            'issued', 'delivered', 'failed', 'result_unknown',
            'red_letter_pending', 'red_lettered', 'canceled'
        )
    );

ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD COLUMN IF NOT EXISTS "decision_reason_code" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "decision_internal_note" VARCHAR(2000) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "reviewed_by_user_id" INT,
    ADD COLUMN IF NOT EXISTS "reviewed_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "provider_not_issued_confirmed_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_compliance_tax_invoice
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_tax_invoice_decision;

ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_decision
    CHECK (
        (
            "status_code" IN ('rejected', 'canceled') AND
            "decision_reason_code" IN (
                'buyer_or_tax_information_incorrect',
                'amount_or_billing_invoice_adjustment_required',
                'duplicate_request', 'business_not_invoiceable',
                'customer_requested_cancellation', 'order_or_refund_changed', 'other'
            )
        ) OR (
            "status_code" NOT IN ('rejected', 'canceled') AND
            "decision_reason_code" = ''
        )
    );

ALTER TABLE plugin_idc_compliance_tax_invoice
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_tax_invoice_not_issued;

ALTER TABLE plugin_idc_compliance_tax_invoice
    ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_not_issued
    CHECK (
        "provider_not_issued_confirmed_at" IS NULL OR
        "status_code" IN ('failed', 'canceled')
    );

DROP INDEX IF EXISTS idx_plugin_idc_compliance_tax_invoice_work;
CREATE INDEX idx_plugin_idc_compliance_tax_invoice_work
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "status_code", "submitted_at", "id"
    ) WHERE "status_code" IN (
        'pending_review', 'approved_pending_submission', 'queued',
        'submitting', 'result_unknown', 'red_letter_pending'
    );

-- Purpose: Persists business records for plugin_idc_compliance_tax_invoice_action.
-- 用途：持久化 plugin_idc_compliance_tax_invoice_action 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_tax_invoice_action (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "action_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_fingerprint" CHAR(64) NOT NULL,
    "actor_user_id" INT NOT NULL,
    "before_status_code" VARCHAR(32) NOT NULL,
    "after_status_code" VARCHAR(32) NOT NULL,
    "before_version" BIGINT NOT NULL,
    "after_version" BIGINT NOT NULL,
    "invoice_type_code" VARCHAR(32) NOT NULL DEFAULT '',
    "provider_code" VARCHAR(32) NOT NULL DEFAULT '',
    "reason_code" VARCHAR(64) NOT NULL DEFAULT '',
    "internal_note" VARCHAR(2000) NOT NULL DEFAULT '',
    "capability_reason_code" VARCHAR(64) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_code
        CHECK ("action_code" IN ('approve', 'reject', 'submit', 'cancel')),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_digest
        CHECK (
            "idempotency_digest" ~ '^[0-9a-f]{64}$' AND
            "request_fingerprint" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_actor
        CHECK ("actor_user_id" > 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_version
        CHECK ("before_version" > 0 AND "after_version" = "before_version" + 1),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_action_reason
        CHECK (
            (
                "action_code" IN ('reject', 'cancel') AND
                "reason_code" IN (
                    'buyer_or_tax_information_incorrect',
                    'amount_or_billing_invoice_adjustment_required',
                    'duplicate_request', 'business_not_invoiceable',
                    'customer_requested_cancellation', 'order_or_refund_changed', 'other'
                )
            ) OR ("action_code" IN ('approve', 'submit') AND "reason_code" = '')
        ),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_action_no
        UNIQUE ("action_no"),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_action_replay
        UNIQUE ("tax_invoice_no", "action_code", "idempotency_digest"),
    CONSTRAINT fk_plugin_idc_compliance_tax_invoice_action_invoice
        FOREIGN KEY ("tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice ("tax_invoice_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_action_timeline
    ON plugin_idc_compliance_tax_invoice_action (
        "tenant_id", "tax_invoice_no", "created_at", "id"
    );

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_tax_inv_action_append
    ON plugin_idc_compliance_tax_invoice_action;
CREATE TRIGGER trg_plugin_idc_compliance_tax_inv_action_append
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_tax_invoice_action
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

ALTER TABLE plugin_idc_compliance_tax_invoice_item
    ADD COLUMN IF NOT EXISTS "allocation_no" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "billing_invoice_item_no" VARCHAR(64);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_compliance_tax_invoice_item
        WHERE "allocation_no" IS NULL OR "billing_invoice_item_no" IS NULL
    ) THEN
        RAISE EXCEPTION 'tax-invoice source-item ownership must be populated before migration';
    END IF;

    ALTER TABLE plugin_idc_compliance_tax_invoice_item
        ALTER COLUMN "allocation_no" SET NOT NULL,
        ALTER COLUMN "billing_invoice_item_no" SET NOT NULL;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_compliance_tax_invoice_item_source'
    ) THEN
        ALTER TABLE plugin_idc_compliance_tax_invoice_item
            ADD CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_source
            CHECK ("allocation_no" <> '' AND "billing_invoice_item_no" <> '');
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_plugin_idc_compliance_tax_invoice_item_source'
    ) THEN
        ALTER TABLE plugin_idc_compliance_tax_invoice_item
            ADD CONSTRAINT uk_plugin_idc_compliance_tax_invoice_item_source
            UNIQUE ("tax_invoice_no", "billing_invoice_item_no");
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_plugin_idc_compliance_tax_invoice_item_allocation'
    ) THEN
        ALTER TABLE plugin_idc_compliance_tax_invoice_item
            ADD CONSTRAINT fk_plugin_idc_compliance_tax_invoice_item_allocation
            FOREIGN KEY ("allocation_no", "tax_invoice_no")
            REFERENCES plugin_idc_compliance_tax_invoice_allocation
                ("allocation_no", "tax_invoice_no")
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_item_usage
    ON plugin_idc_compliance_tax_invoice_item (
        "tenant_id", "allocation_no", "billing_invoice_item_no", "tax_invoice_no"
    );

CREATE OR REPLACE FUNCTION plugin_idc_compliance_lock_tax_invoice_request()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'tax invoice legal records cannot be deleted';
    END IF;

    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
        OR NEW."tax_invoice_no" IS DISTINCT FROM OLD."tax_invoice_no"
        OR NEW."request_sha256" IS DISTINCT FROM OLD."request_sha256"
        OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
        OR NEW."principal_no" IS DISTINCT FROM OLD."principal_no"
        OR NEW."seller_snapshot_ref" IS DISTINCT FROM OLD."seller_snapshot_ref"
        OR NEW."seller_snapshot_sha256" IS DISTINCT FROM OLD."seller_snapshot_sha256"
        OR NEW."buyer_snapshot_ref" IS DISTINCT FROM OLD."buyer_snapshot_ref"
        OR NEW."buyer_snapshot_sha256" IS DISTINCT FROM OLD."buyer_snapshot_sha256"
        OR NEW."tax_currency_code" IS DISTINCT FROM OLD."tax_currency_code"
        OR NEW."gross_amount_minor" IS DISTINCT FROM OLD."gross_amount_minor"
        OR NEW."net_amount_minor" IS DISTINCT FROM OLD."net_amount_minor"
        OR NEW."tax_amount_minor" IS DISTINCT FROM OLD."tax_amount_minor"
        OR NEW."billing_currency_code" IS DISTINCT FROM OLD."billing_currency_code"
        OR NEW."exchange_rate_numerator" IS DISTINCT FROM OLD."exchange_rate_numerator"
        OR NEW."exchange_rate_denominator" IS DISTINCT FROM OLD."exchange_rate_denominator"
        OR NEW."exchange_rate_date" IS DISTINCT FROM OLD."exchange_rate_date"
        OR NEW."exchange_rate_source_code" IS DISTINCT FROM OLD."exchange_rate_source_code"
    THEN
        RAISE EXCEPTION 'tax invoice request snapshot is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_tax_invoice_request_immutable
    ON plugin_idc_compliance_tax_invoice;
CREATE TRIGGER trg_plugin_idc_compliance_tax_invoice_request_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_tax_invoice
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_lock_tax_invoice_request();

CREATE OR REPLACE FUNCTION plugin_idc_compliance_lock_tax_invoice_allocation()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'tax invoice allocations cannot be deleted';
    END IF;

    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
        OR NEW."allocation_no" IS DISTINCT FROM OLD."allocation_no"
        OR NEW."tax_invoice_no" IS DISTINCT FROM OLD."tax_invoice_no"
        OR NEW."billing_invoice_no" IS DISTINCT FROM OLD."billing_invoice_no"
        OR NEW."order_no" IS DISTINCT FROM OLD."order_no"
        OR NEW."allocated_gross_minor" IS DISTINCT FROM OLD."allocated_gross_minor"
        OR NEW."allocated_net_minor" IS DISTINCT FROM OLD."allocated_net_minor"
        OR NEW."allocated_tax_minor" IS DISTINCT FROM OLD."allocated_tax_minor"
    THEN
        RAISE EXCEPTION 'tax invoice allocation source is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_tax_alloc_immutable
    ON plugin_idc_compliance_tax_invoice_allocation;
CREATE TRIGGER trg_plugin_idc_compliance_tax_alloc_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_tax_invoice_allocation
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_lock_tax_invoice_allocation();

CREATE OR REPLACE FUNCTION plugin_idc_compliance_lock_tax_invoice_item()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'tax invoice item snapshots cannot be changed or deleted';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_tax_invoice_item_immutable
    ON plugin_idc_compliance_tax_invoice_item;
CREATE TRIGGER trg_plugin_idc_compliance_tax_invoice_item_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_tax_invoice_item
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_lock_tax_invoice_item();

COMMENT ON COLUMN plugin_idc_compliance_tax_invoice."invoice_type_code" IS
    'Customer requests use pending_confirmation; an authorized reviewer selects a supported type later';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice."decision_internal_note" IS
    'Optional administrator-only note; customer projections must never expose it';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice."provider_not_issued_confirmed_at" IS
    'Authoritative positive evidence required before safely canceling a failed provider attempt';
COMMENT ON TABLE plugin_idc_compliance_tax_invoice_action IS
    'Append-only exact review, submission-queue, and safe-cancellation decisions';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice_item."allocation_no" IS
    'Immutable local allocation owning this source tax item';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice_item."billing_invoice_item_no" IS
    'Opaque core billing item reference used only for bounded capacity accounting';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, '开票申请状态', 'idc_compliance_tax_invoice_status', 1, 1, FALSE,
     '客户开票申请从审核到交付及红字处理的稳定状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '开票申请关闭原因', 'idc_compliance_tax_invoice_decision_reason', 1, 1, FALSE,
     '后台驳回或安全取消开票申请时面向客户的稳定原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '发票票种', 'idc_compliance_tax_invoice_type', 1, 1, FALSE,
     '客户待后台确认及供应商能力门禁允许的发票票种', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status",
     "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_compliance_tax_invoice_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '待审核', 'pending_review', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '待提交开票', 'approved_pending_submission', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '已驳回', 'rejected', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '排队提交中', 'queued', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '正在提交', 'submitting', 6, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '已开票', 'issued', 7, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '已交付', 'delivered', 8, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '开票失败', 'failed', 9, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '结果待确认', 'result_unknown', 10, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '红字处理中', 'red_letter_pending', 11, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '已红冲', 'red_lettered', 12, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_status', '已取消', 'canceled', 13, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '抬头或税务信息有误', 'buyer_or_tax_information_incorrect', 1, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '金额或账单需要调整', 'amount_or_billing_invoice_adjustment_required', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '重复申请', 'duplicate_request', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '当前业务无法开具', 'business_not_invoiceable', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '客户要求取消', 'customer_requested_cancellation', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '订单或退款情况变化', 'order_or_refund_changed', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_decision_reason', '其他原因', 'other', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_type', '待后台确认', 'pending_confirmation', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_type', '增值税电子普通发票', 'vat_electronic_normal', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_type', '增值税电子专用发票', 'vat_electronic_special', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_type', '数电普通发票', 'digital_electronic_normal', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_compliance_tax_invoice_type', '数电专用发票', 'digital_electronic_special', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
