-- 088: Freeze renewal authority from the published fulfillment contract.
-- Existing development rows deliberately remain unavailable. They must be
-- rebuilt from a newly published product version; no plugin-id inference or
-- compatibility fallback is permitted.

-- Renewal-only records name their authority directly. Shared commercial
-- records use an explicit generic subject instead of pretending an
-- entitlement is a server service.
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD COLUMN IF NOT EXISTS "subject_type_code" VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS "subject_no" VARCHAR(40),
    ALTER COLUMN "service_no" DROP NOT NULL;

ALTER TABLE plugin_idc_core_service_price_component_snapshot
    ADD COLUMN IF NOT EXISTS "subject_type_code" VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS "subject_no" VARCHAR(40),
    ALTER COLUMN "service_no" DROP NOT NULL;

ALTER TABLE plugin_idc_core_business_operation
    ADD COLUMN IF NOT EXISTS "subject_type_code" VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS "subject_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "expected_subject_version" BIGINT,
    ALTER COLUMN "service_no" DROP NOT NULL,
    ALTER COLUMN "expected_service_version" DROP NOT NULL;

ALTER TABLE plugin_idc_core_business_operation_event
    ADD COLUMN IF NOT EXISTS "subject_type_code" VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS "subject_no" VARCHAR(40),
    ALTER COLUMN "service_no" DROP NOT NULL;

CREATE OR REPLACE FUNCTION plugin_idc_core_normalize_shared_subject()
RETURNS TRIGGER AS $$ BEGIN
    IF NEW."subject_type_code"='service' AND NEW."subject_no" IS NULL AND NEW."service_no" IS NOT NULL THEN
        NEW."subject_no" := NEW."service_no";
    END IF;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_normalize_business_operation_subject()
RETURNS TRIGGER AS $$ BEGIN
    IF NEW."subject_type_code"='service' AND NEW."subject_no" IS NULL AND NEW."service_no" IS NOT NULL THEN
        NEW."subject_no" := NEW."service_no";
    END IF;
    IF NEW."subject_type_code"='service' AND NEW."expected_subject_version" IS NULL THEN
        NEW."expected_subject_version" := NEW."expected_service_version";
    END IF;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_price_snapshot_normalize_subject ON plugin_idc_core_service_price_snapshot;
CREATE TRIGGER trg_idc_price_snapshot_normalize_subject
BEFORE INSERT ON plugin_idc_core_service_price_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_shared_subject();
DROP TRIGGER IF EXISTS trg_idc_price_component_normalize_subject ON plugin_idc_core_service_price_component_snapshot;
CREATE TRIGGER trg_idc_price_component_normalize_subject
BEFORE INSERT ON plugin_idc_core_service_price_component_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_shared_subject();
DROP TRIGGER IF EXISTS trg_idc_business_operation_normalize_subject ON plugin_idc_core_business_operation;
CREATE TRIGGER trg_idc_business_operation_normalize_subject
BEFORE INSERT ON plugin_idc_core_business_operation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_business_operation_subject();
DROP TRIGGER IF EXISTS trg_idc_business_operation_event_normalize_subject ON plugin_idc_core_business_operation_event;
CREATE TRIGGER trg_idc_business_operation_event_normalize_subject
BEFORE INSERT ON plugin_idc_core_business_operation_event
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_shared_subject();

