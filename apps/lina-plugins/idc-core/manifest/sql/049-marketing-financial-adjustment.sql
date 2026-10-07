-- 049: Reviewed customer-gift ledger postings for marketing value grants.

ALTER TABLE plugin_idc_core_billing_ledger_account
    DROP CONSTRAINT IF EXISTS ck_idc_bill_account_owner;
ALTER TABLE plugin_idc_core_billing_ledger_account
    ADD CONSTRAINT ck_idc_bill_account_owner CHECK (
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
                'adjustment_clearing', 'bad_debt_expense', 'waiver_expense',
                'marketing_expense'
            ))
    );

ALTER TABLE plugin_idc_core_billing_ledger_account
    DROP CONSTRAINT IF EXISTS ck_idc_bill_account_nature;
ALTER TABLE plugin_idc_core_billing_ledger_account
    ADD CONSTRAINT ck_idc_bill_account_nature CHECK (
        ("account_type_code" IN (
            'customer_balance', 'customer_gift', 'customer_commission',
            'refund_clearing', 'revenue', 'tax_payable', 'adjustment_clearing'
        ) AND "normal_balance_code" = 'credit')
        OR ("account_type_code" IN (
            'customer_credit', 'merchant_cash', 'accounts_receivable',
            'payment_clearing', 'bad_debt_expense', 'waiver_expense',
            'marketing_expense'
        ) AND "normal_balance_code" = 'debit')
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_type;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_type CHECK (
        "adjustment_type_code" IN (
            'balance_credit', 'balance_debit', 'invoice_waiver', 'writeoff',
            'manual_receipt', 'marketing_grant_post', 'marketing_grant_reverse'
        )
    );

ALTER TABLE plugin_idc_core_marketing_value_entry
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_value_entry_source;
ALTER TABLE plugin_idc_core_marketing_value_entry
    ADD CONSTRAINT ck_idc_marketing_value_entry_source CHECK (
        "source_type_code" IN (
            'order', 'refund', 'activity', 'manual', 'migration', 'expiry',
            'financial'
        )
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_target;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (
            "adjustment_type_code" IN ('balance_credit', 'balance_debit', 'manual_receipt')
            AND "invoice_no" IS NULL AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_balance'
        ) OR (
            "adjustment_type_code" IN ('marketing_grant_post', 'marketing_grant_reverse')
            AND "invoice_no" IS NULL AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_gift'
        ) OR (
            "adjustment_type_code" IN ('invoice_waiver', 'writeoff')
            AND "invoice_no" IS NOT NULL AND "account_no" IS NULL
            AND "account_type_code" IS NULL
        ) OR (
            "adjustment_type_code" = 'manual_receipt'
            AND "invoice_no" IS NOT NULL AND "account_no" IS NULL
            AND "account_type_code" IS NULL
        )
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_receipt_evidence;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_receipt_evidence CHECK (
        (
            "adjustment_type_code" = 'manual_receipt'
            AND "proof_reference" IS NOT NULL
            AND "receiving_account_reference" IS NOT NULL
            AND "business_purpose_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
        ) OR (
            "adjustment_type_code" IN ('marketing_grant_post', 'marketing_grant_reverse')
            AND "proof_reference" IS NOT NULL
            AND "receiving_account_reference" IS NULL
            AND "business_purpose_code" IS NULL
        ) OR (
            "adjustment_type_code" NOT IN (
                'manual_receipt', 'marketing_grant_post', 'marketing_grant_reverse'
            )
            AND "proof_reference" IS NULL
            AND "receiving_account_reference" IS NULL
            AND "business_purpose_code" IS NULL
        )
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_posting;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_posting CHECK (
        ("status_code" = 'posted' AND "ledger_transaction_no" IS NOT NULL
            AND "ledger_transaction_type_code" IN (
                'adjustment', 'writeoff', 'manual_receipt', 'reversal', 'gift'
            ) AND "posted_at" IS NOT NULL)
        OR ("status_code" <> 'posted' AND "ledger_transaction_no" IS NULL
            AND "ledger_transaction_type_code" IS NULL AND "posted_at" IS NULL)
    );

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_billing_account_type', '营销费用', 'marketing_expense', 14, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '营销权益入账', 'marketing_grant_post', 6, 'success',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_billing_adjustment_type', '营销权益冲正', 'marketing_grant_reverse', 7, 'warning',
     1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON COLUMN plugin_idc_core_billing_adjustment_request."proof_reference"
    IS 'Verified receipt evidence or the exact marketing value-grant business number';
