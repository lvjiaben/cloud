-- 034: Currency-scoped credit applications, limits, exposure, statements,
-- repayments, delinquency, write-offs, and append-only credit movements.

-- Purpose: Append-only per-currency credit risk and repayment policy revision.
-- 用途：保存信用策略修订相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_policy_revision (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "maximum_limit_minor" BIGINT NOT NULL,
    "dual_review_threshold_minor" BIGINT NOT NULL,
    "billing_day" SMALLINT NOT NULL,
    "repayment_period_days" SMALLINT NOT NULL,
    "grace_period_days" SMALLINT NOT NULL DEFAULT 0,
    "daily_late_fee_basis_points" INT NOT NULL DEFAULT 0,
    "maximum_late_fee_minor" BIGINT NOT NULL DEFAULT 0,
    "configured_by" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_policy_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_credit_policy_values CHECK (
        "policy_version" > 0
        AND "maximum_limit_minor" > 0
        AND "dual_review_threshold_minor" BETWEEN 1 AND "maximum_limit_minor"
        AND "billing_day" BETWEEN 1 AND 28
        AND "repayment_period_days" BETWEEN 1 AND 90
        AND "grace_period_days" BETWEEN 0 AND 90
        AND "daily_late_fee_basis_points" BETWEEN 0 AND 10000
        AND "maximum_late_fee_minor" >= 0
        AND "configured_by" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_credit_policy_late_fee CHECK (
        ("daily_late_fee_basis_points" = 0 AND "maximum_late_fee_minor" = 0)
        OR ("daily_late_fee_basis_points" > 0 AND "maximum_late_fee_minor" > 0)
    ),
    CONSTRAINT uk_idc_credit_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_credit_policy_version UNIQUE (
        "policy_no", "policy_version", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_policy_currency_version UNIQUE (
        "tenant_id", "currency_code", "policy_version"
    )
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_policy_current
    ON plugin_idc_core_credit_policy_revision (
        "tenant_id", "currency_code", "policy_version" DESC, "id" DESC
    );

-- Purpose: Versioned per-currency customer credit application without raw risk material.
-- 用途：保存信用申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_application (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "application_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "requested_limit_minor" BIGINT NOT NULL,
    "requested_valid_from" TIMESTAMPTZ NOT NULL,
    "requested_valid_until" TIMESTAMPTZ NOT NULL,
    "required_approval_count" SMALLINT NOT NULL,
    "risk_evidence_reference" VARCHAR(128) NOT NULL,
    "risk_evidence_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    "request_actor_type_code" VARCHAR(16) NOT NULL,
    "requested_by" INT NOT NULL,
    "decided_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_application_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_application_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_credit_application_values CHECK (
        "requested_limit_minor" > 0
        AND "requested_valid_until" > "requested_valid_from"
        AND "required_approval_count" IN (1, 2)
        AND "risk_evidence_reference" ~
            '^private:[A-Za-z0-9][A-Za-z0-9._/-]{0,119}$'
        AND "risk_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "request_actor_type_code" IN ('customer', 'staff')
        AND "requested_by" > 0
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_application_status CHECK (
        "status_code" IN (
            'pending_review', 'approved', 'rejected', 'cancelled', 'expired'
        )
    ),
    CONSTRAINT ck_idc_credit_application_lifecycle CHECK (
        ("status_code" = 'pending_review' AND "decided_at" IS NULL
            AND "cancelled_at" IS NULL)
        OR ("status_code" IN ('approved', 'rejected', 'expired')
            AND "decided_at" IS NOT NULL AND "cancelled_at" IS NULL)
        OR ("status_code" = 'cancelled' AND "decided_at" IS NULL
            AND "cancelled_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_credit_application_no UNIQUE ("application_no"),
    CONSTRAINT uk_idc_credit_application_owner UNIQUE (
        "application_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_application_digest UNIQUE (
        "tenant_id", "request_digest"
    ),
    CONSTRAINT fk_idc_credit_application_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_application_policy FOREIGN KEY (
        "policy_no", "policy_version", "currency_code"
    ) REFERENCES plugin_idc_core_credit_policy_revision (
        "policy_no", "policy_version", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_application_review_work
    ON plugin_idc_core_credit_application (
        "tenant_id", "status_code", "created_at", "id"
    ) WHERE "status_code" = 'pending_review';

CREATE INDEX IF NOT EXISTS idx_idc_credit_application_customer
    ON plugin_idc_core_credit_application (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only independent credit application approval or rejection evidence.
-- 用途：保存信用申请复核相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_application_review (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "review_no" VARCHAR(40) NOT NULL,
    "application_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "review_sequence" SMALLINT NOT NULL,
    "decision_code" VARCHAR(16) NOT NULL,
    "reviewer_id" INT NOT NULL,
    "comment_digest" CHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_review_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_review_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "review_sequence" IN (1, 2)
        AND "decision_code" IN ('approved', 'rejected')
        AND "reviewer_id" > 0
        AND "comment_digest" ~ '^[0-9a-f]{64}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_credit_review_no UNIQUE ("review_no"),
    CONSTRAINT uk_idc_credit_review_sequence UNIQUE (
        "application_no", "review_sequence"
    ),
    CONSTRAINT uk_idc_credit_review_reviewer UNIQUE (
        "application_no", "reviewer_id"
    ),
    CONSTRAINT fk_idc_credit_review_application FOREIGN KEY (
        "application_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_application (
        "application_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

-- Purpose: Current per-currency credit projection; every version must match one immutable movement.
-- 用途：保存信用账户相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_account (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "ledger_account_no" VARCHAR(40) NOT NULL,
    "ledger_account_type_code" VARCHAR(32) NOT NULL DEFAULT 'customer_credit',
    "origin_application_no" VARCHAR(40) NOT NULL,
    "current_terms_no" VARCHAR(40) NOT NULL,
    "current_terms_version" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_activation',
    "total_limit_minor" BIGINT NOT NULL,
    "available_limit_minor" BIGINT NOT NULL,
    "unbilled_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "billed_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "delinquent_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "written_off_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "valid_from" TIMESTAMPTZ NOT NULL,
    "valid_until" TIMESTAMPTZ NOT NULL,
    "freeze_reason_code" VARCHAR(128),
    "frozen_at" TIMESTAMPTZ,
    "closed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_account_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_account_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_credit_account_ledger_type CHECK (
        "ledger_account_type_code" = 'customer_credit'
    ),
    CONSTRAINT ck_idc_credit_account_status CHECK (
        "status_code" IN (
            'pending_activation', 'active', 'frozen', 'expired',
            'closed', 'bad_debt'
        )
    ),
    CONSTRAINT ck_idc_credit_account_amounts CHECK (
        "total_limit_minor" > 0
        AND "available_limit_minor" >= 0
        AND "unbilled_principal_minor" >= 0
        AND "billed_principal_minor" >= 0
        AND "delinquent_principal_minor" >= 0
        AND "written_off_principal_minor" >= 0
        AND "delinquent_principal_minor" <= "billed_principal_minor"
        AND "total_limit_minor" = "available_limit_minor"
            + "unbilled_principal_minor" + "billed_principal_minor"
            + "written_off_principal_minor"
    ),
    CONSTRAINT ck_idc_credit_account_dates CHECK (
        "valid_until" > "valid_from" AND "current_terms_version" > 0
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_account_lifecycle CHECK (
        ("status_code" IN ('pending_activation', 'active', 'expired')
            AND "freeze_reason_code" IS NULL AND "frozen_at" IS NULL
            AND ("status_code" <> 'closed' AND "closed_at" IS NULL))
        OR ("status_code" IN ('frozen', 'bad_debt')
            AND "freeze_reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
            AND "frozen_at" IS NOT NULL AND "closed_at" IS NULL)
        OR ("status_code" = 'closed' AND "closed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_credit_account_closed_balance CHECK (
        "status_code" <> 'closed'
        OR ("available_limit_minor" = "total_limit_minor"
            AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
    ),
    CONSTRAINT uk_idc_credit_account_no UNIQUE ("credit_account_no"),
    CONSTRAINT uk_idc_credit_account_owner UNIQUE (
        "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_account_currency UNIQUE (
        "credit_account_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_account_customer_currency UNIQUE (
        "tenant_id", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_account_origin UNIQUE ("origin_application_no"),
    CONSTRAINT fk_idc_credit_account_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_account_origin FOREIGN KEY (
        "origin_application_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_application (
        "application_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_account_ledger FOREIGN KEY (
        "ledger_account_no", "customer_no", "currency_code",
        "ledger_account_type_code"
    ) REFERENCES plugin_idc_core_billing_ledger_account (
        "account_no", "customer_no", "currency_code", "account_type_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_account_customer
    ON plugin_idc_core_credit_account (
        "tenant_id", "customer_no", "currency_code", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_credit_account_expiry_work
    ON plugin_idc_core_credit_account (
        "tenant_id", "status_code", "valid_until", "id"
    ) WHERE "status_code" IN ('pending_activation', 'active', 'frozen');

-- Purpose: Append-only approved credit terms snapshot bound to one policy revision.
-- 用途：保存信用账户条款相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_account_terms (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "terms_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "terms_version" BIGINT NOT NULL,
    "application_no" VARCHAR(40) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "total_limit_minor" BIGINT NOT NULL,
    "billing_day" SMALLINT NOT NULL,
    "repayment_period_days" SMALLINT NOT NULL,
    "grace_period_days" SMALLINT NOT NULL,
    "daily_late_fee_basis_points" INT NOT NULL,
    "maximum_late_fee_minor" BIGINT NOT NULL,
    "effective_from" TIMESTAMPTZ NOT NULL,
    "effective_until" TIMESTAMPTZ NOT NULL,
    "approved_by" INT NOT NULL,
    "approval_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_terms_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_terms_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "terms_version" > 0 AND "total_limit_minor" > 0
        AND "billing_day" BETWEEN 1 AND 28
        AND "repayment_period_days" BETWEEN 1 AND 90
        AND "grace_period_days" BETWEEN 0 AND 90
        AND "daily_late_fee_basis_points" BETWEEN 0 AND 10000
        AND "maximum_late_fee_minor" >= 0
        AND "effective_until" > "effective_from"
        AND "approved_by" > 0
        AND "approval_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_credit_terms_late_fee CHECK (
        ("daily_late_fee_basis_points" = 0 AND "maximum_late_fee_minor" = 0)
        OR ("daily_late_fee_basis_points" > 0 AND "maximum_late_fee_minor" > 0)
    ),
    CONSTRAINT uk_idc_credit_terms_no UNIQUE ("terms_no"),
    CONSTRAINT uk_idc_credit_terms_account_version UNIQUE (
        "credit_account_no", "terms_version"
    ),
    CONSTRAINT uk_idc_credit_terms_owner UNIQUE (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_terms_application UNIQUE ("application_no"),
    CONSTRAINT fk_idc_credit_terms_account FOREIGN KEY (
        "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account (
        "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_terms_application FOREIGN KEY (
        "application_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_application (
        "application_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_terms_policy FOREIGN KEY (
        "policy_no", "policy_version", "currency_code"
    ) REFERENCES plugin_idc_core_credit_policy_revision (
        "policy_no", "policy_version", "currency_code"
    ) ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_credit_account
    DROP CONSTRAINT IF EXISTS fk_idc_credit_account_current_terms;
ALTER TABLE plugin_idc_core_credit_account
    ADD CONSTRAINT fk_idc_credit_account_current_terms FOREIGN KEY (
        "current_terms_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account_terms (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;

-- Purpose: Conserved principal exposure from reservation through billing, repayment, release, or write-off.
-- 用途：保存信用曝光相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_exposure (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "exposure_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "order_no" VARCHAR(40),
    "invoice_no" VARCHAR(40),
    "original_principal_minor" BIGINT NOT NULL,
    "unbilled_principal_minor" BIGINT NOT NULL,
    "billed_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "repaid_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "released_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "written_off_principal_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'unbilled',
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_exposure_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_exposure_source CHECK (
        "source_type_code" IN ('order', 'renewal', 'change', 'manual', 'migration')
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
        AND (("source_type_code" = 'order' AND "order_no" IS NOT NULL)
            OR "source_type_code" <> 'order')
    ),
    CONSTRAINT ck_idc_credit_exposure_status CHECK (
        "status_code" IN (
            'unbilled', 'partially_billed', 'billed', 'partially_repaid',
            'settled', 'released', 'written_off'
        )
    ),
    CONSTRAINT ck_idc_credit_exposure_amounts CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "original_principal_minor" > 0
        AND "unbilled_principal_minor" >= 0
        AND "billed_principal_minor" >= 0
        AND "repaid_principal_minor" >= 0
        AND "released_principal_minor" >= 0
        AND "written_off_principal_minor" >= 0
        AND "original_principal_minor" = "unbilled_principal_minor"
            + "billed_principal_minor" + "repaid_principal_minor"
            + "released_principal_minor" + "written_off_principal_minor"
        AND "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_exposure_lifecycle CHECK (
        ("status_code" = 'unbilled' AND "unbilled_principal_minor" > 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" = 0
            AND "released_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'partially_billed' AND "unbilled_principal_minor" > 0
            AND "billed_principal_minor" > 0 AND "repaid_principal_minor" = 0
            AND "released_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'billed' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" > 0 AND "repaid_principal_minor" = 0
            AND "released_principal_minor" = 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'partially_repaid'
            AND ("unbilled_principal_minor" + "billed_principal_minor") > 0
            AND "repaid_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'settled' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'released' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0 AND "repaid_principal_minor" = 0
            AND "released_principal_minor" > 0
            AND "written_off_principal_minor" = 0)
        OR ("status_code" = 'written_off' AND "unbilled_principal_minor" = 0
            AND "billed_principal_minor" = 0
            AND "written_off_principal_minor" > 0)
    ),
    CONSTRAINT uk_idc_credit_exposure_no UNIQUE ("exposure_no"),
    CONSTRAINT uk_idc_credit_exposure_owner UNIQUE (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_exposure_source UNIQUE (
        "credit_account_no", "source_type_code", "source_reference"
    ),
    CONSTRAINT fk_idc_credit_exposure_account FOREIGN KEY (
        "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account (
        "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_exposure_order FOREIGN KEY (
        "order_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_commerce_order (
        "order_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_exposure_invoice FOREIGN KEY (
        "invoice_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_invoice (
        "invoice_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_exposure_unbilled
    ON plugin_idc_core_credit_exposure (
        "tenant_id", "credit_account_no", "status_code", "created_at", "id"
    ) WHERE "unbilled_principal_minor" > 0;

CREATE INDEX IF NOT EXISTS idx_idc_credit_exposure_customer_history
    ON plugin_idc_core_credit_exposure (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Versioned credit statement projection reconciled to immutable lines, repayments, and write-offs.
-- 用途：保存信用对账单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_statement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "statement_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "terms_no" VARCHAR(40) NOT NULL,
    "terms_version" BIGINT NOT NULL,
    "period_started_at" TIMESTAMPTZ NOT NULL,
    "period_ended_at" TIMESTAMPTZ NOT NULL,
    "line_count" INT NOT NULL,
    "principal_minor" BIGINT NOT NULL,
    "fee_minor" BIGINT NOT NULL DEFAULT 0,
    "adjustment_minor" BIGINT NOT NULL DEFAULT 0,
    "total_amount_minor" BIGINT NOT NULL,
    "paid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "written_off_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "outstanding_amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'open',
    "content_digest" CHAR(64) NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "payment_due_at" TIMESTAMPTZ NOT NULL,
    "overdue_at" TIMESTAMPTZ,
    "settled_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "written_off_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_statement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_statement_status CHECK (
        "status_code" IN (
            'open', 'partially_paid', 'paid', 'overdue',
            'written_off', 'cancelled'
        )
    ),
    CONSTRAINT ck_idc_credit_statement_amounts CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "line_count" BETWEEN 1 AND 10000
        AND "principal_minor" >= 0 AND "fee_minor" >= 0
        AND "total_amount_minor" = "principal_minor" + "fee_minor"
            + "adjustment_minor"
        AND "total_amount_minor" >= 0
        AND "paid_amount_minor" >= 0
        AND "written_off_amount_minor" >= 0
        AND "outstanding_amount_minor" >= 0
        AND "total_amount_minor" = "paid_amount_minor"
            + "written_off_amount_minor" + "outstanding_amount_minor"
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_statement_dates CHECK (
        "period_ended_at" > "period_started_at"
        AND "issued_at" >= "period_ended_at"
        AND "payment_due_at" > "issued_at"
    ),
    CONSTRAINT ck_idc_credit_statement_lifecycle CHECK (
        ("status_code" = 'open' AND "paid_amount_minor" = 0
            AND "written_off_amount_minor" = 0
            AND "overdue_at" IS NULL AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'partially_paid' AND "paid_amount_minor" > 0
            AND "outstanding_amount_minor" > 0 AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'paid' AND "outstanding_amount_minor" = 0
            AND "written_off_amount_minor" = 0 AND "settled_at" IS NOT NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'overdue' AND "outstanding_amount_minor" > 0
            AND "overdue_at" IS NOT NULL AND "settled_at" IS NULL
            AND "cancelled_at" IS NULL AND "written_off_at" IS NULL)
        OR ("status_code" = 'written_off' AND "outstanding_amount_minor" = 0
            AND "written_off_amount_minor" > 0 AND "written_off_at" IS NOT NULL
            AND "cancelled_at" IS NULL)
        OR ("status_code" = 'cancelled' AND "paid_amount_minor" = 0
            AND "written_off_amount_minor" = 0 AND "cancelled_at" IS NOT NULL
            AND "settled_at" IS NULL AND "written_off_at" IS NULL)
    ),
    CONSTRAINT uk_idc_credit_statement_no UNIQUE ("statement_no"),
    CONSTRAINT uk_idc_credit_statement_currency UNIQUE (
        "statement_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_statement_owner UNIQUE (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_statement_period UNIQUE (
        "credit_account_no", "period_started_at", "period_ended_at"
    ),
    CONSTRAINT fk_idc_credit_statement_account FOREIGN KEY (
        "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account (
        "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_statement_terms FOREIGN KEY (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account_terms (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_statement_due_work
    ON plugin_idc_core_credit_statement (
        "tenant_id", "status_code", "payment_due_at", "id"
    ) WHERE "status_code" IN ('open', 'partially_paid', 'overdue');

CREATE INDEX IF NOT EXISTS idx_idc_credit_statement_customer_history
    ON plugin_idc_core_credit_statement (
        "tenant_id", "customer_no", "currency_code", "issued_at" DESC, "id" DESC
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'uk_idc_credit_statement_customer_owner'
          AND conrelid = 'plugin_idc_core_credit_statement'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_credit_statement
            ADD CONSTRAINT uk_idc_credit_statement_customer_owner UNIQUE (
                "statement_no", "customer_no", "currency_code"
            );
    END IF;
END;
$constraint$;

ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD COLUMN IF NOT EXISTS "credit_statement_no" VARCHAR(40);

ALTER TABLE plugin_idc_core_billing_adjustment_request
    DROP CONSTRAINT IF EXISTS ck_idc_bill_adjust_target;
ALTER TABLE plugin_idc_core_billing_adjustment_request
    ADD CONSTRAINT ck_idc_bill_adjust_target CHECK (
        (
            "adjustment_type_code" IN ('balance_credit', 'balance_debit')
            AND "invoice_no" IS NULL AND "credit_statement_no" IS NULL
            AND "account_no" IS NOT NULL
            AND "account_type_code" = 'customer_balance'
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

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_bill_adjust_credit_statement'
          AND conrelid = 'plugin_idc_core_billing_adjustment_request'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_billing_adjustment_request
            ADD CONSTRAINT fk_idc_bill_adjust_credit_statement FOREIGN KEY (
                "credit_statement_no", "customer_no", "currency_code"
            ) REFERENCES plugin_idc_core_credit_statement (
                "statement_no", "customer_no", "currency_code"
            ) ON DELETE RESTRICT;
    END IF;
END;
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_bill_adjust_credit_statement
    ON plugin_idc_core_billing_adjustment_request (
        "tenant_id", "credit_statement_no", "created_at" DESC, "id" DESC
    ) WHERE "credit_statement_no" IS NOT NULL;

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
        OLD."credit_statement_no", OLD."account_no", OLD."account_type_code",
        OLD."currency_code", OLD."adjustment_type_code", OLD."amount_minor",
        OLD."reason_code", OLD."evidence_digest", OLD."idempotency_digest",
        OLD."request_digest", OLD."approval_required", OLD."requested_by",
        OLD."proof_reference", OLD."receiving_account_reference",
        OLD."business_purpose_code", OLD."created_at", OLD."policy_no",
        OLD."policy_version"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."adjustment_no", NEW."customer_no", NEW."invoice_no",
        NEW."credit_statement_no", NEW."account_no", NEW."account_type_code",
        NEW."currency_code", NEW."adjustment_type_code", NEW."amount_minor",
        NEW."reason_code", NEW."evidence_digest", NEW."idempotency_digest",
        NEW."request_digest", NEW."approval_required", NEW."requested_by",
        NEW."proof_reference", NEW."receiving_account_reference",
        NEW."business_purpose_code", NEW."created_at", NEW."policy_no",
        NEW."policy_version"
    ) THEN
        RAISE EXCEPTION 'billing adjustment request identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

-- Purpose: Append-only principal, late-fee, adjustment, and refund statement composition.
-- 用途：保存信用对账单明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_statement_line (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "statement_line_no" VARCHAR(40) NOT NULL,
    "statement_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "line_number" INT NOT NULL,
    "line_type_code" VARCHAR(24) NOT NULL,
    "exposure_no" VARCHAR(40),
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_statement_line_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_statement_line_type CHECK (
        "line_type_code" IN ('principal', 'late_fee', 'adjustment', 'refund')
    ),
    CONSTRAINT ck_idc_credit_statement_line_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "line_number" BETWEEN 1 AND 10000
        AND "amount_minor" <> 0
        AND (("line_type_code" IN ('principal', 'late_fee') AND "amount_minor" > 0)
            OR ("line_type_code" = 'adjustment')
            OR ("line_type_code" = 'refund' AND "amount_minor" < 0))
        AND (("line_type_code" IN ('principal', 'refund')
                AND "exposure_no" IS NOT NULL)
            OR "line_type_code" NOT IN ('principal', 'refund'))
        AND "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$'
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_credit_statement_line_no UNIQUE ("statement_line_no"),
    CONSTRAINT uk_idc_credit_statement_line_order UNIQUE (
        "statement_no", "line_number"
    ),
    CONSTRAINT uk_idc_credit_statement_line_source UNIQUE (
        "statement_no", "source_type_code", "source_reference"
    ),
    CONSTRAINT fk_idc_credit_statement_line_statement FOREIGN KEY (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_statement (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_statement_line_exposure FOREIGN KEY (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_exposure (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_statement_line_statement
    ON plugin_idc_core_credit_statement_line (
        "tenant_id", "statement_no", "line_number", "id"
    );

-- Purpose: Versioned online, customer-balance, manual, or migration repayment projection.
-- 用途：保存信用还款相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_repayment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "repayment_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "repayment_type_code" VARCHAR(24) NOT NULL,
    "funding_source_type_code" VARCHAR(24) NOT NULL,
    "funding_source_reference" VARCHAR(128) NOT NULL,
    "ledger_transaction_no" VARCHAR(40),
    "amount_minor" BIGINT NOT NULL,
    "allocated_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "unallocated_amount_minor" BIGINT NOT NULL,
    "refunded_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "confirmation_evidence_digest" CHAR(64),
    "confirmed_at" TIMESTAMPTZ,
    "failed_at" TIMESTAMPTZ,
    "refunded_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_repayment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_repayment_type CHECK (
        "repayment_type_code" IN ('statement', 'prepayment', 'bad_debt_recovery')
        AND "funding_source_type_code" IN (
            'online_payment', 'customer_balance', 'manual_receipt', 'migration'
        )
    ),
    CONSTRAINT ck_idc_credit_repayment_status CHECK (
        "status_code" IN (
            'pending', 'confirmed', 'partially_allocated',
            'allocated', 'refunded', 'failed'
        )
    ),
    CONSTRAINT ck_idc_credit_repayment_amounts CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND length(btrim("funding_source_reference")) BETWEEN 1 AND 128
        AND "amount_minor" > 0
        AND "allocated_amount_minor" >= 0
        AND "unallocated_amount_minor" >= 0
        AND "refunded_amount_minor" >= 0
        AND "amount_minor" = "allocated_amount_minor"
            + "unallocated_amount_minor" + "refunded_amount_minor"
        AND "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND ("confirmation_evidence_digest" IS NULL
            OR "confirmation_evidence_digest" ~ '^[0-9a-f]{64}$')
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_repayment_lifecycle CHECK (
        ("status_code" = 'pending' AND "ledger_transaction_no" IS NULL
            AND "confirmation_evidence_digest" IS NULL AND "confirmed_at" IS NULL
            AND "failed_at" IS NULL AND "refunded_at" IS NULL
            AND "allocated_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'failed' AND "ledger_transaction_no" IS NULL
            AND "confirmation_evidence_digest" IS NULL
            AND "confirmed_at" IS NULL AND "failed_at" IS NOT NULL
            AND "refunded_at" IS NULL
            AND "allocated_amount_minor" = 0 AND "refunded_amount_minor" = 0)
        OR ("status_code" = 'confirmed' AND "ledger_transaction_no" IS NOT NULL
            AND "confirmation_evidence_digest" IS NOT NULL AND "confirmed_at" IS NOT NULL
            AND "allocated_amount_minor" = 0 AND "unallocated_amount_minor" > 0
            AND "failed_at" IS NULL AND "refunded_at" IS NULL)
        OR ("status_code" = 'partially_allocated' AND "ledger_transaction_no" IS NOT NULL
            AND "confirmation_evidence_digest" IS NOT NULL AND "confirmed_at" IS NOT NULL
            AND "allocated_amount_minor" > 0 AND "unallocated_amount_minor" > 0
            AND "failed_at" IS NULL AND "refunded_at" IS NULL)
        OR ("status_code" = 'allocated' AND "ledger_transaction_no" IS NOT NULL
            AND "confirmation_evidence_digest" IS NOT NULL AND "confirmed_at" IS NOT NULL
            AND "allocated_amount_minor" > 0 AND "unallocated_amount_minor" = 0
            AND "refunded_amount_minor" = 0 AND "failed_at" IS NULL)
        OR ("status_code" = 'refunded' AND "ledger_transaction_no" IS NOT NULL
            AND "confirmation_evidence_digest" IS NOT NULL
            AND "confirmed_at" IS NOT NULL AND "failed_at" IS NULL
            AND "refunded_amount_minor" > 0
            AND "unallocated_amount_minor" = 0 AND "refunded_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_credit_repayment_no UNIQUE ("repayment_no"),
    CONSTRAINT uk_idc_credit_repayment_owner UNIQUE (
        "repayment_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_credit_repayment_source UNIQUE (
        "funding_source_type_code", "funding_source_reference"
    ),
    CONSTRAINT fk_idc_credit_repayment_account FOREIGN KEY (
        "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account (
        "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_repayment_transaction FOREIGN KEY (
        "ledger_transaction_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_repayment_unallocated
    ON plugin_idc_core_credit_repayment (
        "tenant_id", "credit_account_no", "status_code", "confirmed_at", "id"
    ) WHERE "unallocated_amount_minor" > 0;

CREATE INDEX IF NOT EXISTS idx_idc_credit_repayment_customer_history
    ON plugin_idc_core_credit_repayment (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only repayment or prepayment allocation with one exact optional reversal.
-- 用途：保存信用还款分配相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_repayment_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "allocation_no" VARCHAR(40) NOT NULL,
    "repayment_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "statement_no" VARCHAR(40),
    "exposure_no" VARCHAR(40),
    "allocation_type_code" VARCHAR(32) NOT NULL,
    "direction_code" VARCHAR(8) NOT NULL DEFAULT 'apply',
    "amount_minor" BIGINT NOT NULL,
    "original_allocation_no" VARCHAR(40),
    "source_reference" VARCHAR(128) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_allocation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_allocation_type CHECK (
        "allocation_type_code" IN (
            'statement_principal', 'statement_fee', 'unbilled_principal',
            'written_off_recovery'
        )
        AND "direction_code" IN ('apply', 'reverse')
    ),
    CONSTRAINT ck_idc_credit_allocation_target CHECK (
        ("allocation_type_code" IN ('statement_principal', 'statement_fee')
            AND "statement_no" IS NOT NULL)
        OR ("allocation_type_code" = 'unbilled_principal'
            AND "exposure_no" IS NOT NULL AND "statement_no" IS NULL)
        OR ("allocation_type_code" = 'written_off_recovery'
            AND "statement_no" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_credit_allocation_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "amount_minor" > 0
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (("direction_code" = 'apply' AND "original_allocation_no" IS NULL)
            OR ("direction_code" = 'reverse' AND "original_allocation_no" IS NOT NULL))
    ),
    CONSTRAINT uk_idc_credit_allocation_no UNIQUE ("allocation_no"),
    CONSTRAINT uk_idc_credit_allocation_source UNIQUE (
        "repayment_no", "source_reference"
    ),
    CONSTRAINT uk_idc_credit_allocation_reversal UNIQUE ("original_allocation_no"),
    CONSTRAINT fk_idc_credit_allocation_repayment FOREIGN KEY (
        "repayment_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_repayment (
        "repayment_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_allocation_statement FOREIGN KEY (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_statement (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_allocation_exposure FOREIGN KEY (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_exposure (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_allocation_original FOREIGN KEY (
        "original_allocation_no"
    ) REFERENCES plugin_idc_core_credit_repayment_allocation (
        "allocation_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_allocation_repayment
    ON plugin_idc_core_credit_repayment_allocation (
        "tenant_id", "repayment_no", "created_at", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_credit_allocation_statement
    ON plugin_idc_core_credit_repayment_allocation (
        "tenant_id", "statement_no", "created_at", "id"
    ) WHERE "statement_no" IS NOT NULL;

-- Purpose: Versioned overdue, freeze, service-action, write-off, and resolution workflow projection.
-- 用途：保存信用逾期案件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_delinquency_case (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "case_no" VARCHAR(40) NOT NULL,
    "statement_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "terms_no" VARCHAR(40) NOT NULL,
    "terms_version" BIGINT NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'grace_period',
    "principal_minor" BIGINT NOT NULL,
    "late_fee_minor" BIGINT NOT NULL DEFAULT 0,
    "outstanding_amount_minor" BIGINT NOT NULL,
    "opened_at" TIMESTAMPTZ NOT NULL,
    "grace_until" TIMESTAMPTZ NOT NULL,
    "frozen_at" TIMESTAMPTZ,
    "service_action_requested_at" TIMESTAMPTZ,
    "written_off_at" TIMESTAMPTZ,
    "resolved_at" TIMESTAMPTZ,
    "reason_code" VARCHAR(128) NOT NULL,
    "source_snapshot_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_delinquency_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_delinquency_status CHECK (
        "status_code" IN (
            'grace_period', 'frozen', 'service_action_pending',
            'written_off', 'resolved'
        )
    ),
    CONSTRAINT ck_idc_credit_delinquency_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "terms_version" > 0
        AND "principal_minor" >= 0
        AND "late_fee_minor" >= 0
        AND "outstanding_amount_minor" >= 0
        AND "outstanding_amount_minor" = "principal_minor" + "late_fee_minor"
        AND (("status_code" = 'resolved' AND "outstanding_amount_minor" = 0)
            OR ("status_code" <> 'resolved' AND "outstanding_amount_minor" > 0))
        AND "grace_until" >= "opened_at"
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "source_snapshot_digest" ~ '^[0-9a-f]{64}$'
        AND "version" > 0
    ),
    CONSTRAINT ck_idc_credit_delinquency_lifecycle CHECK (
        ("status_code" = 'grace_period' AND "frozen_at" IS NULL
            AND "service_action_requested_at" IS NULL
            AND "written_off_at" IS NULL AND "resolved_at" IS NULL)
        OR ("status_code" = 'frozen' AND "frozen_at" IS NOT NULL
            AND "service_action_requested_at" IS NULL
            AND "written_off_at" IS NULL AND "resolved_at" IS NULL)
        OR ("status_code" = 'service_action_pending' AND "frozen_at" IS NOT NULL
            AND "service_action_requested_at" IS NOT NULL
            AND "written_off_at" IS NULL AND "resolved_at" IS NULL)
        OR ("status_code" = 'written_off' AND "frozen_at" IS NOT NULL
            AND "written_off_at" IS NOT NULL AND "resolved_at" IS NULL)
        OR ("status_code" = 'resolved' AND "resolved_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_credit_delinquency_no UNIQUE ("case_no"),
    CONSTRAINT uk_idc_credit_delinquency_statement UNIQUE ("statement_no"),
    CONSTRAINT uk_idc_credit_delinquency_owner UNIQUE (
        "case_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_credit_delinquency_statement FOREIGN KEY (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_statement (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_delinquency_terms FOREIGN KEY (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account_terms (
        "terms_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_delinquency_work
    ON plugin_idc_core_credit_delinquency_case (
        "tenant_id", "status_code", "grace_until", "id"
    ) WHERE "status_code" IN ('grace_period', 'frozen', 'service_action_pending');

CREATE INDEX IF NOT EXISTS idx_idc_credit_delinquency_customer
    ON plugin_idc_core_credit_delinquency_case (
        "tenant_id", "customer_no", "currency_code", "opened_at" DESC, "id" DESC
    );

-- Purpose: Append-only dual-controlled bad-debt write-off or exact reversal linked to the ledger.
-- 用途：保存信用核销相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_writeoff (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "writeoff_no" VARCHAR(40) NOT NULL,
    "statement_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "writeoff_type_code" VARCHAR(16) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "original_writeoff_no" VARCHAR(40),
    "ledger_transaction_no" VARCHAR(40) NOT NULL,
    "reason_code" VARCHAR(128) NOT NULL,
    "requested_by" INT NOT NULL,
    "reviewed_by" INT NOT NULL,
    "approval_evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "posted_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_writeoff_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_writeoff_type CHECK (
        "writeoff_type_code" IN ('writeoff', 'reversal')
    ),
    CONSTRAINT ck_idc_credit_writeoff_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "amount_minor" > 0
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "requested_by" > 0 AND "reviewed_by" > 0
        AND "requested_by" <> "reviewed_by"
        AND "approval_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (("writeoff_type_code" = 'writeoff' AND "original_writeoff_no" IS NULL)
            OR ("writeoff_type_code" = 'reversal' AND "original_writeoff_no" IS NOT NULL))
    ),
    CONSTRAINT uk_idc_credit_writeoff_no UNIQUE ("writeoff_no"),
    CONSTRAINT uk_idc_credit_writeoff_reversal UNIQUE ("original_writeoff_no"),
    CONSTRAINT uk_idc_credit_writeoff_owner UNIQUE (
        "writeoff_no", "statement_no", "credit_account_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_credit_writeoff_statement FOREIGN KEY (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_statement (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_writeoff_transaction FOREIGN KEY (
        "ledger_transaction_no", "currency_code"
    ) REFERENCES plugin_idc_core_billing_ledger_transaction (
        "transaction_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_writeoff_original FOREIGN KEY (
        "original_writeoff_no"
    ) REFERENCES plugin_idc_core_credit_writeoff ("writeoff_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_writeoff_statement
    ON plugin_idc_core_credit_writeoff (
        "tenant_id", "statement_no", "posted_at", "id"
    );

-- Purpose: Append-only complete before-and-after credit projection chain and idempotency evidence.
-- 用途：保存信用账户变动相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_account_movement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "movement_no" VARCHAR(40) NOT NULL,
    "credit_account_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "exposure_no" VARCHAR(40),
    "statement_no" VARCHAR(40),
    "repayment_no" VARCHAR(40),
    "movement_type_code" VARCHAR(24) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "account_version_before" BIGINT NOT NULL,
    "account_version_after" BIGINT NOT NULL,
    "status_before_code" VARCHAR(24),
    "status_after_code" VARCHAR(24) NOT NULL,
    "total_limit_before_minor" BIGINT NOT NULL,
    "total_limit_after_minor" BIGINT NOT NULL,
    "available_before_minor" BIGINT NOT NULL,
    "available_after_minor" BIGINT NOT NULL,
    "unbilled_before_minor" BIGINT NOT NULL,
    "unbilled_after_minor" BIGINT NOT NULL,
    "billed_before_minor" BIGINT NOT NULL,
    "billed_after_minor" BIGINT NOT NULL,
    "delinquent_before_minor" BIGINT NOT NULL,
    "delinquent_after_minor" BIGINT NOT NULL,
    "written_off_before_minor" BIGINT NOT NULL,
    "written_off_after_minor" BIGINT NOT NULL,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "actor_type_code" VARCHAR(16) NOT NULL,
    "actor_id" INT NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_credit_movement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_movement_type CHECK (
        "movement_type_code" IN (
            'limit_granted', 'limit_adjusted', 'reserve', 'release', 'bill',
            'repayment', 'prepayment', 'refund', 'late_fee', 'freeze',
            'unfreeze', 'expire', 'close', 'delinquency', 'cure',
            'writeoff', 'recovery', 'migration'
        )
    ),
    CONSTRAINT ck_idc_credit_movement_status CHECK (
        ("status_before_code" IS NULL OR "status_before_code" IN (
            'pending_activation', 'active', 'frozen', 'expired', 'closed', 'bad_debt'
        ))
        AND "status_after_code" IN (
            'pending_activation', 'active', 'frozen', 'expired', 'closed', 'bad_debt'
        )
    ),
    CONSTRAINT ck_idc_credit_movement_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "account_version_before" >= 0
        AND "account_version_after" = "account_version_before" + 1
        AND "total_limit_before_minor" >= 0 AND "total_limit_after_minor" > 0
        AND "available_before_minor" >= 0 AND "available_after_minor" >= 0
        AND "unbilled_before_minor" >= 0 AND "unbilled_after_minor" >= 0
        AND "billed_before_minor" >= 0 AND "billed_after_minor" >= 0
        AND "delinquent_before_minor" >= 0 AND "delinquent_after_minor" >= 0
        AND "written_off_before_minor" >= 0 AND "written_off_after_minor" >= 0
        AND "delinquent_before_minor" <= "billed_before_minor"
        AND "delinquent_after_minor" <= "billed_after_minor"
        AND "total_limit_before_minor" = "available_before_minor"
            + "unbilled_before_minor" + "billed_before_minor" + "written_off_before_minor"
        AND "total_limit_after_minor" = "available_after_minor"
            + "unbilled_after_minor" + "billed_after_minor" + "written_off_after_minor"
        AND length(btrim("source_reference")) BETWEEN 1 AND 128
        AND "source_type_code" ~ '^[a-z][a-z0-9_.-]{0,31}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "actor_type_code" IN ('customer', 'staff', 'system', 'migration')
        AND "actor_id" >= 0
        AND (("movement_type_code" IN ('freeze', 'unfreeze', 'expire', 'close')
                AND "amount_minor" = 0)
            OR ("movement_type_code" IN ('writeoff', 'recovery')
                AND "amount_minor" >= 0)
            OR ("movement_type_code" NOT IN (
                    'freeze', 'unfreeze', 'expire', 'close', 'writeoff', 'recovery')
                AND "amount_minor" > 0))
    ),
    CONSTRAINT ck_idc_credit_movement_initial CHECK (
        "account_version_before" <> 0
        OR ("status_before_code" IS NULL
            AND "total_limit_before_minor" = 0
            AND "available_before_minor" = 0
            AND "unbilled_before_minor" = 0
            AND "billed_before_minor" = 0
            AND "delinquent_before_minor" = 0
            AND "written_off_before_minor" = 0
            AND "movement_type_code" IN ('limit_granted', 'migration'))
    ),
    CONSTRAINT ck_idc_credit_movement_targets CHECK (
        ("movement_type_code" IN ('reserve', 'release', 'refund')
            AND "exposure_no" IS NOT NULL)
        OR ("movement_type_code" = 'bill' AND "statement_no" IS NOT NULL)
        OR ("movement_type_code" = 'repayment' AND "statement_no" IS NOT NULL
            AND "repayment_no" IS NOT NULL)
        OR ("movement_type_code" = 'prepayment' AND "exposure_no" IS NOT NULL
            AND "repayment_no" IS NOT NULL)
        OR ("movement_type_code" = 'writeoff' AND "statement_no" IS NOT NULL)
        OR ("movement_type_code" = 'recovery' AND "repayment_no" IS NOT NULL)
        OR "movement_type_code" NOT IN (
            'reserve', 'release', 'refund', 'bill', 'repayment',
            'prepayment', 'writeoff', 'recovery'
        )
    ),
    CONSTRAINT uk_idc_credit_movement_no UNIQUE ("movement_no"),
    CONSTRAINT uk_idc_credit_movement_version UNIQUE (
        "credit_account_no", "account_version_after"
    ),
    CONSTRAINT uk_idc_credit_movement_source UNIQUE (
        "credit_account_no", "source_type_code", "source_reference"
    ),
    CONSTRAINT uk_idc_credit_movement_idempotency UNIQUE (
        "tenant_id", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_credit_movement_account FOREIGN KEY (
        "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_account (
        "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT fk_idc_credit_movement_exposure FOREIGN KEY (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_exposure (
        "exposure_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_movement_statement FOREIGN KEY (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_statement (
        "statement_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_credit_movement_repayment FOREIGN KEY (
        "repayment_no", "credit_account_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_credit_repayment (
        "repayment_no", "credit_account_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_movement_account_history
    ON plugin_idc_core_credit_account_movement (
        "tenant_id", "credit_account_no", "account_version_after" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_credit_movement_customer_history
    ON plugin_idc_core_credit_account_movement (
        "tenant_id", "customer_no", "currency_code", "occurred_at" DESC, "id" DESC
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_application()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit applications cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."application_no", OLD."customer_no", OLD."currency_code",
        OLD."policy_no", OLD."policy_version", OLD."requested_limit_minor",
        OLD."requested_valid_from", OLD."requested_valid_until",
        OLD."required_approval_count", OLD."risk_evidence_reference",
        OLD."risk_evidence_digest", OLD."request_digest",
        OLD."request_actor_type_code", OLD."requested_by", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."application_no", NEW."customer_no", NEW."currency_code",
        NEW."policy_no", NEW."policy_version", NEW."requested_limit_minor",
        NEW."requested_valid_from", NEW."requested_valid_until",
        NEW."required_approval_count", NEW."risk_evidence_reference",
        NEW."risk_evidence_digest", NEW."request_digest",
        NEW."request_actor_type_code", NEW."requested_by", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit application intent is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit application version must advance exactly once';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_account()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit accounts cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."credit_account_no", OLD."customer_no", OLD."currency_code",
        OLD."ledger_account_no", OLD."ledger_account_type_code",
        OLD."origin_application_no", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."credit_account_no", NEW."customer_no", NEW."currency_code",
        NEW."ledger_account_no", NEW."ledger_account_type_code",
        NEW."origin_application_no", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit account identity is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit account version must advance exactly once';
    END IF;
    IF OLD."status_code" = NEW."status_code"
       AND ROW(OLD."freeze_reason_code", OLD."frozen_at", OLD."closed_at")
           IS DISTINCT FROM
           ROW(NEW."freeze_reason_code", NEW."frozen_at", NEW."closed_at") THEN
        RAISE EXCEPTION 'credit account lifecycle evidence can change only with status';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_exposure()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit exposures cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."exposure_no", OLD."credit_account_no",
        OLD."customer_no", OLD."currency_code", OLD."source_type_code",
        OLD."source_reference", OLD."order_no", OLD."original_principal_minor",
        OLD."source_snapshot_digest", OLD."content_digest", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."exposure_no", NEW."credit_account_no",
        NEW."customer_no", NEW."currency_code", NEW."source_type_code",
        NEW."source_reference", NEW."order_no", NEW."original_principal_minor",
        NEW."source_snapshot_digest", NEW."content_digest", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit exposure identity is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit exposure version must advance exactly once';
    END IF;
    IF OLD."invoice_no" IS NOT NULL AND NEW."invoice_no" IS DISTINCT FROM OLD."invoice_no" THEN
        RAISE EXCEPTION 'credit exposure invoice reference is immutable once assigned';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_statement()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit statements cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."statement_no", OLD."credit_account_no",
        OLD."customer_no", OLD."currency_code", OLD."terms_no", OLD."terms_version",
        OLD."period_started_at", OLD."period_ended_at",
        OLD."principal_minor", OLD."adjustment_minor",
        OLD."content_digest", OLD."issued_at",
        OLD."payment_due_at", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."statement_no", NEW."credit_account_no",
        NEW."customer_no", NEW."currency_code", NEW."terms_no", NEW."terms_version",
        NEW."period_started_at", NEW."period_ended_at",
        NEW."principal_minor", NEW."adjustment_minor",
        NEW."content_digest", NEW."issued_at",
        NEW."payment_due_at", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit statement composition is immutable';
    END IF;
    IF ROW(OLD."line_count", OLD."fee_minor", OLD."total_amount_minor")
       IS DISTINCT FROM ROW(NEW."line_count", NEW."fee_minor", NEW."total_amount_minor")
       AND NOT (
           OLD."status_code" = 'overdue' AND NEW."status_code" = 'overdue'
           AND NEW."line_count" = OLD."line_count" + 1
           AND NEW."fee_minor" > OLD."fee_minor"
           AND NEW."total_amount_minor" - OLD."total_amount_minor" =
               NEW."fee_minor" - OLD."fee_minor"
           AND NEW."outstanding_amount_minor" - OLD."outstanding_amount_minor" =
               NEW."fee_minor" - OLD."fee_minor"
       ) THEN
        RAISE EXCEPTION 'credit statement fees can only advance through overdue accrual';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit statement version must advance exactly once';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_repayment()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit repayments cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."repayment_no", OLD."credit_account_no",
        OLD."customer_no", OLD."currency_code", OLD."repayment_type_code",
        OLD."funding_source_type_code", OLD."funding_source_reference",
        OLD."amount_minor", OLD."source_snapshot_digest", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."repayment_no", NEW."credit_account_no",
        NEW."customer_no", NEW."currency_code", NEW."repayment_type_code",
        NEW."funding_source_type_code", NEW."funding_source_reference",
        NEW."amount_minor", NEW."source_snapshot_digest", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit repayment intent is immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit repayment version must advance exactly once';
    END IF;
    IF (OLD."ledger_transaction_no" IS NOT NULL
            AND NEW."ledger_transaction_no" IS DISTINCT FROM OLD."ledger_transaction_no")
       OR (OLD."confirmation_evidence_digest" IS NOT NULL
            AND NEW."confirmation_evidence_digest" IS DISTINCT FROM OLD."confirmation_evidence_digest")
       OR (OLD."confirmed_at" IS NOT NULL
            AND NEW."confirmed_at" IS DISTINCT FROM OLD."confirmed_at") THEN
        RAISE EXCEPTION 'credit repayment confirmation evidence is immutable once assigned';
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_credit_delinquency()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'credit delinquency cases cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."case_no", OLD."statement_no", OLD."credit_account_no",
        OLD."customer_no", OLD."currency_code", OLD."terms_no", OLD."terms_version",
        OLD."opened_at", OLD."grace_until",
        OLD."reason_code", OLD."source_snapshot_digest", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."case_no", NEW."statement_no", NEW."credit_account_no",
        NEW."customer_no", NEW."currency_code", NEW."terms_no", NEW."terms_version",
        NEW."opened_at", NEW."grace_until",
        NEW."reason_code", NEW."source_snapshot_digest", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'credit delinquency identity is immutable';
    END IF;
    IF NEW."principal_minor" > OLD."principal_minor" THEN
        RAISE EXCEPTION 'credit delinquency principal cannot increase';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'credit delinquency version must advance exactly once';
    END IF;
    RETURN NEW;
END;
$$;

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
    invalid_reviewer_count BIGINT;
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
    IF (application_row."requested_limit_minor" >= policy_row."dual_review_threshold_minor"
            AND application_row."required_approval_count" <> 2)
       OR (application_row."requested_limit_minor" < policy_row."dual_review_threshold_minor"
            AND application_row."required_approval_count" <> 1) THEN
        RAISE EXCEPTION 'credit application % has the wrong approval count', target_application_no;
    END IF;

    SELECT COUNT(*),
           COUNT(*) FILTER (WHERE "decision_code" = 'approved'),
           COUNT(*) FILTER (WHERE "decision_code" = 'rejected'),
           COALESCE(MAX("review_sequence"), 0),
           COUNT(*) FILTER (
               WHERE application_row."request_actor_type_code" = 'staff'
                 AND "reviewer_id" = application_row."requested_by"
           )
    INTO review_count, approved_count, rejected_count,
         maximum_sequence, invalid_reviewer_count
    FROM plugin_idc_core_credit_application_review
    WHERE "application_no" = target_application_no;

    IF invalid_reviewer_count > 0 THEN
        RAISE EXCEPTION 'credit application % cannot be self reviewed', target_application_no;
    END IF;
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
       AND (review_count <> application_row."required_approval_count"
         OR approved_count <> application_row."required_approval_count"
         OR maximum_sequence <> application_row."required_approval_count") THEN
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
           review_count = application_row."required_approval_count"
           AND approved_count = application_row."required_approval_count"
           AND maximum_sequence = application_row."required_approval_count"
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

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_account_terms()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_account_no VARCHAR(40);
    target_terms_no VARCHAR(40);
    account_row RECORD;
    terms_row RECORD;
    application_row RECORD;
    policy_row RECORD;
    last_reviewer INT;
    origin_application_status VARCHAR(24);
BEGIN
    target_account_no := COALESCE(NEW."credit_account_no", OLD."credit_account_no");
    SELECT * INTO account_row
    FROM plugin_idc_core_credit_account
    WHERE "credit_account_no" = target_account_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    IF TG_TABLE_NAME = 'plugin_idc_core_credit_account_terms' THEN
        target_terms_no := COALESCE(NEW."terms_no", OLD."terms_no");
    ELSE
        target_terms_no := account_row."current_terms_no";
    END IF;
    SELECT * INTO terms_row
    FROM plugin_idc_core_credit_account_terms
    WHERE "terms_no" = target_terms_no
      AND "credit_account_no" = target_account_no;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'credit account % has no current terms', target_account_no;
    END IF;
    SELECT * INTO application_row
    FROM plugin_idc_core_credit_application
    WHERE "application_no" = terms_row."application_no";
    SELECT * INTO policy_row
    FROM plugin_idc_core_credit_policy_revision
    WHERE "policy_no" = terms_row."policy_no"
      AND "policy_version" = terms_row."policy_version"
      AND "currency_code" = terms_row."currency_code";
    SELECT "reviewer_id" INTO last_reviewer
    FROM plugin_idc_core_credit_application_review
    WHERE "application_no" = terms_row."application_no"
      AND "decision_code" = 'approved'
    ORDER BY "review_sequence" DESC
    LIMIT 1;
    SELECT "status_code" INTO origin_application_status
    FROM plugin_idc_core_credit_application
    WHERE "application_no" = account_row."origin_application_no";

    IF application_row."status_code" <> 'approved'
       OR origin_application_status <> 'approved'
       OR terms_row."total_limit_minor" <> application_row."requested_limit_minor"
       OR terms_row."effective_from" <> application_row."requested_valid_from"
       OR terms_row."effective_until" <> application_row."requested_valid_until"
       OR terms_row."billing_day" <> policy_row."billing_day"
       OR terms_row."repayment_period_days" <> policy_row."repayment_period_days"
       OR terms_row."grace_period_days" <> policy_row."grace_period_days"
       OR terms_row."daily_late_fee_basis_points" <> policy_row."daily_late_fee_basis_points"
       OR terms_row."maximum_late_fee_minor" <> policy_row."maximum_late_fee_minor"
       OR terms_row."approved_by" IS DISTINCT FROM last_reviewer THEN
        RAISE EXCEPTION 'credit account % terms do not match approved application and policy', target_account_no;
    END IF;
    IF account_row."current_terms_no" = terms_row."terms_no"
       AND (terms_row."terms_version" <> account_row."current_terms_version"
         OR terms_row."total_limit_minor" <> account_row."total_limit_minor"
         OR terms_row."effective_from" <> account_row."valid_from"
         OR terms_row."effective_until" <> account_row."valid_until") THEN
        RAISE EXCEPTION 'credit account % projection does not match current terms', target_account_no;
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_statement()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_statement_no VARCHAR(40);
    statement_row RECORD;
    terms_row RECORD;
    line_totals RECORD;
    paid_total NUMERIC;
    writeoff_total NUMERIC;
    invalid_count BIGINT;
BEGIN
    target_statement_no := COALESCE(NEW."statement_no", OLD."statement_no");
    IF target_statement_no IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT * INTO statement_row
    FROM plugin_idc_core_credit_statement
    WHERE "statement_no" = target_statement_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT * INTO terms_row
    FROM plugin_idc_core_credit_account_terms
    WHERE "terms_no" = statement_row."terms_no"
      AND "credit_account_no" = statement_row."credit_account_no";
    IF terms_row."terms_version" IS DISTINCT FROM statement_row."terms_version" THEN
        RAISE EXCEPTION 'credit statement % terms version is inconsistent', target_statement_no;
    END IF;
    IF statement_row."payment_due_at" <>
       statement_row."issued_at" + make_interval(days => terms_row."repayment_period_days")
       OR statement_row."fee_minor" > terms_row."maximum_late_fee_minor" THEN
        RAISE EXCEPTION 'credit statement % repayment or fee terms are inconsistent', target_statement_no;
    END IF;
    IF statement_row."status_code" = 'overdue'
       AND (statement_row."overdue_at" < statement_row."payment_due_at"
         OR CURRENT_TIMESTAMP < statement_row."payment_due_at") THEN
        RAISE EXCEPTION 'credit statement % became overdue before its due time', target_statement_no;
    END IF;

    SELECT COUNT(*) AS "line_count",
           COALESCE(SUM("amount_minor") FILTER (WHERE "line_type_code" = 'principal'), 0) AS "principal_minor",
           COALESCE(SUM("amount_minor") FILTER (WHERE "line_type_code" = 'late_fee'), 0) AS "fee_minor",
           COALESCE(SUM("amount_minor") FILTER (WHERE "line_type_code" IN ('adjustment', 'refund')), 0) AS "adjustment_minor"
    INTO line_totals
    FROM plugin_idc_core_credit_statement_line
    WHERE "statement_no" = target_statement_no;
    IF line_totals."line_count" <> statement_row."line_count"
       OR line_totals."principal_minor" <> statement_row."principal_minor"
       OR line_totals."fee_minor" <> statement_row."fee_minor"
       OR line_totals."adjustment_minor" <> statement_row."adjustment_minor" THEN
        RAISE EXCEPTION 'credit statement % line totals do not match', target_statement_no;
    END IF;

    SELECT COALESCE(SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO paid_total
    FROM plugin_idc_core_credit_repayment_allocation
    WHERE "statement_no" = target_statement_no
      AND "allocation_type_code" IN ('statement_principal', 'statement_fee');
    IF paid_total <> statement_row."paid_amount_minor" THEN
        RAISE EXCEPTION 'credit statement % repayment allocations do not match', target_statement_no;
    END IF;

    SELECT COALESCE(SUM(CASE "writeoff_type_code" WHEN 'writeoff' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO writeoff_total
    FROM plugin_idc_core_credit_writeoff
    WHERE "statement_no" = target_statement_no;
    IF writeoff_total <> statement_row."written_off_amount_minor" THEN
        RAISE EXCEPTION 'credit statement % write-offs do not match', target_statement_no;
    END IF;
    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_credit_writeoff reversal
    LEFT JOIN plugin_idc_core_credit_writeoff "original"
      ON "original"."writeoff_no" = reversal."original_writeoff_no"
    WHERE reversal."statement_no" = target_statement_no
      AND reversal."writeoff_type_code" = 'reversal'
      AND ("original"."writeoff_no" IS NULL
        OR "original"."writeoff_type_code" <> 'writeoff'
        OR "original"."statement_no" <> reversal."statement_no"
        OR "original"."credit_account_no" <> reversal."credit_account_no"
        OR "original"."amount_minor" <> reversal."amount_minor");
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit statement % has an invalid write-off reversal', target_statement_no;
    END IF;
    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_credit_writeoff writeoff
    JOIN plugin_idc_core_billing_ledger_transaction transaction
      ON transaction."transaction_no" = writeoff."ledger_transaction_no"
     AND transaction."currency_code" = writeoff."currency_code"
    LEFT JOIN plugin_idc_core_credit_writeoff "original"
      ON "original"."writeoff_no" = writeoff."original_writeoff_no"
    WHERE writeoff."statement_no" = target_statement_no
      AND (transaction."source_reference" <> writeoff."writeoff_no"
        OR (writeoff."writeoff_type_code" = 'writeoff'
            AND transaction."transaction_type_code" <> 'writeoff')
        OR (writeoff."writeoff_type_code" = 'reversal'
            AND (transaction."transaction_type_code" <> 'reversal'
              OR transaction."original_transaction_no" IS DISTINCT FROM
                 "original"."ledger_transaction_no")));
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit statement % write-off ledger evidence is inconsistent', target_statement_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM (
        SELECT "allocation_type_code",
               SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END) AS net_amount
        FROM plugin_idc_core_credit_repayment_allocation
        WHERE "statement_no" = target_statement_no
          AND "allocation_type_code" IN ('statement_principal', 'statement_fee')
        GROUP BY "allocation_type_code"
    ) allocation_totals
    WHERE net_amount < 0
       OR ("allocation_type_code" = 'statement_principal'
           AND net_amount > statement_row."principal_minor")
       OR ("allocation_type_code" = 'statement_fee'
           AND net_amount > statement_row."fee_minor");
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit statement % allocation exceeds its component', target_statement_no;
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_exposure_billing()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_exposure_no VARCHAR(40);
    exposure_row RECORD;
    statement_principal_total NUMERIC;
    prepayment_total NUMERIC;
BEGIN
    target_exposure_no := COALESCE(NEW."exposure_no", OLD."exposure_no");
    IF target_exposure_no IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT * INTO exposure_row
    FROM plugin_idc_core_credit_exposure
    WHERE "exposure_no" = target_exposure_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    SELECT COALESCE(SUM("amount_minor"), 0)
    INTO statement_principal_total
    FROM plugin_idc_core_credit_statement_line
    WHERE "exposure_no" = target_exposure_no
      AND "line_type_code" IN ('principal', 'refund');
    SELECT COALESCE(SUM(
        CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END
    ), 0)
    INTO prepayment_total
    FROM plugin_idc_core_credit_repayment_allocation
    WHERE "exposure_no" = target_exposure_no
      AND "allocation_type_code" = 'unbilled_principal';

    IF prepayment_total < 0
       OR prepayment_total > exposure_row."repaid_principal_minor"
       OR statement_principal_total < 0
       OR statement_principal_total <>
          exposure_row."original_principal_minor"
          - exposure_row."unbilled_principal_minor"
          - exposure_row."released_principal_minor"
          - prepayment_total THEN
        RAISE EXCEPTION 'credit exposure % billing and prepayment history is inconsistent', target_exposure_no;
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_delinquency()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_case_no VARCHAR(40);
    case_row RECORD;
    statement_row RECORD;
    account_row RECORD;
    terms_row RECORD;
BEGIN
    target_case_no := COALESCE(NEW."case_no", OLD."case_no");
    SELECT * INTO case_row
    FROM plugin_idc_core_credit_delinquency_case
    WHERE "case_no" = target_case_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT * INTO statement_row
    FROM plugin_idc_core_credit_statement
    WHERE "statement_no" = case_row."statement_no";
    SELECT * INTO account_row
    FROM plugin_idc_core_credit_account
    WHERE "credit_account_no" = case_row."credit_account_no";
    SELECT * INTO terms_row
    FROM plugin_idc_core_credit_account_terms
    WHERE "terms_no" = case_row."terms_no"
      AND "credit_account_no" = case_row."credit_account_no";

    IF case_row."terms_no" <> statement_row."terms_no"
       OR case_row."terms_version" <> statement_row."terms_version" THEN
        RAISE EXCEPTION 'credit delinquency % terms do not match its statement', target_case_no;
    END IF;
    IF case_row."opened_at" < statement_row."payment_due_at"
       OR case_row."grace_until" <>
          case_row."opened_at" + make_interval(days => terms_row."grace_period_days")
       OR case_row."late_fee_minor" > terms_row."maximum_late_fee_minor" THEN
        RAISE EXCEPTION 'credit delinquency % timing or fee exceeds its terms', target_case_no;
    END IF;
    IF case_row."status_code" IN ('grace_period', 'frozen', 'service_action_pending')
       AND (statement_row."status_code" <> 'overdue'
         OR case_row."outstanding_amount_minor" <> statement_row."outstanding_amount_minor") THEN
        RAISE EXCEPTION 'credit delinquency % does not match the overdue statement', target_case_no;
    END IF;
    IF case_row."status_code" IN ('frozen', 'service_action_pending', 'written_off')
       AND account_row."status_code" NOT IN ('frozen', 'bad_debt') THEN
        RAISE EXCEPTION 'credit delinquency % requires a frozen credit account', target_case_no;
    END IF;
    IF case_row."status_code" = 'written_off'
       AND statement_row."status_code" <> 'written_off' THEN
        RAISE EXCEPTION 'credit delinquency % lacks a written-off statement', target_case_no;
    END IF;
    IF case_row."status_code" = 'resolved'
       AND statement_row."status_code" NOT IN ('paid', 'written_off', 'cancelled') THEN
        RAISE EXCEPTION 'credit delinquency % cannot resolve an unsettled statement', target_case_no;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_credit_policy_immutable
    ON plugin_idc_core_credit_policy_revision;
CREATE TRIGGER trg_idc_credit_policy_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_policy_revision
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_review_immutable
    ON plugin_idc_core_credit_application_review;
CREATE TRIGGER trg_idc_credit_review_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_application_review
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_terms_immutable
    ON plugin_idc_core_credit_account_terms;
CREATE TRIGGER trg_idc_credit_terms_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_account_terms
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_statement_line_immutable
    ON plugin_idc_core_credit_statement_line;
CREATE TRIGGER trg_idc_credit_statement_line_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_statement_line
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_allocation_immutable
    ON plugin_idc_core_credit_repayment_allocation;
CREATE TRIGGER trg_idc_credit_allocation_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_repayment_allocation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_writeoff_immutable
    ON plugin_idc_core_credit_writeoff;
CREATE TRIGGER trg_idc_credit_writeoff_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_writeoff
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_movement_immutable
    ON plugin_idc_core_credit_account_movement;
CREATE TRIGGER trg_idc_credit_movement_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_account_movement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_billing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_credit_application_guard
    ON plugin_idc_core_credit_application;
CREATE TRIGGER trg_idc_credit_application_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_application
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_application();

DROP TRIGGER IF EXISTS trg_idc_credit_account_guard
    ON plugin_idc_core_credit_account;
CREATE TRIGGER trg_idc_credit_account_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_account
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_account();

DROP TRIGGER IF EXISTS trg_idc_credit_exposure_guard
    ON plugin_idc_core_credit_exposure;
CREATE TRIGGER trg_idc_credit_exposure_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_exposure
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_exposure();

DROP TRIGGER IF EXISTS trg_idc_credit_statement_guard
    ON plugin_idc_core_credit_statement;
CREATE TRIGGER trg_idc_credit_statement_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_statement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_statement();

DROP TRIGGER IF EXISTS trg_idc_credit_repayment_guard
    ON plugin_idc_core_credit_repayment;
CREATE TRIGGER trg_idc_credit_repayment_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_repayment
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_repayment();

DROP TRIGGER IF EXISTS trg_idc_credit_delinquency_guard
    ON plugin_idc_core_credit_delinquency_case;
CREATE TRIGGER trg_idc_credit_delinquency_guard
BEFORE UPDATE OR DELETE ON plugin_idc_core_credit_delinquency_case
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_credit_delinquency();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_application'::regclass
           AND conname = 'trg_idc_credit_application_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_application'::regclass
               AND constraint_row.conname = 'trg_idc_credit_application_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_application_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_application()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_application_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_application_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_application
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_application();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_application_review'::regclass
           AND conname = 'trg_idc_credit_review_application_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_application_review'::regclass
               AND constraint_row.conname = 'trg_idc_credit_review_application_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_review_application_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_application()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_review_application_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_review_application_validation
        AFTER INSERT ON plugin_idc_core_credit_application_review
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_application();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_account'::regclass
           AND conname = 'trg_idc_credit_account_terms_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_account'::regclass
               AND constraint_row.conname = 'trg_idc_credit_account_terms_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_account_terms_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_account_terms()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_account_terms_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_account_terms_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_account
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_account_terms();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_account_terms'::regclass
           AND conname = 'trg_idc_credit_terms_account_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_account_terms'::regclass
               AND constraint_row.conname = 'trg_idc_credit_terms_account_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_terms_account_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_account_terms()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_terms_account_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_terms_account_validation
        AFTER INSERT ON plugin_idc_core_credit_account_terms
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_account_terms();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_statement'::regclass
           AND conname = 'trg_idc_credit_statement_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_statement'::regclass
               AND constraint_row.conname = 'trg_idc_credit_statement_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_statement_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_statement()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_statement_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_statement_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_statement
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_statement();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_statement_line'::regclass
           AND conname = 'trg_idc_credit_statement_line_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_statement_line'::regclass
               AND constraint_row.conname = 'trg_idc_credit_statement_line_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_statement_line_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_statement()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_statement_line_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_statement_line_validation
        AFTER INSERT ON plugin_idc_core_credit_statement_line
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_statement();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_exposure'::regclass
           AND conname = 'trg_idc_credit_exposure_billing_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_exposure'::regclass
               AND constraint_row.conname = 'trg_idc_credit_exposure_billing_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_exposure_billing_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_exposure_billing()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_exposure_billing_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_exposure_billing_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_exposure
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_exposure_billing();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_statement_line'::regclass
           AND conname = 'trg_idc_credit_statement_line_exposure_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_statement_line'::regclass
               AND constraint_row.conname = 'trg_idc_credit_statement_line_exposure_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_statement_line_exposure_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_exposure_billing()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_statement_line_exposure_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_statement_line_exposure_validation
        AFTER INSERT ON plugin_idc_core_credit_statement_line
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_exposure_billing();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
           AND conname = 'trg_idc_credit_allocation_statement_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_credit_allocation_statement_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_allocation_statement_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_statement()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_allocation_statement_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_allocation_statement_validation
        AFTER INSERT ON plugin_idc_core_credit_repayment_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_statement();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
           AND conname = 'trg_idc_credit_allocation_exposure_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_credit_allocation_exposure_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_allocation_exposure_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_exposure_billing()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_allocation_exposure_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_allocation_exposure_validation
        AFTER INSERT ON plugin_idc_core_credit_repayment_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_exposure_billing();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_writeoff'::regclass
           AND conname = 'trg_idc_credit_writeoff_statement_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_writeoff'::regclass
               AND constraint_row.conname = 'trg_idc_credit_writeoff_statement_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_writeoff_statement_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_statement()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_writeoff_statement_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_writeoff_statement_validation
        AFTER INSERT ON plugin_idc_core_credit_writeoff
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_statement();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_delinquency_case'::regclass
           AND conname = 'trg_idc_credit_delinquency_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_delinquency_case'::regclass
               AND constraint_row.conname = 'trg_idc_credit_delinquency_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_delinquency_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_delinquency()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_delinquency_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_delinquency_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_delinquency_case
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_delinquency();
    END IF;
END;
$constraint_trigger$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_repayment()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_repayment_no VARCHAR(40);
    repayment_row RECORD;
    transaction_row RECORD;
    allocation_total NUMERIC;
    invalid_count BIGINT;
BEGIN
    target_repayment_no := COALESCE(NEW."repayment_no", OLD."repayment_no");
    SELECT * INTO repayment_row
    FROM plugin_idc_core_credit_repayment
    WHERE "repayment_no" = target_repayment_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    IF repayment_row."ledger_transaction_no" IS NOT NULL THEN
        SELECT * INTO transaction_row
        FROM plugin_idc_core_billing_ledger_transaction
        WHERE "transaction_no" = repayment_row."ledger_transaction_no"
          AND "currency_code" = repayment_row."currency_code";
        IF transaction_row."source_reference" <> repayment_row."funding_source_reference"
           OR transaction_row."debit_total_minor" <> repayment_row."amount_minor"
           OR transaction_row."credit_total_minor" <> repayment_row."amount_minor"
           OR NOT (
               (repayment_row."funding_source_type_code" = 'online_payment'
                    AND transaction_row."transaction_type_code" = 'credit')
               OR (repayment_row."funding_source_type_code" = 'customer_balance'
                    AND transaction_row."transaction_type_code" = 'balance_payment')
               OR (repayment_row."funding_source_type_code" = 'manual_receipt'
                    AND transaction_row."transaction_type_code" = 'manual_receipt')
               OR (repayment_row."funding_source_type_code" = 'migration'
                    AND transaction_row."transaction_type_code" = 'migration')
           ) THEN
            RAISE EXCEPTION 'credit repayment % ledger source is inconsistent', target_repayment_no;
        END IF;
    END IF;

    SELECT COALESCE(SUM(CASE "direction_code" WHEN 'apply' THEN "amount_minor" ELSE -"amount_minor" END), 0)
    INTO allocation_total
    FROM plugin_idc_core_credit_repayment_allocation
    WHERE "repayment_no" = target_repayment_no;
    IF allocation_total < 0
       OR allocation_total <> repayment_row."allocated_amount_minor" THEN
        RAISE EXCEPTION 'credit repayment % allocations do not match', target_repayment_no;
    END IF;

    SELECT COUNT(*) INTO invalid_count
    FROM plugin_idc_core_credit_repayment_allocation allocation
    LEFT JOIN plugin_idc_core_credit_repayment_allocation "original"
      ON "original"."allocation_no" = allocation."original_allocation_no"
    WHERE allocation."repayment_no" = target_repayment_no
      AND (
          (allocation."direction_code" = 'reverse' AND (
              "original"."allocation_no" IS NULL
              OR "original"."direction_code" <> 'apply'
              OR "original"."repayment_no" <> allocation."repayment_no"
              OR "original"."credit_account_no" <> allocation."credit_account_no"
              OR "original"."allocation_type_code" <> allocation."allocation_type_code"
              OR "original"."statement_no" IS DISTINCT FROM allocation."statement_no"
              OR "original"."exposure_no" IS DISTINCT FROM allocation."exposure_no"
              OR "original"."amount_minor" <> allocation."amount_minor"
          ))
          OR (repayment_row."repayment_type_code" = 'statement'
              AND allocation."allocation_type_code" NOT IN ('statement_principal', 'statement_fee'))
          OR (repayment_row."repayment_type_code" = 'prepayment'
              AND allocation."allocation_type_code" <> 'unbilled_principal')
          OR (repayment_row."repayment_type_code" = 'bad_debt_recovery'
              AND allocation."allocation_type_code" <> 'written_off_recovery')
      );
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit repayment % contains an invalid allocation', target_repayment_no;
    END IF;
    RETURN NULL;
END;
$$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_repayment'::regclass
           AND conname = 'trg_idc_credit_repayment_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_repayment'::regclass
               AND constraint_row.conname = 'trg_idc_credit_repayment_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_repayment_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_repayment()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_repayment_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_repayment_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_repayment
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_repayment();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
           AND conname = 'trg_idc_credit_allocation_repayment_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_repayment_allocation'::regclass
               AND constraint_row.conname = 'trg_idc_credit_allocation_repayment_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_allocation_repayment_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_repayment()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_allocation_repayment_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_allocation_repayment_validation
        AFTER INSERT ON plugin_idc_core_credit_repayment_allocation
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_repayment();
    END IF;
END;
$constraint_trigger$;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_credit_movement_chain()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_account_no VARCHAR(40);
    account_row RECORD;
    latest_row RECORD;
    invalid_count BIGINT;
BEGIN
    target_account_no := COALESCE(NEW."credit_account_no", OLD."credit_account_no");
    SELECT * INTO account_row
    FROM plugin_idc_core_credit_account
    WHERE "credit_account_no" = target_account_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;

    WITH chain AS (
        SELECT movement.*,
               ROW_NUMBER() OVER (ORDER BY "account_version_after") AS "expected_version",
               LAG("status_after_code") OVER (ORDER BY "account_version_after") AS "previous_status",
               LAG("total_limit_after_minor") OVER (ORDER BY "account_version_after") AS previous_total,
               LAG("available_after_minor") OVER (ORDER BY "account_version_after") AS previous_available,
               LAG("unbilled_after_minor") OVER (ORDER BY "account_version_after") AS previous_unbilled,
               LAG("billed_after_minor") OVER (ORDER BY "account_version_after") AS previous_billed,
               LAG("delinquent_after_minor") OVER (ORDER BY "account_version_after") AS previous_delinquent,
               LAG("written_off_after_minor") OVER (ORDER BY "account_version_after") AS previous_written_off
        FROM plugin_idc_core_credit_account_movement movement
        WHERE "credit_account_no" = target_account_no
    )
    SELECT COUNT(*) INTO invalid_count
    FROM chain
    WHERE "account_version_after" <> "expected_version"
       OR "account_version_before" <> "account_version_after" - 1
       OR ("account_version_before" > 0 AND (
            "status_before_code" IS DISTINCT FROM "previous_status"
            OR "total_limit_before_minor" IS DISTINCT FROM previous_total
            OR "available_before_minor" IS DISTINCT FROM previous_available
            OR "unbilled_before_minor" IS DISTINCT FROM previous_unbilled
            OR "billed_before_minor" IS DISTINCT FROM previous_billed
            OR "delinquent_before_minor" IS DISTINCT FROM previous_delinquent
            OR "written_off_before_minor" IS DISTINCT FROM previous_written_off
       ))
       OR NOT (
           ("movement_type_code" = 'limit_granted'
                AND "account_version_before" = 0
                AND "status_after_code" IN ('pending_activation', 'active')
                AND "total_limit_after_minor" = "amount_minor"
                AND "available_after_minor" = "amount_minor"
                AND "unbilled_after_minor" = 0 AND "billed_after_minor" = 0
                AND "delinquent_after_minor" = 0 AND "written_off_after_minor" = 0)
           OR ("movement_type_code" = 'limit_adjusted'
                AND ABS("total_limit_after_minor" - "total_limit_before_minor") = "amount_minor"
                AND "available_after_minor" - "available_before_minor" =
                    "total_limit_after_minor" - "total_limit_before_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'reserve'
                AND "status_before_code" = 'active'
                AND "status_after_code" = 'active'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "available_before_minor" - "available_after_minor" = "amount_minor"
                AND "unbilled_after_minor" - "unbilled_before_minor" = "amount_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'bill'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "unbilled_before_minor" - "unbilled_after_minor" = "amount_minor"
                AND "billed_after_minor" - "billed_before_minor" = "amount_minor"
                AND "available_after_minor" = "available_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'repayment'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "billed_before_minor" - "billed_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "delinquent_after_minor" BETWEEN
                    GREATEST(0, "delinquent_before_minor" - "amount_minor")
                    AND "delinquent_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'prepayment'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "unbilled_before_minor" - "unbilled_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" IN ('release', 'refund')
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND ("unbilled_before_minor" + "billed_before_minor")
                    - ("unbilled_after_minor" + "billed_after_minor") = "amount_minor"
                AND "delinquent_before_minor" - "delinquent_after_minor"
                    BETWEEN 0 AND "amount_minor"
                AND "written_off_after_minor" = "written_off_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'writeoff'
                AND "status_before_code" IN ('frozen', 'bad_debt')
                AND "status_after_code" IN ('frozen', 'bad_debt')
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "billed_before_minor" - "billed_after_minor" = "amount_minor"
                AND "written_off_after_minor" - "written_off_before_minor" = "amount_minor"
                AND "available_after_minor" = "available_before_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "delinquent_before_minor" - "delinquent_after_minor"
                    BETWEEN 0 AND "amount_minor"
                AND ("status_after_code" = "status_before_code"
                    OR ("status_before_code" = 'frozen' AND "status_after_code" = 'bad_debt')))
           OR ("movement_type_code" = 'recovery'
                AND "total_limit_after_minor" = "total_limit_before_minor"
                AND "written_off_before_minor" - "written_off_after_minor" = "amount_minor"
                AND "available_after_minor" - "available_before_minor" = "amount_minor"
                AND "unbilled_after_minor" = "unbilled_before_minor"
                AND "billed_after_minor" = "billed_before_minor"
                AND "delinquent_after_minor" = "delinquent_before_minor"
                AND "status_after_code" = "status_before_code")
           OR ("movement_type_code" = 'late_fee'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor",
                        "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor",
                        "status_before_code"))
           OR ("movement_type_code" = 'delinquency'
                AND "delinquent_after_minor" - "delinquent_before_minor" = "amount_minor"
                AND "delinquent_after_minor" <= "billed_after_minor"
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "written_off_after_minor", "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "written_off_before_minor", "status_before_code"))
           OR ("movement_type_code" = 'cure'
                AND "delinquent_before_minor" - "delinquent_after_minor" = "amount_minor"
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "written_off_after_minor", "status_after_code")
                    IS NOT DISTINCT FROM
                    ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "written_off_before_minor", "status_before_code"))
           OR ("movement_type_code" = 'freeze'
                AND "status_before_code" IN ('active', 'expired')
                AND "status_after_code" IN ('frozen', 'bad_debt')
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'unfreeze'
                AND "status_before_code" IN ('frozen', 'bad_debt')
                AND "status_after_code" IN ('active', 'expired')
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'expire'
                AND "status_after_code" = 'expired'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR ("movement_type_code" = 'close'
                AND "status_after_code" = 'closed'
                AND ROW("total_limit_after_minor", "available_after_minor",
                        "unbilled_after_minor", "billed_after_minor",
                        "delinquent_after_minor", "written_off_after_minor")
                    = ROW("total_limit_before_minor", "available_before_minor",
                        "unbilled_before_minor", "billed_before_minor",
                        "delinquent_before_minor", "written_off_before_minor"))
           OR "movement_type_code" = 'migration'
       );
    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'credit account % movement chain is inconsistent', target_account_no;
    END IF;

    SELECT * INTO latest_row
    FROM plugin_idc_core_credit_account_movement
    WHERE "credit_account_no" = target_account_no
    ORDER BY "account_version_after" DESC
    LIMIT 1;
    IF NOT FOUND
       OR ROW(
           latest_row."account_version_after", latest_row."status_after_code",
           latest_row."total_limit_after_minor", latest_row."available_after_minor",
           latest_row."unbilled_after_minor", latest_row."billed_after_minor",
           latest_row."delinquent_after_minor", latest_row."written_off_after_minor"
       ) IS DISTINCT FROM ROW(
           account_row."version", account_row."status_code",
           account_row."total_limit_minor", account_row."available_limit_minor",
           account_row."unbilled_principal_minor", account_row."billed_principal_minor",
           account_row."delinquent_principal_minor", account_row."written_off_principal_minor"
       ) THEN
        RAISE EXCEPTION 'credit account % projection has no matching movement', target_account_no;
    END IF;
    RETURN NULL;
END;
$$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_account'::regclass
           AND conname = 'trg_idc_credit_account_movement_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_account'::regclass
               AND constraint_row.conname = 'trg_idc_credit_account_movement_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_account_movement_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_movement_chain()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_account_movement_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_account_movement_validation
        AFTER INSERT OR UPDATE ON plugin_idc_core_credit_account
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_movement_chain();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_credit_account_movement'::regclass
           AND conname = 'trg_idc_credit_movement_account_validation'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_credit_account_movement'::regclass
               AND constraint_row.conname = 'trg_idc_credit_movement_account_validation'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_credit_movement_account_validation'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_credit_movement_chain()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_credit_movement_account_validation catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_credit_movement_account_validation
        AFTER INSERT ON plugin_idc_core_credit_account_movement
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_credit_movement_chain();
    END IF;
END;
$constraint_trigger$;

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 信用申请状态', 'idc_credit_application_status', 1, 1, FALSE,
     '逐币种信用额度申请及审批状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用账户状态', 'idc_credit_account_status', 1, 1, FALSE,
     '逐币种信用账户生命周期状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用敞口状态', 'idc_credit_exposure_status', 1, 1, FALSE,
     '额度占用、出账、偿还、释放和坏账状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用账单状态', 'idc_credit_statement_status', 1, 1, FALSE,
     '信用账单支付、逾期和坏账状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用还款状态', 'idc_credit_repayment_status', 1, 1, FALSE,
     '信用还款确认、分配、退款及失败状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用逾期状态', 'idc_credit_delinquency_status', 1, 1, FALSE,
     '信用逾期宽限、冻结、服务处置和结案状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用流水类型', 'idc_credit_movement_type', 1, 1, FALSE,
     '不可变信用额度投影变更类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 信用坏账类型', 'idc_credit_writeoff_type', 1, 1, FALSE,
     '信用坏账核销及冲正类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_credit_application_status', '待审批', 'pending_review', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_application_status', '已批准', 'approved', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_application_status', '已拒绝', 'rejected', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_application_status', '已取消', 'cancelled', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_application_status', '已过期', 'expired', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '待激活', 'pending_activation', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '正常', 'active', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '已冻结', 'frozen', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '已到期', 'expired', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '已关闭', 'closed', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_account_status', '坏账', 'bad_debt', 6, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '未出账', 'unbilled', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '部分出账', 'partially_billed', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '已出账', 'billed', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '部分偿还', 'partially_repaid', 4, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '已结清', 'settled', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '已释放', 'released', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_exposure_status', '已核销', 'written_off', 7, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '待还款', 'open', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '部分还款', 'partially_paid', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '已还清', 'paid', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '已逾期', 'overdue', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '已核销', 'written_off', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_statement_status', '已取消', 'cancelled', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '待确认', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '已确认', 'confirmed', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '部分分配', 'partially_allocated', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '已分配', 'allocated', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '已退款', 'refunded', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_repayment_status', '失败', 'failed', 6, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_delinquency_status', '宽限期', 'grace_period', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_delinquency_status', '已冻结', 'frozen', 2, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_delinquency_status', '等待服务处置', 'service_action_pending', 3, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_delinquency_status', '已核销', 'written_off', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_delinquency_status', '已解决', 'resolved', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '授予额度', 'limit_granted', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '调整额度', 'limit_adjusted', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '占用额度', 'reserve', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '释放额度', 'release', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '生成账单', 'bill', 5, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '账单还款', 'repayment', 6, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '提前还款', 'prepayment', 7, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '退款释放', 'refund', 8, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '逾期费用', 'late_fee', 9, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '冻结账户', 'freeze', 10, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '解除冻结', 'unfreeze', 11, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '额度到期', 'expire', 12, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '关闭账户', 'close', 13, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '标记逾期', 'delinquency', 14, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '解除逾期', 'cure', 15, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '坏账核销', 'writeoff', 16, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '坏账收回', 'recovery', 17, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_movement_type', '迁移导入', 'migration', 18, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_writeoff_type', '坏账核销', 'writeoff', 1, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_credit_writeoff_type', '核销冲正', 'reversal', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON TABLE plugin_idc_core_credit_policy_revision
    IS 'Append-only per-currency credit risk and repayment policy revision';
COMMENT ON TABLE plugin_idc_core_credit_application
    IS 'Versioned per-currency customer credit application without raw risk material';
COMMENT ON TABLE plugin_idc_core_credit_application_review
    IS 'Append-only independent credit application approval or rejection evidence';
COMMENT ON TABLE plugin_idc_core_credit_account
    IS 'Current per-currency credit projection; every version must match one immutable movement';
COMMENT ON TABLE plugin_idc_core_credit_account_terms
    IS 'Append-only approved credit terms snapshot bound to one policy revision';
COMMENT ON TABLE plugin_idc_core_credit_exposure
    IS 'Conserved principal exposure from reservation through billing, repayment, release, or write-off';
COMMENT ON TABLE plugin_idc_core_credit_statement
    IS 'Versioned credit statement projection reconciled to immutable lines, repayments, and write-offs';
COMMENT ON TABLE plugin_idc_core_credit_statement_line
    IS 'Append-only principal, late-fee, adjustment, and refund statement composition';
COMMENT ON TABLE plugin_idc_core_credit_repayment
    IS 'Versioned online, customer-balance, manual, or migration repayment projection';
COMMENT ON TABLE plugin_idc_core_credit_repayment_allocation
    IS 'Append-only repayment or prepayment allocation with one exact optional reversal';
COMMENT ON TABLE plugin_idc_core_credit_delinquency_case
    IS 'Versioned overdue, freeze, service-action, write-off, and resolution workflow projection';
COMMENT ON TABLE plugin_idc_core_credit_writeoff
    IS 'Append-only dual-controlled bad-debt write-off or exact reversal linked to the ledger';
COMMENT ON TABLE plugin_idc_core_credit_account_movement
    IS 'Append-only complete before-and-after credit projection chain and idempotency evidence';
