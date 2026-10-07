-- 060: Snapshot and backup objects receive a local entitlement that is bound
-- after Provider success, renews independently, and survives price changes.

ALTER TABLE plugin_idc_core_server_accessory_quote
    ADD COLUMN IF NOT EXISTS "baseline_object_nos" JSONB NOT NULL DEFAULT '[]'::JSONB,
    ADD COLUMN IF NOT EXISTS "baseline_snapshot_no" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "baseline_observed_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_core_server_accessory_quote
    DROP CONSTRAINT IF EXISTS ck_idc_server_accessory_quote_baseline;
ALTER TABLE plugin_idc_core_server_accessory_quote
    ADD CONSTRAINT ck_idc_server_accessory_quote_baseline CHECK (
        jsonb_typeof("baseline_object_nos") = 'array'
        AND jsonb_array_length("baseline_object_nos") BETWEEN 0 AND 10000
        AND (
            ("accessory_kind" IN ('snapshot', 'backup')
                AND length(btrim("baseline_snapshot_no")) BETWEEN 1 AND 64
                AND "baseline_observed_at" IS NOT NULL)
            OR ("accessory_kind" = 'traffic'
                AND "baseline_snapshot_no" = ''
                AND "baseline_observed_at" IS NULL)
        )
    ) NOT VALID;

