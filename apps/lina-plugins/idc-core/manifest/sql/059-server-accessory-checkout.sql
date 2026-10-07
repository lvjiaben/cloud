-- 059: Accessory quotes participate in the ordinary order and financial
-- settlement pipeline without creating server inventory reservations.

-- Purpose: Stores Server Accessory Kind records.
-- 用途：保存服务器附属资源种类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_kind (
    "accessory_kind" VARCHAR(32) PRIMARY KEY,
    "name_zh_cn" VARCHAR(64) NOT NULL,
    "name_en_us" VARCHAR(64) NOT NULL,
    CONSTRAINT ck_idc_server_accessory_kind_values CHECK (
        "accessory_kind" IN ('snapshot', 'backup', 'traffic')
        AND length(btrim("name_zh_cn")) BETWEEN 1 AND 64
        AND length(btrim("name_en_us")) BETWEEN 1 AND 64
    )
);

INSERT INTO plugin_idc_core_server_accessory_kind
    ("accessory_kind", "name_zh_cn", "name_en_us")
VALUES
    ('snapshot', '服务器快照', 'Server snapshot'),
    ('backup', '服务器备份', 'Server backup'),
    ('traffic', '服务器流量包', 'Server traffic package')
ON CONFLICT ("accessory_kind") DO UPDATE SET
    "name_zh_cn" = EXCLUDED."name_zh_cn",
    "name_en_us" = EXCLUDED."name_en_us";

ALTER TABLE plugin_idc_core_server_accessory_price_policy_version
    DROP CONSTRAINT IF EXISTS ck_idc_server_accessory_policy_version_values;
ALTER TABLE plugin_idc_core_server_accessory_price_policy_version
    ADD CONSTRAINT ck_idc_server_accessory_policy_version_values CHECK (
        "policy_version" > 0
        AND "free_count_per_server" BETWEEN 0 AND 10000
        AND "unit_amount_minor" BETWEEN 0 AND 1000000000
        AND "published_by" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND (
            ("accessory_kind" IN ('snapshot', 'backup') AND (
                ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
                OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
                OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10)
            ))
            OR ("accessory_kind" = 'traffic'
                AND "free_count_per_server" = 0
                AND "period_unit_code" = 'once'
                AND "period_count" = 1)
        )
    );

ALTER TABLE plugin_idc_core_server_accessory_quote
    ADD COLUMN IF NOT EXISTS "parameters" JSONB NOT NULL DEFAULT '{}'::JSONB,
    ADD COLUMN IF NOT EXISTS "customer_subject_version" BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "expected_service_version" BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "net_amount_minor" BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "tax_policy_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "tax_treatment_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "tax_rate_basis_points" INT,
    ADD COLUMN IF NOT EXISTS "tax_category_code" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "rounding_scope_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "rounding_mode_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "rounding_quantum_minor" BIGINT;

ALTER TABLE plugin_idc_core_server_accessory_quote
    DROP CONSTRAINT IF EXISTS ck_idc_server_accessory_quote_checkout;