-- Preserve only historical subject evidence that can be derived without
-- guessing. The real migration runner commits every split statement, so all
-- temporary trigger changes and all backfills deliberately live inside this
-- single atomic DO statement. Any error rolls the entire statement back and
-- cannot leave a trigger disabled or a partial backfill committed.
DO $historical_subject_backfill$
DECLARE
    operation_subject_trigger_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM pg_trigger
        WHERE tgrelid='plugin_idc_core_business_operation'::regclass
          AND tgname='trg_idc_business_operation_subject'
          AND NOT tgisinternal
    ) INTO operation_subject_trigger_exists;

    EXECUTE 'ALTER TABLE plugin_idc_core_service_price_snapshot DISABLE TRIGGER trg_idc_service_price_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_service_price_component_snapshot DISABLE TRIGGER trg_idc_service_price_component_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER trg_idc_business_operation_intent_guard';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER trg_idc_business_operation_event_composition';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation_event DISABLE TRIGGER trg_idc_business_operation_event_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation_event DISABLE TRIGGER trg_idc_business_operation_event_item_composition';
    IF operation_subject_trigger_exists THEN
        EXECUTE 'ALTER TABLE plugin_idc_core_business_operation DISABLE TRIGGER trg_idc_business_operation_subject';
    END IF;

    UPDATE plugin_idc_core_service_price_snapshot
    SET "subject_no"="service_no"
    WHERE "subject_type_code"='service' AND "subject_no" IS NULL AND "service_no" IS NOT NULL;
    UPDATE plugin_idc_core_service_price_component_snapshot
    SET "subject_no"="service_no"
    WHERE "subject_type_code"='service' AND "subject_no" IS NULL AND "service_no" IS NOT NULL;
    UPDATE plugin_idc_core_business_operation
    SET "subject_no"="service_no",
        "expected_subject_version"="expected_service_version"
    WHERE "subject_type_code"='service'
      AND "service_no" IS NOT NULL
      AND ("subject_no" IS NULL OR "expected_subject_version" IS NULL);
    UPDATE plugin_idc_core_business_operation_event
    SET "subject_no"="service_no"
    WHERE "subject_type_code"='service' AND "subject_no" IS NULL AND "service_no" IS NOT NULL;

    IF EXISTS (
        SELECT 1 FROM plugin_idc_core_service_price_snapshot
        WHERE "subject_no" IS NULL OR "subject_type_code" NOT IN('service','entitlement')
           OR ("subject_type_code"='service' AND ("service_no" IS NULL OR "subject_no"<>"service_no"))
           OR ("subject_type_code"='entitlement' AND "service_no" IS NOT NULL)
    ) OR EXISTS (
        SELECT 1 FROM plugin_idc_core_service_price_component_snapshot
        WHERE "subject_no" IS NULL OR "subject_type_code" NOT IN('service','entitlement')
           OR ("subject_type_code"='service' AND ("service_no" IS NULL OR "subject_no"<>"service_no"))
           OR ("subject_type_code"='entitlement' AND "service_no" IS NOT NULL)
    ) OR EXISTS (
        SELECT 1 FROM plugin_idc_core_business_operation
        WHERE "subject_no" IS NULL OR "expected_subject_version" IS NULL
           OR "subject_type_code" NOT IN('service','entitlement')
           OR ("subject_type_code"='service' AND (
               "service_no" IS NULL OR "subject_no"<>"service_no"
               OR "expected_service_version" IS NULL
               OR "expected_subject_version"<>"expected_service_version"
           ))
           OR ("subject_type_code"='entitlement' AND (
               "service_no" IS NOT NULL OR "expected_service_version" IS NOT NULL
           ))
    ) OR EXISTS (
        SELECT 1 FROM plugin_idc_core_business_operation_event
        WHERE "subject_no" IS NULL OR "subject_type_code" NOT IN('service','entitlement')
           OR ("subject_type_code"='service' AND ("service_no" IS NULL OR "subject_no"<>"service_no"))
           OR ("subject_type_code"='entitlement' AND "service_no" IS NOT NULL)
    ) THEN
        RAISE EXCEPTION 'historical service subject evidence is ambiguous';
    END IF;

    EXECUTE 'ALTER TABLE plugin_idc_core_service_price_snapshot ENABLE TRIGGER trg_idc_service_price_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_service_price_component_snapshot ENABLE TRIGGER trg_idc_service_price_component_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER trg_idc_business_operation_intent_guard';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER trg_idc_business_operation_event_composition';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation_event ENABLE TRIGGER trg_idc_business_operation_event_immutable';
    EXECUTE 'ALTER TABLE plugin_idc_core_business_operation_event ENABLE TRIGGER trg_idc_business_operation_event_item_composition';
    IF operation_subject_trigger_exists THEN
        EXECUTE 'ALTER TABLE plugin_idc_core_business_operation ENABLE TRIGGER trg_idc_business_operation_subject';
    END IF;
