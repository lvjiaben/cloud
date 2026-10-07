-- Resource-owned product fulfillment reliability and secret-free interface projection.

ALTER TABLE plugin_idc_core_product_automation_interface_instance
    ADD COLUMN IF NOT EXISTS "interface_origin_code" VARCHAR(16) NOT NULL DEFAULT 'managed',
    ADD COLUMN IF NOT EXISTS "resource_owner_id" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "resource_channel_ref" VARCHAR(64) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_product_automation_interface_origin'
          AND conrelid = 'plugin_idc_core_product_automation_interface_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_product_automation_interface_instance
            ADD CONSTRAINT ck_idc_product_automation_interface_origin CHECK (
                ("interface_origin_code" = 'managed' AND "resource_owner_id" = '' AND "resource_channel_ref" = '')
                OR
                ("interface_origin_code" = 'resource' AND "resource_owner_id" = 'idc-resource'
                 AND "resource_channel_ref" ~ '^CHN_[A-Za-z0-9_-]{1,60}$')
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_idc_product_automation_resource_projection
    ON plugin_idc_core_product_automation_interface_instance (
        "tenant_id", "resource_owner_id", "resource_channel_ref"
    ) WHERE "interface_origin_code" = 'resource';

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_automation_interface_update()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD."ever_referenced" THEN
            RAISE EXCEPTION 'referenced product automation interface instances cannot be deleted';
        END IF;
        RETURN OLD;
    END IF;
    NEW."changed_at" := CURRENT_TIMESTAMP;
    IF ROW(
        NEW."id", NEW."tenant_id", NEW."interface_instance_no",
        NEW."plugin_id", NEW."extension_point_code", NEW."plugin_release_version",
        NEW."contract_version", NEW."authorized_methods", NEW."release_digest",
        NEW."declaration_digest", NEW."created_by", NEW."created_at",
        NEW."interface_origin_code", NEW."resource_owner_id", NEW."resource_channel_ref"
    ) IS DISTINCT FROM ROW(
        OLD."id", OLD."tenant_id", OLD."interface_instance_no",
        OLD."plugin_id", OLD."extension_point_code", OLD."plugin_release_version",
        OLD."contract_version", OLD."authorized_methods", OLD."release_digest",
        OLD."declaration_digest", OLD."created_by", OLD."created_at",
        OLD."interface_origin_code", OLD."resource_owner_id", OLD."resource_channel_ref"
    ) THEN
        RAISE EXCEPTION 'product automation interface identity and evidence are immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1 OR NEW."changed_by" <= 0 OR NEW."changed_at" < OLD."changed_at" THEN
        RAISE EXCEPTION 'product automation interface revision is invalid';
    END IF;
    IF (CASE WHEN NEW."display_name" IS DISTINCT FROM OLD."display_name" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced" THEN 1 ELSE 0 END) <> 1 THEN
        RAISE EXCEPTION 'product automation interface update must change exactly one managed fact';
    END IF;
    IF NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state"
       AND (OLD."new_business_state", NEW."new_business_state") NOT IN (('accepting', 'stopped'), ('stopped', 'accepting')) THEN
        RAISE EXCEPTION 'product automation interface state transition is invalid';
    END IF;
    IF NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced"
       AND (OLD."ever_referenced" OR NOT NEW."ever_referenced") THEN
        RAISE EXCEPTION 'product automation interface reference history is irreversible';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Purpose: Store durable resource-owner outcome reports without supplier secrets.
-- 用途：保存不含供应商秘密的持久资源履约结果回报。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_fulfillment_resource_outcome_outbox (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) PRIMARY KEY,
    "provider_task_no" VARCHAR(64) NOT NULL,
    "report_payload" TEXT NOT NULL,
    "report_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "next_attempt_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "lease_token" VARCHAR(40) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "reported_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_fulfillment_resource_outcome_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_fulfillment_resource_outcome_digest CHECK ("report_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_product_fulfillment_resource_outcome_payload CHECK (jsonb_typeof("report_payload"::jsonb) = 'object'),
    CONSTRAINT ck_idc_product_fulfillment_resource_outcome_status CHECK (
        "status_code" IN ('pending','delivering','retry_wait','reported','manual_attention')
        AND "attempt_count" BETWEEN 0 AND 10
        AND (("status_code" = 'delivering' AND "lease_token" <> '' AND "lease_expires_at" IS NOT NULL)
          OR ("status_code" <> 'delivering' AND "lease_token" = '' AND "lease_expires_at" IS NULL))
        AND (("status_code" = 'reported' AND "reported_at" IS NOT NULL)
          OR ("status_code" <> 'reported' AND "reported_at" IS NULL))
    ),
    CONSTRAINT fk_idc_product_fulfillment_resource_outcome_task FOREIGN KEY ("task_no")
        REFERENCES plugin_idc_core_product_fulfillment_task ("task_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_product_fulfillment_resource_provider_task
    ON plugin_idc_core_product_fulfillment_resource_outcome_outbox ("provider_task_no");
CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_resource_outcome_due
    ON plugin_idc_core_product_fulfillment_resource_outcome_outbox (
        "tenant_id", "status_code", "next_attempt_at", "task_no"
    ) WHERE "status_code" IN ('pending','retry_wait','delivering');

COMMENT ON TABLE plugin_idc_core_product_fulfillment_resource_outcome_outbox IS
    'Durable post-commit resource-owner outcome reports; payload contains frozen business evidence and no supplier credential';
