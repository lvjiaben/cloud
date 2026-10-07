-- 064: Versioned customer IP-change policy, immutable quote, and reservation binding.

-- Purpose: Stores Server Ip Change Policy records.
-- 用途：保存服务器IP变更策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_ip_change_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "current_version" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_ip_change_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_ip_change_policy_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_server_ip_change_policy_version CHECK ("current_version" > 0),
    CONSTRAINT uk_idc_server_ip_change_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_server_ip_change_policy_currency UNIQUE ("currency_code"),
    CONSTRAINT uk_idc_server_ip_change_policy_pointer UNIQUE ("policy_no", "current_version")
);

-- Purpose: Append-only per-currency customer IP-change qualification and final-price rules.
-- 用途：保存服务器IP变更策略版本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_ip_change_policy_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "enabled" BOOLEAN NOT NULL,
    "free_changes_per_window" INT NOT NULL,
    "window_days" INT NOT NULL,
    "max_changes_per_day" INT NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "maximum_snapshot_age_seconds" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "published_by" INT NOT NULL,
    "published_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_ip_change_policy_version_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_ip_change_policy_version_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_server_ip_change_policy_version_values CHECK (
        "policy_version" > 0
        AND "free_changes_per_window" BETWEEN 0 AND 100
        AND "window_days" BETWEEN 1 AND 365
        AND "max_changes_per_day" BETWEEN 1 AND 20
        AND "amount_minor" BETWEEN 0 AND 1000000000
        AND "maximum_snapshot_age_seconds" BETWEEN 60 AND 86400
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "published_by" > 0
    ),
    CONSTRAINT uk_idc_server_ip_change_policy_version UNIQUE ("policy_no", "policy_version"),
    CONSTRAINT uk_idc_server_ip_change_policy_version_identity UNIQUE (
        "policy_no", "policy_version", "currency_code"
    ),
    CONSTRAINT fk_idc_server_ip_change_policy_version_header FOREIGN KEY ("policy_no")
        REFERENCES plugin_idc_core_server_ip_change_policy ("policy_no") ON DELETE RESTRICT
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_server_ip_change_policy_current'
          AND conrelid = 'plugin_idc_core_server_ip_change_policy'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_server_ip_change_policy
            ADD CONSTRAINT fk_idc_server_ip_change_policy_current FOREIGN KEY (
                "policy_no", "current_version"
            ) REFERENCES plugin_idc_core_server_ip_change_policy_version (
                "policy_no", "policy_version"
            ) DEFERRABLE INITIALLY DEFERRED;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_idc_server_ip_change_policy_version_history
    ON plugin_idc_core_server_ip_change_policy_version (
        "tenant_id", "policy_no", "policy_version" DESC, "id" DESC
    );