END;
$historical_subject_backfill$;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF ROW(OLD."tenant_id",OLD."operation_no",OLD."operation_type_code",OLD."customer_no",
           OLD."service_no",OLD."subject_type_code",OLD."subject_no",OLD."currency_code",
           OLD."price_snapshot_no",OLD."quote_no",OLD."order_no",OLD."invoice_no",
           OLD."financial_settlement_required",OLD."execution_mode_code",OLD."resource_owner_code",
           OLD."channel_no",OLD."pool_no",OLD."capability_evidence_no",OLD."capability_evidence_digest",
           OLD."idempotency_digest",OLD."request_digest",OLD."content_digest",
           OLD."expected_service_version",OLD."expected_subject_version",OLD."created_at")
       IS DISTINCT FROM
       ROW(NEW."tenant_id",NEW."operation_no",NEW."operation_type_code",NEW."customer_no",
           NEW."service_no",NEW."subject_type_code",NEW."subject_no",NEW."currency_code",
           NEW."price_snapshot_no",NEW."quote_no",NEW."order_no",NEW."invoice_no",
           NEW."financial_settlement_required",NEW."execution_mode_code",NEW."resource_owner_code",
           NEW."channel_no",NEW."pool_no",NEW."capability_evidence_no",NEW."capability_evidence_digest",
           NEW."idempotency_digest",NEW."request_digest",NEW."content_digest",
           NEW."expected_service_version",NEW."expected_subject_version",NEW."created_at") THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    RETURN NEW;
END; $$;

ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy
    ADD COLUMN IF NOT EXISTS "renewal_mode_code" VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    ADD COLUMN IF NOT EXISTS "renewal_method" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "entitlement_owner_id" VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_core_product_fulfillment_intent
    ADD COLUMN IF NOT EXISTS "renewal_mode_code" VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    ADD COLUMN IF NOT EXISTS "renewal_method" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "entitlement_owner_id" VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_core_product_delivery_record
    ADD COLUMN IF NOT EXISTS "renewal_mode_code" VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    ADD COLUMN IF NOT EXISTS "renewal_method" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "entitlement_owner_id" VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_core_service_entitlement
    ADD COLUMN IF NOT EXISTS "renewal_mode_code" VARCHAR(32) NOT NULL DEFAULT 'unavailable',
    ADD COLUMN IF NOT EXISTS "renewal_method" VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "entitlement_owner_id" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "currency_code" CHAR(3) NOT NULL DEFAULT 'CNY',
    ADD COLUMN IF NOT EXISTS "billing_rule_no" VARCHAR(40) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "charge_type_code" VARCHAR(16) NOT NULL DEFAULT 'one_time',
    ADD COLUMN IF NOT EXISTS "period_unit_code" VARCHAR(16) NOT NULL DEFAULT 'once',
    ADD COLUMN IF NOT EXISTS "period_count" INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "current_renewal_policy_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "current_renewal_policy_version" BIGINT,
    ADD COLUMN IF NOT EXISTS "current_price_snapshot_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "last_renewal_operation_no" VARCHAR(40);

