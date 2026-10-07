-- 028: Preserve immutable source invoices while recording atomic same-customer,
-- same-currency invoice merge and whole-item split derivations.

ALTER TABLE plugin_idc_core_billing_invoice_item
    DROP CONSTRAINT IF EXISTS ck_idc_bill_item_type;

ALTER TABLE plugin_idc_core_billing_invoice_item
    ADD CONSTRAINT ck_idc_bill_item_type CHECK (
        "item_type_code" IN (
            'order_item', 'renewal', 'change', 'manual', 'adjustment', 'derived'
        )
    );

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_due;

ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_due CHECK (
        "version" > 0
        AND (
            "source_code" = 'consolidation'
            OR "payment_due_at" > "created_at"
        )
    );

-- Purpose: Immutable atomic invoice merge or whole-item split header.
-- 用途：保存账务账单转换相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_transform (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transform_no" VARCHAR(40) NOT NULL,
    "transform_type_code" VARCHAR(16) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_count" INT NOT NULL,
    "destination_count" INT NOT NULL,
    "item_count" INT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "requested_by" INT NOT NULL,
    "completed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_transform_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_transform_type CHECK (
        "transform_type_code" IN ('merge', 'split')
    ),
    CONSTRAINT ck_idc_bill_transform_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_bill_transform_counts CHECK (
        "source_count" BETWEEN 1 AND 100
        AND "destination_count" BETWEEN 1 AND 100
        AND "item_count" BETWEEN 2 AND 10000
        AND (
            ("transform_type_code" = 'merge'
                AND "source_count" >= 2 AND "destination_count" = 1)
            OR ("transform_type_code" = 'split'
                AND "source_count" = 1 AND "destination_count" >= 2)
        )
    ),
    CONSTRAINT ck_idc_bill_transform_digests CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_bill_transform_requester CHECK ("requested_by" > 0),
    CONSTRAINT uk_idc_bill_transform_no UNIQUE ("transform_no"),
    CONSTRAINT uk_idc_bill_transform_idempotency UNIQUE ("idempotency_digest"),
    CONSTRAINT uk_idc_bill_transform_owner UNIQUE (
        "transform_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_transform_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_transform_customer
    ON plugin_idc_core_billing_invoice_transform (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Immutable source invoice membership for one transform.
-- 用途：保存账务账单转换来源相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_transform_source (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transform_no" VARCHAR(40) NOT NULL,
    "source_invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "position" INT NOT NULL,
    "source_content_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_transform_src_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_transform_src_position CHECK (
        "position" BETWEEN 1 AND 100
    ),
    CONSTRAINT ck_idc_bill_transform_src_digests CHECK (
        "source_content_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_transform_src_invoice UNIQUE ("source_invoice_no"),
    CONSTRAINT uk_idc_bill_transform_src_position UNIQUE (
        "transform_no", "position"
    ),
    CONSTRAINT uk_idc_bill_transform_src_owner UNIQUE (
        "transform_no", "source_invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_transform_src_header FOREIGN KEY (
        "transform_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_transform (
        "transform_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_transform_src_invoice FOREIGN KEY (
        "source_invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

-- Purpose: Immutable destination invoice membership for one transform.
-- 用途：保存账务账单转换目标相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_transform_destination (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transform_no" VARCHAR(40) NOT NULL,
    "destination_invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "position" INT NOT NULL,
    "destination_content_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_transform_dst_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_transform_dst_position CHECK (
        "position" BETWEEN 1 AND 100
    ),
    CONSTRAINT ck_idc_bill_transform_dst_digests CHECK (
        "destination_content_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_transform_dst_invoice UNIQUE ("destination_invoice_no"),
    CONSTRAINT uk_idc_bill_transform_dst_position UNIQUE (
        "transform_no", "position"
    ),
    CONSTRAINT uk_idc_bill_transform_dst_owner UNIQUE (
        "transform_no", "destination_invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_transform_dst_header FOREIGN KEY (
        "transform_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_transform (
        "transform_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_transform_dst_invoice FOREIGN KEY (
        "destination_invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

-- Purpose: One-to-one immutable source-to-destination invoice item derivation.
-- 用途：保存账务账单明细派生相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_item_derivation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transform_no" VARCHAR(40) NOT NULL,
    "source_invoice_no" VARCHAR(40) NOT NULL,
    "source_invoice_item_no" VARCHAR(40) NOT NULL,
    "destination_invoice_no" VARCHAR(40) NOT NULL,
    "destination_invoice_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_content_digest" CHAR(64) NOT NULL,
    "destination_content_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_derivation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_derivation_digests CHECK (
        "source_content_digest" ~ '^[0-9a-f]{64}$'
        AND "destination_content_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_derivation_source UNIQUE (
        "source_invoice_no", "source_invoice_item_no"
    ),
    CONSTRAINT uk_idc_bill_derivation_destination UNIQUE (
        "destination_invoice_no", "destination_invoice_item_no"
    ),
    CONSTRAINT fk_idc_bill_derivation_source_transform FOREIGN KEY (
        "transform_no", "source_invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_transform_source (
        "transform_no", "source_invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_derivation_destination_transform FOREIGN KEY (
        "transform_no", "destination_invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_transform_destination (
        "transform_no", "destination_invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_derivation_source_item FOREIGN KEY (
        "source_invoice_no", "source_invoice_item_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_derivation_destination_item FOREIGN KEY (
        "destination_invoice_no", "destination_invoice_item_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_derivation_transform
    ON plugin_idc_core_billing_invoice_item_derivation (
        "tenant_id", "transform_no", "source_invoice_no", "id"
    );

DROP TRIGGER IF EXISTS trg_idc_bill_transform_immutable
    ON plugin_idc_core_billing_invoice_transform;
CREATE TRIGGER trg_idc_bill_transform_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_transform
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_transform_src_immutable
    ON plugin_idc_core_billing_invoice_transform_source;
CREATE TRIGGER trg_idc_bill_transform_src_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_transform_source
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_transform_dst_immutable
    ON plugin_idc_core_billing_invoice_transform_destination;
CREATE TRIGGER trg_idc_bill_transform_dst_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_transform_destination
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_derivation_immutable
    ON plugin_idc_core_billing_invoice_item_derivation;
CREATE TRIGGER trg_idc_bill_derivation_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_item_derivation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

COMMENT ON TABLE plugin_idc_core_billing_invoice_transform
    IS 'Immutable atomic invoice merge or whole-item split header';
COMMENT ON TABLE plugin_idc_core_billing_invoice_transform_source
    IS 'Immutable source invoice membership for one transform';
COMMENT ON TABLE plugin_idc_core_billing_invoice_transform_destination
    IS 'Immutable destination invoice membership for one transform';
COMMENT ON TABLE plugin_idc_core_billing_invoice_item_derivation
    IS 'One-to-one immutable source-to-destination invoice item derivation';