-- Purpose: Immutable short-lived IP-change quote bound to a resource-owner stock reservation.
-- 用途：保存服务器IP变更报价相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_ip_change_quote (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "ip_quantity" INT NOT NULL,
    "free_change" BOOLEAN NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "maximum_snapshot_age_seconds" INT NOT NULL,
    "resource_reservation_no" VARCHAR(64) NOT NULL,
    "resource_snapshot_no" VARCHAR(64) NOT NULL,
    "customer_subject_version" BIGINT NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "old_ip_set_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_idc_server_ip_change_quote_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_ip_change_quote_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_server_ip_change_quote_values CHECK (
        "ip_quantity" BETWEEN 1 AND 64
        AND "amount_minor" BETWEEN 0 AND 1000000000
        AND "free_change" = ("amount_minor" = 0)
        AND "maximum_snapshot_age_seconds" BETWEEN 60 AND 86400
        AND "customer_subject_version" > 0
        AND "expected_service_version" > 0
        AND "old_ip_set_digest" ~ '^[0-9a-f]{64}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "expires_at" > "issued_at"
    ),
    CONSTRAINT uk_idc_server_ip_change_quote_no UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_server_ip_change_quote_request UNIQUE ("customer_no", "idempotency_digest"),
    CONSTRAINT uk_idc_server_ip_change_quote_reservation UNIQUE ("resource_reservation_no"),
    CONSTRAINT fk_idc_server_ip_change_quote_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_ip_change_quote_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_ip_change_quote_policy FOREIGN KEY (
        "policy_no", "policy_version", "currency_code"
    ) REFERENCES plugin_idc_core_server_ip_change_policy_version (
        "policy_no", "policy_version", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_ip_change_quote_service
    ON plugin_idc_core_server_ip_change_quote (
        "tenant_id", "service_no", "issued_at" DESC, "id" DESC
    );

-- Purpose: Core-owned link between one IP-change business operation and resource reservation.
-- 用途：保存服务器IP变更意图相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_ip_change_intent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_no" VARCHAR(40) NOT NULL,
    "quote_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "resource_reservation_no" VARCHAR(64) NOT NULL,
    "ip_quantity" INT NOT NULL,
    "old_ip_set_digest" CHAR(64) NOT NULL,
    "new_ip_set_digest" CHAR(64),
    "reservation_status_code" VARCHAR(16) NOT NULL DEFAULT 'reserved',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_ip_change_intent_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_ip_change_intent_values CHECK (
        "ip_quantity" BETWEEN 1 AND 64
        AND "old_ip_set_digest" ~ '^[0-9a-f]{64}$'
        AND ("new_ip_set_digest" IS NULL OR "new_ip_set_digest" ~ '^[0-9a-f]{64}$')
        AND "reservation_status_code" IN ('reserved', 'consumed', 'released', 'unknown')
        AND (("reservation_status_code" = 'consumed' AND "new_ip_set_digest" IS NOT NULL)
             OR ("reservation_status_code" <> 'consumed' AND "new_ip_set_digest" IS NULL))
    ),
    CONSTRAINT uk_idc_server_ip_change_intent_operation UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_server_ip_change_intent_quote UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_server_ip_change_intent_reservation UNIQUE ("resource_reservation_no"),
    CONSTRAINT fk_idc_server_ip_change_intent_operation FOREIGN KEY ("operation_no")
        REFERENCES plugin_idc_core_business_operation ("operation_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_ip_change_intent_quote FOREIGN KEY ("quote_no")
        REFERENCES plugin_idc_core_server_ip_change_quote ("quote_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_ip_change_intent_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_ip_change_intent_service
    ON plugin_idc_core_server_ip_change_intent (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only terminal reservation evidence with domain-separated old and new address-set digests.
-- 用途：保存服务器IP变更事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_ip_change_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "reservation_no" VARCHAR(64) NOT NULL,
    "event_type_code" VARCHAR(16) NOT NULL,
    "old_ip_set_digest" CHAR(64) NOT NULL,
    "new_ip_set_digest" CHAR(64),
    "evidence_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_ip_change_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_ip_change_event_values CHECK (
        "event_type_code" IN ('consumed', 'released')
        AND "old_ip_set_digest" ~ '^[0-9a-f]{64}$'
        AND ("new_ip_set_digest" IS NULL OR "new_ip_set_digest" ~ '^[0-9a-f]{64}$')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND (("event_type_code" = 'consumed' AND "new_ip_set_digest" IS NOT NULL)
             OR ("event_type_code" = 'released' AND "new_ip_set_digest" IS NULL))
    ),
    CONSTRAINT uk_idc_server_ip_change_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_idc_server_ip_change_event_terminal UNIQUE ("operation_no", "event_type_code"),
    CONSTRAINT fk_idc_server_ip_change_event_intent FOREIGN KEY ("operation_no")
        REFERENCES plugin_idc_core_server_ip_change_intent ("operation_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_ip_change_event_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_ip_change_event_service
    ON plugin_idc_core_server_ip_change_event (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_ip_change_quote
    ON plugin_idc_core_business_operation ("tenant_id", "quote_no")
    WHERE "operation_type_code" = 'ip_change' AND "quote_no" IS NOT NULL;

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_phase;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT ck_idc_service_price_phase CHECK (
        "phase_code" IN ('new_purchase', 'renewal', 'addon', 'ip_change')
        AND (("source_quote_no" IS NULL AND "source_quote_item_no" IS NULL)
            OR ("source_quote_no" IS NOT NULL AND "source_quote_item_no" IS NOT NULL))
    );

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_renewal_mode;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT ck_idc_service_price_renewal_mode CHECK (
        "renewal_mode_code" IN ('unresolved', 'provider_confirmed', 'local_entitlement')
        AND (
            ("phase_code" = 'new_purchase' AND "renewal_mode_code" = 'unresolved'
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL
                AND "capability_evidence_observed_at" IS NULL
                AND "capability_evidence_expires_at" IS NULL
                AND "original_expires_at" IS NULL)
            OR ("phase_code" = 'renewal' AND "renewal_mode_code" <> 'unresolved'
                AND "capability_evidence_no" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NOT NULL
                AND "projected_expires_at" > "original_expires_at")
            OR ("phase_code" IN ('addon', 'ip_change') AND "renewal_mode_code" = 'provider_confirmed'
                AND "capability_evidence_no" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NULL
                AND "projected_expires_at" IS NULL)
        )
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."operation_no", OLD."operation_type_code",
        OLD."customer_no", OLD."service_no", OLD."currency_code",
        OLD."price_snapshot_no", OLD."quote_no",
        OLD."order_no", OLD."invoice_no", OLD."financial_settlement_required",
        OLD."execution_mode_code", OLD."resource_owner_code", OLD."channel_no",
        OLD."pool_no", OLD."capability_evidence_no", OLD."capability_evidence_digest",
        OLD."idempotency_digest", OLD."request_digest", OLD."content_digest",
        OLD."expected_service_version", OLD."source_fulfillment_no", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."operation_no", NEW."operation_type_code",
        NEW."customer_no", NEW."service_no", NEW."currency_code",
        NEW."price_snapshot_no", NEW."quote_no",
        NEW."order_no", NEW."invoice_no", NEW."financial_settlement_required",
        NEW."execution_mode_code", NEW."resource_owner_code", NEW."channel_no",
        NEW."pool_no", NEW."capability_evidence_no", NEW."capability_evidence_digest",
        NEW."idempotency_digest", NEW."request_digest", NEW."content_digest",
        NEW."expected_service_version", NEW."source_fulfillment_no", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    IF OLD."settlement_no" IS DISTINCT FROM NEW."settlement_no" AND NOT (
        OLD."settlement_no" IS NULL AND NEW."settlement_no" IS NOT NULL
        AND OLD."operation_type_code" IN ('addon', 'ip_change')
        AND OLD."status_code" = 'awaiting_financial_settlement'
        AND NEW."status_code" IN ('financially_settled', 'ready')
    ) THEN
        RAISE EXCEPTION 'business operation settlement link is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DO $ip_change_history_function$
DECLARE
    legacy_function REGPROCEDURE := to_regprocedure(
        'plugin_idc_reject_server_ip_change_history_mutation()'
    );
    current_function REGPROCEDURE := to_regprocedure(
        'plugin_idc_core_reject_server_ip_change_history_mutation()'
    );
    unexpected_dependencies BIGINT;
BEGIN
    IF legacy_function IS NOT NULL AND current_function IS NOT NULL THEN
        RAISE EXCEPTION 'IP-change history guard has both legacy and current functions';
    END IF;
    IF legacy_function IS NOT NULL THEN
        SELECT count(*)
          INTO unexpected_dependencies
          FROM pg_trigger AS trigger_row
         WHERE trigger_row.tgfoid = legacy_function
           AND NOT trigger_row.tgisinternal
           AND (trigger_row.tgrelid, trigger_row.tgname) NOT IN (
                (to_regclass('plugin_idc_core_server_ip_change_policy_version'),
                    'trg_idc_server_ip_change_policy_version_immutable'),
                (to_regclass('plugin_idc_core_server_ip_change_quote'),
                    'trg_idc_server_ip_change_quote_immutable'),
                (to_regclass('plugin_idc_core_server_ip_change_event'),
                    'trg_idc_server_ip_change_event_immutable')
           );
        IF unexpected_dependencies <> 0 THEN
            RAISE EXCEPTION 'legacy IP-change history guard has unexpected trigger dependencies';
        END IF;
        ALTER FUNCTION plugin_idc_reject_server_ip_change_history_mutation()
            RENAME TO plugin_idc_core_reject_server_ip_change_history_mutation;
        current_function := legacy_function;
    END IF;
    IF current_function IS NOT NULL AND (
        NOT EXISTS (
            SELECT 1
              FROM pg_proc AS function_row
             WHERE function_row.oid = current_function
               AND function_row.prokind = 'f'
               AND function_row.prorettype = 'trigger'::regtype
        ) OR EXISTS (
            SELECT 1
              FROM pg_trigger AS trigger_row
             WHERE trigger_row.tgfoid = current_function
               AND NOT trigger_row.tgisinternal
               AND (trigger_row.tgrelid, trigger_row.tgname) NOT IN (
                    (to_regclass('plugin_idc_core_server_ip_change_policy_version'),
                        'trg_idc_server_ip_change_policy_version_immutable'),
                    (to_regclass('plugin_idc_core_server_ip_change_quote'),
                        'trg_idc_server_ip_change_quote_immutable'),
                    (to_regclass('plugin_idc_core_server_ip_change_event'),
                        'trg_idc_server_ip_change_event_immutable')
               )
        )
    ) THEN
        RAISE EXCEPTION 'current IP-change history guard catalog contract is invalid';
    END IF;
END;
$ip_change_history_function$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_server_ip_change_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'server IP-change policy versions and quotes are append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_server_ip_change_policy_version_immutable
    ON plugin_idc_core_server_ip_change_policy_version;
CREATE TRIGGER trg_idc_server_ip_change_policy_version_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_ip_change_policy_version
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_server_ip_change_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_server_ip_change_quote_immutable
    ON plugin_idc_core_server_ip_change_quote;
CREATE TRIGGER trg_idc_server_ip_change_quote_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_ip_change_quote
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_server_ip_change_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_server_ip_change_event_immutable
    ON plugin_idc_core_server_ip_change_event;
CREATE TRIGGER trg_idc_server_ip_change_event_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_ip_change_event
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_server_ip_change_history_mutation();

COMMENT ON TABLE plugin_idc_core_server_ip_change_policy_version
    IS 'Append-only per-currency customer IP-change qualification and final-price rules';
COMMENT ON TABLE plugin_idc_core_server_ip_change_quote
    IS 'Immutable short-lived IP-change quote bound to a resource-owner stock reservation';
COMMENT ON TABLE plugin_idc_core_server_ip_change_intent
    IS 'Core-owned link between one IP-change business operation and resource reservation';
COMMENT ON TABLE plugin_idc_core_server_ip_change_event
    IS 'Append-only terminal reservation evidence with domain-separated old and new address-set digests';
