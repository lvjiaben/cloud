-- 024: Merchant checkout policy, durable order cancellation/review evidence,
-- and reversible unpaid-order marketing consumption.

-- Purpose: Single-install merchant checkout deadline policy snapshotted by each order.
-- 用途：保存交易结账策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_checkout_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL DEFAULT 'default',
    "payment_validity_seconds" INT NOT NULL DEFAULT 1800,
    "version" BIGINT NOT NULL DEFAULT 1,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_checkout_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_checkout_policy_identity CHECK (
        "policy_no" = 'default' AND "version" > 0 AND "updated_by" >= 0
    ),
    CONSTRAINT ck_idc_com_checkout_policy_validity CHECK (
        "payment_validity_seconds" BETWEEN 300 AND 86400
    ),
    CONSTRAINT uk_idc_com_checkout_policy_tenant UNIQUE ("tenant_id"),
    CONSTRAINT uk_idc_com_checkout_policy_no UNIQUE ("policy_no")
);

INSERT INTO plugin_idc_core_commerce_checkout_policy (
    "tenant_id", "policy_no", "payment_validity_seconds", "version", "updated_by"
) VALUES (0, 'default', 1800, 1, 0)
ON CONFLICT ("tenant_id") DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_order_owner
    ON plugin_idc_core_commerce_order ("order_no", "customer_no");

CREATE INDEX IF NOT EXISTS idx_idc_com_order_lifecycle_expiry
    ON plugin_idc_core_commerce_order (
        "tenant_id", "payment_due_at", "id"
    ) WHERE "status_code" IN ('pending_payment', 'payment_processing', 'manual_review');

