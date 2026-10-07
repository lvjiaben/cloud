-- Private customer data export artifacts and short-lived, session-bound tickets.

-- Purpose: Stores Customer Data Export Artifact records.
-- 用途：保存客户数据导出产物相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_data_export_artifact (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "task_no" VARCHAR(40) NOT NULL,
    "object_ref" VARCHAR(512) NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "size_bytes" BIGINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'ready',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_customer_export_artifact_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_customer_export_artifact_digest CHECK ("content_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_customer_export_artifact_size CHECK ("size_bytes" BETWEEN 1 AND 10485760),
    CONSTRAINT ck_idc_customer_export_artifact_status CHECK ("status_code" IN ('ready','expired','deleted')),
    CONSTRAINT ck_idc_customer_export_artifact_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT uk_idc_customer_export_artifact_request UNIQUE ("request_no"),
    CONSTRAINT uk_idc_customer_export_artifact_task UNIQUE ("task_no"),
    CONSTRAINT fk_idc_customer_export_artifact_request FOREIGN KEY ("request_no")
        REFERENCES plugin_idc_core_customer_data_export_request ("request_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_customer_export_artifact_cleanup
    ON plugin_idc_core_customer_data_export_artifact ("tenant_id","status_code","expires_at","id")
    WHERE "status_code" = 'ready';

-- Purpose: Stores Customer Data Export Ticket records.
-- 用途：保存客户数据导出工单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_data_export_ticket (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "token_digest" CHAR(64) NOT NULL,
    "request_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "user_id" INT NOT NULL,
    "session_digest" CHAR(64) NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_customer_export_ticket_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_customer_export_ticket_values CHECK (
        "ticket_no" ~ '^EDT_[0-9a-f]{32}$' AND "token_digest" ~ '^[0-9a-f]{64}$'
        AND "session_digest" ~ '^[0-9a-f]{64}$' AND "user_id" > 0 AND "expires_at" > "created_at"
    ),
    CONSTRAINT uk_idc_customer_export_ticket_no UNIQUE ("ticket_no"),
    CONSTRAINT uk_idc_customer_export_ticket_token UNIQUE ("token_digest"),
    CONSTRAINT fk_idc_customer_export_ticket_request FOREIGN KEY ("request_no")
        REFERENCES plugin_idc_core_customer_data_export_artifact ("request_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_customer_export_ticket_expiry
    ON plugin_idc_core_customer_data_export_ticket ("tenant_id","expires_at","id")
    WHERE "consumed_at" IS NULL;