DO $constraints$ BEGIN
    ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy
        DROP CONSTRAINT IF EXISTS ck_idc_fulfillment_policy_renewal;
    ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy
        ADD CONSTRAINT ck_idc_fulfillment_policy_renewal CHECK (
            ("renewal_mode_code" = 'provider_confirmed' AND "ongoing_management" AND "entitlement_owner_id" <> '' AND
             "renewal_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND
             "renewal_method" <> "provision_method" AND "renewal_method" <> "query_method") OR
            ("renewal_mode_code" = 'local_entitlement' AND "ongoing_management" AND "renewal_method" = '' AND "entitlement_owner_id" <> '') OR
            ("renewal_mode_code" = 'unavailable' AND "renewal_method" = '' AND
             (("ongoing_management" AND "entitlement_owner_id" <> '') OR (NOT "ongoing_management" AND "entitlement_owner_id" = '')))
        ) NOT VALID;

    ALTER TABLE plugin_idc_core_product_fulfillment_intent
        DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_renewal;
    ALTER TABLE plugin_idc_core_product_fulfillment_intent
        ADD CONSTRAINT ck_idc_product_fulfillment_renewal CHECK (
            ("renewal_mode_code" = 'provider_confirmed' AND "ongoing_management" AND "entitlement_owner_id" <> '' AND
             "renewal_method" ~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$' AND
             "renewal_method" <> "provision_method" AND "renewal_method" <> "query_method") OR
            ("renewal_mode_code" = 'local_entitlement' AND "ongoing_management" AND "renewal_method" = '' AND "entitlement_owner_id" <> '') OR
            ("renewal_mode_code" = 'unavailable' AND "renewal_method" = '' AND
             (("ongoing_management" AND "entitlement_owner_id" <> '') OR (NOT "ongoing_management" AND "entitlement_owner_id" = '')))
        ) NOT VALID;

    ALTER TABLE plugin_idc_core_product_delivery_record
        DROP CONSTRAINT IF EXISTS ck_idc_product_delivery_renewal;
    ALTER TABLE plugin_idc_core_product_delivery_record
        ADD CONSTRAINT ck_idc_product_delivery_renewal CHECK (
            ("renewal_mode_code" = 'provider_confirmed' AND "ongoing_management" AND "renewal_method" <> '' AND "entitlement_owner_id" <> '') OR
            ("renewal_mode_code" = 'local_entitlement' AND "ongoing_management" AND "renewal_method" = '' AND "entitlement_owner_id" <> '') OR
            ("renewal_mode_code" = 'unavailable' AND "renewal_method" = '' AND
             (("ongoing_management" AND "entitlement_owner_id" <> '') OR (NOT "ongoing_management" AND "entitlement_owner_id" = '')))
        ) NOT VALID;

    ALTER TABLE plugin_idc_core_service_entitlement
        DROP CONSTRAINT IF EXISTS ck_idc_service_entitlement_renewal;
    ALTER TABLE plugin_idc_core_service_entitlement
        ADD CONSTRAINT ck_idc_service_entitlement_renewal CHECK (
            ("renewal_mode_code" = 'provider_confirmed' AND "renewal_method" <> '' AND "entitlement_owner_id" <> '' AND
             "period_unit_code" <> 'once' AND "charge_type_code" <> 'one_time') OR
            ("renewal_mode_code" = 'local_entitlement' AND "renewal_method" = '' AND "entitlement_owner_id" <> '' AND
             "period_unit_code" <> 'once' AND "charge_type_code" <> 'one_time') OR
            ("renewal_mode_code" = 'unavailable' AND "renewal_method" = '')
        ) NOT VALID;
END $constraints$;

CREATE OR REPLACE FUNCTION plugin_idc_core_freeze_entitlement_renewal()
RETURNS TRIGGER AS $$
DECLARE
    frozen_mode VARCHAR(32);
    frozen_method VARCHAR(128);
    frozen_owner VARCHAR(64);
    frozen_ongoing BOOLEAN;
    frozen_currency CHAR(3);
    frozen_billing_rule VARCHAR(40);
    frozen_charge_type VARCHAR(16);
    frozen_period_unit VARCHAR(16);
    frozen_period_count INT;
