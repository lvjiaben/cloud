-- 058: Published per-currency accessory pricing and immutable customer quotes.

-- Purpose: Current pointer for one merchant-published accessory price in one currency.
-- 用途：保存服务器附属资源价格策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_price_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "accessory_kind" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "current_version" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_accessory_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_accessory_policy_kind CHECK (
        "accessory_kind" IN ('snapshot', 'backup', 'traffic')
    ),
    CONSTRAINT ck_idc_server_accessory_policy_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_server_accessory_policy_version CHECK ("current_version" > 0),
    CONSTRAINT uk_idc_server_accessory_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_server_accessory_policy_kind_currency UNIQUE (
        "accessory_kind", "currency_code"
    ),
    CONSTRAINT uk_idc_server_accessory_policy_pointer UNIQUE (
        "policy_no", "current_version"
    )
);

-- Purpose: Append-only per-server free quota and unit-price policy versions.
-- 用途：保存服务器附属资源价格策略版本相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_price_policy_version (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "accessory_kind" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "free_count_per_server" INT NOT NULL,
    "unit_amount_minor" BIGINT NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "published_by" INT NOT NULL,
    "published_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_accessory_policy_version_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_accessory_policy_version_kind CHECK (
        "accessory_kind" IN ('snapshot', 'backup', 'traffic')
    ),
    CONSTRAINT ck_idc_server_accessory_policy_version_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_server_accessory_policy_version_values CHECK (
        "policy_version" > 0
        AND "free_count_per_server" BETWEEN 0 AND 10000
        AND "unit_amount_minor" BETWEEN 0 AND 1000000000
        AND "published_by" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (
            ("accessory_kind" IN ('snapshot', 'backup')
                AND "period_unit_code" IN ('day', 'month', 'year')
                AND "period_count" BETWEEN 1 AND 3650)
            OR ("accessory_kind" = 'traffic'
                AND "free_count_per_server" = 0
                AND "period_unit_code" = 'once'
                AND "period_count" = 1)
        )
    ),
    CONSTRAINT uk_idc_server_accessory_policy_version UNIQUE (
        "policy_no", "policy_version"
    ),
    CONSTRAINT uk_idc_server_accessory_policy_version_identity UNIQUE (
        "policy_no", "policy_version", "accessory_kind", "currency_code"
    ),
    CONSTRAINT fk_idc_server_accessory_policy_version_header FOREIGN KEY (
        "policy_no"
    ) REFERENCES plugin_idc_core_server_accessory_price_policy (
        "policy_no"
    ) ON DELETE RESTRICT
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_server_accessory_policy_current'
          AND conrelid = 'plugin_idc_core_server_accessory_price_policy'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_server_accessory_price_policy
            ADD CONSTRAINT fk_idc_server_accessory_policy_current FOREIGN KEY (
                "policy_no", "current_version"
            ) REFERENCES plugin_idc_core_server_accessory_price_policy_version (
                "policy_no", "policy_version"
            ) DEFERRABLE INITIALLY DEFERRED;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_idc_server_accessory_policy_version_history
    ON plugin_idc_core_server_accessory_price_policy_version (
        "tenant_id", "policy_no", "policy_version" DESC, "id" DESC
    );

