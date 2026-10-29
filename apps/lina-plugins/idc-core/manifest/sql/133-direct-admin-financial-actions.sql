-- 133: Permission-authorized admin operations post directly; historical requests
-- remain immutable and can be completed by their original authorized operator.
ALTER TABLE plugin_idc_core_billing_adjustment_approval
    DROP CONSTRAINT IF EXISTS ck_idc_bill_approval_distinct_staff;
ALTER TABLE plugin_idc_core_billing_adjustment_approval
    ADD CONSTRAINT ck_idc_bill_approval_distinct_staff CHECK (
        "requester_id_snapshot" > 0 AND "reviewer_id" > 0
    );
ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_receipt_evidence;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_receipt_evidence CHECK (
        (
            "adjustment_type_code" = 'manual_receipt'
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

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_application()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_application_no VARCHAR(40);
    application_row RECORD;
    policy_row RECORD;
    review_count BIGINT;
    approved_count BIGINT;
    rejected_count BIGINT;
    maximum_sequence SMALLINT;
BEGIN
    target_application_no := COALESCE(NEW."application_no", OLD."application_no");
    SELECT * INTO application_row
    FROM plugin_idc_core_credit_application
    WHERE "application_no" = target_application_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT * INTO policy_row
    FROM plugin_idc_core_credit_policy_revision
    WHERE "policy_no" = application_row."policy_no"
      AND "policy_version" = application_row."policy_version"
      AND "currency_code" = application_row."currency_code";
    IF application_row."requested_limit_minor" > policy_row."maximum_limit_minor" THEN
        RAISE EXCEPTION 'credit application % exceeds the policy maximum', target_application_no;
    END IF;
    SELECT COUNT(*),
           COUNT(*) FILTER (WHERE "decision_code" = 'approved'),
           COUNT(*) FILTER (WHERE "decision_code" = 'rejected'),
           COALESCE(MAX("review_sequence"), 0)
    INTO review_count, approved_count, rejected_count, maximum_sequence
    FROM plugin_idc_core_credit_application_review
    WHERE "application_no" = target_application_no;

    IF application_row."status_code" = 'pending_review'
       AND (review_count >= application_row."required_approval_count"
         OR rejected_count <> 0
         OR approved_count <> review_count
         OR maximum_sequence <> review_count) THEN
        RAISE EXCEPTION 'credit application % has an invalid partial approval set', target_application_no;
    END IF;
    IF application_row."status_code" = 'cancelled'
       AND review_count <> 0 THEN
        RAISE EXCEPTION 'credit application % was cancelled after review began', target_application_no;
    END IF;
    IF application_row."status_code" = 'approved'
       AND (review_count < 1 OR review_count > application_row."required_approval_count"
         OR approved_count <> review_count
         OR maximum_sequence <> review_count) THEN
        RAISE EXCEPTION 'credit application % lacks required approvals', target_application_no;
    END IF;
    IF application_row."status_code" = 'rejected'
       AND (review_count < 1
         OR review_count > application_row."required_approval_count"
         OR rejected_count <> 1
         OR approved_count <> review_count - 1
         OR maximum_sequence <> review_count) THEN
        RAISE EXCEPTION 'credit application % lacks one rejection decision', target_application_no;
    END IF;
    IF application_row."status_code" = 'expired'
       AND NOT (review_count = 0 OR (
           review_count BETWEEN 1 AND application_row."required_approval_count"
           AND approved_count = review_count
           AND maximum_sequence = review_count
       )) THEN
        RAISE EXCEPTION 'credit application % has an invalid expired review set', target_application_no;
    END IF;
    IF application_row."status_code" <> 'approved'
       AND EXISTS (
           SELECT 1
           FROM plugin_idc_core_credit_account_terms terms
           JOIN plugin_idc_core_credit_account account
             ON account."credit_account_no" = terms."credit_account_no"
            AND account."current_terms_no" = terms."terms_no"
           WHERE terms."application_no" = target_application_no
       ) THEN
        RAISE EXCEPTION 'credit application % backs an active account terms snapshot', target_application_no;
    END IF;
    RETURN NULL;
END;
$$;


ALTER TABLE IF EXISTS plugin_idc_core_credit_writeoff
    DROP CONSTRAINT IF EXISTS ck_idc_credit_writeoff_values;
ALTER TABLE IF EXISTS plugin_idc_core_credit_writeoff
    ADD CONSTRAINT ck_idc_credit_writeoff_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "amount_minor" > 0
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "requested_by" > 0 AND "reviewed_by" > 0
        AND "approval_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (("writeoff_type_code" = 'writeoff' AND "original_writeoff_no" IS NULL)
            OR ("writeoff_type_code" = 'reversal' AND "original_writeoff_no" IS NOT NULL))
    );

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_policy_snapshot;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_policy_snapshot CHECK (
        ("policy_no" IS NULL AND "policy_version" IS NULL)
        OR ("policy_no" IS NOT NULL AND "policy_version" IS NOT NULL)
    );
CREATE OR REPLACE FUNCTION plugin_idc_core_validate_adjustment_policy_snapshot()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE threshold_minor BIGINT;
BEGIN
    -- New direct actions need no obsolete threshold snapshot. Historical
    -- non-null snapshots still retain their original consistency check.
    IF NEW."policy_no" IS NULL THEN RETURN NULL; END IF;
    SELECT "review_threshold_minor" INTO threshold_minor
    FROM plugin_idc_core_billing_adjustment_policy
    WHERE "policy_no" = NEW."policy_no" AND "version" = NEW."policy_version";
    IF NOT FOUND OR NEW."approval_required" IS DISTINCT FROM
        (NEW."amount_minor" >= threshold_minor) THEN
        RAISE EXCEPTION 'billing adjustment approval threshold snapshot is invalid';
    END IF;
    RETURN NULL;
END;
$$;

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_target;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (
            "adjustment_type_code" IN ('balance_debit')
            AND "invoice_no" IS NULL AND "credit_statement_no" IS NULL
            AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_balance'
        ) OR (
            "adjustment_type_code" IN ('balance_credit', 'marketing_grant_post', 'marketing_grant_reverse')
            AND "invoice_no" IS NULL AND "credit_statement_no" IS NULL
            AND "account_no" IS NOT NULL AND "account_type_code" = 'customer_gift'
        ) OR (
            "adjustment_type_code" IN ('invoice_waiver', 'writeoff')
            AND "invoice_no" IS NOT NULL AND "credit_statement_no" IS NULL
            AND "account_no" IS NULL AND "account_type_code" IS NULL
        ) OR (
            "adjustment_type_code" = 'manual_receipt'
            AND (
                ("invoice_no" IS NOT NULL AND "credit_statement_no" IS NULL
                    AND "account_no" IS NULL AND "account_type_code" IS NULL)
                OR ("invoice_no" IS NULL AND "credit_statement_no" IS NOT NULL
                    AND "account_no" IS NULL AND "account_type_code" IS NULL)
                OR ("invoice_no" IS NULL AND "credit_statement_no" IS NULL
                    AND "account_no" IS NOT NULL
                    AND "account_type_code" = 'customer_balance')
            )
        )
    );

