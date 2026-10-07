-- Purpose: Persists business records for idc_gateway_price_version.
-- 用途：持久化 idc_gateway_price_version 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_price_version (
    "price_version_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "status" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "published_by_actor_ref" VARCHAR(128) NOT NULL,
    "published_authorization_sha256" BYTEA NOT NULL,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_price_version_owner UNIQUE (
        "price_version_no", "provider_code", "operation_scope", "currency_code"
    ),
    CONSTRAINT ck_gateway_price_version_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo')
    ),
    CONSTRAINT ck_gateway_price_version_scope CHECK (
        "operation_scope" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_price_version_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_gateway_price_version_status CHECK (
        "status" IN ('draft', 'published')
    ),
    CONSTRAINT ck_gateway_price_version_actor CHECK (
        "published_by_actor_ref" = btrim("published_by_actor_ref")
        AND char_length("published_by_actor_ref") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_gateway_price_version_authorization CHECK (
        octet_length("published_authorization_sha256") = 32
    ),
    CONSTRAINT ck_gateway_price_version_time CHECK (
        ("status" = 'draft' AND published_at IS NULL)
        OR ("status" = 'published' AND published_at IS NOT NULL
            AND published_at >= created_at)
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_price_version_lookup
    ON idc_gateway_price_version (
        "provider_code", "operation_scope", "currency_code", "status", published_at DESC,
        "price_version_no"
    );

-- Purpose: Persists business records for idc_gateway_price_item.
-- 用途：持久化 idc_gateway_price_item 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_price_item (
    "price_item_no" VARCHAR(40) PRIMARY KEY,
    "price_version_no" VARCHAR(40) NOT NULL,
    "ordinal" SMALLINT NOT NULL,
    "charge_code" VARCHAR(64) NOT NULL,
    "billing_unit" VARCHAR(32) NOT NULL,
    "unit_price_minor" BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_price_item_version FOREIGN KEY ("price_version_no")
        REFERENCES idc_gateway_price_version ("price_version_no"),
    CONSTRAINT uq_gateway_price_item_ordinal UNIQUE ("price_version_no", "ordinal"),
    CONSTRAINT uq_gateway_price_item_charge UNIQUE ("price_version_no", "charge_code"),
    CONSTRAINT uq_gateway_price_item_snapshot UNIQUE (
        "price_item_no", "price_version_no", "charge_code", "billing_unit", "unit_price_minor"
    ),
    CONSTRAINT ck_gateway_price_item_ordinal CHECK ("ordinal" BETWEEN 1 AND 16),
    CONSTRAINT ck_gateway_price_item_charge CHECK (
        "charge_code" ~ '^[a-z][a-z0-9_.:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_price_item_unit CHECK (
        "billing_unit" ~ '^[a-z][a-z0-9_.:-]{0,31}$'
    ),
    CONSTRAINT ck_gateway_price_item_amount CHECK (
        "unit_price_minor" BETWEEN 1 AND 9000000000000000
    )
);

-- Purpose: Persists business records for idc_gateway_settlement_account.
-- 用途：持久化 idc_gateway_settlement_account 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_account (
    "account_no" VARCHAR(40) PRIMARY KEY,
    "merchant_ref" VARCHAR(128) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "settlement_mode" VARCHAR(16) NOT NULL,
    "status" VARCHAR(24) NOT NULL DEFAULT 'active',
    "prepaid_available_minor" BIGINT NOT NULL DEFAULT 0,
    "held_minor" BIGINT NOT NULL DEFAULT 0,
    "postpaid_outstanding_minor" BIGINT NOT NULL DEFAULT 0,
    "postpaid_limit_minor" BIGINT NOT NULL DEFAULT 0,
    postpaid_valid_until TIMESTAMPTZ,
    "last_ledger_entry_no" VARCHAR(40),
    "version" BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_settlement_account_currency UNIQUE (
        "merchant_ref", "currency_code"
    ),
    CONSTRAINT uq_gateway_settlement_account_owner UNIQUE (
        "account_no", "merchant_ref", "currency_code"
    ),
    CONSTRAINT ck_gateway_settlement_account_merchant CHECK (
        "merchant_ref" = btrim("merchant_ref")
        AND char_length("merchant_ref") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_gateway_settlement_account_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_gateway_settlement_account_mode CHECK (
        "settlement_mode" IN ('prepaid', 'postpaid')
    ),
    CONSTRAINT ck_gateway_settlement_account_status CHECK (
        "status" IN ('active', 'debt_blocked', 'suspended', 'closed')
    ),
    CONSTRAINT ck_gateway_settlement_account_amounts CHECK (
        "prepaid_available_minor" BETWEEN 0 AND 9000000000000000
        AND "held_minor" BETWEEN 0 AND 9000000000000000
        AND "postpaid_outstanding_minor" BETWEEN 0 AND 9000000000000000
        AND "postpaid_limit_minor" BETWEEN 0 AND 9000000000000000
    ),
    CONSTRAINT ck_gateway_settlement_account_projection CHECK (
        ("settlement_mode" = 'prepaid'
            AND "postpaid_outstanding_minor" = 0
            AND "postpaid_limit_minor" = 0
            AND postpaid_valid_until IS NULL)
        OR ("settlement_mode" = 'postpaid'
            AND "prepaid_available_minor" = 0
            AND "postpaid_limit_minor" > 0
            AND postpaid_valid_until IS NOT NULL
            AND "held_minor"::numeric + "postpaid_outstanding_minor"::numeric
                <= "postpaid_limit_minor"::numeric)
    ),
    CONSTRAINT ck_gateway_settlement_account_status_projection CHECK (
        ("status" <> 'debt_blocked'
            OR ("settlement_mode" = 'postpaid' AND "postpaid_outstanding_minor" > 0))
        AND ("status" <> 'closed'
            OR ("prepaid_available_minor" = 0 AND "held_minor" = 0
                AND "postpaid_outstanding_minor" = 0))
    ),
    CONSTRAINT ck_gateway_settlement_account_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_settlement_account_time CHECK (updated_at >= created_at)
);

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_account_status
    ON idc_gateway_settlement_account (
        "status", "settlement_mode", postpaid_valid_until, "account_no"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_instance_merchant_owner
    ON idc_gateway_instance ("instance_no", "merchant_ref");

-- Purpose: Persists business records for idc_gateway_settlement_hold.
-- 用途：持久化 idc_gateway_settlement_hold 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_hold (
    "hold_no" VARCHAR(40) PRIMARY KEY,
    "account_no" VARCHAR(40) NOT NULL,
    "merchant_ref" VARCHAR(128) NOT NULL,
    "instance_no" VARCHAR(40) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "operation_scope" VARCHAR(64) NOT NULL,
    "price_version_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "estimated_amount_minor" BIGINT NOT NULL,
    "charged_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "released_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status" VARCHAR(16) NOT NULL DEFAULT 'held',
    "reconciliation_status" VARCHAR(24) NOT NULL DEFAULT 'none',
    "version" BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    settled_at TIMESTAMPTZ,
    CONSTRAINT uq_gateway_settlement_hold_request UNIQUE ("request_no"),
    CONSTRAINT uq_gateway_settlement_hold_owner UNIQUE (
        "hold_no", "account_no", "merchant_ref", "instance_no", "request_no", "currency_code"
    ),
    CONSTRAINT fk_gateway_settlement_hold_account FOREIGN KEY (
        "account_no", "merchant_ref", "currency_code"
    ) REFERENCES idc_gateway_settlement_account (
        "account_no", "merchant_ref", "currency_code"
    ),
    CONSTRAINT fk_gateway_settlement_hold_instance FOREIGN KEY (
        "instance_no", "merchant_ref"
    ) REFERENCES idc_gateway_instance ("instance_no", "merchant_ref"),
    CONSTRAINT fk_gateway_settlement_hold_request FOREIGN KEY (
        "request_no", "instance_no", "operation_scope"
    ) REFERENCES idc_gateway_business_request (
        "request_no", "instance_no", "operation_scope"
    ),
    CONSTRAINT fk_gateway_settlement_hold_price FOREIGN KEY (
        "price_version_no", "provider_code", "operation_scope", "currency_code"
    ) REFERENCES idc_gateway_price_version (
        "price_version_no", "provider_code", "operation_scope", "currency_code"
    ),
    CONSTRAINT ck_gateway_settlement_hold_amounts CHECK (
        "estimated_amount_minor" BETWEEN 1 AND 9000000000000000
        AND "charged_amount_minor" BETWEEN 0 AND "estimated_amount_minor"
        AND "released_amount_minor" BETWEEN 0 AND "estimated_amount_minor"
    ),
    CONSTRAINT ck_gateway_settlement_hold_status CHECK (
        "status" IN ('held', 'charged', 'released')
    ),
    CONSTRAINT ck_gateway_settlement_hold_reconciliation CHECK (
        "reconciliation_status" IN ('none', 'query_required', 'manual_review')
    ),
    CONSTRAINT ck_gateway_settlement_hold_lifecycle CHECK (
        ("status" = 'held' AND "charged_amount_minor" = 0
            AND "released_amount_minor" = 0 AND settled_at IS NULL)
        OR ("status" = 'charged' AND "charged_amount_minor" > 0
            AND "charged_amount_minor"::numeric + "released_amount_minor"::numeric
                = "estimated_amount_minor"::numeric
            AND "reconciliation_status" = 'none' AND settled_at IS NOT NULL)
        OR ("status" = 'released' AND "charged_amount_minor" = 0
            AND "released_amount_minor" = "estimated_amount_minor"
            AND "reconciliation_status" = 'none' AND settled_at IS NOT NULL)
    ),
    CONSTRAINT ck_gateway_settlement_hold_version CHECK ("version" > 0),
    CONSTRAINT ck_gateway_settlement_hold_time CHECK (
        updated_at >= created_at
        AND (settled_at IS NULL OR settled_at >= created_at)
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_hold_account
    ON idc_gateway_settlement_hold (
        "account_no", "status", created_at, "hold_no"
    );

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_hold_reconcile
    ON idc_gateway_settlement_hold (
        "reconciliation_status", updated_at, "hold_no"
    ) WHERE "status" = 'held' AND "reconciliation_status" <> 'none';

-- Purpose: Persists business records for idc_gateway_settlement_hold_line.
-- 用途：持久化 idc_gateway_settlement_hold_line 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_hold_line (
    "hold_line_no" VARCHAR(40) PRIMARY KEY,
    "hold_no" VARCHAR(40) NOT NULL,
    "price_version_no" VARCHAR(40) NOT NULL,
    "price_item_no" VARCHAR(40) NOT NULL,
    "ordinal" SMALLINT NOT NULL,
    "charge_code" VARCHAR(64) NOT NULL,
    "billing_unit" VARCHAR(32) NOT NULL,
    "unit_price_minor" BIGINT NOT NULL,
    "estimated_units" BIGINT NOT NULL,
    "held_amount_minor" BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_settlement_line_hold FOREIGN KEY ("hold_no")
        REFERENCES idc_gateway_settlement_hold ("hold_no"),
    CONSTRAINT fk_gateway_settlement_line_price FOREIGN KEY (
        "price_item_no", "price_version_no", "charge_code", "billing_unit", "unit_price_minor"
    ) REFERENCES idc_gateway_price_item (
        "price_item_no", "price_version_no", "charge_code", "billing_unit", "unit_price_minor"
    ),
    CONSTRAINT uq_gateway_settlement_line_ordinal UNIQUE ("hold_no", "ordinal"),
    CONSTRAINT uq_gateway_settlement_line_charge UNIQUE ("hold_no", "charge_code"),
    CONSTRAINT uq_gateway_settlement_line_owner UNIQUE (
        "hold_line_no", "hold_no", "price_version_no"
    ),
    CONSTRAINT ck_gateway_settlement_line_ordinal CHECK ("ordinal" BETWEEN 1 AND 16),
    CONSTRAINT ck_gateway_settlement_line_units CHECK (
        "estimated_units" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_gateway_settlement_line_amount CHECK (
        "held_amount_minor" BETWEEN 0 AND 9000000000000000
        AND "held_amount_minor"::numeric
            = "unit_price_minor"::numeric * "estimated_units"::numeric
    )
);

-- Purpose: Persists business records for idc_gateway_settlement_usage_line.
-- 用途：持久化 idc_gateway_settlement_usage_line 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_usage_line (
    "usage_line_no" VARCHAR(40) PRIMARY KEY,
    "hold_line_no" VARCHAR(40) NOT NULL,
    "hold_no" VARCHAR(40) NOT NULL,
    "price_version_no" VARCHAR(40) NOT NULL,
    "actual_units" BIGINT NOT NULL,
    "charged_amount_minor" BIGINT NOT NULL,
    "released_amount_minor" BIGINT NOT NULL,
    "evidence_kind" VARCHAR(32) NOT NULL,
    "evidence_ref" VARCHAR(128) NOT NULL,
    "evidence_sha256" BYTEA NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_usage_line_hold FOREIGN KEY (
        "hold_line_no", "hold_no", "price_version_no"
    ) REFERENCES idc_gateway_settlement_hold_line (
        "hold_line_no", "hold_no", "price_version_no"
    ),
    CONSTRAINT uq_gateway_usage_line_hold_line UNIQUE ("hold_line_no"),
    CONSTRAINT ck_gateway_usage_line_units CHECK (
        "actual_units" BETWEEN 0 AND 1000000
    ),
    CONSTRAINT ck_gateway_usage_line_amounts CHECK (
        "charged_amount_minor" BETWEEN 0 AND 9000000000000000
        AND "released_amount_minor" BETWEEN 0 AND 9000000000000000
    ),
    CONSTRAINT ck_gateway_usage_line_evidence CHECK (
        "evidence_kind" IN (
            'supplier_callback', 'active_query', 'definitive_failure',
            'manual_takeover'
        )
        AND "evidence_ref" = btrim("evidence_ref")
        AND char_length("evidence_ref") BETWEEN 1 AND 128
        AND octet_length("evidence_sha256") = 32
    ),
    CONSTRAINT ck_gateway_usage_line_time CHECK (created_at >= observed_at)
);

-- Purpose: Persists business records for idc_gateway_settlement_ledger_entry.
-- 用途：持久化 idc_gateway_settlement_ledger_entry 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_settlement_ledger_entry (
    "ledger_entry_no" VARCHAR(40) PRIMARY KEY,
    "account_no" VARCHAR(40) NOT NULL,
    "merchant_ref" VARCHAR(128) NOT NULL,
    "instance_no" VARCHAR(40),
    "request_no" VARCHAR(40),
    "hold_no" VARCHAR(40),
    "related_ledger_entry_no" VARCHAR(40),
    "currency_code" CHAR(3) NOT NULL,
    "entry_kind" VARCHAR(32) NOT NULL,
    "operation_hmac" BYTEA NOT NULL,
    "request_sha256" BYTEA NOT NULL,
    "before_prepaid_available_minor" BIGINT NOT NULL,
    "delta_prepaid_available_minor" BIGINT NOT NULL,
    "after_prepaid_available_minor" BIGINT NOT NULL,
    "before_held_minor" BIGINT NOT NULL,
    "delta_held_minor" BIGINT NOT NULL,
    "after_held_minor" BIGINT NOT NULL,
    "before_postpaid_outstanding_minor" BIGINT NOT NULL,
    "delta_postpaid_outstanding_minor" BIGINT NOT NULL,
    "after_postpaid_outstanding_minor" BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_gateway_settlement_ledger_operation UNIQUE ("operation_hmac"),
    CONSTRAINT uq_gateway_settlement_ledger_owner UNIQUE (
        "ledger_entry_no", "account_no"
    ),
    CONSTRAINT fk_gateway_settlement_ledger_account FOREIGN KEY (
        "account_no", "merchant_ref", "currency_code"
    ) REFERENCES idc_gateway_settlement_account (
        "account_no", "merchant_ref", "currency_code"
    ),
    CONSTRAINT fk_gateway_settlement_ledger_hold FOREIGN KEY (
        "hold_no", "account_no", "merchant_ref", "instance_no", "request_no", "currency_code"
    ) REFERENCES idc_gateway_settlement_hold (
        "hold_no", "account_no", "merchant_ref", "instance_no", "request_no", "currency_code"
    ),
    CONSTRAINT fk_gateway_settlement_ledger_related FOREIGN KEY (
        "related_ledger_entry_no", "account_no"
    ) REFERENCES idc_gateway_settlement_ledger_entry (
        "ledger_entry_no", "account_no"
    ) DEFERRABLE INITIALLY DEFERRED,
    CONSTRAINT ck_gateway_settlement_ledger_kind CHECK (
        "entry_kind" IN (
            'prepaid_fund', 'prepaid_hold', 'prepaid_charge', 'prepaid_release',
            'prepaid_adjustment',
            'postpaid_hold', 'postpaid_charge', 'postpaid_release',
            'postpaid_payment', 'postpaid_adjustment'
        )
    ),
    CONSTRAINT ck_gateway_settlement_ledger_digest CHECK (
        octet_length("operation_hmac") = 32 AND octet_length("request_sha256") = 32
    ),
    CONSTRAINT ck_gateway_settlement_ledger_owner CHECK (
        ("hold_no" IS NULL AND "instance_no" IS NULL AND "request_no" IS NULL
            AND "entry_kind" IN (
                'prepaid_fund', 'prepaid_adjustment',
                'postpaid_payment', 'postpaid_adjustment'
            ))
        OR ("hold_no" IS NOT NULL AND "instance_no" IS NOT NULL AND "request_no" IS NOT NULL
            AND "entry_kind" NOT IN (
                'prepaid_fund', 'prepaid_adjustment',
                'postpaid_payment', 'postpaid_adjustment'
            ))
    ),
    CONSTRAINT ck_gateway_settlement_ledger_relation CHECK (
        ("entry_kind" IN ('prepaid_adjustment', 'postpaid_adjustment')
            AND "related_ledger_entry_no" IS NOT NULL
            AND "related_ledger_entry_no" <> "ledger_entry_no")
        OR ("entry_kind" NOT IN ('prepaid_adjustment', 'postpaid_adjustment')
            AND "related_ledger_entry_no" IS NULL)
    ),
    CONSTRAINT ck_gateway_settlement_ledger_buckets CHECK (
        "before_prepaid_available_minor" BETWEEN 0 AND 9000000000000000
        AND "after_prepaid_available_minor" BETWEEN 0 AND 9000000000000000
        AND "before_held_minor" BETWEEN 0 AND 9000000000000000
        AND "after_held_minor" BETWEEN 0 AND 9000000000000000
        AND "before_postpaid_outstanding_minor" BETWEEN 0 AND 9000000000000000
        AND "after_postpaid_outstanding_minor" BETWEEN 0 AND 9000000000000000
        AND "after_prepaid_available_minor"::numeric
            = "before_prepaid_available_minor"::numeric
                + "delta_prepaid_available_minor"::numeric
        AND "after_held_minor"::numeric
            = "before_held_minor"::numeric + "delta_held_minor"::numeric
        AND "after_postpaid_outstanding_minor"::numeric
            = "before_postpaid_outstanding_minor"::numeric
                + "delta_postpaid_outstanding_minor"::numeric
    ),
    CONSTRAINT ck_gateway_settlement_ledger_semantics CHECK (
        ("entry_kind" = 'prepaid_fund'
            AND "delta_prepaid_available_minor" > 0
            AND "delta_held_minor" = 0 AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'prepaid_adjustment'
            AND "delta_prepaid_available_minor" <> 0
            AND "delta_held_minor" = 0 AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'prepaid_hold'
            AND "delta_prepaid_available_minor" < 0
            AND "delta_held_minor" = -"delta_prepaid_available_minor"
            AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'prepaid_charge'
            AND "delta_prepaid_available_minor" >= 0 AND "delta_held_minor" < 0
            AND "delta_prepaid_available_minor" < -"delta_held_minor"
            AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'prepaid_release'
            AND "delta_prepaid_available_minor" > 0
            AND "delta_held_minor" = -"delta_prepaid_available_minor"
            AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'postpaid_hold'
            AND "delta_prepaid_available_minor" = 0 AND "delta_held_minor" > 0
            AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'postpaid_charge'
            AND "delta_prepaid_available_minor" = 0 AND "delta_held_minor" < 0
            AND "delta_postpaid_outstanding_minor" > 0
            AND "delta_postpaid_outstanding_minor" <= -"delta_held_minor")
        OR ("entry_kind" = 'postpaid_release'
            AND "delta_prepaid_available_minor" = 0 AND "delta_held_minor" < 0
            AND "delta_postpaid_outstanding_minor" = 0)
        OR ("entry_kind" = 'postpaid_payment'
            AND "delta_prepaid_available_minor" = 0 AND "delta_held_minor" = 0
            AND "delta_postpaid_outstanding_minor" < 0)
        OR ("entry_kind" = 'postpaid_adjustment'
            AND "delta_prepaid_available_minor" = 0 AND "delta_held_minor" = 0
            AND "delta_postpaid_outstanding_minor" <> 0)
    ),
    CONSTRAINT ck_gateway_settlement_ledger_time CHECK (created_at = occurred_at)
);

ALTER TABLE idc_gateway_settlement_account
    DROP CONSTRAINT IF EXISTS fk_gateway_settlement_account_last_ledger;
ALTER TABLE idc_gateway_settlement_account
    ADD CONSTRAINT fk_gateway_settlement_account_last_ledger FOREIGN KEY (
        "last_ledger_entry_no", "account_no"
    ) REFERENCES idc_gateway_settlement_ledger_entry (
        "ledger_entry_no", "account_no"
    ) DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_ledger_account
    ON idc_gateway_settlement_ledger_entry (
        "account_no", occurred_at, "ledger_entry_no"
    );

CREATE INDEX IF NOT EXISTS idx_gateway_settlement_ledger_request
    ON idc_gateway_settlement_ledger_entry (
        "request_no", occurred_at, "ledger_entry_no"
    ) WHERE "request_no" IS NOT NULL;

CREATE OR REPLACE FUNCTION idc_gateway_reject_settlement_history_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway settlement history is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_price_version_update()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."status" <> 'draft' OR NEW.published_at IS NOT NULL THEN
            RAISE EXCEPTION 'gateway price version must start as draft';
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE'
        OR OLD."price_version_no" IS DISTINCT FROM NEW."price_version_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."currency_code" IS DISTINCT FROM NEW."currency_code"
        OR OLD."published_by_actor_ref" IS DISTINCT FROM NEW."published_by_actor_ref"
        OR OLD."published_authorization_sha256"
            IS DISTINCT FROM NEW."published_authorization_sha256"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status" <> 'draft' OR NEW."status" <> 'published'
        OR OLD.published_at IS NOT NULL OR NEW.published_at IS NULL
    THEN
        RAISE EXCEPTION 'gateway published price version is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_price_item_mutation()
RETURNS TRIGGER AS $$
DECLARE
    version_status VARCHAR(16);
BEGIN
    IF TG_OP <> 'INSERT' THEN
        RAISE EXCEPTION 'gateway published price items are immutable';
    END IF;
    SELECT "status"
    INTO version_status
    FROM idc_gateway_price_version
    WHERE "price_version_no" = NEW."price_version_no"
    FOR UPDATE;
    IF version_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'gateway published price items are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_settlement_hold_update()
RETURNS TRIGGER AS $$
DECLARE
    price_status VARCHAR(16);
BEGIN
    IF TG_OP = 'INSERT' THEN
        SELECT "status"
        INTO price_status
        FROM idc_gateway_price_version
        WHERE "price_version_no" = NEW."price_version_no";
        IF NEW."status" <> 'held' OR NEW."reconciliation_status" <> 'none'
            OR NEW."charged_amount_minor" <> 0 OR NEW."released_amount_minor" <> 0
            OR NEW."version" <> 1 OR NEW.settled_at IS NOT NULL
            OR NEW.created_at <> NEW.updated_at
            OR price_status IS DISTINCT FROM 'published'
        THEN
            RAISE EXCEPTION 'gateway settlement hold must start held';
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'gateway settlement hold transition is invalid';
    END IF;
    IF OLD."hold_no" IS DISTINCT FROM NEW."hold_no"
        OR OLD."account_no" IS DISTINCT FROM NEW."account_no"
        OR OLD."merchant_ref" IS DISTINCT FROM NEW."merchant_ref"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD."price_version_no" IS DISTINCT FROM NEW."price_version_no"
        OR OLD."currency_code" IS DISTINCT FROM NEW."currency_code"
        OR OLD."estimated_amount_minor" IS DISTINCT FROM NEW."estimated_amount_minor"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR NEW."version" <> OLD."version" + 1
        OR NOT (
            (OLD."status" = 'held' AND NEW."status" = 'held'
                AND OLD."charged_amount_minor" = NEW."charged_amount_minor"
                AND OLD."released_amount_minor" = NEW."released_amount_minor"
                AND OLD.settled_at IS NOT DISTINCT FROM NEW.settled_at)
            OR (OLD."status" = 'held' AND NEW."status" IN ('charged', 'released'))
        )
    THEN
        RAISE EXCEPTION 'gateway settlement hold transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_guard_settlement_account_update()
RETURNS TRIGGER AS $$
DECLARE
    buckets_changed BOOLEAN;
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW."prepaid_available_minor" <> 0 OR NEW."held_minor" <> 0
            OR NEW."postpaid_outstanding_minor" <> 0 OR NEW."version" <> 1
            OR NEW."last_ledger_entry_no" IS NOT NULL
            OR NEW.created_at <> NEW.updated_at
        THEN
            RAISE EXCEPTION 'gateway settlement account must start empty';
        END IF;
        RETURN NEW;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'gateway settlement account transition is invalid';
    END IF;
    buckets_changed := OLD."prepaid_available_minor" IS DISTINCT FROM NEW."prepaid_available_minor"
        OR OLD."held_minor" IS DISTINCT FROM NEW."held_minor"
        OR OLD."postpaid_outstanding_minor" IS DISTINCT FROM NEW."postpaid_outstanding_minor";
    IF OLD."account_no" IS DISTINCT FROM NEW."account_no"
        OR OLD."merchant_ref" IS DISTINCT FROM NEW."merchant_ref"
        OR OLD."currency_code" IS DISTINCT FROM NEW."currency_code"
        OR OLD."settlement_mode" IS DISTINCT FROM NEW."settlement_mode"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR NEW."version" <> OLD."version" + 1
        OR NEW.updated_at <= OLD.updated_at
        OR (buckets_changed
            AND (NEW."last_ledger_entry_no" IS NULL
                OR NEW."last_ledger_entry_no" IS NOT DISTINCT FROM OLD."last_ledger_entry_no"))
        OR (NOT buckets_changed
            AND NEW."last_ledger_entry_no" IS DISTINCT FROM OLD."last_ledger_entry_no")
    THEN
        RAISE EXCEPTION 'gateway settlement account transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_settlement_account_ledger()
RETURNS TRIGGER AS $$
DECLARE
    ledger_count INTEGER;
BEGIN
    IF OLD."prepaid_available_minor" IS NOT DISTINCT FROM NEW."prepaid_available_minor"
        AND OLD."held_minor" IS NOT DISTINCT FROM NEW."held_minor"
        AND OLD."postpaid_outstanding_minor" IS NOT DISTINCT FROM NEW."postpaid_outstanding_minor"
    THEN
        RETURN NULL;
    END IF;
    SELECT COUNT(*)
    INTO ledger_count
    FROM idc_gateway_settlement_ledger_entry
    WHERE "ledger_entry_no" = NEW."last_ledger_entry_no"
      AND "account_no" = NEW."account_no"
      AND "merchant_ref" = NEW."merchant_ref"
      AND "currency_code" = NEW."currency_code"
      AND "before_prepaid_available_minor" = OLD."prepaid_available_minor"
      AND "after_prepaid_available_minor" = NEW."prepaid_available_minor"
      AND "before_held_minor" = OLD."held_minor"
      AND "after_held_minor" = NEW."held_minor"
      AND "before_postpaid_outstanding_minor" = OLD."postpaid_outstanding_minor"
      AND "after_postpaid_outstanding_minor" = NEW."postpaid_outstanding_minor"
      AND occurred_at = NEW.updated_at;
    IF ledger_count <> 1 THEN
        RAISE EXCEPTION 'gateway settlement account update has no exact ledger proof';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_settlement_ledger_account()
RETURNS TRIGGER AS $$
DECLARE
    account_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO account_count
    FROM idc_gateway_settlement_account
    WHERE "account_no" = NEW."account_no"
      AND "merchant_ref" = NEW."merchant_ref"
      AND "currency_code" = NEW."currency_code"
      AND "last_ledger_entry_no" = NEW."ledger_entry_no"
      AND "prepaid_available_minor" = NEW."after_prepaid_available_minor"
      AND "held_minor" = NEW."after_held_minor"
      AND "postpaid_outstanding_minor" = NEW."after_postpaid_outstanding_minor"
      AND updated_at = NEW.occurred_at;
    IF account_count <> 1 THEN
        RAISE EXCEPTION 'gateway settlement ledger has no exact account projection';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_price_version_items()
RETURNS TRIGGER AS $$
DECLARE
    "target_version_no" VARCHAR(40);
    "item_count" INTEGER;
    minimum_ordinal INTEGER;
    maximum_ordinal INTEGER;
BEGIN
    "target_version_no" := COALESCE(NEW."price_version_no", OLD."price_version_no");
    SELECT COUNT(*), MIN("ordinal"), MAX("ordinal")
    INTO "item_count", minimum_ordinal, maximum_ordinal
    FROM idc_gateway_price_item
    WHERE "price_version_no" = "target_version_no";
    IF "item_count" NOT BETWEEN 1 AND 16 OR minimum_ordinal <> 1
        OR maximum_ordinal <> "item_count"
    THEN
        RAISE EXCEPTION 'gateway price version item count is invalid';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION idc_gateway_validate_settlement_hold_lines()
RETURNS TRIGGER AS $$
DECLARE
    target_hold_no VARCHAR(40);
    hold_status VARCHAR(16);
    hold_estimated BIGINT;
    hold_charged BIGINT;
    hold_released BIGINT;
    hold_price_version_no VARCHAR(40);
    "line_count" INTEGER;
    line_total NUMERIC;
    line_minimum_ordinal INTEGER;
    line_maximum_ordinal INTEGER;
    wrong_price_line_count INTEGER;
    usage_count INTEGER;
    usage_charged NUMERIC;
    usage_released NUMERIC;
    invalid_usage_count INTEGER;
    hold_ledger_count INTEGER;
    terminal_ledger_count INTEGER;
    total_ledger_count INTEGER;
    account_mode VARCHAR(16);
BEGIN
    target_hold_no := COALESCE(NEW."hold_no", OLD."hold_no");
    SELECT "status", "estimated_amount_minor", "charged_amount_minor", "released_amount_minor",
           "price_version_no"
    INTO hold_status, hold_estimated, hold_charged, hold_released,
         hold_price_version_no
    FROM idc_gateway_settlement_hold
    WHERE "hold_no" = target_hold_no;
    IF NOT FOUND THEN
        RETURN NULL;
    END IF;
    SELECT COUNT(*), COALESCE(SUM("held_amount_minor"::numeric), 0),
           MIN("ordinal"), MAX("ordinal"),
           COUNT(*) FILTER (WHERE "price_version_no" <> hold_price_version_no)
    INTO "line_count", line_total, line_minimum_ordinal, line_maximum_ordinal,
         wrong_price_line_count
    FROM idc_gateway_settlement_hold_line
    WHERE "hold_no" = target_hold_no;
    IF "line_count" NOT BETWEEN 1 AND 16 OR line_total <> hold_estimated::numeric
        OR line_minimum_ordinal <> 1 OR line_maximum_ordinal <> "line_count"
        OR wrong_price_line_count <> 0
    THEN
        RAISE EXCEPTION 'gateway settlement hold lines do not balance';
    END IF;
    SELECT COUNT(*), COALESCE(SUM("charged_amount_minor"::numeric), 0),
           COALESCE(SUM("released_amount_minor"::numeric), 0)
    INTO usage_count, usage_charged, usage_released
    FROM idc_gateway_settlement_usage_line
    WHERE "hold_no" = target_hold_no;
    IF hold_status = 'held' AND usage_count <> 0 THEN
        RAISE EXCEPTION 'gateway held settlement cannot contain usage';
    END IF;
    IF hold_status IN ('charged', 'released')
        AND (usage_count <> "line_count"
            OR usage_charged <> hold_charged::numeric
            OR usage_released <> hold_released::numeric
            OR usage_charged + usage_released <> hold_estimated::numeric)
    THEN
        RAISE EXCEPTION 'gateway settlement usage does not balance';
    END IF;
    SELECT COUNT(*)
    INTO invalid_usage_count
    FROM idc_gateway_settlement_usage_line AS usage
    JOIN idc_gateway_settlement_hold_line AS line
      ON line."hold_line_no" = usage."hold_line_no"
     AND line."hold_no" = usage."hold_no"
     AND line."price_version_no" = usage."price_version_no"
    WHERE usage."hold_no" = target_hold_no
      AND (usage."actual_units" > line."estimated_units"
        OR usage."charged_amount_minor"::numeric
            <> line."unit_price_minor"::numeric * usage."actual_units"::numeric
        OR usage."charged_amount_minor"::numeric
            + usage."released_amount_minor"::numeric <> line."held_amount_minor"::numeric);
    IF invalid_usage_count <> 0 THEN
        RAISE EXCEPTION 'gateway settlement usage line does not match frozen price';
    END IF;
    SELECT account."settlement_mode"
    INTO account_mode
    FROM idc_gateway_settlement_hold AS settlement_hold
    JOIN idc_gateway_settlement_account AS account
      ON account."account_no" = settlement_hold."account_no"
    WHERE settlement_hold."hold_no" = target_hold_no;
    SELECT COUNT(*), COUNT(*) FILTER (WHERE
               (account_mode = 'prepaid' AND "entry_kind" = 'prepaid_hold'
                    AND "delta_prepaid_available_minor" = -hold_estimated
                    AND "delta_held_minor" = hold_estimated)
            OR (account_mode = 'postpaid' AND "entry_kind" = 'postpaid_hold'
                    AND "delta_prepaid_available_minor" = 0
                    AND "delta_held_minor" = hold_estimated)),
           COUNT(*) FILTER (WHERE
               (hold_status = 'charged' AND account_mode = 'prepaid'
                    AND "entry_kind" = 'prepaid_charge'
                    AND "delta_prepaid_available_minor" = hold_released
                    AND "delta_held_minor" = -hold_estimated
                    AND "delta_postpaid_outstanding_minor" = 0)
            OR (hold_status = 'charged' AND account_mode = 'postpaid'
                    AND "entry_kind" = 'postpaid_charge'
                    AND "delta_prepaid_available_minor" = 0
                    AND "delta_held_minor" = -hold_estimated
                    AND "delta_postpaid_outstanding_minor" = hold_charged)
            OR (hold_status = 'released' AND account_mode = 'prepaid'
                    AND "entry_kind" = 'prepaid_release'
                    AND "delta_prepaid_available_minor" = hold_estimated
                    AND "delta_held_minor" = -hold_estimated)
            OR (hold_status = 'released' AND account_mode = 'postpaid'
                    AND "entry_kind" = 'postpaid_release'
                    AND "delta_prepaid_available_minor" = 0
                    AND "delta_held_minor" = -hold_estimated
                    AND "delta_postpaid_outstanding_minor" = 0))
    INTO total_ledger_count, hold_ledger_count, terminal_ledger_count
    FROM idc_gateway_settlement_ledger_entry
    WHERE "hold_no" = target_hold_no;
    IF hold_ledger_count <> 1
        OR (hold_status = 'held'
            AND (total_ledger_count <> 1 OR terminal_ledger_count <> 0))
        OR (hold_status IN ('charged', 'released')
            AND (total_ledger_count <> 2 OR terminal_ledger_count <> 1))
    THEN
        RAISE EXCEPTION 'gateway settlement hold has no exact ledger lifecycle';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_price_version_immutable ON idc_gateway_price_version;
CREATE TRIGGER trg_gateway_price_version_immutable
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_price_version
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_price_version_update();

DROP TRIGGER IF EXISTS trg_gateway_price_item_immutable ON idc_gateway_price_item;
CREATE TRIGGER trg_gateway_price_item_immutable
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_price_item
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_price_item_mutation();

DROP TRIGGER IF EXISTS trg_gateway_price_version_items ON idc_gateway_price_version;
CREATE CONSTRAINT TRIGGER trg_gateway_price_version_items
AFTER UPDATE ON idc_gateway_price_version
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_price_version_items();

DROP TRIGGER IF EXISTS trg_gateway_price_item_count ON idc_gateway_price_item;
CREATE CONSTRAINT TRIGGER trg_gateway_price_item_count
AFTER INSERT ON idc_gateway_price_item
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_price_version_items();

DROP TRIGGER IF EXISTS trg_gateway_settlement_account_guard
    ON idc_gateway_settlement_account;
CREATE TRIGGER trg_gateway_settlement_account_guard
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_settlement_account
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_settlement_account_update();

DROP TRIGGER IF EXISTS trg_gateway_settlement_account_ledger
    ON idc_gateway_settlement_account;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_account_ledger
AFTER UPDATE ON idc_gateway_settlement_account
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_account_ledger();

DROP TRIGGER IF EXISTS trg_gateway_settlement_hold_guard ON idc_gateway_settlement_hold;
CREATE TRIGGER trg_gateway_settlement_hold_guard
BEFORE INSERT OR UPDATE OR DELETE ON idc_gateway_settlement_hold
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_settlement_hold_update();

DROP TRIGGER IF EXISTS trg_gateway_settlement_line_immutable
    ON idc_gateway_settlement_hold_line;
CREATE TRIGGER trg_gateway_settlement_line_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_hold_line
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_history_mutation();

DROP TRIGGER IF EXISTS trg_gateway_usage_line_immutable
    ON idc_gateway_settlement_usage_line;
CREATE TRIGGER trg_gateway_usage_line_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_usage_line
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_history_mutation();

DROP TRIGGER IF EXISTS trg_gateway_settlement_ledger_immutable
    ON idc_gateway_settlement_ledger_entry;
CREATE TRIGGER trg_gateway_settlement_ledger_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_settlement_ledger_entry
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_settlement_history_mutation();

DROP TRIGGER IF EXISTS trg_gateway_settlement_ledger_account
    ON idc_gateway_settlement_ledger_entry;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_ledger_account
AFTER INSERT ON idc_gateway_settlement_ledger_entry
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_ledger_account();

DROP TRIGGER IF EXISTS trg_gateway_settlement_ledger_hold_balance
    ON idc_gateway_settlement_ledger_entry;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_ledger_hold_balance
AFTER INSERT ON idc_gateway_settlement_ledger_entry
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_hold_lines();

DROP TRIGGER IF EXISTS trg_gateway_settlement_hold_balance
    ON idc_gateway_settlement_hold;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_hold_balance
AFTER INSERT OR UPDATE ON idc_gateway_settlement_hold
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_hold_lines();

DROP TRIGGER IF EXISTS trg_gateway_settlement_line_balance
    ON idc_gateway_settlement_hold_line;
CREATE CONSTRAINT TRIGGER trg_gateway_settlement_line_balance
AFTER INSERT ON idc_gateway_settlement_hold_line
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_hold_lines();

DROP TRIGGER IF EXISTS trg_gateway_usage_line_balance
    ON idc_gateway_settlement_usage_line;
CREATE CONSTRAINT TRIGGER trg_gateway_usage_line_balance
AFTER INSERT ON idc_gateway_settlement_usage_line
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION idc_gateway_validate_settlement_hold_lines();
