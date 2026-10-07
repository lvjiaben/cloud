-- 105: Authoritative two-phase channel quotes for resize and IP change.
-- The channel may preview a frozen quote without creating a financial fact.
-- Confirmation reuses this immutable row and only then creates the core order,
-- payer invoice, business operation, and Provider task.

-- Purpose: Stores immutable authoritative quotes for priced channel service operations.
-- 用途：保存渠道服务收费操作的不可变权威报价。
CREATE TABLE IF NOT EXISTS plugin_idc_core_channel_service_quote (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "quote_no" VARCHAR(40) NOT NULL UNIQUE,
    "channel_id" VARCHAR(80) NOT NULL,
    "payer_customer_no" VARCHAR(40) NOT NULL,
    "service_owner_customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "direction_code" VARCHAR(16) NOT NULL DEFAULT '',
    "current_configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "target_configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "target_version_no" VARCHAR(40) NOT NULL,
    "target_billing_rule_no" VARCHAR(40) NOT NULL,
    "current_recurring_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "target_recurring_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "target_subtotal_minor" BIGINT NOT NULL DEFAULT 0,
    "target_net_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "target_tax_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "subtotal_minor" BIGINT NOT NULL,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "provider_parameters" JSONB NOT NULL,
    "resource_channel_no" VARCHAR(64) NOT NULL,
    "resource_pool_no" VARCHAR(64) NOT NULL,
    "resource_reservation_no" VARCHAR(64),
    "resource_snapshot_no" VARCHAR(64) NOT NULL,
    "ip_quantity" INT NOT NULL DEFAULT 0,
    "maximum_snapshot_age_seconds" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40),
    "policy_version" BIGINT,
    "old_ip_set_digest" CHAR(64),
    "expected_service_version" BIGINT NOT NULL,
    "customer_subject_version" BIGINT NOT NULL,
    "payer_customer_subject_version" BIGINT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "pricing_rule_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "effective_at" TIMESTAMPTZ NOT NULL,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_channel_service_quote_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_channel_service_quote_action CHECK (
        ("action_code"='resize' AND "direction_code" IN ('upgrade','downgrade')
         AND "resource_reservation_no" IS NULL AND "policy_no" IS NULL
         AND "policy_version" IS NULL AND "old_ip_set_digest" IS NULL
         AND "ip_quantity"=0 AND "maximum_snapshot_age_seconds"=0)
        OR
        ("action_code"='ip_change' AND "direction_code"=''
         AND "resource_reservation_no" IS NOT NULL AND "policy_no" IS NOT NULL
         AND "policy_version">0 AND "old_ip_set_digest" ~ '^[0-9a-f]{64}$'
         AND "ip_quantity" BETWEEN 1 AND 64
         AND "maximum_snapshot_age_seconds" BETWEEN 60 AND 86400)
    ),
    CONSTRAINT ck_idc_channel_service_quote_amount CHECK (
        "current_recurring_amount_minor">=0 AND "target_recurring_amount_minor">=0
        AND "target_subtotal_minor">=0 AND "target_net_amount_minor"="target_subtotal_minor"
        AND "target_tax_amount_minor">=0
        AND "target_recurring_amount_minor"="target_net_amount_minor"+"target_tax_amount_minor"
        AND "subtotal_minor">=0 AND "net_amount_minor"="subtotal_minor"
        AND "tax_amount_minor">=0 AND "amount_minor"="net_amount_minor"+"tax_amount_minor"
    ),
    CONSTRAINT ck_idc_channel_service_quote_payload CHECK (
        jsonb_typeof("provider_parameters")='object'
        AND octet_length("provider_parameters"::TEXT)<=16384
    ),
    CONSTRAINT ck_idc_channel_service_quote_evidence CHECK (
        "expected_service_version">0 AND "customer_subject_version">0
        AND "payer_customer_subject_version">0
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "pricing_rule_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "effective_at">="issued_at" AND "expires_at">"issued_at"
    ),
    CONSTRAINT uk_idc_channel_service_quote_request UNIQUE (
        "channel_id","idempotency_digest"
    ),
    CONSTRAINT uk_idc_channel_service_quote_reservation UNIQUE (
        "resource_reservation_no"
    ),
    CONSTRAINT fk_idc_channel_service_quote_commerce FOREIGN KEY ("quote_no")
        REFERENCES plugin_idc_core_commerce_quote_snapshot("quote_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_service_quote_payer FOREIGN KEY ("payer_customer_no")
        REFERENCES plugin_idc_core_customer_subject("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_service_quote_owner FOREIGN KEY (
        "tenant_id","service_no","service_owner_customer_no"
    ) REFERENCES plugin_idc_core_service_entitlement(
        "tenant_id","entitlement_no","customer_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_channel_service_quote_service
    ON plugin_idc_core_channel_service_quote(
        "tenant_id","service_no","issued_at" DESC,"id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_channel_service_quote_expiry
    ON plugin_idc_core_channel_service_quote("tenant_id","expires_at","id");

ALTER TABLE plugin_idc_core_channel_service_quote
    ADD COLUMN IF NOT EXISTS "payer_customer_subject_version" BIGINT;
DROP TRIGGER IF EXISTS trg_idc_channel_service_quote_immutable
    ON plugin_idc_core_channel_service_quote;
UPDATE plugin_idc_core_channel_service_quote quote
SET "payer_customer_subject_version"=customer."version"
FROM plugin_idc_core_customer_subject customer
WHERE quote."tenant_id"=customer."tenant_id"
  AND quote."payer_customer_no"=customer."customer_no"
  AND quote."payer_customer_subject_version" IS NULL;
ALTER TABLE plugin_idc_core_channel_service_quote
    ALTER COLUMN "payer_customer_subject_version" SET NOT NULL,
    DROP CONSTRAINT IF EXISTS ck_idc_channel_service_quote_evidence;
ALTER TABLE plugin_idc_core_channel_service_quote
    ADD CONSTRAINT ck_idc_channel_service_quote_evidence CHECK (
        "expected_service_version">0 AND "customer_subject_version">0
        AND "payer_customer_subject_version">0
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "pricing_rule_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "effective_at">="issued_at" AND "expires_at">"issued_at"
    );

-- Priced operations must exist durably before the Provider call. A null task
-- number therefore means either ready to dispatch or a scheduled downgrade;
-- it is filled exactly once after the resource owner accepts the task.
ALTER TABLE plugin_idc_core_channel_service_operation
    ALTER COLUMN "provider_task_no" DROP NOT NULL,
    DROP CONSTRAINT IF EXISTS ck_idc_channel_service_operation_status,
    DROP CONSTRAINT IF EXISTS ck_idc_channel_service_operation_evidence;

ALTER TABLE plugin_idc_core_channel_service_operation
    ADD CONSTRAINT ck_idc_channel_service_operation_status CHECK (
        "status_code" IN (
            'scheduled','ready','pending','running','succeeded','failed',
            'result_unknown','manual_review'
        )
    ),
    ADD CONSTRAINT ck_idc_channel_service_operation_evidence CHECK (
        "expected_service_version" > 0
        AND (("action_code" = 'terminate' AND "expected_expires_at" IS NOT NULL)
          OR ("action_code" <> 'terminate' AND "expected_expires_at" IS NULL))
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "task_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND ("failure_code" = '' OR "failure_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
        AND (("status_code" IN ('ready','scheduled','manual_review') AND "provider_task_no" IS NULL)
          OR ("status_code" NOT IN ('ready','scheduled') AND "provider_task_no" IS NOT NULL))
        AND (("status_code" IN ('succeeded','failed') AND "completed_at" IS NOT NULL)
          OR ("status_code" NOT IN ('succeeded','failed') AND "completed_at" IS NULL))
        AND "version" > 0
    );

DROP INDEX IF EXISTS uk_idc_channel_service_operation_active;
CREATE UNIQUE INDEX uk_idc_channel_service_operation_active
    ON plugin_idc_core_channel_service_operation("tenant_id", "service_no")
    WHERE "status_code" IN ('scheduled','ready','pending','running','result_unknown');

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_channel_service_quote_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    RAISE EXCEPTION 'channel service quotes are immutable';
END; $$;

DROP TRIGGER IF EXISTS trg_idc_channel_service_quote_immutable
    ON plugin_idc_core_channel_service_quote;
CREATE TRIGGER trg_idc_channel_service_quote_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_channel_service_quote
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_channel_service_quote_mutation();

COMMENT ON TABLE plugin_idc_core_channel_service_quote
    IS 'Immutable authoritative channel quote for generic-entitlement resize or IP change';

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_channel_service_operation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'channel service operations cannot be deleted';
    END IF;
    IF ROW(OLD."tenant_id",OLD."operation_no",OLD."channel_id",OLD."payer_customer_no",
           OLD."service_owner_customer_no",OLD."service_no",OLD."action_code",OLD."parameters",
           OLD."resource_channel_no",OLD."resource_pool_no",OLD."expected_service_version",
           OLD."expected_expires_at",OLD."request_digest",OLD."created_at")
       IS DISTINCT FROM
       ROW(NEW."tenant_id",NEW."operation_no",NEW."channel_id",NEW."payer_customer_no",
           NEW."service_owner_customer_no",NEW."service_no",NEW."action_code",NEW."parameters",
           NEW."resource_channel_no",NEW."resource_pool_no",NEW."expected_service_version",
           NEW."expected_expires_at",NEW."request_digest",NEW."created_at") THEN
        RAISE EXCEPTION 'channel service operation intent is immutable';
    END IF;
    IF OLD."provider_task_no" IS DISTINCT FROM NEW."provider_task_no" AND NOT (
        OLD."provider_task_no" IS NULL AND NEW."provider_task_no" IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'channel service operation task link is immutable';
    END IF;
    RETURN NEW;
END; $$;
