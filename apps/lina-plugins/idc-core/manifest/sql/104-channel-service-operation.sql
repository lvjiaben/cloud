-- Core-owned channel lifecycle intent. Provider credentials and upstream
-- resource identities remain owned by idc-resource.
DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_service_entitlement'::regclass
          AND conname = 'uk_idc_service_entitlement_owner'
    ) THEN
        ALTER TABLE plugin_idc_core_service_entitlement
            ADD CONSTRAINT uk_idc_service_entitlement_owner UNIQUE (
                "tenant_id", "entitlement_no", "customer_no"
            );
    END IF;
END
$constraint$;

-- Purpose: Stores immutable channel service-operation intents and provider task evidence.
-- 用途：保存不可变的渠道服务操作意图和供应商任务证据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_channel_service_operation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "operation_no" VARCHAR(40) NOT NULL,
    "channel_id" VARCHAR(80) NOT NULL,
    "payer_customer_no" VARCHAR(40) NOT NULL,
    "service_owner_customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "parameters" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "resource_channel_no" VARCHAR(64) NOT NULL,
    "resource_pool_no" VARCHAR(64) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "expected_expires_at" TIMESTAMPTZ,
    "provider_task_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "request_digest" CHAR(64) NOT NULL,
    "task_evidence_digest" CHAR(64) NOT NULL,
    "output_bindings" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_channel_service_operation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_channel_service_operation_action CHECK (
        "action_code" IN ('resize','reinstall','console','ip_change','terminate')
    ),
    CONSTRAINT ck_idc_channel_service_operation_payload CHECK (
        jsonb_typeof("parameters") = 'object'
        AND octet_length("parameters"::TEXT) <= 16384
        AND jsonb_typeof("output_bindings") = 'array'
        AND jsonb_array_length("output_bindings") <= 8
    ),
    CONSTRAINT ck_idc_channel_service_operation_status CHECK (
        "status_code" IN (
            'pending','running','succeeded','failed','result_unknown','manual_review'
        )
    ),
    CONSTRAINT ck_idc_channel_service_operation_evidence CHECK (
        "expected_service_version" > 0
        AND (("action_code" = 'terminate' AND "expected_expires_at" IS NOT NULL)
          OR ("action_code" <> 'terminate' AND "expected_expires_at" IS NULL))
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "task_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND ("failure_code" = '' OR "failure_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
        AND (("status_code" IN ('succeeded','failed') AND "completed_at" IS NOT NULL)
          OR ("status_code" NOT IN ('succeeded','failed') AND "completed_at" IS NULL))
        AND "version" > 0
    ),
    CONSTRAINT uk_idc_channel_service_operation_no UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_channel_service_operation_task UNIQUE ("provider_task_no"),
    CONSTRAINT fk_idc_channel_service_operation_business FOREIGN KEY ("operation_no")
        REFERENCES plugin_idc_core_business_operation("operation_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_service_operation_payer FOREIGN KEY ("payer_customer_no")
        REFERENCES plugin_idc_core_customer_subject("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_service_operation_owner FOREIGN KEY (
        "tenant_id", "service_no", "service_owner_customer_no"
    ) REFERENCES plugin_idc_core_service_entitlement(
        "tenant_id", "entitlement_no", "customer_no"
    ) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_channel_service_operation_active
    ON plugin_idc_core_channel_service_operation("tenant_id", "service_no")
    WHERE "status_code" IN ('pending','running','result_unknown');

CREATE INDEX IF NOT EXISTS idx_idc_channel_service_operation_channel
    ON plugin_idc_core_channel_service_operation(
        "tenant_id", "channel_id", "created_at" DESC, "id" DESC
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_channel_service_operation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'channel service operations cannot be deleted';
    END IF;
    IF ROW(OLD."tenant_id",OLD."operation_no",OLD."channel_id",OLD."payer_customer_no",
           OLD."service_owner_customer_no",OLD."service_no",OLD."action_code",OLD."parameters",
           OLD."resource_channel_no",OLD."resource_pool_no",OLD."expected_service_version",
           OLD."expected_expires_at",
           OLD."provider_task_no",OLD."request_digest",OLD."created_at")
       IS DISTINCT FROM
       ROW(NEW."tenant_id",NEW."operation_no",NEW."channel_id",NEW."payer_customer_no",
           NEW."service_owner_customer_no",NEW."service_no",NEW."action_code",NEW."parameters",
           NEW."resource_channel_no",NEW."resource_pool_no",NEW."expected_service_version",
           NEW."expected_expires_at",
           NEW."provider_task_no",NEW."request_digest",NEW."created_at") THEN
        RAISE EXCEPTION 'channel service operation intent is immutable';
    END IF;
    RETURN NEW;
END; $$;

DROP TRIGGER IF EXISTS trg_idc_channel_service_operation_immutable
    ON plugin_idc_core_channel_service_operation;
CREATE TRIGGER trg_idc_channel_service_operation_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_channel_service_operation
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_channel_service_operation();