BEGIN
    SELECT intent."renewal_mode_code",intent."renewal_method",intent."entitlement_owner_id",intent."ongoing_management",
           item."currency_code",item."billing_rule_no",item."charge_type_code",
           item."period_unit_code",item."period_count"
      INTO frozen_mode,frozen_method,frozen_owner,frozen_ongoing,frozen_currency,
           frozen_billing_rule,frozen_charge_type,frozen_period_unit,frozen_period_count
      FROM plugin_idc_core_product_fulfillment_intent intent
      JOIN plugin_idc_core_commerce_order_item item
        ON item."tenant_id"=intent."tenant_id" AND item."order_item_no"=intent."order_item_no"
     WHERE intent."tenant_id" = NEW."tenant_id" AND intent."intent_no" = NEW."source_intent_no";

    IF NOT FOUND OR NOT frozen_ongoing THEN
        RAISE EXCEPTION 'service entitlement has no frozen ongoing-management contract'
            USING ERRCODE = '23514';
    END IF;
    NEW."renewal_mode_code" := frozen_mode;
    NEW."renewal_method" := frozen_method;
    NEW."entitlement_owner_id" := frozen_owner;
    NEW."currency_code" := frozen_currency;
    NEW."billing_rule_no" := frozen_billing_rule;
    NEW."charge_type_code" := frozen_charge_type;
    NEW."period_unit_code" := frozen_period_unit;
    NEW."period_count" := frozen_period_count;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_freeze_entitlement_renewal
    ON plugin_idc_core_service_entitlement;
CREATE TRIGGER trg_idc_freeze_entitlement_renewal
BEFORE INSERT ON plugin_idc_core_service_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_freeze_entitlement_renewal();

CREATE INDEX IF NOT EXISTS idx_idc_service_entitlement_customer_renewal
    ON plugin_idc_core_service_entitlement(
        "tenant_id","customer_no","renewal_mode_code","status_code","expires_at","id" DESC
    );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_service_entitlement_renewal_owner
    ON plugin_idc_core_service_entitlement("entitlement_no","customer_no","currency_code");

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_price_snapshot_subject
    ON plugin_idc_core_service_price_snapshot("price_snapshot_no","subject_no");
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_price_snapshot_subject_currency
    ON plugin_idc_core_service_price_snapshot("price_snapshot_no","subject_no","currency_code");
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_business_operation_subject
    ON plugin_idc_core_business_operation("operation_no","subject_no");

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_price_snapshot_subject,
    ADD CONSTRAINT ck_idc_price_snapshot_subject CHECK (
        "subject_no" IS NOT NULL AND "subject_type_code" IN ('service','entitlement') AND
        (("subject_type_code"='service' AND "service_no"="subject_no") OR
         ("subject_type_code"='entitlement' AND "service_no" IS NULL))
    ) NOT VALID;
ALTER TABLE plugin_idc_core_service_price_component_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_price_component_subject,
    ADD CONSTRAINT ck_idc_price_component_subject CHECK (
        "subject_no" IS NOT NULL AND "subject_type_code" IN ('service','entitlement') AND
        (("subject_type_code"='service' AND "service_no"="subject_no") OR
         ("subject_type_code"='entitlement' AND "service_no" IS NULL))
    ) NOT VALID;
ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_subject,
    ADD CONSTRAINT ck_idc_business_operation_subject CHECK (
        "subject_no" IS NOT NULL AND "subject_type_code" IN ('service','entitlement') AND "expected_subject_version">0 AND
        (("subject_type_code"='service' AND "service_no"="subject_no" AND "expected_service_version"="expected_subject_version") OR
         ("subject_type_code"='entitlement' AND "service_no" IS NULL AND "expected_service_version" IS NULL))
    ) NOT VALID;
ALTER TABLE plugin_idc_core_business_operation_event
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_event_subject,
    ADD CONSTRAINT ck_idc_business_operation_event_subject CHECK (
        "subject_no" IS NOT NULL AND "subject_type_code" IN ('service','entitlement') AND
        (("subject_type_code"='service' AND "service_no"="subject_no") OR
         ("subject_type_code"='entitlement' AND "service_no" IS NULL))
    ) NOT VALID;