-- Purpose: Immutable short-lived accessory quote based on fresh owner directory and current reservations.
-- 用途：保存服务器附属资源报价相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_quote (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "accessory_kind" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "requested_quantity" INT NOT NULL,
    "existing_object_count" INT NOT NULL,
    "reserved_create_count" INT NOT NULL,
    "free_count_per_server" INT NOT NULL,
    "charged_quantity" INT NOT NULL,
    "unit_amount_minor" BIGINT NOT NULL,
    "total_amount_minor" BIGINT NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_idc_server_accessory_quote_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_accessory_quote_action CHECK (
        "action_code" IN ('snapshot_create', 'backup_create', 'traffic_add')
    ),
    CONSTRAINT ck_idc_server_accessory_quote_kind CHECK (
        "accessory_kind" IN ('snapshot', 'backup', 'traffic')
    ),
    CONSTRAINT ck_idc_server_accessory_quote_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_server_accessory_quote_counts CHECK (
        "requested_quantity" BETWEEN 1 AND 1000000
        AND "existing_object_count" BETWEEN 0 AND 10000
        AND "reserved_create_count" BETWEEN 0 AND 10000
        AND "free_count_per_server" BETWEEN 0 AND 10000
        AND "charged_quantity" BETWEEN 0 AND 1000000
        AND "charged_quantity" <= "requested_quantity"
    ),
    CONSTRAINT ck_idc_server_accessory_quote_amount CHECK (
        "unit_amount_minor" BETWEEN 0 AND 1000000000
        AND "total_amount_minor" = "unit_amount_minor" * "charged_quantity"
    ),
    CONSTRAINT ck_idc_server_accessory_quote_period CHECK (
        ("accessory_kind" IN ('snapshot', 'backup')
            AND "period_unit_code" IN ('day', 'month', 'year')
            AND "period_count" BETWEEN 1 AND 3650)
        OR ("accessory_kind" = 'traffic'
            AND "period_unit_code" = 'once' AND "period_count" = 1
            AND "free_count_per_server" = 0
            AND "charged_quantity" = "requested_quantity")
    ),
    CONSTRAINT ck_idc_server_accessory_quote_digests CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_server_accessory_quote_expiry CHECK ("expires_at" > "issued_at"),
    CONSTRAINT uk_idc_server_accessory_quote_no UNIQUE ("quote_no"),
    CONSTRAINT uk_idc_server_accessory_quote_request UNIQUE (
        "customer_no", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_server_accessory_quote_customer FOREIGN KEY (
        "customer_no"
    ) REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_accessory_quote_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_accessory_quote_policy FOREIGN KEY (
        "policy_no", "policy_version", "accessory_kind", "currency_code"
    ) REFERENCES plugin_idc_core_server_accessory_price_policy_version (
        "policy_no", "policy_version", "accessory_kind", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_accessory_quote_service
    ON plugin_idc_core_server_accessory_quote (
        "tenant_id", "service_no", "issued_at" DESC, "id" DESC
    );

DO $accessory_history_function$
DECLARE
    legacy_function REGPROCEDURE := to_regprocedure(
        'plugin_idc_reject_accessory_pricing_history_mutation()'
    );
    current_function REGPROCEDURE := to_regprocedure(
        'plugin_idc_core_reject_accessory_pricing_history_mutation()'
    );
    unexpected_dependencies BIGINT;
BEGIN
    IF legacy_function IS NOT NULL AND current_function IS NOT NULL THEN
        RAISE EXCEPTION 'accessory history guard has both legacy and current functions';
    END IF;
    IF legacy_function IS NOT NULL THEN
        SELECT count(*)
          INTO unexpected_dependencies
          FROM pg_trigger AS trigger_row
         WHERE trigger_row.tgfoid = legacy_function
           AND NOT trigger_row.tgisinternal
           AND (trigger_row.tgrelid, trigger_row.tgname) NOT IN (
                (to_regclass('plugin_idc_core_server_accessory_price_policy_version'),
                    'trg_idc_server_accessory_policy_version_immutable'),
                (to_regclass('plugin_idc_core_server_accessory_quote'),
                    'trg_idc_server_accessory_quote_immutable')
           );
        IF unexpected_dependencies <> 0 THEN
            RAISE EXCEPTION 'legacy accessory history guard has unexpected trigger dependencies';
        END IF;
        ALTER FUNCTION plugin_idc_reject_accessory_pricing_history_mutation()
            RENAME TO plugin_idc_core_reject_accessory_pricing_history_mutation;
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
                    (to_regclass('plugin_idc_core_server_accessory_price_policy_version'),
                        'trg_idc_server_accessory_policy_version_immutable'),
                    (to_regclass('plugin_idc_core_server_accessory_quote'),
                        'trg_idc_server_accessory_quote_immutable')
               )
        )
    ) THEN
        RAISE EXCEPTION 'current accessory history guard catalog contract is invalid';
    END IF;
END;
$accessory_history_function$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_accessory_pricing_history_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'server accessory pricing history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_server_accessory_policy_version_immutable
    ON plugin_idc_core_server_accessory_price_policy_version;
CREATE TRIGGER trg_idc_server_accessory_policy_version_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_accessory_price_policy_version
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_accessory_pricing_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_server_accessory_quote_immutable
    ON plugin_idc_core_server_accessory_quote;
CREATE TRIGGER trg_idc_server_accessory_quote_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_accessory_quote
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_accessory_pricing_history_mutation();

COMMENT ON TABLE plugin_idc_core_server_accessory_price_policy
    IS 'Current pointer for one merchant-published accessory price in one currency';
COMMENT ON TABLE plugin_idc_core_server_accessory_price_policy_version
    IS 'Append-only per-server free quota and unit-price policy versions';
COMMENT ON TABLE plugin_idc_core_server_accessory_quote
    IS 'Immutable short-lived accessory quote based on fresh owner directory and current reservations';
