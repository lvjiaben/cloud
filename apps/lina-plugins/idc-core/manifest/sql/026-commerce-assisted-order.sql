-- Assisted quotes keep staff intent, approval evidence, and authoritative
-- checkout facts separate. They never permit paid or service-state overrides.

-- Purpose: Staff-assisted quote intent with independent approval and prepayment-only terms.
-- 用途：保存交易协助报价申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_assisted_quote_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_approval',
    "item_count" INT NOT NULL,
    "manual_discount_minor" BIGINT NOT NULL DEFAULT 0,
    "payment_term_code" VARCHAR(24) NOT NULL DEFAULT 'standard_prepay',
    "payment_due_seconds" INT NOT NULL,
    "reason" VARCHAR(1000) NOT NULL,
    "internal_note" VARCHAR(2000) NOT NULL DEFAULT '',
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "requested_by" INT NOT NULL,
    "decision_code" VARCHAR(16),
    "decision_by" INT,
    "decision_at" TIMESTAMPTZ,
    "decision_reason" VARCHAR(1000),
    "decision_request_digest" CHAR(64),
    "decision_evidence_digest" CHAR(64),
    "quote_no" VARCHAR(40),
    "order_no" VARCHAR(40),
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_assisted_request_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_assisted_request_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_com_assisted_request_status CHECK (
        "status_code" IN ('pending_approval', 'approved', 'rejected', 'ordered')
    ),
    CONSTRAINT ck_idc_com_assisted_request_items CHECK ("item_count" BETWEEN 1 AND 8),
    CONSTRAINT ck_idc_com_assisted_request_discount CHECK ("manual_discount_minor" >= 0),
    CONSTRAINT ck_idc_com_assisted_request_term CHECK (
        ("payment_term_code" = 'standard_prepay'
            AND "payment_due_seconds" BETWEEN 300 AND 86400)
        OR ("payment_term_code" = 'extended_prepay'
            AND "payment_due_seconds" BETWEEN 86401 AND 604800)
    ),
    CONSTRAINT ck_idc_com_assisted_request_text CHECK (
        length(btrim("reason")) BETWEEN 10 AND 1000
        AND length("internal_note") <= 2000
        AND ("decision_reason" IS NULL
            OR length(btrim("decision_reason")) BETWEEN 2 AND 1000)
    ),
    CONSTRAINT ck_idc_com_assisted_request_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND ("decision_request_digest" IS NULL
            OR "decision_request_digest" ~ '^[0-9a-f]{64}$')
        AND ("decision_evidence_digest" IS NULL
            OR "decision_evidence_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_com_assisted_request_actor CHECK (
        "requested_by" > 0
        AND ("decision_by" IS NULL OR "decision_by" > 0)
        AND ("decision_by" IS NULL OR "decision_by" <> "requested_by")
    ),
    CONSTRAINT ck_idc_com_assisted_request_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT ck_idc_com_assisted_request_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_com_assisted_request_lifecycle CHECK (
        ("status_code" = 'pending_approval'
            AND "decision_code" IS NULL AND "decision_by" IS NULL
            AND "decision_at" IS NULL AND "decision_reason" IS NULL
            AND "decision_request_digest" IS NULL
            AND "decision_evidence_digest" IS NULL
            AND "quote_no" IS NULL AND "order_no" IS NULL)
        OR ("status_code" = 'approved'
            AND "decision_code" = 'approved' AND "decision_by" IS NOT NULL
            AND "decision_at" IS NOT NULL AND "decision_reason" IS NOT NULL
            AND "decision_request_digest" IS NOT NULL
            AND "decision_evidence_digest" IS NOT NULL
            AND "quote_no" IS NOT NULL AND "order_no" IS NULL)
        OR ("status_code" = 'rejected'
            AND "decision_code" = 'rejected' AND "decision_by" IS NOT NULL
            AND "decision_at" IS NOT NULL AND "decision_reason" IS NOT NULL
            AND "decision_request_digest" IS NOT NULL
            AND "decision_evidence_digest" IS NOT NULL
            AND "quote_no" IS NULL AND "order_no" IS NULL)
        OR ("status_code" = 'ordered'
            AND "decision_code" = 'approved' AND "decision_by" IS NOT NULL
            AND "decision_at" IS NOT NULL AND "decision_reason" IS NOT NULL
            AND "decision_request_digest" IS NOT NULL
            AND "decision_evidence_digest" IS NOT NULL
            AND "quote_no" IS NOT NULL AND "order_no" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_com_assisted_request_no UNIQUE ("request_no"),
    CONSTRAINT uk_idc_com_assisted_request_staff_key UNIQUE (
        "requested_by", "idempotency_digest"
    ),
    CONSTRAINT uk_idc_com_assisted_request_currency UNIQUE (
        "request_no", "currency_code"
    ),
    CONSTRAINT uk_idc_com_assisted_request_quote UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_com_assisted_request_order UNIQUE ("order_no"),
    CONSTRAINT fk_idc_com_assisted_request_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_assisted_request_quote FOREIGN KEY (
        "quote_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_quote_snapshot (
        "quote_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_assisted_request_queue
    ON plugin_idc_core_commerce_assisted_quote_request (
        "tenant_id", "status_code", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_assisted_request_customer
    ON plugin_idc_core_commerce_assisted_quote_request (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_com_assisted_request_expiry
    ON plugin_idc_core_commerce_assisted_quote_request (
        "tenant_id", "expires_at", "id"
    ) WHERE "status_code" IN ('pending_approval', 'approved');

-- Purpose: Immutable assisted quote configuration intent revalidated during approval.
-- 用途：保存交易协助报价申请明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_assisted_quote_request_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_item_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "line_number" INT NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "billing_rule_no" VARCHAR(40) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "product_name_zh_cn" VARCHAR(256) NOT NULL,
    "product_name_en_us" VARCHAR(256) NOT NULL,
    "server_quantity" INT NOT NULL,
    "purchase_cycles" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_com_assisted_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_com_assisted_item_line CHECK (
        "line_number" BETWEEN 1 AND 8
        AND "server_quantity" BETWEEN 1 AND 1000
        AND "purchase_cycles" BETWEEN 1 AND 120
    ),
    CONSTRAINT ck_idc_com_assisted_item_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_com_assisted_item_digest CHECK ("content_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_com_assisted_item_names CHECK (
        length(btrim("product_name_zh_cn")) BETWEEN 1 AND 256
        AND length(btrim("product_name_en_us")) BETWEEN 1 AND 256
    ),
    CONSTRAINT uk_idc_com_assisted_item_no UNIQUE ("request_item_no"),
    CONSTRAINT uk_idc_com_assisted_item_line UNIQUE ("request_no", "line_number"),
    CONSTRAINT fk_idc_com_assisted_item_request FOREIGN KEY (
        "request_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_assisted_quote_request (
        "request_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_assisted_item_product_version FOREIGN KEY (
        "product_no", "version_no"
    ) REFERENCES plugin_idc_core_catalog_product_version (
        "product_no", "version_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_assisted_item_billing FOREIGN KEY (
        "version_no", "billing_rule_no"
    ) REFERENCES plugin_idc_core_catalog_billing_rule (
        "version_no", "billing_rule_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_com_assisted_item_configuration FOREIGN KEY (
        "configuration_snapshot_no", "version_no"
    ) REFERENCES plugin_idc_core_commerce_configuration_snapshot (
        "configuration_snapshot_no", "version_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_com_assisted_item_request
    ON plugin_idc_core_commerce_assisted_quote_request_item (
        "tenant_id", "request_no", "line_number", "id"
    );

ALTER TABLE plugin_idc_core_commerce_order
    ADD COLUMN IF NOT EXISTS "assisted_request_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "payment_term_code" VARCHAR(24) NOT NULL DEFAULT 'standard_prepay';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_order_payment_term'
          AND conrelid = 'plugin_idc_core_commerce_order'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order
            ADD CONSTRAINT ck_idc_com_order_payment_term CHECK (
                "payment_term_code" IN ('standard_prepay', 'extended_prepay')
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_com_order_assisted_source'
          AND conrelid = 'plugin_idc_core_commerce_order'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order
            ADD CONSTRAINT ck_idc_com_order_assisted_source CHECK (
                ("source_code" = 'admin' AND "assisted_request_no" IS NOT NULL)
                OR ("source_code" <> 'admin' AND "assisted_request_no" IS NULL
                    AND "payment_term_code" = 'standard_prepay')
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_com_order_assisted_request'
          AND conrelid = 'plugin_idc_core_commerce_order'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order
            ADD CONSTRAINT uk_idc_com_order_assisted_request UNIQUE ("assisted_request_no");
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_com_order_assisted_request'
          AND conrelid = 'plugin_idc_core_commerce_order'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_order
            ADD CONSTRAINT fk_idc_com_order_assisted_request FOREIGN KEY ("assisted_request_no")
            REFERENCES plugin_idc_core_commerce_assisted_quote_request ("request_no")
            ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_com_assisted_request_order'
          AND conrelid = 'plugin_idc_core_commerce_assisted_quote_request'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
            ADD CONSTRAINT fk_idc_com_assisted_request_order FOREIGN KEY ("order_no")
            REFERENCES plugin_idc_core_commerce_order ("order_no")
            ON DELETE RESTRICT;
    END IF;
END $$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_assisted_request_identity_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
       OR NEW."request_no" IS DISTINCT FROM OLD."request_no"
       OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
       OR NEW."currency_code" IS DISTINCT FROM OLD."currency_code"
       OR NEW."item_count" IS DISTINCT FROM OLD."item_count"
       OR NEW."manual_discount_minor" IS DISTINCT FROM OLD."manual_discount_minor"
       OR NEW."payment_term_code" IS DISTINCT FROM OLD."payment_term_code"
       OR NEW."payment_due_seconds" IS DISTINCT FROM OLD."payment_due_seconds"
       OR NEW."reason" IS DISTINCT FROM OLD."reason"
       OR NEW."internal_note" IS DISTINCT FROM OLD."internal_note"
       OR NEW."idempotency_digest" IS DISTINCT FROM OLD."idempotency_digest"
       OR NEW."request_digest" IS DISTINCT FROM OLD."request_digest"
       OR NEW."expires_at" IS DISTINCT FROM OLD."expires_at"
       OR NEW."requested_by" IS DISTINCT FROM OLD."requested_by"
       OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'assisted quote request identity is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_com_assisted_request_identity_immutable
    ON plugin_idc_core_commerce_assisted_quote_request;
CREATE TRIGGER trg_idc_com_assisted_request_identity_immutable
BEFORE UPDATE ON plugin_idc_core_commerce_assisted_quote_request
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_assisted_request_identity_change();

DROP TRIGGER IF EXISTS trg_idc_com_assisted_item_immutable
    ON plugin_idc_core_commerce_assisted_quote_request_item;
CREATE TRIGGER trg_idc_com_assisted_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_commerce_assisted_quote_request_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_commerce_snapshot_mutation();

COMMENT ON TABLE plugin_idc_core_commerce_assisted_quote_request
    IS 'Staff-assisted quote intent with independent approval and prepayment-only terms';
COMMENT ON TABLE plugin_idc_core_commerce_assisted_quote_request_item
    IS 'Immutable assisted quote configuration intent revalidated during approval';

INSERT INTO sys_dict_type (
    "tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
    "remark", "created_at", "updated_at"
) VALUES
    (0, '协助报价状态', 'idc_commerce_assisted_quote_status', 1, 1, FALSE,
     '后台协助报价的独立审批与下单状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '协助报价付款条件', 'idc_commerce_assisted_payment_term', 1, 1, FALSE,
     '只允许预付款及其付款截止时间延期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, '协助报价审批决定', 'idc_commerce_assisted_quote_decision', 1, 1, FALSE,
     '独立员工对协助报价的审批决定', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort", "tag_style",
    "status", "is_builtin", "created_at", "updated_at"
) VALUES
    (0, 'idc_commerce_assisted_quote_status', '待审批', 'pending_approval', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_quote_status', '已批准', 'approved', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_quote_status', '已拒绝', 'rejected', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_quote_status', '已下单', 'ordered', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_payment_term', '标准预付款', 'standard_prepay', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_payment_term', '延期预付款', 'extended_prepay', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_quote_decision', '批准', 'approved', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_commerce_assisted_quote_decision', '拒绝', 'rejected', 2, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