-- Purpose: Durable idempotent order cancellation intent and payment-closure outcome.
-- 用途：保存交易订单取消相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_cancellation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "cancellation_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(32) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL,
    "payment_closure_outcome_code" VARCHAR(24),
    "payment_closure_reference" VARCHAR(128),
    "payment_closure_digest" CHAR(64),
    "requested_by" INT NOT NULL DEFAULT 0,
    "requested_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_order_cancel_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_order_cancel_source CHECK (
        "source_code" IN ('customer', 'system', 'admin', 'payment')
    ),
    CONSTRAINT ck_idc_com_order_cancel_reason CHECK (
        "reason_code" IN (
            'customer_requested', 'payment_expired', 'risk_rejected', 'manual_rejected'
        )
    ),
    CONSTRAINT ck_idc_com_order_cancel_request CHECK (
        "request_digest" ~ '^[0-9a-f]{64}$' AND "requested_by" >= 0 AND "version" > 0
    ),
    CONSTRAINT ck_idc_com_order_cancel_status CHECK (
        "status_code" IN ('pending_payment_closure', 'completed', 'manual_review')
    ),
    CONSTRAINT ck_idc_com_order_cancel_closure_digest CHECK (
        "payment_closure_digest" IS NULL
        OR "payment_closure_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_com_order_cancel_lifecycle CHECK (
        ("status_code" = 'pending_payment_closure'
            AND "payment_closure_reference" IS NULL
            AND "payment_closure_digest" IS NULL
            AND "completed_at" IS NULL)
        OR ("status_code" = 'completed' AND "completed_at" IS NOT NULL)
        OR ("status_code" = 'manual_review'
            AND "payment_closure_reference" IS NOT NULL
            AND "payment_closure_digest" IS NOT NULL
            AND "completed_at" IS NULL)
    ),
    CONSTRAINT uk_idc_com_order_cancel_no UNIQUE ("cancellation_no"),
    CONSTRAINT uk_idc_com_order_cancel_order UNIQUE ("order_no"),
    CONSTRAINT uk_idc_com_order_cancel_request UNIQUE (
        "customer_no", "request_digest"
    ),
    CONSTRAINT fk_idc_com_order_cancel_order FOREIGN KEY ("order_no", "customer_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no", "customer_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_order_cancel_payment_work
    ON plugin_idc_core_commerce_order_cancellation (
        "tenant_id", "status_code", "requested_at", "id"
    ) WHERE "status_code" = 'pending_payment_closure';

ALTER TABLE plugin_idc_core_commerce_order_cancellation
    ADD COLUMN IF NOT EXISTS "payment_closure_outcome_code" VARCHAR(24);
ALTER TABLE plugin_idc_core_commerce_order_cancellation
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_cancel_closure_outcome;
ALTER TABLE plugin_idc_core_commerce_order_cancellation
    ADD CONSTRAINT ck_idc_com_order_cancel_closure_outcome CHECK (
        "payment_closure_outcome_code" IS NULL
        OR "payment_closure_outcome_code" IN (
            'closed', 'no_attempts', 'confirmed_success', 'unknown'
        )
    );
ALTER TABLE plugin_idc_core_commerce_order_cancellation
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_cancel_lifecycle;
ALTER TABLE plugin_idc_core_commerce_order_cancellation
    ADD CONSTRAINT ck_idc_com_order_cancel_lifecycle CHECK (
        ("status_code" = 'pending_payment_closure'
            AND "payment_closure_outcome_code" IS NULL
            AND "payment_closure_reference" IS NULL
            AND "payment_closure_digest" IS NULL
            AND "completed_at" IS NULL)
        OR ("status_code" = 'completed' AND "completed_at" IS NOT NULL
            AND (
                ("payment_closure_outcome_code" IS NULL
                    AND "payment_closure_reference" IS NULL
                    AND "payment_closure_digest" IS NULL)
                OR ("payment_closure_outcome_code" IN ('closed', 'no_attempts')
                    AND "payment_closure_reference" IS NOT NULL
                    AND "payment_closure_digest" IS NOT NULL)
            ))
        OR ("status_code" = 'manual_review'
            AND "payment_closure_outcome_code" IN ('confirmed_success', 'unknown')
            AND "payment_closure_reference" IS NOT NULL
            AND "payment_closure_digest" IS NOT NULL
            AND "completed_at" IS NULL)
    );

-- Purpose: Independent staff risk review evidence that cannot bypass other manual gates.
-- 用途：保存交易订单复核相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_order_review (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "review_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "review_type_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "reason_code" VARCHAR(32) NOT NULL DEFAULT '',
    "request_digest" CHAR(64),
    "evidence_digest" CHAR(64),
    "reviewed_by" INT,
    "requested_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "reviewed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_order_review_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_order_review_type CHECK ("review_type_code" = 'risk'),
    CONSTRAINT ck_idc_com_order_review_status CHECK (
        "status_code" IN ('pending', 'approved', 'rejected', 'cancelled')
    ),
    CONSTRAINT ck_idc_com_order_review_digests CHECK (
        ("request_digest" IS NULL OR "request_digest" ~ '^[0-9a-f]{64}$')
        AND ("evidence_digest" IS NULL OR "evidence_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_com_order_review_lifecycle CHECK (
        ("status_code" = 'pending' AND "reason_code" = ''
            AND "request_digest" IS NULL AND "evidence_digest" IS NULL
            AND "reviewed_by" IS NULL AND "reviewed_at" IS NULL)
        OR ("status_code" IN ('approved', 'rejected')
            AND "reason_code" IN ('risk_approved', 'risk_rejected')
            AND "request_digest" IS NOT NULL AND "evidence_digest" IS NOT NULL
            AND "reviewed_by" > 0 AND "reviewed_at" IS NOT NULL)
        OR ("status_code" = 'cancelled'
            AND "reason_code" IN ('order_cancelled', 'payment_expired')
            AND "request_digest" IS NOT NULL AND "evidence_digest" IS NOT NULL
            AND "reviewed_by" IS NULL AND "reviewed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_com_order_review_no UNIQUE ("review_no"),
    CONSTRAINT uk_idc_com_order_review_type UNIQUE ("order_no", "review_type_code"),
    CONSTRAINT fk_idc_com_order_review_order FOREIGN KEY ("order_no", "customer_no")
        REFERENCES plugin_idc_core_commerce_order ("order_no", "customer_no")
        ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_commerce_order_review
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_review_status;
ALTER TABLE plugin_idc_core_commerce_order_review
    ADD CONSTRAINT ck_idc_com_order_review_status CHECK (
        "status_code" IN ('pending', 'approved', 'rejected', 'cancelled')
    );
ALTER TABLE plugin_idc_core_commerce_order_review
    DROP CONSTRAINT IF EXISTS ck_idc_com_order_review_lifecycle;
ALTER TABLE plugin_idc_core_commerce_order_review
    ADD CONSTRAINT ck_idc_com_order_review_lifecycle CHECK (
        ("status_code" = 'pending' AND "reason_code" = ''
            AND "request_digest" IS NULL AND "evidence_digest" IS NULL
            AND "reviewed_by" IS NULL AND "reviewed_at" IS NULL)
        OR ("status_code" IN ('approved', 'rejected')
            AND "reason_code" IN ('risk_approved', 'risk_rejected')
            AND "request_digest" IS NOT NULL AND "evidence_digest" IS NOT NULL
            AND "reviewed_by" > 0 AND "reviewed_at" IS NOT NULL)
        OR ("status_code" = 'cancelled'
            AND "reason_code" IN ('order_cancelled', 'payment_expired')
            AND "request_digest" IS NOT NULL AND "evidence_digest" IS NOT NULL
            AND "reviewed_by" IS NULL AND "reviewed_at" IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_order_review_work
    ON plugin_idc_core_commerce_order_review (
        "tenant_id", "status_code", "review_type_code", "requested_at", "id"
    ) WHERE "status_code" = 'pending';

ALTER TABLE plugin_idc_core_marketing_campaign_reservation
    ADD COLUMN IF NOT EXISTS "reversed_at" TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_marketing_campaign_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_campaign_res_status;
ALTER TABLE plugin_idc_core_marketing_campaign_reservation
    ADD CONSTRAINT ck_idc_marketing_campaign_res_status CHECK (
        "status_code" IN ('active', 'consumed', 'released', 'expired', 'reversed')
    );
ALTER TABLE plugin_idc_core_marketing_campaign_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_campaign_res_terminal;
ALTER TABLE plugin_idc_core_marketing_campaign_reservation
    ADD CONSTRAINT ck_idc_marketing_campaign_res_terminal CHECK (
        ("status_code" = 'active' AND "consumed_at" IS NULL
            AND "released_at" IS NULL AND "reversed_at" IS NULL)
        OR ("status_code" = 'consumed' AND "consumed_at" >= "created_at"
            AND "released_at" IS NULL AND "reversed_at" IS NULL)
        OR ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL
            AND "released_at" >= "created_at" AND "reversed_at" IS NULL)
        OR ("status_code" = 'reversed' AND "consumed_at" >= "created_at"
            AND "released_at" IS NULL AND "reversed_at" >= "consumed_at")
    );

ALTER TABLE plugin_idc_core_marketing_asset_reservation
    ADD COLUMN IF NOT EXISTS "reversed_at" TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_marketing_asset_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_asset_res_status;
ALTER TABLE plugin_idc_core_marketing_asset_reservation
    ADD CONSTRAINT ck_idc_marketing_asset_res_status CHECK (
        "status_code" IN ('active', 'consumed', 'released', 'expired', 'reversed')
    );
ALTER TABLE plugin_idc_core_marketing_asset_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_asset_res_terminal;
ALTER TABLE plugin_idc_core_marketing_asset_reservation
    ADD CONSTRAINT ck_idc_marketing_asset_res_terminal CHECK (
        ("status_code" = 'active' AND "consumed_at" IS NULL
            AND "released_at" IS NULL AND "reversed_at" IS NULL)
        OR ("status_code" = 'consumed' AND "consumed_at" >= "created_at"
            AND "released_at" IS NULL AND "reversed_at" IS NULL)
        OR ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL
            AND "released_at" >= "created_at" AND "reversed_at" IS NULL)
        OR ("status_code" = 'reversed' AND "consumed_at" >= "created_at"
            AND "released_at" IS NULL AND "reversed_at" >= "consumed_at")
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_protect_order_lifecycle_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        NEW."tenant_id", NEW."cancellation_no", NEW."order_no", NEW."customer_no",
        NEW."source_code", NEW."reason_code", NEW."request_digest",
        NEW."requested_by", NEW."requested_at", NEW."created_at"
    ) IS DISTINCT FROM ROW(
        OLD."tenant_id", OLD."cancellation_no", OLD."order_no", OLD."customer_no",
        OLD."source_code", OLD."reason_code", OLD."request_digest",
        OLD."requested_by", OLD."requested_at", OLD."created_at"
    ) THEN
        RAISE EXCEPTION 'order cancellation identity is immutable';
    END IF;
    IF OLD."status_code" IN ('completed', 'manual_review') AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'terminal order cancellation is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_com_order_cancel_identity
    ON plugin_idc_core_commerce_order_cancellation;
CREATE TRIGGER trg_idc_com_order_cancel_identity
BEFORE UPDATE ON plugin_idc_core_commerce_order_cancellation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_protect_order_lifecycle_identity();

CREATE OR REPLACE FUNCTION plugin_idc_core_protect_order_review_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        NEW."tenant_id", NEW."review_no", NEW."order_no", NEW."customer_no",
        NEW."review_type_code", NEW."requested_at", NEW."created_at"
    ) IS DISTINCT FROM ROW(
        OLD."tenant_id", OLD."review_no", OLD."order_no", OLD."customer_no",
        OLD."review_type_code", OLD."requested_at", OLD."created_at"
    ) THEN
        RAISE EXCEPTION 'order review identity is immutable';
    END IF;
    IF OLD."status_code" IN ('approved', 'rejected', 'cancelled') AND NEW IS DISTINCT FROM OLD THEN
        RAISE EXCEPTION 'terminal order review is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_com_order_review_identity
    ON plugin_idc_core_commerce_order_review;
CREATE TRIGGER trg_idc_com_order_review_identity
BEFORE UPDATE ON plugin_idc_core_commerce_order_review
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_protect_order_review_identity();

COMMENT ON TABLE plugin_idc_core_commerce_checkout_policy
    IS 'Single-install merchant checkout deadline policy snapshotted by each order';
COMMENT ON TABLE plugin_idc_core_commerce_order_cancellation
    IS 'Durable idempotent order cancellation intent and payment-closure outcome';
COMMENT ON TABLE plugin_idc_core_commerce_order_review
    IS 'Independent staff risk review evidence that cannot bypass other manual gates';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 订单取消来源', 'idc_commerce_cancellation_source', 1, 1, FALSE,
     '订单取消意图的可信来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单取消原因', 'idc_commerce_cancellation_reason', 1, 1, FALSE,
     '订单取消的稳定业务原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单取消状态', 'idc_commerce_cancellation_status', 1, 1, FALSE,
     '订单取消与支付关单协同状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单审核类型', 'idc_commerce_order_review_type', 1, 1, FALSE,
     '互不替代的订单人工审核类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单审核状态', 'idc_commerce_order_review_status', 1, 1, FALSE,
     '订单人工审核生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 订单审核原因', 'idc_commerce_order_review_reason', 1, 1, FALSE,
     '订单人工审核决定原因', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_commerce_cancellation_source', '客户', 'customer', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_source', '系统', 'system', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_source', '管理员', 'admin', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_source', '支付系统', 'payment', 4, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_reason', '客户主动取消', 'customer_requested', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_reason', '付款超时', 'payment_expired', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_reason', '风险审核拒绝', 'risk_rejected', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_reason', '人工审核拒绝', 'manual_rejected', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_status', '等待支付关单', 'pending_payment_closure', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_status', '已完成', 'completed', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_cancellation_status', '需要人工核对', 'manual_review', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_type', '风险审核', 'risk', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_status', '待审核', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_status', '已批准', 'approved', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_status', '已拒绝', 'rejected', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_status', '已取消', 'cancelled', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_reason', '风险审核通过', 'risk_approved', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_reason', '风险审核拒绝', 'risk_rejected', 2, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_reason', '订单已取消', 'order_cancelled', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_order_review_reason', '付款已超时', 'payment_expired', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
