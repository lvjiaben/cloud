-- Direct assistance preserves all historic decisions; new orders require no approval.
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request DROP CONSTRAINT IF EXISTS ck_idc_com_assisted_request_status;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request ADD CONSTRAINT ck_idc_com_assisted_request_status CHECK (status_code IN ('pending_approval','approved','rejected','direct_ready','ordered'));
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request DROP CONSTRAINT IF EXISTS ck_idc_com_assisted_request_text;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request ADD CONSTRAINT ck_idc_com_assisted_request_text CHECK (length(reason)<=1000 AND length(internal_note)<=2000 AND (decision_reason IS NULL OR length(btrim(decision_reason)) BETWEEN 2 AND 1000));
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request DROP CONSTRAINT IF EXISTS ck_idc_com_assisted_request_lifecycle;
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request ADD CONSTRAINT ck_idc_com_assisted_request_lifecycle CHECK (
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
    OR (status_code IN ('direct_ready','ordered')
 AND decision_code IS NULL AND decision_by IS NULL AND decision_at IS NULL
 AND decision_reason IS NULL AND decision_request_digest IS NULL AND decision_evidence_digest IS NULL
 AND ((status_code='direct_ready' AND order_no IS NULL)
   OR (status_code='ordered' AND quote_no IS NOT NULL AND order_no IS NOT NULL)))
);
