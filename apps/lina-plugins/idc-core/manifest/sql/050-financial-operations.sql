-- Phase 6.15: staff financial-operation conveniences remain plugin-owned and fail closed.

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_contact_billing
    ON plugin_idc_core_customer_contact_role ("customer_no", "role_code")
    WHERE "status_code" = 'active' AND "role_code" = 'billing';

-- Purpose: Stores Financial Delivery Intent records.
-- 用途：保存财务交付意图相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_delivery_intent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "intent_no" VARCHAR(40) NOT NULL,
    "invoice_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "invoice_version" BIGINT NOT NULL,
    "notification_event_no" VARCHAR(40) NOT NULL,
    "requested_by" INT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_delivery_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_delivery_version CHECK ("invoice_version" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_delivery_actor CHECK ("requested_by" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_delivery_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_financial_delivery_no UNIQUE ("intent_no"),
    CONSTRAINT uk_plugin_idc_core_financial_delivery_idempotency UNIQUE ("invoice_no", "idempotency_digest"),
    CONSTRAINT fk_plugin_idc_core_financial_delivery_invoice FOREIGN KEY ("invoice_no")
        REFERENCES plugin_idc_core_billing_invoice ("invoice_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_financial_delivery_contact FOREIGN KEY ("customer_no", "contact_no")
        REFERENCES plugin_idc_core_customer_contact ("customer_no", "contact_no") ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_delivery_invoice
    ON plugin_idc_core_financial_delivery_intent ("tenant_id", "invoice_no", "created_at" DESC, "id" DESC);

-- Purpose: Staff-only append-only order, billing-invoice, and service notes; never exposed to customer read models.
-- 用途：保存财务内部备注相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_internal_note (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "note_no" VARCHAR(40) NOT NULL,
    "target_type_code" VARCHAR(16) NOT NULL,
    "target_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "content" VARCHAR(2000) NOT NULL,
    "corrects_note_no" VARCHAR(40),
    "created_by" INT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_note_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_note_target CHECK ("target_type_code" IN ('order', 'invoice', 'service')),
    CONSTRAINT ck_plugin_idc_core_financial_note_content CHECK (length(btrim("content")) BETWEEN 1 AND 2000),
    CONSTRAINT ck_plugin_idc_core_financial_note_actor CHECK ("created_by" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_note_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_financial_note_no UNIQUE ("note_no"),
    CONSTRAINT uk_plugin_idc_core_financial_note_idempotency UNIQUE ("created_by", "target_type_code", "target_no", "idempotency_digest"),
    CONSTRAINT fk_plugin_idc_core_financial_note_correction FOREIGN KEY ("corrects_note_no")
        REFERENCES plugin_idc_core_financial_internal_note ("note_no") ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_note_target
    ON plugin_idc_core_financial_internal_note ("tenant_id", "target_type_code", "target_no", "created_at" DESC, "id" DESC);

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_financial_note_mutation()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'financial internal notes are append-only';
END;
$$ LANGUAGE plpgsql;

DO $legacy_financial_note_trigger_name$
DECLARE
    legacy_trigger pg_trigger%ROWTYPE;
    current_trigger pg_trigger%ROWTYPE;
BEGIN
    SELECT * INTO legacy_trigger
      FROM pg_trigger
     WHERE tgrelid = 'plugin_idc_core_financial_internal_note'::REGCLASS
       AND tgname = 'trg_plugin_linapro_idc_core_financial_note_immutable';
    IF FOUND THEN
        IF legacy_trigger.tgisinternal
           OR legacy_trigger.tgconstraint <> 0
           OR legacy_trigger.tgfoid <> 'plugin_idc_core_reject_financial_note_mutation()'::REGPROCEDURE THEN
            RAISE EXCEPTION 'legacy financial note trigger catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;

        SELECT * INTO current_trigger
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_financial_internal_note'::REGCLASS
           AND tgname = 'trg_plugin_idc_core_financial_note_immutable';
        IF FOUND AND (
            current_trigger.tgisinternal
            OR current_trigger.tgconstraint <> 0
            OR current_trigger.tgfoid <> 'plugin_idc_core_reject_financial_note_mutation()'::REGPROCEDURE
            OR ROW(current_trigger.tgtype, current_trigger.tgnargs,
                   current_trigger.tgargs, current_trigger.tgattr::TEXT,
                   current_trigger.tgqual::TEXT, current_trigger.tgoldtable,
                   current_trigger.tgnewtable)
               IS DISTINCT FROM
               ROW(legacy_trigger.tgtype, legacy_trigger.tgnargs,
                   legacy_trigger.tgargs, legacy_trigger.tgattr::TEXT,
                   legacy_trigger.tgqual::TEXT, legacy_trigger.tgoldtable,
                   legacy_trigger.tgnewtable)
        ) THEN
            RAISE EXCEPTION 'legacy and current financial note triggers are not equivalent'
                USING ERRCODE = '23514';
        END IF;

        DROP TRIGGER trg_plugin_linapro_idc_core_financial_note_immutable
            ON plugin_idc_core_financial_internal_note;
    END IF;
END;
$legacy_financial_note_trigger_name$;

DROP TRIGGER IF EXISTS trg_plugin_idc_core_financial_note_immutable
    ON plugin_idc_core_financial_internal_note;
CREATE TRIGGER trg_plugin_idc_core_financial_note_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_core_financial_internal_note
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_financial_note_mutation();

-- Purpose: Personal staff financial filters that never grant or snapshot data access.
-- 用途：保存财务已保存筛选相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_saved_filter (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "filter_no" VARCHAR(40) NOT NULL,
    "owner_user_id" INT NOT NULL,
    "view_code" VARCHAR(24) NOT NULL,
    "display_name" VARCHAR(80) NOT NULL,
    "filter_json" JSONB NOT NULL,
    "filter_digest" CHAR(64) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_filter_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_filter_owner CHECK ("owner_user_id" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_filter_view CHECK ("view_code" IN ('invoice', 'transaction', 'adjustment', 'receivable')),
    CONSTRAINT ck_plugin_idc_core_financial_filter_name CHECK (length(btrim("display_name")) BETWEEN 1 AND 80),
    CONSTRAINT ck_plugin_idc_core_financial_filter_json CHECK (jsonb_typeof("filter_json") = 'object'),
    CONSTRAINT ck_plugin_idc_core_financial_filter_digest CHECK ("filter_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_financial_filter_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_financial_filter_no UNIQUE ("filter_no"),
    CONSTRAINT uk_plugin_idc_core_financial_filter_name UNIQUE ("owner_user_id", "view_code", "display_name")
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_filter_owner
    ON plugin_idc_core_financial_saved_filter ("tenant_id", "owner_user_id", "view_code", "updated_at" DESC, "id" DESC);

-- Purpose: Permission-fenced asynchronous XLSX export requests; CSV is intentionally unsupported.
-- 用途：保存财务导出申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_export_request (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "export_type_code" VARCHAR(16) NOT NULL,
    "filter_json" JSONB NOT NULL,
    "filter_digest" CHAR(64) NOT NULL,
    "permission_code" VARCHAR(128) NOT NULL,
    "data_scope_code" INT NOT NULL,
    "requested_by" INT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "authorization_checked_at" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'queued',
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "generated_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_export_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_export_type CHECK ("export_type_code" IN ('invoice', 'transaction', 'adjustment', 'receivable')),
    CONSTRAINT ck_plugin_idc_core_financial_export_filter CHECK (jsonb_typeof("filter_json") = 'object'),
    CONSTRAINT ck_plugin_idc_core_financial_export_scope CHECK ("data_scope_code" IN (1, 2)),
    CONSTRAINT ck_plugin_idc_core_financial_export_actor CHECK ("requested_by" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_export_status CHECK ("status_code" IN ('queued', 'generating', 'ready', 'failed', 'revoked', 'expired')),
    CONSTRAINT ck_plugin_idc_core_financial_export_digest CHECK (
        "filter_digest" ~ '^[0-9a-f]{64}$' AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_financial_export_no UNIQUE ("request_no"),
    CONSTRAINT uk_plugin_idc_core_financial_export_idempotency UNIQUE ("requested_by", "export_type_code", "idempotency_digest")
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_export_owner
    ON plugin_idc_core_financial_export_request ("tenant_id", "requested_by", "created_at" DESC, "id" DESC);

-- Purpose: Stores Financial Export Artifact records.
-- 用途：保存财务导出产物相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_export_artifact (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "object_ref" VARCHAR(512) NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "size_bytes" BIGINT NOT NULL,
    "row_count" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'ready',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_export_artifact_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_export_artifact_digest CHECK ("content_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_financial_export_artifact_size CHECK ("size_bytes" > 0 AND "size_bytes" <= 52428800),
    CONSTRAINT ck_plugin_idc_core_financial_export_artifact_rows CHECK ("row_count" BETWEEN 0 AND 100000),
    CONSTRAINT ck_plugin_idc_core_financial_export_artifact_status CHECK ("status_code" IN ('ready', 'revoked', 'deleted')),
    CONSTRAINT uk_plugin_idc_core_financial_export_artifact_request UNIQUE ("request_no"),
    CONSTRAINT fk_plugin_idc_core_financial_export_artifact_request FOREIGN KEY ("request_no")
        REFERENCES plugin_idc_core_financial_export_request ("request_no") ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_export_artifact_expiry
    ON plugin_idc_core_financial_export_artifact ("tenant_id", "status_code", "expires_at", "id");

-- Purpose: Stores Financial Export Ticket records.
-- 用途：保存财务导出工单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_financial_export_ticket (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "token_digest" CHAR(64) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "requested_by" INT NOT NULL,
    "permission_code" VARCHAR(128) NOT NULL,
    "data_scope_code" INT NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_financial_export_ticket_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_financial_export_ticket_digest CHECK ("token_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_core_financial_export_ticket_actor CHECK ("requested_by" > 0),
    CONSTRAINT ck_plugin_idc_core_financial_export_ticket_scope CHECK ("data_scope_code" IN (1, 2)),
    CONSTRAINT uk_plugin_idc_core_financial_export_ticket_no UNIQUE ("ticket_no"),
    CONSTRAINT uk_plugin_idc_core_financial_export_ticket_token UNIQUE ("token_digest"),
    CONSTRAINT fk_plugin_idc_core_financial_export_ticket_request FOREIGN KEY ("request_no")
        REFERENCES plugin_idc_core_financial_export_request ("request_no") ON DELETE RESTRICT
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_financial_export_ticket_lookup
    ON plugin_idc_core_financial_export_ticket ("tenant_id", "token_digest", "expires_at")
    WHERE "consumed_at" IS NULL;

COMMENT ON TABLE plugin_idc_core_financial_internal_note IS 'Staff-only append-only order, billing-invoice, and service notes; never exposed to customer read models';
COMMENT ON TABLE plugin_idc_core_financial_saved_filter IS 'Personal staff financial filters that never grant or snapshot data access';
COMMENT ON TABLE plugin_idc_core_financial_export_request IS 'Permission-fenced asynchronous XLSX export requests; CSV is intentionally unsupported';
