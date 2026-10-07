-- Recoverable customer-owned payment preparation context. Verification facts
-- remain customer/compliance-owned and are re-read before every ready result.

-- Purpose: Recoverable payment preparation projection; current identity and order facts must be re-evaluated before payment.
-- 用途：保存交易支付上下文相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_payment_context (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "context_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "identity_status_code" VARCHAR(24) NOT NULL,
    "identity_case_no" VARCHAR(40) NOT NULL DEFAULT '',
    "identity_version" BIGINT NOT NULL DEFAULT 0,
    "first_requested_at" TIMESTAMPTZ NOT NULL,
    "last_evaluated_at" TIMESTAMPTZ NOT NULL,
    "ready_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_payment_context_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_payment_context_customer_type CHECK (
        "customer_type_code" IN ('individual', 'enterprise')
    ),
    CONSTRAINT ck_idc_com_payment_context_status CHECK (
        "status_code" IN (
            'awaiting_identity', 'awaiting_order_review', 'ready',
            'expired', 'cancelled', 'closed'
        )
    ),
    CONSTRAINT ck_idc_com_payment_context_reason CHECK (
        "reason_code" IN (
            'identity_unverified', 'identity_pending', 'identity_rejected',
            'identity_expired', 'identity_revoked', 'order_review_required',
            'payment_ready', 'payment_expired', 'order_cancelled',
            'order_not_payable'
        )
    ),
    CONSTRAINT ck_idc_com_payment_context_identity_status CHECK (
        "identity_status_code" IN (
            'unverified', 'pending', 'verified', 'rejected', 'expired', 'revoked'
        )
    ),
    CONSTRAINT ck_idc_com_payment_context_identity_version CHECK ("identity_version" >= 0),
    CONSTRAINT ck_idc_com_payment_context_timeline CHECK (
        "last_evaluated_at" >= "first_requested_at"
    ),
    CONSTRAINT ck_idc_com_payment_context_ready CHECK (
        (
            "status_code" = 'ready' AND
            "reason_code" = 'payment_ready' AND
            "identity_status_code" = 'verified' AND
            "identity_case_no" <> '' AND
            "identity_version" > 0 AND
            "ready_at" IS NOT NULL
        ) OR (
            "status_code" <> 'ready' AND "ready_at" IS NULL
        )
    ),
    CONSTRAINT uk_idc_com_payment_context_no UNIQUE ("context_no"),
    CONSTRAINT uk_idc_com_payment_context_order UNIQUE ("order_no"),
    CONSTRAINT fk_idc_com_payment_context_order_owner FOREIGN KEY ("order_no", "customer_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no", "customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_payment_context_invoice FOREIGN KEY ("invoice_no")
        REFERENCES plugin_idc_core_billing_invoice ("invoice_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_payment_context_customer FOREIGN KEY ("customer_no", "customer_type_code")
        REFERENCES plugin_idc_core_customer_subject ("customer_no", "customer_type_code") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_payment_context_customer
    ON plugin_idc_core_commerce_payment_context (
        "tenant_id", "customer_no", "status_code", "last_evaluated_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_com_payment_context_expiry
    ON plugin_idc_core_commerce_payment_context (
        "tenant_id", "expires_at", "id"
    ) WHERE "status_code" IN ('awaiting_identity', 'awaiting_order_review', 'ready');

COMMENT ON TABLE plugin_idc_core_commerce_payment_context IS
    'Recoverable payment preparation projection; current identity and order facts must be re-evaluated before payment';
COMMENT ON COLUMN plugin_idc_core_commerce_payment_context."identity_case_no" IS
    'Opaque compliance-owned case reference from the most recent evaluation, never raw identity material';

CREATE OR REPLACE FUNCTION plugin_idc_core_protect_payment_context_identity()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
       OR NEW."context_no" IS DISTINCT FROM OLD."context_no"
       OR NEW."order_no" IS DISTINCT FROM OLD."order_no"
       OR NEW."invoice_no" IS DISTINCT FROM OLD."invoice_no"
       OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
       OR NEW."customer_type_code" IS DISTINCT FROM OLD."customer_type_code"
       OR NEW."first_requested_at" IS DISTINCT FROM OLD."first_requested_at"
       OR NEW."expires_at" IS DISTINCT FROM OLD."expires_at"
       OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'payment context commercial identity is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_com_payment_context_identity
    ON plugin_idc_core_commerce_payment_context;
CREATE TRIGGER trg_idc_com_payment_context_identity
BEFORE UPDATE ON plugin_idc_core_commerce_payment_context
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_protect_payment_context_identity();

INSERT INTO sys_dict_type (
    "tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at"
) VALUES
    (0, '付款准备上下文状态', 'idc_commerce_payment_context_status', 1, 1, FALSE, '订单付款准备与实名恢复上下文状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '付款准备阻断原因', 'idc_commerce_payment_context_reason', 1, 1, FALSE, '付款准备的稳定阻断或放行原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at"
) VALUES
    (0, 'idc_commerce_payment_context_status', '等待实名认证', 'awaiting_identity', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_status', '等待订单审核', 'awaiting_order_review', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_status', '可以付款', 'ready', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_status', '已过期', 'expired', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_status', '已取消', 'cancelled', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_status', '已关闭', 'closed', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '尚未实名认证', 'identity_unverified', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '实名认证处理中', 'identity_pending', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '实名认证未通过', 'identity_rejected', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '实名认证已过期', 'identity_expired', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '实名认证已撤销', 'identity_revoked', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '订单仍需独立审核', 'order_review_required', 6, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '订单可以付款', 'payment_ready', 7, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '付款期限已过', 'payment_expired', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '订单已取消', 'order_cancelled', 9, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_payment_context_reason', '订单当前不可付款', 'order_not_payable', 10, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
