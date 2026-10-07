-- 038: Track core-owned convergence of one immutable paid service intent with
-- the resource owner's idempotent provisioning task. Provider identities and
-- credentials remain exclusively owned by idc-resource.

-- One reservation can cover multiple per-server fulfillment units. Preserve
-- the exact split when only part of a multi-server order provisions.
ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    ALTER COLUMN "status_code" TYPE VARCHAR(24),
    ADD COLUMN IF NOT EXISTS "consumed_quantity" INT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS "released_quantity" INT NOT NULL DEFAULT 0;

UPDATE plugin_idc_core_commerce_inventory_reservation
SET "consumed_quantity" = CASE
        WHEN "status_code" = 'consumed' THEN "reserved_quantity"
        ELSE "consumed_quantity"
    END,
    "released_quantity" = CASE
        WHEN "status_code" IN ('released', 'expired') THEN "reserved_quantity"
        ELSE "released_quantity"
    END;

ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    DROP CONSTRAINT IF EXISTS ck_idc_com_inventory_res_status,
    DROP CONSTRAINT IF EXISTS ck_idc_com_inventory_res_lifecycle,
    DROP CONSTRAINT IF EXISTS ck_idc_com_inventory_res_settlement;
ALTER TABLE plugin_idc_core_commerce_inventory_reservation
    ADD CONSTRAINT ck_idc_com_inventory_res_status CHECK (
        "status_code" IN (
            'held', 'committed', 'consumed', 'partial', 'released', 'expired'
        )
    ),
    ADD CONSTRAINT ck_idc_com_inventory_res_settlement CHECK (
        "consumed_quantity" BETWEEN 0 AND "reserved_quantity"
        AND "released_quantity" BETWEEN 0 AND "reserved_quantity"
        AND "consumed_quantity" + "released_quantity" <= "reserved_quantity"
    ),
    ADD CONSTRAINT ck_idc_com_inventory_res_lifecycle CHECK (
        ("status_code" = 'held' AND "committed_at" IS NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = 0 AND "released_quantity" = 0)
        OR ("status_code" = 'committed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = 0 AND "released_quantity" = 0)
        OR ("status_code" = 'consumed' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NOT NULL AND "released_at" IS NULL
            AND "release_reason_code" = ''
            AND "consumed_quantity" = "reserved_quantity"
            AND "released_quantity" = 0)
        OR ("status_code" = 'partial' AND "committed_at" IS NOT NULL
            AND "consumed_at" IS NOT NULL AND "released_at" IS NOT NULL
            AND "release_reason_code" = 'fulfillment_failed'
            AND "consumed_quantity" > 0 AND "released_quantity" > 0
            AND "consumed_quantity" + "released_quantity" = "reserved_quantity")
        OR ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL
            AND "released_at" IS NOT NULL
            AND "release_reason_code" IN (
                'order_cancelled', 'payment_expired', 'rerouted',
                'fulfillment_failed', 'manual_release'
            )
            AND "consumed_quantity" = 0
            AND "released_quantity" = "reserved_quantity")
    );

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_commerce_reservation_status', '部分结算', 'partial', 6,
     'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

-- Purpose: Core-owned secret-free convergence record for a resource-owned provisioning task.
-- 用途：保存服务开通执行相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_provision_execution (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "execution_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "resource_owner_code" VARCHAR(64) NOT NULL,
    "provider_task_no" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "capability_evidence_no" VARCHAR(64) NOT NULL,
    "capability_evidence_digest" CHAR(64) NOT NULL,
    "task_evidence_digest" CHAR(64) NOT NULL,
    "last_failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "outcome_report_status" VARCHAR(16) NOT NULL DEFAULT 'not_ready',
    "outcome_reported_at" TIMESTAMPTZ,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_provision_exec_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_provision_exec_owner CHECK (
        "resource_owner_code" = 'idc-resource'
        AND length(btrim("provider_task_no")) BETWEEN 1 AND 64
        AND length(btrim("channel_no")) BETWEEN 1 AND 64
    ),
    CONSTRAINT ck_idc_service_provision_exec_status CHECK (
        "status_code" IN (
            'pending', 'running', 'succeeded', 'failed',
            'result_unknown', 'manual_review'
        )
    ),
    CONSTRAINT ck_idc_service_provision_exec_evidence CHECK (
        length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "task_evidence_digest" ~ '^[0-9a-f]{64}$'
        AND (
            ("status_code" IN ('failed', 'result_unknown', 'manual_review')
                AND "last_failure_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
            OR ("status_code" IN ('pending', 'running', 'succeeded')
                AND "last_failure_code" = '')
        )
    ),
    CONSTRAINT ck_idc_service_provision_exec_lifecycle CHECK (
        ("status_code" IN ('pending', 'running', 'result_unknown')
            AND "completed_at" IS NULL)
        OR ("status_code" IN ('succeeded', 'failed', 'manual_review')
            AND "completed_at" IS NOT NULL
            AND "completed_at" >= "observed_at")
    ),
    CONSTRAINT ck_idc_service_provision_exec_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_service_provision_exec_report CHECK (
        ("outcome_report_status" IN ('not_ready', 'pending', 'not_required')
            AND "outcome_reported_at" IS NULL)
        OR ("outcome_report_status" = 'reported'
            AND "outcome_reported_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_service_provision_exec_no UNIQUE ("execution_no"),
    CONSTRAINT uk_idc_service_provision_exec_operation UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_service_provision_exec_service UNIQUE ("service_no"),
    CONSTRAINT uk_idc_service_provision_exec_task UNIQUE ("provider_task_no"),
    CONSTRAINT fk_idc_service_provision_exec_operation FOREIGN KEY (
        "operation_no", "service_no"
    ) REFERENCES plugin_idc_core_business_operation (
        "operation_no", "service_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_provision_exec_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no")
        ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_service_provision_execution
    ADD COLUMN IF NOT EXISTS "outcome_report_status" VARCHAR(16)
        NOT NULL DEFAULT 'not_ready',
    ADD COLUMN IF NOT EXISTS "outcome_reported_at" TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_service_provision_execution
    DROP CONSTRAINT IF EXISTS ck_idc_service_provision_exec_report;
ALTER TABLE plugin_idc_core_service_provision_execution
    ADD CONSTRAINT ck_idc_service_provision_exec_report CHECK (
        ("outcome_report_status" IN ('not_ready', 'pending', 'not_required')
            AND "outcome_reported_at" IS NULL)
        OR ("outcome_report_status" = 'reported'
            AND "outcome_reported_at" IS NOT NULL)
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_provision_exec_work
    ON plugin_idc_core_service_provision_execution (
        "tenant_id", "status_code", "updated_at", "id"
    ) WHERE "status_code" IN ('pending', 'running', 'result_unknown');

CREATE INDEX IF NOT EXISTS idx_idc_service_provision_exec_history
    ON plugin_idc_core_service_provision_execution (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_provision_exec_report
    ON plugin_idc_core_service_provision_execution (
        "tenant_id", "outcome_report_status", "updated_at", "id"
    ) WHERE "outcome_report_status" = 'pending';

COMMENT ON TABLE plugin_idc_core_service_provision_execution
    IS 'Core-owned secret-free convergence record for a resource-owned provisioning task';
COMMENT ON COLUMN plugin_idc_core_service_provision_execution."provider_task_no"
    IS 'Opaque resource-owner task reference; never a Provider task identifier';