-- Purpose: Local free or paid ownership and renewal state for one Provider snapshot or backup object.
-- 用途：保存服务器附属资源权益相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_entitlement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "create_operation_no" VARCHAR(40) NOT NULL,
    "object_no" VARCHAR(128),
    "accessory_kind" VARCHAR(24) NOT NULL,
    "requested_name" VARCHAR(128) NOT NULL DEFAULT '',
    "billing_mode_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "auto_renew_enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "currency_code" CHAR(3) NOT NULL,
    "period_unit_code" VARCHAR(16) NOT NULL,
    "period_count" INT NOT NULL,
    "current_period_start_at" TIMESTAMPTZ,
    "current_period_end_at" TIMESTAMPTZ,
    "pending_invoice_no" VARCHAR(40),
    "pending_policy_no" VARCHAR(40),
    "pending_policy_version" BIGINT,
    "pending_period_unit_code" VARCHAR(16),
    "pending_period_count" INT,
    "pending_total_amount_minor" BIGINT,
    "last_renewal_attempt_at" TIMESTAMPTZ,
    "baseline_object_nos" JSONB NOT NULL,
    "baseline_snapshot_no" VARCHAR(64) NOT NULL,
    "baseline_observed_at" TIMESTAMPTZ NOT NULL,
    "binding_attempt_count" INT NOT NULL DEFAULT 0,
    "last_binding_attempt_at" TIMESTAMPTZ,
    "deleted_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_accessory_entitlement_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_accessory_entitlement_kind CHECK ("accessory_kind" IN ('snapshot', 'backup')),
    CONSTRAINT ck_idc_server_accessory_entitlement_billing CHECK ("billing_mode_code" IN ('free', 'paid')),
    CONSTRAINT ck_idc_server_accessory_entitlement_status CHECK (
        "status_code" IN ('pending_binding', 'active', 'payment_pending', 'manual_review', 'deleted')
    ),
    CONSTRAINT ck_idc_server_accessory_entitlement_period CHECK (
        ("period_unit_code" = 'day' AND "period_count" BETWEEN 1 AND 3650)
        OR ("period_unit_code" = 'month' AND "period_count" BETWEEN 1 AND 120)
        OR ("period_unit_code" = 'year' AND "period_count" BETWEEN 1 AND 10)
    ),
    CONSTRAINT ck_idc_server_accessory_entitlement_dates CHECK (
        ("billing_mode_code" = 'free' AND "current_period_start_at" IS NULL AND "current_period_end_at" IS NULL)
        OR ("billing_mode_code" = 'paid' AND (
            ("status_code" IN ('pending_binding', 'manual_review')
                AND "current_period_start_at" IS NULL AND "current_period_end_at" IS NULL)
            OR ("status_code" IN ('active', 'payment_pending', 'deleted')
                AND "current_period_start_at" IS NOT NULL
                AND "current_period_end_at" > "current_period_start_at")
        ))
    ),
    CONSTRAINT ck_idc_server_accessory_entitlement_delete CHECK (
        ("status_code" = 'deleted' AND "deleted_at" IS NOT NULL AND NOT "auto_renew_enabled")
        OR ("status_code" <> 'deleted' AND "deleted_at" IS NULL)
    ),
    CONSTRAINT ck_idc_server_accessory_entitlement_pending_invoice CHECK (
        ("pending_invoice_no" IS NULL AND "pending_policy_no" IS NULL
            AND "pending_policy_version" IS NULL AND "pending_period_unit_code" IS NULL
            AND "pending_period_count" IS NULL AND "pending_total_amount_minor" IS NULL)
        OR ("pending_invoice_no" IS NOT NULL
            AND length(btrim("pending_policy_no")) BETWEEN 1 AND 40
            AND "pending_policy_version" > 0
            AND "pending_period_unit_code" IN ('day', 'month', 'year')
            AND "pending_period_count" > 0
            AND "pending_total_amount_minor" >= 0)
    ),
    CONSTRAINT ck_idc_server_accessory_entitlement_baseline CHECK (
        jsonb_typeof("baseline_object_nos") = 'array'
        AND jsonb_array_length("baseline_object_nos") BETWEEN 0 AND 10000
        AND length(btrim("baseline_snapshot_no")) BETWEEN 1 AND 64
    ),
    CONSTRAINT uk_idc_server_accessory_entitlement_no UNIQUE ("entitlement_no"),
    CONSTRAINT uk_idc_server_accessory_entitlement_create UNIQUE ("create_operation_no"),
    CONSTRAINT fk_idc_server_accessory_entitlement_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_accessory_entitlement_service FOREIGN KEY ("service_no", "customer_no", "currency_code")
        REFERENCES plugin_idc_core_service_instance ("service_no", "customer_no", "currency_code") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_accessory_entitlement_operation FOREIGN KEY ("create_operation_no")
        REFERENCES plugin_idc_core_business_operation ("operation_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_server_accessory_entitlement_object
    ON plugin_idc_core_server_accessory_entitlement ("tenant_id", "service_no", "accessory_kind", "object_no")
    WHERE "object_no" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_server_accessory_entitlement_binding
    ON plugin_idc_core_server_accessory_entitlement ("tenant_id", "status_code", "updated_at", "id")
    WHERE "status_code" = 'pending_binding';
CREATE INDEX IF NOT EXISTS idx_idc_server_accessory_entitlement_renewal
    ON plugin_idc_core_server_accessory_entitlement ("tenant_id", "current_period_end_at", "id")
    WHERE "billing_mode_code" = 'paid' AND "auto_renew_enabled" AND "status_code" IN ('active', 'payment_pending');

-- Purpose: Stores Server Accessory Renewal Work Item records.
-- 用途：保存服务器附属资源续费工作明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_renewal_work_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "work_item_no" VARCHAR(40) NOT NULL,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "required_permission" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'open',
    "reason_code" VARCHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL,
    "resolved_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_idc_server_accessory_renewal_work_item_status CHECK (
        ("status_code" = 'open' AND "resolved_at" IS NULL)
        OR ("status_code" = 'resolved' AND "resolved_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_server_accessory_renewal_work_item_no UNIQUE ("work_item_no"),
    CONSTRAINT uk_idc_server_accessory_renewal_work_item_invoice UNIQUE ("invoice_no"),
    CONSTRAINT fk_idc_server_accessory_renewal_work_item_owner FOREIGN KEY ("entitlement_no")
        REFERENCES plugin_idc_core_server_accessory_entitlement ("entitlement_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_accessory_renewal_work_item_invoice FOREIGN KEY ("invoice_no")
        REFERENCES plugin_idc_core_billing_invoice ("invoice_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_accessory_renewal_work_queue
    ON plugin_idc_core_server_accessory_renewal_work_item
        ("tenant_id", "required_permission", "status_code", "created_at", "id");

-- Purpose: Stores Server Accessory Entitlement Event records.
-- 用途：保存服务器附属资源权益事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_accessory_entitlement_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "entitlement_no" VARCHAR(40) NOT NULL,
    "event_type_code" VARCHAR(40) NOT NULL,
    "from_status_code" VARCHAR(24),
    "to_status_code" VARCHAR(24) NOT NULL,
    "operation_no" VARCHAR(40),
    "invoice_no" VARCHAR(40),
    "evidence_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT uk_idc_server_accessory_entitlement_event_no UNIQUE ("event_no"),
    CONSTRAINT fk_idc_server_accessory_entitlement_event_owner FOREIGN KEY ("entitlement_no")
        REFERENCES plugin_idc_core_server_accessory_entitlement ("entitlement_no") ON DELETE RESTRICT
);

COMMENT ON TABLE plugin_idc_core_server_accessory_entitlement
    IS 'Local free or paid ownership and renewal state for one Provider snapshot or backup object';
COMMENT ON COLUMN plugin_idc_core_server_accessory_entitlement."object_no"
    IS 'Resource-owner opaque object number; Provider identifiers never cross this boundary';
