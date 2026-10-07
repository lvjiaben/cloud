-- 027: Allow overdue invoices to retain a partial payment while preserving
-- the same aggregate, settlement, cancellation, and refund invariants.

ALTER TABLE plugin_idc_core_billing_invoice
    DROP CONSTRAINT IF EXISTS ck_idc_bill_invoice_lifecycle;

ALTER TABLE plugin_idc_core_billing_invoice
    ADD CONSTRAINT ck_idc_bill_invoice_lifecycle CHECK (
        ("status_code" = 'draft' AND "issued_at" IS NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0)
        OR ("status_code" = 'pending_payment' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" = 0)
        OR ("status_code" = 'partially_paid' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" > 0
            AND "paid_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'overdue' AND "issued_at" IS NOT NULL
            AND "settled_at" IS NULL AND "cancelled_at" IS NULL
            AND "paid_amount_minor" >= 0
            AND "paid_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
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
            AND "paid_amount_minor" < "total_amount_minor"
            AND "refundable_amount_minor" = 0 AND "refunded_amount_minor" = 0)
    );

COMMENT ON CONSTRAINT ck_idc_bill_invoice_lifecycle
    ON plugin_idc_core_billing_invoice
    IS 'Derived invoice lifecycle; overdue and bad debt preserve effective partial payments';
