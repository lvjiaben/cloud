-- 131: Separate staff gifts from real receipts and allow available-cash deductions.

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_target;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (
            "adjustment_type_code" IN ('balance_debit', 'manual_receipt')
            AND "invoice_no" IS NULL AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_balance'
        ) OR (
            "adjustment_type_code" IN ('balance_credit', 'marketing_grant_post', 'marketing_grant_reverse')
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