ALTER TABLE plugin_idc_core_server_accessory_quote
    ADD CONSTRAINT ck_idc_server_accessory_quote_checkout CHECK (
        jsonb_typeof("parameters") = 'object'
        AND octet_length("parameters"::TEXT) BETWEEN 2 AND 16384
        AND "customer_subject_version" > 0
        AND "expected_service_version" > 0
        AND "subtotal_minor" >= 0
        AND "net_amount_minor" = "subtotal_minor"
        AND "tax_amount_minor" >= 0
        AND "total_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        AND (
            ("total_amount_minor" = 0
                AND "tax_policy_no" IS NULL
                AND "tax_treatment_code" IS NULL
                AND "tax_rate_basis_points" IS NULL
                AND "tax_category_code" IS NULL
                AND "rounding_scope_code" IS NULL
                AND "rounding_mode_code" IS NULL
                AND "rounding_quantum_minor" IS NULL)
            OR ("total_amount_minor" > 0
                AND length(btrim("tax_policy_no")) BETWEEN 1 AND 40
                AND "tax_treatment_code" IN ('taxable', 'zero_rated', 'exempt')
                AND (("tax_treatment_code" = 'taxable'
                        AND "tax_rate_basis_points" BETWEEN 0 AND 10000)
                    OR ("tax_treatment_code" = 'zero_rated'
                        AND "tax_rate_basis_points" = 0)
                    OR ("tax_treatment_code" = 'exempt'
                        AND "tax_rate_basis_points" IS NULL))
                AND "tax_category_code" ~ '^[a-z0-9][a-z0-9._-]{0,63}$'
                AND "rounding_scope_code" IN ('line', 'total')
                AND "rounding_mode_code" IN ('half_up', 'down', 'up')
                AND "rounding_quantum_minor" BETWEEN 1 AND 1000000)
        )
    );

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_service_price_phase;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT ck_idc_service_price_phase CHECK (
        "phase_code" IN ('new_purchase', 'renewal', 'addon')
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
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NOT NULL
                AND "projected_expires_at" > "original_expires_at")
            OR ("phase_code" = 'addon' AND "renewal_mode_code" = 'provider_confirmed'
                AND "capability_evidence_no" IS NOT NULL
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
                AND "capability_evidence_observed_at" IS NOT NULL
                AND "capability_evidence_expires_at" > "capability_evidence_observed_at"
                AND "original_expires_at" IS NULL
                AND "projected_expires_at" IS NULL)
        )
    );

ALTER TABLE plugin_idc_core_financial_settlement_delivery
    DROP CONSTRAINT IF EXISTS ck_idc_fin_delivery_counts;
ALTER TABLE plugin_idc_core_financial_settlement_delivery
    ADD CONSTRAINT ck_idc_fin_delivery_counts CHECK (
        "fulfillment_count" BETWEEN 0 AND 1000
        AND "ready_count" BETWEEN 0 AND "fulfillment_count"
        AND "waiting_gate_count" BETWEEN 0 AND "fulfillment_count"
        AND "manual_review_count" BETWEEN 0 AND "fulfillment_count"
        AND "ready_count" + "waiting_gate_count" + "manual_review_count"
            = "fulfillment_count"
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_addon_quote
    ON plugin_idc_core_business_operation ("tenant_id", "quote_no")
    WHERE "operation_type_code" = 'addon' AND "quote_no" IS NOT NULL;

-- Intent fields stay immutable. The settlement number is the sole one-time
-- attachment permitted when an awaiting paid operation becomes executable.
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
        OLD."pool_no",
        OLD."capability_evidence_no", OLD."capability_evidence_digest",
        OLD."idempotency_digest", OLD."request_digest", OLD."content_digest",
        OLD."expected_service_version", OLD."source_fulfillment_no",
        OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."operation_no", NEW."operation_type_code",
        NEW."customer_no", NEW."service_no", NEW."currency_code",
        NEW."price_snapshot_no", NEW."quote_no",
        NEW."order_no", NEW."invoice_no", NEW."financial_settlement_required",
        NEW."execution_mode_code", NEW."resource_owner_code", NEW."channel_no",
        NEW."pool_no",
        NEW."capability_evidence_no", NEW."capability_evidence_digest",
        NEW."idempotency_digest", NEW."request_digest", NEW."content_digest",
        NEW."expected_service_version", NEW."source_fulfillment_no",
        NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    IF OLD."settlement_no" IS DISTINCT FROM NEW."settlement_no" AND NOT (
        OLD."settlement_no" IS NULL
        AND NEW."settlement_no" IS NOT NULL
        AND OLD."operation_type_code" = 'addon'
        AND OLD."status_code" = 'awaiting_financial_settlement'
        AND NEW."status_code" IN ('financially_settled', 'ready')
    ) THEN
        RAISE EXCEPTION 'business operation settlement link is immutable';
    END IF;
    RETURN NEW;
END;
$$;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at")
SELECT 0, 'idc_service_price_phase', '附属能力', 'addon', 3, 'primary',
       1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (
    SELECT 1 FROM sys_dict_data
    WHERE "tenant_id" = 0 AND "dict_type" = 'idc_service_price_phase' AND "value" = 'addon'
);

COMMENT ON COLUMN plugin_idc_core_server_accessory_quote."parameters"
    IS 'Canonical provider parameters frozen by the customer-visible quote';
