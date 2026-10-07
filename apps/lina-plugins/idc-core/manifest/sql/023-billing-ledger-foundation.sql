-- 023: Receivable invoices, immutable double-entry ledger, balance projections,
-- fund holds, allocations, and reviewed adjustment intents.

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_order_owner_currency
    ON plugin_idc_core_commerce_order (
        "order_no", "customer_no", "currency_code"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_com_order_item_order_currency
    ON plugin_idc_core_commerce_order_item (
        "order_no", "order_item_no", "currency_code"
    );

-- Purpose: Receivable invoice header with explicit currency and lifecycle aggregates.
-- 用途：保存账务账单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "order_no" VARCHAR(40),
    "source_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'draft',
    "item_count" INT NOT NULL,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "paid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "refundable_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "refunded_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "payment_due_at" TIMESTAMPTZ NOT NULL,
    "issued_at" TIMESTAMPTZ,
    "settled_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_invoice_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_invoice_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_bill_invoice_source CHECK (
        "source_code" IN (
            'order', 'renewal', 'change', 'manual', 'consolidation', 'migration'
        )
    ),
    CONSTRAINT ck_idc_bill_invoice_status CHECK (
        "status_code" IN (
            'draft', 'pending_payment', 'partially_paid', 'paid', 'overdue',
            'cancelled', 'partially_refunded', 'refunded', 'bad_debt'
        )
    ),
    CONSTRAINT ck_idc_bill_invoice_amounts CHECK (
        "item_count" BETWEEN 1 AND 10000
        AND "subtotal_minor" >= 0
        AND "discount_minor" >= 0
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "paid_amount_minor" BETWEEN 0 AND "total_amount_minor"
        AND "refundable_amount_minor" >= 0
        AND "refunded_amount_minor" >= 0
        AND "refundable_amount_minor" + "refunded_amount_minor" <= "paid_amount_minor"
    ),
    CONSTRAINT ck_idc_bill_invoice_digests CHECK (
        "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_bill_invoice_order_source CHECK (
        ("source_code" = 'order' AND "order_no" IS NOT NULL)
        OR "source_code" <> 'order'
    ),
    CONSTRAINT ck_idc_bill_invoice_lifecycle CHECK (
        ("status_code" = 'draft' AND "issued_at" IS NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0)
        OR ("status_code" IN ('pending_payment', 'overdue') AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0)
        OR ("status_code" = 'partially_paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" > 0 AND "paid_amount_minor" < "total_amount_minor")
        OR ("status_code" = 'paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'partially_refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" > 0)
        OR ("status_code" = 'refunded' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NOT NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = "total_amount_minor"
            AND "refunded_amount_minor" > 0 AND "refundable_amount_minor" = 0)
        OR ("status_code" = 'cancelled' AND "settled_at" IS NULL
            AND "cancelled_at" IS NOT NULL AND "paid_amount_minor" = 0)
        OR ("status_code" = 'bad_debt' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" < "total_amount_minor")
    ),
    CONSTRAINT ck_idc_bill_invoice_due CHECK (
        "payment_due_at" > "created_at" AND "version" > 0
    ),
    CONSTRAINT uk_idc_bill_invoice_no UNIQUE ("invoice_no"),
    CONSTRAINT uk_idc_bill_invoice_currency UNIQUE ("invoice_no", "currency_code"),
    CONSTRAINT uk_idc_bill_invoice_owner_currency UNIQUE (
        "invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_bill_invoice_order_owner_currency UNIQUE (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_invoice_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_invoice_order FOREIGN KEY (
        "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_order (
        "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_bill_invoice_order_origin
    ON plugin_idc_core_billing_invoice ("tenant_id", "order_no")
    WHERE "source_code" = 'order' AND "order_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_customer_status
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "customer_no", "currency_code", "status_code",
        "payment_due_at", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_invoice_due_work
    ON plugin_idc_core_billing_invoice (
        "tenant_id", "status_code", "payment_due_at", "id"
    ) WHERE "status_code" IN ('pending_payment', 'partially_paid', 'overdue');

-- Purpose: Append-only invoice commercial line copied from an order or other source.
-- 用途：保存账务账单明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_invoice_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "invoice_item_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "line_number" INT NOT NULL,
    "item_type_code" VARCHAR(24) NOT NULL,
    "order_no" VARCHAR(40),
    "order_item_no" VARCHAR(40),
    "source_reference" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(256) NOT NULL,
    "description_en_us" VARCHAR(256) NOT NULL,
    "subtotal_minor" BIGINT NOT NULL,
    "discount_minor" BIGINT NOT NULL DEFAULT 0,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "refundable_cap_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_item_type CHECK (
        "item_type_code" IN ('order_item', 'renewal', 'change', 'manual', 'adjustment')
    ),
    CONSTRAINT ck_idc_bill_item_order CHECK (
        ("item_type_code" = 'order_item' AND "order_no" IS NOT NULL
            AND "order_item_no" IS NOT NULL
            AND "source_reference" = "order_item_no")
        OR "item_type_code" <> 'order_item'
    ),
    CONSTRAINT ck_idc_bill_item_amounts CHECK (
        "line_number" BETWEEN 1 AND 10000
        AND "subtotal_minor" >= 0 AND "discount_minor" >= 0
        AND "net_amount_minor" = "subtotal_minor" - "discount_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND "refundable_cap_minor" BETWEEN 0 AND "total_amount_minor"
    ),
    CONSTRAINT ck_idc_bill_item_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_item_no UNIQUE ("invoice_item_no"),
    CONSTRAINT uk_idc_bill_item_line UNIQUE ("invoice_no", "line_number"),
    CONSTRAINT uk_idc_bill_item_invoice_no UNIQUE ("invoice_no", "invoice_item_no"),
    CONSTRAINT uk_idc_bill_item_owner_currency UNIQUE (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_item_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_item_invoice_order FOREIGN KEY (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_item_order FOREIGN KEY (
        "order_no", "order_item_no", "currency_code"
    )
        REFERENCES plugin_idc_core_commerce_order_item (
            "order_no", "order_item_no", "currency_code"
        ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_item_invoice
    ON plugin_idc_core_billing_invoice_item (
        "tenant_id", "invoice_no", "line_number", "id"
    );

-- Purpose: Currency-specific customer, merchant, or system double-entry account.
-- 用途：保存账务账本账户相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_ledger_account (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "account_no" VARCHAR(40) NOT NULL,
    "owner_type_code" VARCHAR(16) NOT NULL,
    "owner_reference" VARCHAR(128) NOT NULL,
    "customer_no" VARCHAR(40),
    "account_type_code" VARCHAR(32) NOT NULL,
    "normal_balance_code" VARCHAR(8) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "withdrawable" BOOLEAN NOT NULL DEFAULT FALSE,
    "closed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_account_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_account_owner_type CHECK (
        "owner_type_code" IN ('customer', 'merchant', 'system')
    ),
    CONSTRAINT ck_idc_bill_account_owner CHECK (
        ("owner_type_code" = 'customer' AND "customer_no" IS NOT NULL
            AND "owner_reference" = "customer_no"
            AND "account_type_code" IN (
                'customer_balance', 'customer_credit', 'customer_gift',
                'customer_commission'
            ))
        OR ("owner_type_code" IN ('merchant', 'system') AND "customer_no" IS NULL
            AND "account_type_code" IN (
                'merchant_cash', 'accounts_receivable', 'payment_clearing',
                'refund_clearing', 'revenue', 'tax_payable',
                'adjustment_clearing', 'bad_debt_expense'
            ))
    ),
    CONSTRAINT ck_idc_bill_account_nature CHECK (
        ("account_type_code" IN (
            'customer_balance', 'customer_gift', 'customer_commission',
            'refund_clearing', 'revenue', 'tax_payable', 'adjustment_clearing'
        ) AND "normal_balance_code" = 'credit')
        OR ("account_type_code" IN (
            'customer_credit', 'merchant_cash', 'accounts_receivable',
            'payment_clearing', 'bad_debt_expense'
        ) AND "normal_balance_code" = 'debit')
    ),
    CONSTRAINT ck_idc_bill_account_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_bill_account_status CHECK (
        "status_code" IN ('active', 'frozen', 'closed')
    ),
    CONSTRAINT ck_idc_bill_account_lifecycle CHECK (
        ("status_code" IN ('active', 'frozen') AND "closed_at" IS NULL)
        OR ("status_code" = 'closed' AND "closed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_bill_account_withdrawal CHECK (
        NOT "withdrawable" OR "account_type_code" = 'customer_commission'
    ),
    CONSTRAINT ck_idc_bill_account_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_bill_account_no UNIQUE ("account_no"),
    CONSTRAINT uk_idc_bill_account_currency UNIQUE ("account_no", "currency_code"),
    CONSTRAINT uk_idc_bill_account_currency_nature UNIQUE (
        "account_no", "currency_code", "normal_balance_code"
    ),
    CONSTRAINT uk_idc_bill_account_customer_currency UNIQUE (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ),
    CONSTRAINT uk_idc_bill_account_owner_type_currency UNIQUE (
        "tenant_id", "owner_type_code", "owner_reference",
        "account_type_code", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_account_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_account_customer
    ON plugin_idc_core_billing_ledger_account (
        "tenant_id", "customer_no", "currency_code", "account_type_code", "id"
    ) WHERE "customer_no" IS NOT NULL;

-- Purpose: Append-only balanced financial event identified by one business source.
-- 用途：保存账务账本交易相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_ledger_transaction (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "transaction_no" VARCHAR(40) NOT NULL,
    "transaction_type_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "original_transaction_no" VARCHAR(40),
    "entry_count" INT NOT NULL,
    "debit_total_minor" BIGINT NOT NULL,
    "credit_total_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "posted_by" INT NOT NULL DEFAULT 0,
    "posted_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_tx_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_tx_type CHECK (
        "transaction_type_code" IN (
            'invoice_issuance', 'recharge', 'payment', 'balance_payment',
            'refund', 'commission', 'adjustment', 'reversal',
            'manual_receipt', 'writeoff', 'gift', 'credit', 'migration'
        )
    ),
    CONSTRAINT ck_idc_bill_tx_source CHECK (
        "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$'
        AND length("source_reference") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_bill_tx_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_bill_tx_amounts CHECK (
        "entry_count" BETWEEN 2 AND 64
        AND "debit_total_minor" > 0
        AND "debit_total_minor" = "credit_total_minor"
    ),
    CONSTRAINT ck_idc_bill_tx_reversal CHECK (
        ("transaction_type_code" = 'reversal' AND "original_transaction_no" IS NOT NULL)
        OR ("transaction_type_code" <> 'reversal' AND "original_transaction_no" IS NULL)
    ),
    CONSTRAINT ck_idc_bill_tx_digests CHECK (
        "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_bill_tx_poster CHECK ("posted_by" >= 0),
    CONSTRAINT uk_idc_bill_tx_no UNIQUE ("transaction_no"),
    CONSTRAINT uk_idc_bill_tx_currency UNIQUE ("transaction_no", "currency_code"),
    CONSTRAINT uk_idc_bill_tx_currency_type UNIQUE (
        "transaction_no", "currency_code", "transaction_type_code"
    ),
    CONSTRAINT uk_idc_bill_tx_source UNIQUE ("source_type_code", "source_reference"),
    CONSTRAINT fk_idc_bill_tx_original FOREIGN KEY (
        "original_transaction_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_tx_posted
    ON plugin_idc_core_billing_ledger_transaction (
        "tenant_id", "currency_code", "posted_at" DESC, "id" DESC
    );

-- Purpose: Append-only debit or credit entry linked to one same-currency account.
-- 用途：保存账务账本记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_ledger_entry (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entry_no" VARCHAR(40) NOT NULL,
    "transaction_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "line_number" INT NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "direction_code" VARCHAR(8) NOT NULL,
    "entry_role_code" VARCHAR(24) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "invoice_no" VARCHAR(40),
    "invoice_item_no" VARCHAR(40),
    "reason_code" VARCHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_entry_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_entry_direction CHECK (
        "direction_code" IN ('debit', 'credit')
    ),
    CONSTRAINT ck_idc_bill_entry_role CHECK (
        "entry_role_code" IN (
            'cash', 'balance', 'receivable', 'revenue', 'tax', 'refund',
            'commission', 'gift', 'credit', 'clearing', 'expense', 'adjustment'
        )
    ),
    CONSTRAINT ck_idc_bill_entry_amount CHECK (
        "line_number" BETWEEN 1 AND 64 AND "amount_minor" > 0
    ),
    CONSTRAINT ck_idc_bill_entry_invoice_item CHECK (
        "invoice_item_no" IS NULL OR "invoice_no" IS NOT NULL
    ),
    CONSTRAINT ck_idc_bill_entry_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_idc_bill_entry_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_entry_no UNIQUE ("entry_no"),
    CONSTRAINT uk_idc_bill_entry_line UNIQUE ("transaction_no", "line_number"),
    CONSTRAINT fk_idc_bill_entry_tx FOREIGN KEY (
        "transaction_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_entry_account FOREIGN KEY (
        "account_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_account (
        "account_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_entry_invoice FOREIGN KEY (
        "invoice_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_entry_invoice_item FOREIGN KEY (
        "invoice_no", "invoice_item_no"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_entry_account
    ON plugin_idc_core_billing_ledger_entry (
        "tenant_id", "account_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_entry_invoice
    ON plugin_idc_core_billing_ledger_entry (
        "tenant_id", "invoice_no", "invoice_item_no", "id"
    ) WHERE "invoice_no" IS NOT NULL;

-- Purpose: Rebuildable append-only per-account balance projection after a transaction.
-- 用途：保存账务余额快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_balance_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "normal_balance_code" VARCHAR(8) NOT NULL,
    "transaction_no" VARCHAR(40) NOT NULL,
    "sequence_no" BIGINT NOT NULL,
    "previous_snapshot_no" VARCHAR(40),
    "previous_sequence_no" BIGINT,
    "opening_balance_minor" BIGINT NOT NULL,
    "debit_amount_minor" BIGINT NOT NULL,
    "credit_amount_minor" BIGINT NOT NULL,
    "closing_balance_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_snapshot_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_snapshot_nature CHECK (
        "normal_balance_code" IN ('debit', 'credit')
    ),
    CONSTRAINT ck_idc_bill_snapshot_sequence CHECK (
        ("sequence_no" = 1 AND "previous_snapshot_no" IS NULL
            AND "previous_sequence_no" IS NULL AND "opening_balance_minor" = 0)
        OR ("sequence_no" > 1 AND "previous_snapshot_no" IS NOT NULL
            AND "previous_sequence_no" = "sequence_no" - 1)
    ),
    CONSTRAINT ck_idc_bill_snapshot_amounts CHECK (
        "debit_amount_minor" >= 0 AND "credit_amount_minor" >= 0
        AND (
            ("normal_balance_code" = 'debit'
                AND "closing_balance_minor" = "opening_balance_minor"
                    + "debit_amount_minor" - "credit_amount_minor")
            OR ("normal_balance_code" = 'credit'
                AND "closing_balance_minor" = "opening_balance_minor"
                    + "credit_amount_minor" - "debit_amount_minor")
        )
    ),
    CONSTRAINT ck_idc_bill_snapshot_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT uk_idc_bill_snapshot_account_sequence UNIQUE (
        "account_no", "sequence_no"
    ),
    CONSTRAINT uk_idc_bill_snapshot_account_tx UNIQUE (
        "account_no", "transaction_no"
    ),
    CONSTRAINT uk_idc_bill_snapshot_chain UNIQUE (
        "account_no", "sequence_no", "snapshot_no"
    ),
    CONSTRAINT fk_idc_bill_snapshot_account FOREIGN KEY (
        "account_no", "currency_code", "normal_balance_code"
    ) REFERENCES plugin_idc_core_billing_ledger_account (
        "account_no", "currency_code", "normal_balance_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_snapshot_tx FOREIGN KEY (
        "transaction_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_snapshot_previous FOREIGN KEY (
        "account_no", "previous_sequence_no", "previous_snapshot_no"
    ) REFERENCES plugin_idc_core_billing_balance_snapshot (
        "account_no", "sequence_no", "snapshot_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_snapshot_latest
    ON plugin_idc_core_billing_balance_snapshot (
        "tenant_id", "account_no", "sequence_no" DESC, "id" DESC
    );

-- Purpose: Conserved customer balance hold for combined or deferred payment.
-- 用途：保存账务资金锁定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_fund_hold (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "hold_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "account_type_code" VARCHAR(32) NOT NULL DEFAULT 'customer_balance',
    "currency_code" CHAR(3) NOT NULL,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "consumed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "released_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'active',
    "content_digest" CHAR(64) NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_hold_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_hold_account_type CHECK (
        "account_type_code" = 'customer_balance'
    ),
    CONSTRAINT ck_idc_bill_hold_source CHECK (
        "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$'
        AND length("source_reference") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_bill_hold_amounts CHECK (
        "amount_minor" > 0
        AND "consumed_amount_minor" >= 0 AND "released_amount_minor" >= 0
        AND "consumed_amount_minor" + "released_amount_minor" <= "amount_minor"
    ),
    CONSTRAINT ck_idc_bill_hold_status CHECK (
        "status_code" IN (
            'active', 'partially_consumed', 'consumed', 'released', 'expired'
        )
    ),
    CONSTRAINT ck_idc_bill_hold_lifecycle CHECK (
        ("status_code" = 'active' AND "consumed_amount_minor" = 0
            AND "released_amount_minor" = 0
            AND "consumed_at" IS NULL AND "released_at" IS NULL)
        OR ("status_code" = 'partially_consumed' AND "consumed_amount_minor" > 0
            AND "consumed_amount_minor" + "released_amount_minor" < "amount_minor"
            AND "consumed_at" IS NULL AND "released_at" IS NULL)
        OR ("status_code" = 'consumed' AND "consumed_amount_minor" = "amount_minor"
            AND "released_amount_minor" = 0
            AND "consumed_at" IS NOT NULL AND "released_at" IS NULL)
        OR ("status_code" IN ('released', 'expired') AND "released_amount_minor" > 0
            AND "consumed_amount_minor" + "released_amount_minor" = "amount_minor"
            AND "released_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_bill_hold_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_bill_hold_expiry CHECK (
        "expires_at" > "created_at" AND "version" > 0
    ),
    CONSTRAINT uk_idc_bill_hold_no UNIQUE ("hold_no"),
    CONSTRAINT fk_idc_bill_hold_account FOREIGN KEY (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_account (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_bill_hold_active_source
    ON plugin_idc_core_billing_fund_hold (
        "tenant_id", "account_no", "source_type_code", "source_reference"
    ) WHERE "status_code" IN ('active', 'partially_consumed');

CREATE INDEX IF NOT EXISTS idx_idc_bill_hold_expiry
    ON plugin_idc_core_billing_fund_hold (
        "tenant_id", "status_code", "expires_at", "id"
    ) WHERE "status_code" IN ('active', 'partially_consumed');

-- Purpose: Append-only payment allocation or reversal against an invoice.
-- 用途：保存账务支付分配相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_payment_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "allocation_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "invoice_item_no" VARCHAR(40),
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "payment_source_type_code" VARCHAR(24) NOT NULL,
    "payment_source_reference" VARCHAR(128) NOT NULL,
    "ledger_transaction_no" VARCHAR(40) NOT NULL,
    "ledger_transaction_type_code" VARCHAR(24) NOT NULL,
    "direction_code" VARCHAR(8) NOT NULL DEFAULT 'apply',
    "original_allocation_no" VARCHAR(40),
    "amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_pay_alloc_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_pay_alloc_source CHECK (
        "payment_source_type_code" IN (
            'channel_payment', 'balance', 'credit', 'manual_receipt', 'migration'
        ) AND length("payment_source_reference") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_bill_pay_alloc_direction CHECK (
        ("direction_code" = 'apply' AND "original_allocation_no" IS NULL
            AND "ledger_transaction_type_code" IN (
                'payment', 'balance_payment', 'manual_receipt', 'credit', 'migration'
            ))
        OR ("direction_code" = 'reverse' AND "original_allocation_no" IS NOT NULL
            AND "ledger_transaction_type_code" = 'reversal')
    ),
    CONSTRAINT ck_idc_bill_pay_alloc_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_bill_pay_alloc_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_pay_alloc_no UNIQUE ("allocation_no"),
    CONSTRAINT uk_idc_bill_pay_alloc_owner UNIQUE (
        "allocation_no", "invoice_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_pay_alloc_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_pay_alloc_item FOREIGN KEY (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_pay_alloc_tx FOREIGN KEY (
        "ledger_transaction_no", "currency_code", "ledger_transaction_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code", "transaction_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_pay_alloc_original FOREIGN KEY (
        "original_allocation_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_payment_allocation (
        "allocation_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_bill_pay_alloc_source
    ON plugin_idc_core_billing_payment_allocation (
        "tenant_id", "payment_source_type_code", "payment_source_reference",
        "invoice_no", COALESCE("invoice_item_no", ''), "direction_code"
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_pay_alloc_invoice
    ON plugin_idc_core_billing_payment_allocation (
        "tenant_id", "invoice_no", "invoice_item_no", "created_at", "id"
    );

-- Purpose: Append-only refund allocation or reversal traced to item and payment.
-- 用途：保存账务退款分配相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_refund_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "refund_allocation_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "invoice_item_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "payment_allocation_no" VARCHAR(40) NOT NULL,
    "refund_source_type_code" VARCHAR(24) NOT NULL,
    "refund_source_reference" VARCHAR(128) NOT NULL,
    "ledger_transaction_no" VARCHAR(40) NOT NULL,
    "ledger_transaction_type_code" VARCHAR(24) NOT NULL,
    "direction_code" VARCHAR(8) NOT NULL DEFAULT 'apply',
    "original_refund_allocation_no" VARCHAR(40),
    "amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_ref_alloc_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_ref_alloc_source CHECK (
        "refund_source_type_code" IN (
            'channel_refund', 'balance_credit', 'credit_release', 'manual', 'migration'
        ) AND length("refund_source_reference") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_bill_ref_alloc_direction CHECK (
        ("direction_code" = 'apply' AND "original_refund_allocation_no" IS NULL
            AND "ledger_transaction_type_code" IN ('refund', 'migration'))
        OR ("direction_code" = 'reverse' AND "original_refund_allocation_no" IS NOT NULL
            AND "ledger_transaction_type_code" = 'reversal')
    ),
    CONSTRAINT ck_idc_bill_ref_alloc_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_bill_ref_alloc_digest CHECK (
        "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_ref_alloc_no UNIQUE ("refund_allocation_no"),
    CONSTRAINT uk_idc_bill_ref_alloc_owner UNIQUE (
        "refund_allocation_no", "invoice_no", "invoice_item_no",
        "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_bill_ref_alloc_item FOREIGN KEY (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice_item (
        "invoice_no", "invoice_item_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_ref_alloc_payment FOREIGN KEY (
        "payment_allocation_no", "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_payment_allocation (
        "allocation_no", "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_ref_alloc_tx FOREIGN KEY (
        "ledger_transaction_no", "currency_code", "ledger_transaction_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code", "transaction_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_ref_alloc_original FOREIGN KEY (
        "original_refund_allocation_no", "invoice_no", "invoice_item_no",
        "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_refund_allocation (
        "refund_allocation_no", "invoice_no", "invoice_item_no",
        "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_bill_ref_alloc_source
    ON plugin_idc_core_billing_refund_allocation (
        "tenant_id", "refund_source_type_code", "refund_source_reference",
        "invoice_no", "invoice_item_no", "direction_code"
    );

CREATE INDEX IF NOT EXISTS idx_idc_bill_ref_alloc_invoice
    ON plugin_idc_core_billing_refund_allocation (
        "tenant_id", "invoice_no", "invoice_item_no", "created_at", "id"
    );

-- Purpose: Mutable reviewed intent that cannot itself change financial balances.
-- 用途：保存账务调整申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_adjustment_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "adjustment_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40),
    "account_no" VARCHAR(40),
    "account_type_code" VARCHAR(32),
    "currency_code" CHAR(3) NOT NULL,
    "adjustment_type_code" VARCHAR(24) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "approval_required" BOOLEAN NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "requested_by" INT NOT NULL,
    "ledger_transaction_no" VARCHAR(40),
    "ledger_transaction_type_code" VARCHAR(24),
    "posted_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_adjust_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (("account_no" IS NULL AND "account_type_code" IS NULL)
            OR ("account_no" IS NOT NULL AND "account_type_code" IS NOT NULL))
        AND (
            ("adjustment_type_code" IN ('balance_credit', 'balance_debit')
                AND "account_no" IS NOT NULL
                AND "account_type_code" = 'customer_balance')
            OR ("adjustment_type_code" IN (
                    'invoice_waiver', 'writeoff', 'manual_receipt'
                ) AND "invoice_no" IS NOT NULL)
        )
    ),
    CONSTRAINT ck_idc_bill_adjust_type CHECK (
        "adjustment_type_code" IN (
            'balance_credit', 'balance_debit', 'invoice_waiver', 'writeoff',
            'manual_receipt'
        )
    ),
    CONSTRAINT ck_idc_bill_adjust_amount CHECK ("amount_minor" > 0),
    CONSTRAINT ck_idc_bill_adjust_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_idc_bill_adjust_digests CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_bill_adjust_status CHECK (
        "status_code" IN ('pending_review', 'approved', 'rejected', 'cancelled', 'posted')
    ),
    CONSTRAINT ck_idc_bill_adjust_review_mode CHECK (
        ("status_code" = 'pending_review' AND "approval_required")
        OR "status_code" <> 'pending_review'
    ),
    CONSTRAINT ck_idc_bill_adjust_posting CHECK (
        ("status_code" = 'posted' AND "ledger_transaction_no" IS NOT NULL
            AND "ledger_transaction_type_code" IN (
                'adjustment', 'writeoff', 'manual_receipt', 'reversal'
            ) AND "posted_at" IS NOT NULL)
        OR ("status_code" <> 'posted' AND "ledger_transaction_no" IS NULL
            AND "ledger_transaction_type_code" IS NULL AND "posted_at" IS NULL)
    ),
    CONSTRAINT ck_idc_bill_adjust_requester CHECK (
        "requested_by" > 0 AND "version" > 0
    ),
    CONSTRAINT uk_idc_bill_adjust_no UNIQUE ("adjustment_no"),
    CONSTRAINT uk_idc_bill_adjust_requester UNIQUE ("adjustment_no", "requested_by"),
    CONSTRAINT uk_idc_bill_adjust_idempotency UNIQUE (
        "customer_no", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_bill_adjust_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_adjust_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_adjust_account FOREIGN KEY (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_account (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_bill_adjust_tx FOREIGN KEY (
        "ledger_transaction_no", "currency_code", "ledger_transaction_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code", "transaction_type_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjust_review
    ON plugin_idc_core_billing_adjustment_request (
        "tenant_id", "status_code", "created_at", "id"
    ) WHERE "status_code" IN ('pending_review', 'approved');

-- Purpose: Append-only second-person decision for one financial adjustment.
-- 用途：保存账务调整审批相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_billing_adjustment_approval (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "approval_no" VARCHAR(40) NOT NULL,
    "adjustment_no" VARCHAR(40) NOT NULL,
    "requester_id_snapshot" INT NOT NULL,
    "reviewer_id" INT NOT NULL,
    "decision_code" VARCHAR(16) NOT NULL,
    "comment_digest" CHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "decided_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_bill_approval_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_bill_approval_decision CHECK (
        "decision_code" IN ('approved', 'rejected')
    ),
    CONSTRAINT ck_idc_bill_approval_distinct_staff CHECK (
        "requester_id_snapshot" > 0 AND "reviewer_id" > 0
        AND "reviewer_id" <> "requester_id_snapshot"
    ),
    CONSTRAINT ck_idc_bill_approval_digests CHECK (
        "comment_digest" ~ '^[0-9a-f]{64}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_bill_approval_no UNIQUE ("approval_no"),
    CONSTRAINT uk_idc_bill_approval_adjustment UNIQUE ("adjustment_no"),
    CONSTRAINT fk_idc_bill_approval_request FOREIGN KEY (
        "adjustment_no", "requester_id_snapshot"
    ) REFERENCES plugin_idc_core_billing_adjustment_request (
        "adjustment_no", "requested_by"
    ) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_billing_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'billing financial history is append-only';
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_billing_invoice_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'billing invoices cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."invoice_no", OLD."customer_no", OLD."order_no",
        OLD."source_code", OLD."currency_code", OLD."item_count", OLD."subtotal_minor",
        OLD."discount_minor", OLD."net_amount_minor", OLD."tax_amount_minor",
        OLD."total_amount_minor", OLD."source_snapshot_digest", OLD."content_digest",
        OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."invoice_no", NEW."customer_no", NEW."order_no",
        NEW."source_code", NEW."currency_code", NEW."item_count", NEW."subtotal_minor",
        NEW."discount_minor", NEW."net_amount_minor", NEW."tax_amount_minor",
        NEW."total_amount_minor", NEW."source_snapshot_digest", NEW."content_digest",
        NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'billing invoice commercial identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_billing_account_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'billing ledger accounts cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."account_no", OLD."owner_type_code", OLD."owner_reference",
        OLD."customer_no", OLD."account_type_code", OLD."normal_balance_code",
        OLD."currency_code", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."account_no", NEW."owner_type_code", NEW."owner_reference",
        NEW."customer_no", NEW."account_type_code", NEW."normal_balance_code",
        NEW."currency_code", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'billing ledger account identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_billing_hold_identity()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'billing fund holds cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."hold_no", OLD."customer_no", OLD."account_no",
        OLD."account_type_code", OLD."currency_code", OLD."source_type_code",
        OLD."source_reference", OLD."amount_minor", OLD."content_digest",
        OLD."expires_at", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."hold_no", NEW."customer_no", NEW."account_no",
        NEW."account_type_code", NEW."currency_code", NEW."source_type_code",
        NEW."source_reference", NEW."amount_minor", NEW."content_digest",
        NEW."expires_at", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'billing fund hold identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_billing_adjustment_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'billing adjustment requests cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."adjustment_no", OLD."customer_no", OLD."invoice_no",
        OLD."account_no", OLD."account_type_code", OLD."currency_code",
        OLD."adjustment_type_code", OLD."amount_minor", OLD."reason_code",
        OLD."evidence_digest", OLD."idempotency_digest", OLD."request_digest",
        OLD."approval_required", OLD."requested_by", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."adjustment_no", NEW."customer_no", NEW."invoice_no",
        NEW."account_no", NEW."account_type_code", NEW."currency_code",
        NEW."adjustment_type_code", NEW."amount_minor", NEW."reason_code",
        NEW."evidence_digest", NEW."idempotency_digest", NEW."request_digest",
        NEW."approval_required", NEW."requested_by", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'billing adjustment intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_invoice_composition()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    "target_invoice_no" VARCHAR(40);
    invoice_row RECORD;
    item_totals RECORD;
    payment_total NUMERIC;
    refund_total NUMERIC;
    invalid_count BIGINT;
BEGIN
    "target_invoice_no" := COALESCE(NEW."invoice_no", OLD."invoice_no");
    SELECT * INTO invoice_row
    FROM plugin_idc_core_billing_invoice
    WHERE "invoice_no" = "target_invoice_no";
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COUNT(*) AS "item_count",
           COALESCE(SUM("subtotal_minor"), 0) AS "subtotal_minor",
           COALESCE(SUM("discount_minor"), 0) AS "discount_minor",
           COALESCE(SUM("net_amount_minor"), 0) AS "net_amount_minor",
           COALESCE(SUM("tax_amount_minor"), 0) AS "tax_amount_minor",
           COALESCE(SUM("total_amount_minor"), 0) AS "total_amount_minor"
    INTO item_totals
    FROM plugin_idc_core_billing_invoice_item
    WHERE "invoice_no" = "target_invoice_no";

    IF item_totals."item_count" <> invoice_row."item_count"
       OR item_totals."subtotal_minor" <> invoice_row."subtotal_minor"
       OR item_totals."discount_minor" <> invoice_row."discount_minor"
       OR item_totals."net_amount_minor" <> invoice_row."net_amount_minor"
       OR item_totals."tax_amount_minor" <> invoice_row."tax_amount_minor"
       OR item_totals."total_amount_minor" <> invoice_row."total_amount_minor" THEN
        RAISE EXCEPTION 'billing invoice item totals do not match invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_billing_invoice_item item
    JOIN plugin_idc_core_commerce_order_item order_item
      ON order_item."order_no" = item."order_no"
     AND order_item."order_item_no" = item."order_item_no"
     AND order_item."currency_code" = item."currency_code"
    WHERE item."invoice_no" = "target_invoice_no"
      AND item."item_type_code" = 'order_item'
      AND ROW(
          item."subtotal_minor", item."discount_minor", item."net_amount_minor",
          item."tax_amount_minor", item."total_amount_minor"
      ) IS DISTINCT FROM ROW(
          order_item."subtotal_minor", order_item."discount_minor",
          order_item."net_amount_minor", order_item."tax_amount_minor",
          order_item."total_amount_minor"
      );
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing order item amounts differ from order %', "target_invoice_no";
    END IF;

    SELECT COALESCE(SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO payment_total
    FROM plugin_idc_core_billing_payment_allocation
    WHERE "invoice_no" = "target_invoice_no";
    IF payment_total <> invoice_row."paid_amount_minor" THEN
        RAISE EXCEPTION 'billing payment allocations do not match invoice %', "target_invoice_no";
    END IF;

    SELECT COALESCE(SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO refund_total
    FROM plugin_idc_core_billing_refund_allocation
    WHERE "invoice_no" = "target_invoice_no";
    IF refund_total <> invoice_row."refunded_amount_minor" THEN
        RAISE EXCEPTION 'billing refund allocations do not match invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT allocation."original_allocation_no"
        FROM plugin_idc_core_billing_payment_allocation allocation
        JOIN plugin_idc_core_billing_payment_allocation "original"
          ON "original"."allocation_no" = allocation."original_allocation_no"
        WHERE allocation."invoice_no" = "target_invoice_no"
          AND allocation."direction_code" = 'reverse'
        GROUP BY allocation."original_allocation_no", "original"."direction_code", "original"."amount_minor"
        HAVING "original"."direction_code" <> 'apply' OR SUM(allocation."amount_minor") > "original"."amount_minor"
    ) invalid_payment_reversal;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing payment reversal exceeds original allocation for invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT allocation."original_refund_allocation_no"
        FROM plugin_idc_core_billing_refund_allocation allocation
        JOIN plugin_idc_core_billing_refund_allocation "original"
          ON "original"."refund_allocation_no" = allocation."original_refund_allocation_no"
        WHERE allocation."invoice_no" = "target_invoice_no"
          AND allocation."direction_code" = 'reverse'
        GROUP BY allocation."original_refund_allocation_no",
                 "original"."direction_code", "original"."amount_minor"
        HAVING "original"."direction_code" <> 'apply' OR SUM(allocation."amount_minor") > "original"."amount_minor"
    ) invalid_refund_reversal;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing refund reversal exceeds original allocation for invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT payment."allocation_no"
        FROM plugin_idc_core_billing_payment_allocation payment
        JOIN (
            SELECT "payment_allocation_no",
                   SUM(CASE "direction_code"
                       WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END) AS refund_total
            FROM plugin_idc_core_billing_refund_allocation
            WHERE "invoice_no" = "target_invoice_no"
            GROUP BY "payment_allocation_no"
        ) refund ON refund."payment_allocation_no" = payment."allocation_no"
        LEFT JOIN (
            SELECT "original_allocation_no", SUM("amount_minor") AS reversed_total
            FROM plugin_idc_core_billing_payment_allocation
            WHERE "invoice_no" = "target_invoice_no" AND "direction_code" = 'reverse'
            GROUP BY "original_allocation_no"
        ) reversed ON reversed."original_allocation_no" = payment."allocation_no"
        WHERE payment."invoice_no" = "target_invoice_no"
          AND (payment."direction_code" <> 'apply' OR refund.refund_total < 0
            OR refund.refund_total + COALESCE(reversed.reversed_total, 0) > payment."amount_minor")
    ) invalid_refund_payment;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing refunds exceed their original payment allocation for invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT allocation."invoice_item_no"
        FROM plugin_idc_core_billing_payment_allocation allocation
        JOIN plugin_idc_core_billing_invoice_item item
          ON item."invoice_no" = allocation."invoice_no"
         AND item."invoice_item_no" = allocation."invoice_item_no"
        WHERE allocation."invoice_no" = "target_invoice_no"
          AND allocation."invoice_item_no" IS NOT NULL
        GROUP BY allocation."invoice_item_no", item."total_amount_minor"
        HAVING SUM(CASE allocation."direction_code"
            WHEN 'apply' THEN allocation."amount_minor" ELSE -allocation."amount_minor" END)
            NOT BETWEEN 0 AND item."total_amount_minor"
    ) invalid_item_payment;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing item payment allocation is outside item total for invoice %', "target_invoice_no";
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT allocation."invoice_item_no"
        FROM plugin_idc_core_billing_refund_allocation allocation
        JOIN plugin_idc_core_billing_invoice_item item
          ON item."invoice_no" = allocation."invoice_no"
         AND item."invoice_item_no" = allocation."invoice_item_no"
        WHERE allocation."invoice_no" = "target_invoice_no"
        GROUP BY allocation."invoice_item_no", item."refundable_cap_minor"
        HAVING SUM(CASE allocation."direction_code"
            WHEN 'apply' THEN allocation."amount_minor" ELSE -allocation."amount_minor" END)
            NOT BETWEEN 0 AND item."refundable_cap_minor"
    ) invalid_item_refund;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing item refund allocation exceeds refundable cap for invoice %', "target_invoice_no";
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_ledger_transaction()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_transaction_no VARCHAR(40);
    transaction_row RECORD;
    entry_totals RECORD;
    invalid_count BIGINT;
BEGIN
    target_transaction_no := COALESCE(NEW."transaction_no", OLD."transaction_no");
    SELECT * INTO transaction_row
    FROM plugin_idc_core_billing_ledger_transaction
    WHERE "transaction_no" = target_transaction_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COUNT(*) AS "entry_count",
           COALESCE(SUM("amount_minor") FILTER (WHERE "direction_code" = 'debit'), 0) AS debit_total,
           COALESCE(SUM("amount_minor") FILTER (WHERE "direction_code" = 'credit'), 0) AS credit_total
    INTO entry_totals
    FROM plugin_idc_core_billing_ledger_entry
    WHERE "transaction_no" = target_transaction_no;
    IF entry_totals."entry_count" <> transaction_row."entry_count"
       OR entry_totals.debit_total <> transaction_row."debit_total_minor"
       OR entry_totals.credit_total <> transaction_row."credit_total_minor"
       OR entry_totals.debit_total <> entry_totals.credit_total THEN
        RAISE EXCEPTION 'billing ledger entries do not balance transaction %', target_transaction_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT entry."account_no"
        FROM plugin_idc_core_billing_ledger_entry entry
        LEFT JOIN plugin_idc_core_billing_balance_snapshot snapshot
          ON snapshot."transaction_no" = entry."transaction_no"
         AND snapshot."account_no" = entry."account_no"
        WHERE entry."transaction_no" = target_transaction_no
        GROUP BY entry."account_no", snapshot."snapshot_no",
                 snapshot."debit_amount_minor", snapshot."credit_amount_minor"
        HAVING COUNT(DISTINCT snapshot."snapshot_no") <> 1
            OR COALESCE(SUM(entry."amount_minor") FILTER (WHERE entry."direction_code" = 'debit'), 0)
                <> snapshot."debit_amount_minor"
            OR COALESCE(SUM(entry."amount_minor") FILTER (WHERE entry."direction_code" = 'credit'), 0)
                <> snapshot."credit_amount_minor"
    ) invalid_snapshot;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing balance snapshots do not match transaction %', target_transaction_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_billing_balance_snapshot snapshot
    LEFT JOIN plugin_idc_core_billing_ledger_entry entry
      ON entry."transaction_no" = snapshot."transaction_no"
     AND entry."account_no" = snapshot."account_no"
    WHERE snapshot."transaction_no" = target_transaction_no
      AND entry."entry_no" IS NULL;
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'billing transaction % has a snapshot without entries', target_transaction_no;
    END IF;

    IF transaction_row."transaction_type_code" = 'reversal' THEN
        SELECT COUNT(*) INTO invalid_count
        FROM plugin_idc_core_billing_ledger_transaction "original"
        WHERE "original"."transaction_no" = transaction_row."original_transaction_no"
          AND ("original"."transaction_type_code" = 'reversal'
            OR (SELECT COALESCE(SUM(reversal."debit_total_minor"), 0)
                FROM plugin_idc_core_billing_ledger_transaction reversal
                WHERE reversal."original_transaction_no" = "original"."transaction_no")
                > "original"."debit_total_minor");
        IF invalid_count > 0 THEN
            RAISE EXCEPTION 'billing reversal exceeds or reverses a reversal transaction %', target_transaction_no;
        END IF;
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_balance_snapshot_chain()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    previous_row RECORD;
    account_type VARCHAR(32);
BEGIN
    SELECT "account_type_code" INTO account_type
    FROM plugin_idc_core_billing_ledger_account
    WHERE "account_no" = NEW."account_no";
    IF account_type IN ('customer_balance', 'customer_gift', 'customer_commission')
       AND (NEW."opening_balance_minor" < 0 OR NEW."closing_balance_minor" < 0) THEN
        RAISE EXCEPTION 'billing protected customer account % cannot have a negative balance', NEW."account_no";
    END IF;
    IF NEW."sequence_no" = 1 THEN
        RETURN NULL;
    END IF;
    SELECT "closing_balance_minor", "normal_balance_code", "currency_code"
    INTO previous_row
    FROM plugin_idc_core_billing_balance_snapshot
    WHERE "account_no" = NEW."account_no"
      AND "sequence_no" = NEW."previous_sequence_no"
      AND "snapshot_no" = NEW."previous_snapshot_no";
    IF NOT FOUND OR previous_row."closing_balance_minor" <> NEW."opening_balance_minor"
       OR previous_row."normal_balance_code" <> NEW."normal_balance_code"
       OR previous_row."currency_code" <> NEW."currency_code" THEN
        RAISE EXCEPTION 'billing balance snapshot chain is inconsistent for account %', NEW."account_no";
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_adjustment_approval()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_adjustment_no VARCHAR(40);
    adjustment_row RECORD;
    approval_count BIGINT;
    approved_count BIGINT;
    rejected_count BIGINT;
BEGIN
    target_adjustment_no := COALESCE(NEW."adjustment_no", OLD."adjustment_no");
    SELECT * INTO adjustment_row
    FROM plugin_idc_core_billing_adjustment_request
    WHERE "adjustment_no" = target_adjustment_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT COUNT(*),
           COUNT(*) FILTER (WHERE "decision_code" = 'approved'),
           COUNT(*) FILTER (WHERE "decision_code" = 'rejected')
    INTO approval_count, approved_count, rejected_count
    FROM plugin_idc_core_billing_adjustment_approval
    WHERE "adjustment_no" = target_adjustment_no;

    IF NOT adjustment_row."approval_required" AND approval_count <> 0 THEN
        RAISE EXCEPTION 'billing adjustment % does not permit approval rows', target_adjustment_no;
    END IF;
    IF adjustment_row."status_code" = 'pending_review' AND approval_count <> 0 THEN
        RAISE EXCEPTION 'billing pending adjustment % already has a decision', target_adjustment_no;
    END IF;
    IF adjustment_row."approval_required"
       AND adjustment_row."status_code" IN ('approved', 'posted')
       AND (approval_count <> 1 OR approved_count <> 1) THEN
        RAISE EXCEPTION 'billing adjustment % lacks an approval', target_adjustment_no;
    END IF;
    IF adjustment_row."approval_required" AND adjustment_row."status_code" = 'rejected'
       AND (approval_count <> 1 OR rejected_count <> 1) THEN
        RAISE EXCEPTION 'billing adjustment % lacks a rejection', target_adjustment_no;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_bill_invoice_item_immutable
    ON plugin_idc_core_billing_invoice_item;
CREATE TRIGGER trg_idc_bill_invoice_item_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_invoice_identity_guard
    ON plugin_idc_core_billing_invoice;
CREATE TRIGGER trg_idc_bill_invoice_identity_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_invoice
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_billing_invoice_identity();

DROP TRIGGER IF EXISTS trg_idc_bill_account_identity_guard
    ON plugin_idc_core_billing_ledger_account;
CREATE TRIGGER trg_idc_bill_account_identity_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_ledger_account
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_billing_account_identity();

DROP TRIGGER IF EXISTS trg_idc_bill_hold_identity_guard
    ON plugin_idc_core_billing_fund_hold;
CREATE TRIGGER trg_idc_bill_hold_identity_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_fund_hold
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_billing_hold_identity();

DROP TRIGGER IF EXISTS trg_idc_bill_adjustment_intent_guard
    ON plugin_idc_core_billing_adjustment_request;
CREATE TRIGGER trg_idc_bill_adjustment_intent_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_adjustment_request
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_billing_adjustment_intent();

DROP TRIGGER IF EXISTS trg_idc_bill_ledger_tx_immutable
    ON plugin_idc_core_billing_ledger_transaction;
CREATE TRIGGER trg_idc_bill_ledger_tx_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_ledger_transaction
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_ledger_entry_immutable
    ON plugin_idc_core_billing_ledger_entry;
CREATE TRIGGER trg_idc_bill_ledger_entry_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_ledger_entry
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_balance_snapshot_immutable
    ON plugin_idc_core_billing_balance_snapshot;
CREATE TRIGGER trg_idc_bill_balance_snapshot_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_balance_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_payment_allocation_immutable
    ON plugin_idc_core_billing_payment_allocation;
CREATE TRIGGER trg_idc_bill_payment_allocation_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_payment_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_refund_allocation_immutable
    ON plugin_idc_core_billing_refund_allocation;
CREATE TRIGGER trg_idc_bill_refund_allocation_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_refund_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_bill_adjustment_approval_immutable
    ON plugin_idc_core_billing_adjustment_approval;
CREATE TRIGGER trg_idc_bill_adjustment_approval_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_billing_adjustment_approval
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_invoice'::regclass
           AND conname = 'trg_idc_bill_invoice_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_invoice'::regclass
               AND constraint_row.conname = 'trg_idc_bill_invoice_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_invoice_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_invoice_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_invoice_composition
        AFTER INSERT OR UPDATE ON plugin_idc_core_billing_invoice
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_invoice_item'::regclass
           AND conname = 'trg_idc_bill_item_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_invoice_item'::regclass
               AND constraint_row.conname = 'trg_idc_bill_item_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_item_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_item_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_item_composition
        AFTER INSERT ON plugin_idc_core_billing_invoice_item
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_payment_allocation'::regclass
           AND conname = 'trg_idc_bill_payment_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_payment_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_bill_payment_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_payment_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_payment_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_payment_composition
        AFTER INSERT ON plugin_idc_core_billing_payment_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_refund_allocation'::regclass
           AND conname = 'trg_idc_bill_refund_composition'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_refund_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_bill_refund_composition'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_refund_composition'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_invoice_composition()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_refund_composition catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_refund_composition
        AFTER INSERT ON plugin_idc_core_billing_refund_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_invoice_composition();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_ledger_transaction'::regclass
           AND conname = 'trg_idc_bill_tx_balance'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_ledger_transaction'::regclass
               AND constraint_row.conname = 'trg_idc_bill_tx_balance'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_tx_balance'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_ledger_transaction()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_tx_balance catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_tx_balance
        AFTER INSERT ON plugin_idc_core_billing_ledger_transaction
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_ledger_transaction();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_ledger_entry'::regclass
           AND conname = 'trg_idc_bill_entry_balance'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_ledger_entry'::regclass
               AND constraint_row.conname = 'trg_idc_bill_entry_balance'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_entry_balance'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_ledger_transaction()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_entry_balance catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_entry_balance
        AFTER INSERT ON plugin_idc_core_billing_ledger_entry
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_ledger_transaction();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_balance_snapshot'::regclass
           AND conname = 'trg_idc_bill_snapshot_balance'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_balance_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_bill_snapshot_balance'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_snapshot_balance'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_ledger_transaction()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_snapshot_balance catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_snapshot_balance
        AFTER INSERT ON plugin_idc_core_billing_balance_snapshot
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_ledger_transaction();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_balance_snapshot'::regclass
           AND conname = 'trg_idc_bill_snapshot_chain'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_balance_snapshot'::regclass
               AND constraint_row.conname = 'trg_idc_bill_snapshot_chain'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_snapshot_chain'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_balance_snapshot_chain()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_snapshot_chain catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_snapshot_chain
        AFTER INSERT ON plugin_idc_core_billing_balance_snapshot
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_balance_snapshot_chain();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
           AND conname = 'trg_idc_bill_adjustment_state_approval'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
               AND constraint_row.conname = 'trg_idc_bill_adjustment_state_approval'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_adjustment_state_approval'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_adjustment_approval()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_adjustment_state_approval catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_adjustment_state_approval
        AFTER INSERT OR UPDATE ON plugin_idc_core_billing_adjustment_request
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_adjustment_approval();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_billing_adjustment_approval'::regclass
           AND conname = 'trg_idc_bill_approval_state'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_billing_adjustment_approval'::regclass
               AND constraint_row.conname = 'trg_idc_bill_approval_state'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_bill_approval_state'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_adjustment_approval()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_bill_approval_state catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_bill_approval_state
        AFTER INSERT ON plugin_idc_core_billing_adjustment_approval
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_adjustment_approval();
    END IF;
END;
$constraint_trigger$;

COMMENT ON TABLE plugin_idc_core_billing_invoice
    IS 'Receivable invoice header with explicit currency and lifecycle aggregates';
COMMENT ON TABLE plugin_idc_core_billing_invoice_item
    IS 'Append-only invoice commercial line copied from an order or other source';
COMMENT ON TABLE plugin_idc_core_billing_ledger_account
    IS 'Currency-specific customer, merchant, or system double-entry account';
COMMENT ON TABLE plugin_idc_core_billing_ledger_transaction
    IS 'Append-only balanced financial event identified by one business source';
COMMENT ON TABLE plugin_idc_core_billing_ledger_entry
    IS 'Append-only debit or credit entry linked to one same-currency account';
COMMENT ON TABLE plugin_idc_core_billing_balance_snapshot
    IS 'Rebuildable append-only per-account balance projection after a transaction';
COMMENT ON TABLE plugin_idc_core_billing_fund_hold
    IS 'Conserved customer balance hold for combined or deferred payment';
COMMENT ON TABLE plugin_idc_core_billing_payment_allocation
    IS 'Append-only payment allocation or reversal against an invoice';
COMMENT ON TABLE plugin_idc_core_billing_refund_allocation
    IS 'Append-only refund allocation or reversal traced to item and payment';
COMMENT ON TABLE plugin_idc_core_billing_adjustment_request
    IS 'Mutable reviewed intent that cannot itself change financial balances';
COMMENT ON TABLE plugin_idc_core_billing_adjustment_approval
    IS 'Append-only second-person decision for one financial adjustment';

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 账单状态', 'idc_billing_invoice_status', 1, 1, FALSE,
     '应收账单生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 账单来源', 'idc_billing_invoice_source', 1, 1, FALSE,
     '应收账单业务来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 账本账户类型', 'idc_billing_account_type', 1, 1, FALSE,
     '余额、信用、赠送、佣金及系统账户隔离', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 账本账户状态', 'idc_billing_account_status', 1, 1, FALSE,
     '逐币种账本账户生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 正常余额方向', 'idc_billing_normal_balance', 1, 1, FALSE,
     '双式记账账户借贷方向', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 账本交易类型', 'idc_billing_transaction_type', 1, 1, FALSE,
     '不可变资金事件类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 账本分录方向', 'idc_billing_entry_direction', 1, 1, FALSE,
     '借方或贷方分录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 资金锁定状态', 'idc_billing_hold_status', 1, 1, FALSE,
     '同币种客户余额锁定生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 支付分配来源', 'idc_billing_payment_source', 1, 1, FALSE,
     '账单有效入账来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 退款分配来源', 'idc_billing_refund_source', 1, 1, FALSE,
     '账单退款结果来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 调账类型', 'idc_billing_adjustment_type', 1, 1, FALSE,
     '后台财务调账意图类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 调账状态', 'idc_billing_adjustment_status', 1, 1, FALSE,
     '调账申请审核与入账生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 调账审批决定', 'idc_billing_approval_decision', 1, 1, FALSE,
     '第二位财务人员审批决定', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_billing_invoice_status', '草稿', 'draft', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '待支付', 'pending_payment', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '部分支付', 'partially_paid', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '已支付', 'paid', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '已逾期', 'overdue', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '已取消', 'cancelled', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '部分退款', 'partially_refunded', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '已退款', 'refunded', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_status', '坏账', 'bad_debt', 9, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '订单', 'order', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '续费', 'renewal', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '变更', 'change', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '人工', 'manual', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '合并', 'consolidation', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_invoice_source', '迁移', 'migration', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '客户余额', 'customer_balance', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '客户信用', 'customer_credit', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '客户赠送', 'customer_gift', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '客户佣金', 'customer_commission', 4, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '商家现金', 'merchant_cash', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '应收账款', 'accounts_receivable', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '支付清算', 'payment_clearing', 7, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '退款清算', 'refund_clearing', 8, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '收入', 'revenue', 9, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '应付税款', 'tax_payable', 10, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '调账清算', 'adjustment_clearing', 11, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_type', '坏账费用', 'bad_debt_expense', 12, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_status', '活动', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_status', '冻结', 'frozen', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_account_status', '关闭', 'closed', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_normal_balance', '借方', 'debit', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_normal_balance', '贷方', 'credit', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '账单开立', 'invoice_issuance', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '充值', 'recharge', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '支付', 'payment', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '余额支付', 'balance_payment', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '退款', 'refund', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '佣金', 'commission', 6, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '调账', 'adjustment', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '冲销', 'reversal', 8, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '人工收款', 'manual_receipt', 9, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '坏账核销', 'writeoff', 10, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '赠送', 'gift', 11, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '信用', 'credit', 12, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_transaction_type', '迁移', 'migration', 13, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_entry_direction', '借方', 'debit', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_entry_direction', '贷方', 'credit', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_hold_status', '活动', 'active', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_hold_status', '部分消费', 'partially_consumed', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_hold_status', '已消费', 'consumed', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_hold_status', '已释放', 'released', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_hold_status', '已过期', 'expired', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_payment_source', '在线渠道', 'channel_payment', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_payment_source', '客户余额', 'balance', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_payment_source', '客户信用', 'credit', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_payment_source', '人工收款', 'manual_receipt', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_payment_source', '迁移', 'migration', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_refund_source', '渠道退款', 'channel_refund', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_refund_source', '退入余额', 'balance_credit', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_refund_source', '释放信用', 'credit_release', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_refund_source', '人工退款', 'manual', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_refund_source', '迁移', 'migration', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '余额补款', 'balance_credit', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '余额扣款', 'balance_debit', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '账单减免', 'invoice_waiver', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '坏账核销', 'writeoff', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '人工收款', 'manual_receipt', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_status', '待复核', 'pending_review', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_status', '已批准', 'approved', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_status', '已拒绝', 'rejected', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_status', '已取消', 'cancelled', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_status', '已入账', 'posted', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_approval_decision', '批准', 'approved', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_approval_decision', '拒绝', 'rejected', 2, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