ALTER TABLE plugin_idc_core_service_price_component_snapshot
    DROP CONSTRAINT IF EXISTS fk_idc_price_component_subject,
    ADD CONSTRAINT fk_idc_price_component_subject FOREIGN KEY("price_snapshot_no","subject_no")
    REFERENCES plugin_idc_core_service_price_snapshot("price_snapshot_no","subject_no") ON DELETE RESTRICT NOT VALID;
ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS fk_idc_business_operation_price_subject,
    ADD CONSTRAINT fk_idc_business_operation_price_subject
    FOREIGN KEY("price_snapshot_no","subject_no","currency_code")
    REFERENCES plugin_idc_core_service_price_snapshot("price_snapshot_no","subject_no","currency_code")
    ON DELETE RESTRICT NOT VALID;
ALTER TABLE plugin_idc_core_business_operation_event
    DROP CONSTRAINT IF EXISTS fk_idc_business_operation_event_subject,
    ADD CONSTRAINT fk_idc_business_operation_event_subject
    FOREIGN KEY("operation_no","subject_no")
    REFERENCES plugin_idc_core_business_operation("operation_no","subject_no")
    ON DELETE RESTRICT NOT VALID;

-- Purpose: Stores Entitlement Created Outbox records.
-- 用途：保存权益创建发件箱相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_entitlement_created_outbox (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL UNIQUE,
    "entitlement_no" VARCHAR(40) NOT NULL UNIQUE,
    "intent_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "version_no" VARCHAR(40) NOT NULL,
    "product_name_zh_cn" VARCHAR(256) NOT NULL,
    "product_name_en_us" VARCHAR(256) NOT NULL,
    "configuration_snapshot_no" VARCHAR(40) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "plugin_id" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "contract_version" VARCHAR(32) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "owner_id" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token" VARCHAR(64) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "owner_reference" VARCHAR(128) NOT NULL DEFAULT '',
    "receipt_evidence_digest" CHAR(64) NOT NULL DEFAULT '',
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_entitlement_created_outbox_tenant CHECK("tenant_id"=0),
    CONSTRAINT ck_idc_entitlement_created_outbox_status CHECK("status_code" IN
        ('pending','delivering','retry_wait','delivered','manual_attention')),
    CONSTRAINT ck_idc_entitlement_created_outbox_attempt CHECK("attempt_count" BETWEEN 0 AND 10),
    CONSTRAINT ck_idc_entitlement_created_outbox_digest CHECK(
        "declaration_digest" ~ '^[0-9a-f]{64}$' AND
        ("receipt_evidence_digest"='' OR "receipt_evidence_digest" ~ '^[0-9a-f]{64}$')),
    CONSTRAINT fk_idc_entitlement_created_outbox_entitlement FOREIGN KEY("entitlement_no")
        REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_entitlement_created_outbox_work
    ON plugin_idc_core_entitlement_created_outbox("status_code","next_attempt_at","id")
    WHERE "status_code" IN ('pending','retry_wait','delivering');

CREATE OR REPLACE FUNCTION plugin_idc_core_enqueue_entitlement_created()
RETURNS TRIGGER AS $$ BEGIN
    INSERT INTO plugin_idc_core_entitlement_created_outbox(
        "tenant_id","event_no","entitlement_no","intent_no","customer_no","product_no","version_no",
        "product_name_zh_cn","product_name_en_us",
        "configuration_snapshot_no","interface_instance_no","plugin_id","plugin_release_version",
        "contract_version","declaration_digest","owner_id")
    SELECT NEW."tenant_id",'ece_'||substr(md5('entitlement-created:'||NEW."entitlement_no"),1,32),
        NEW."entitlement_no",NEW."source_intent_no",NEW."customer_no",NEW."product_no",NEW."version_no",
        intent."product_name_zh_cn",intent."product_name_en_us",
        NEW."configuration_snapshot_no",NEW."interface_instance_no",NEW."plugin_id",
        NEW."plugin_release_version",NEW."contract_version",NEW."declaration_digest",NEW."entitlement_owner_id"
    FROM plugin_idc_core_product_fulfillment_intent intent
    WHERE intent."tenant_id"=NEW."tenant_id" AND intent."intent_no"=NEW."source_intent_no"
    ON CONFLICT("entitlement_no") DO NOTHING;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_enqueue_entitlement_created
    ON plugin_idc_core_service_entitlement;
CREATE TRIGGER trg_idc_enqueue_entitlement_created
AFTER INSERT ON plugin_idc_core_service_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_enqueue_entitlement_created();

-- Renewal records now use the generic entitlement identity directly.
-- Migration 090 preserves exact historical order-service evidence and fails
-- closed when a legacy row has no unambiguous frozen fulfillment contract.
ALTER TABLE plugin_idc_core_service_renewal_policy
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_policy_service,
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_policy_service_identity,
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_policy_entitlement;
ALTER TABLE plugin_idc_core_service_renewal_policy
    ADD CONSTRAINT fk_idc_service_renewal_policy_entitlement FOREIGN KEY("entitlement_no")
    REFERENCES plugin_idc_core_service_entitlement("entitlement_no") ON DELETE RESTRICT NOT VALID;

ALTER TABLE plugin_idc_core_service_renewal_quote_request
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_quote_request_service,
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_quote_request_service_identity,
    DROP CONSTRAINT IF EXISTS fk_idc_service_renewal_quote_request_entitlement;
ALTER TABLE plugin_idc_core_service_renewal_quote_request
    ADD CONSTRAINT fk_idc_service_renewal_quote_request_entitlement
    FOREIGN KEY("entitlement_no","customer_no","currency_code")
    REFERENCES plugin_idc_core_service_entitlement("entitlement_no","customer_no","currency_code")
    ON DELETE RESTRICT NOT VALID;

-- No historical row is allowed to remain outside the published renewal
-- contract. NOT VALID is used only to add each constraint without a long table
-- lock; these explicit validations make the migration fail closed before it
-- can be considered complete.
ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy
    VALIDATE CONSTRAINT ck_idc_fulfillment_policy_renewal;
ALTER TABLE plugin_idc_core_product_fulfillment_intent
    VALIDATE CONSTRAINT ck_idc_product_fulfillment_renewal;
ALTER TABLE plugin_idc_core_product_delivery_record
    VALIDATE CONSTRAINT ck_idc_product_delivery_renewal;
ALTER TABLE plugin_idc_core_service_entitlement
    VALIDATE CONSTRAINT ck_idc_service_entitlement_renewal;
ALTER TABLE plugin_idc_core_service_renewal_policy
    VALIDATE CONSTRAINT fk_idc_service_renewal_policy_entitlement;
ALTER TABLE plugin_idc_core_service_renewal_quote_request
    VALIDATE CONSTRAINT fk_idc_service_renewal_quote_request_entitlement;

ALTER TABLE plugin_idc_core_service_price_snapshot
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_service,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_service_identity,
    DROP CONSTRAINT IF EXISTS fk_idc_service_price_service_subject;
ALTER TABLE plugin_idc_core_service_price_snapshot
    ADD CONSTRAINT fk_idc_service_price_service_subject
    FOREIGN KEY("service_no","customer_no","currency_code")
    REFERENCES plugin_idc_core_service_instance("service_no","customer_no","currency_code")
    ON DELETE RESTRICT NOT VALID;

-- Every historical row that could be mapped without guessing has now been
-- backfilled. Validation turns the generic subject contract into a complete
-- database invariant instead of leaving old rows outside it indefinitely.
ALTER TABLE plugin_idc_core_service_price_snapshot
    VALIDATE CONSTRAINT ck_idc_price_snapshot_subject,
    VALIDATE CONSTRAINT fk_idc_service_price_service_subject;
ALTER TABLE plugin_idc_core_service_price_component_snapshot
    VALIDATE CONSTRAINT ck_idc_price_component_subject,
    VALIDATE CONSTRAINT fk_idc_price_component_subject;
ALTER TABLE plugin_idc_core_business_operation
    VALIDATE CONSTRAINT ck_idc_business_operation_subject,
    VALIDATE CONSTRAINT fk_idc_business_operation_price_subject;
ALTER TABLE plugin_idc_core_business_operation_event
    VALIDATE CONSTRAINT ck_idc_business_operation_event_subject,
    VALIDATE CONSTRAINT fk_idc_business_operation_event_subject;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_price_snapshot_subject()
RETURNS TRIGGER AS $$ BEGIN
    IF NEW."subject_type_code"='entitlement' AND NOT EXISTS (
        SELECT 1 FROM plugin_idc_core_service_entitlement entitlement
        WHERE entitlement."tenant_id"=NEW."tenant_id"
          AND entitlement."entitlement_no"=NEW."subject_no"
          AND entitlement."customer_no"=NEW."customer_no"
          AND entitlement."currency_code"=NEW."currency_code"
    ) THEN
        RAISE EXCEPTION 'price snapshot entitlement subject is unavailable' USING ERRCODE='23503';
    END IF;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_idc_price_snapshot_subject ON plugin_idc_core_service_price_snapshot;
CREATE TRIGGER trg_idc_price_snapshot_subject
BEFORE INSERT OR UPDATE OF "subject_type_code","subject_no","customer_no","currency_code"
ON plugin_idc_core_service_price_snapshot
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_price_snapshot_subject();

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS fk_idc_business_operation_service,
    DROP CONSTRAINT IF EXISTS fk_idc_business_operation_service_identity;

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_business_operation_subject()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW."subject_type_code" = 'entitlement' THEN
        IF NOT EXISTS (
            SELECT 1 FROM plugin_idc_core_service_entitlement entitlement
             WHERE entitlement."tenant_id"=NEW."tenant_id"
               AND entitlement."entitlement_no"=NEW."subject_no"
               AND entitlement."customer_no"=NEW."customer_no"
               AND entitlement."currency_code"=NEW."currency_code"
        ) THEN
            RAISE EXCEPTION 'business operation entitlement subject is unavailable'
                USING ERRCODE='23503';
        END IF;
    ELSIF NEW."subject_type_code" = 'service' AND NOT EXISTS (
        SELECT 1 FROM plugin_idc_core_service_instance service
         WHERE service."tenant_id"=NEW."tenant_id" AND service."service_no"=NEW."subject_no"
           AND service."customer_no"=NEW."customer_no" AND service."currency_code"=NEW."currency_code"
    ) THEN
        RAISE EXCEPTION 'business operation service subject is unavailable'
            USING ERRCODE='23503';
    ELSIF NEW."subject_type_code" NOT IN ('entitlement','service') THEN
        RAISE EXCEPTION 'business operation subject type is unavailable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_business_operation'::regclass
          AND conname = 'trg_idc_business_operation_subject' AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1 FROM pg_constraint AS constraint_row
            JOIN pg_trigger AS trigger_row ON trigger_row.tgconstraint = constraint_row.oid
            WHERE constraint_row.conrelid = 'plugin_idc_core_business_operation'::regclass
              AND constraint_row.conname = 'trg_idc_business_operation_subject'
              AND constraint_row.contype = 't'
              AND trigger_row.tgrelid = constraint_row.conrelid
              AND NOT trigger_row.tgisinternal
              AND trigger_row.tgname = 'trg_idc_business_operation_subject'
              AND trigger_row.tgfoid = 'plugin_idc_core_validate_business_operation_subject()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_business_operation_subject catalog contract is invalid' USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_business_operation_subject
        AFTER INSERT OR UPDATE OF "subject_type_code","subject_no","customer_no","currency_code"
        ON plugin_idc_core_business_operation DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_business_operation_subject();
    END IF;
END;
$constraint_trigger$;
