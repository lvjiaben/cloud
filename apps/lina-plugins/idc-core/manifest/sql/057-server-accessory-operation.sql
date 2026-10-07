-- 057: Extend the core-owned server-operation fence for server accessories.
-- Traffic suspension provenance is kept separately so an unrelated suspension
-- can never authorize a traffic-package write.

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_execution;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_execution CHECK (
        "execution_mode_code" IN (
            'unresolved', 'provider_confirmed', 'local_entitlement', 'manual_review'
        )
        AND ("execution_mode_code" <> 'local_entitlement'
            OR "operation_type_code" = 'renewal')
        AND ("execution_mode_code" <> 'unresolved'
            OR "operation_type_code" IN (
                'new_purchase', 'addon', 'start', 'stop', 'reboot',
                'reset_password', 'console', 'reinstall', 'resize', 'ip_change'
            ))
        AND (
            ("execution_mode_code" = 'unresolved'
                AND "channel_no" IS NULL
                AND "provider_task_no" IS NULL
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL)
            OR ("execution_mode_code" = 'manual_review'
                AND "channel_no" IS NULL
                AND "capability_evidence_no" IS NULL
                AND "capability_evidence_digest" IS NULL)
            OR ("execution_mode_code" IN ('provider_confirmed', 'local_entitlement')
                AND "channel_no" IS NOT NULL
                AND "capability_evidence_no" IS NOT NULL
                AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
                AND "capability_evidence_digest" IS NOT NULL
                AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$')
        )
        AND ("provider_task_no" IS NULL
            OR "execution_mode_code" = 'provider_confirmed')
    );

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_reference;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_reference CHECK (
        ("operation_type_code" IN ('new_purchase', 'renewal', 'change')
            AND "price_snapshot_no" IS NOT NULL)
        OR ("operation_type_code" IN ('addon', 'ip_change', 'resize')
            AND (("financial_settlement_required" AND "price_snapshot_no" IS NOT NULL)
                OR (NOT "financial_settlement_required" AND "price_snapshot_no" IS NULL)))
        OR ("operation_type_code" IN (
                'refund', 'suspend', 'resume', 'terminate', 'start', 'stop',
                'reboot', 'reset_password', 'console', 'reinstall'
            ) AND "price_snapshot_no" IS NULL)
    );

ALTER TABLE plugin_idc_core_server_operation_execution
    DROP CONSTRAINT IF EXISTS ck_idc_server_operation_exec_action;
ALTER TABLE plugin_idc_core_server_operation_execution
    ADD CONSTRAINT ck_idc_server_operation_exec_action CHECK (
        "action_code" IN (
            'start', 'stop', 'reboot', 'reset_password', 'console',
            'reinstall', 'resize', 'ip_change',
            'snapshot_create', 'snapshot_delete', 'snapshot_restore',
            'backup_create', 'backup_delete', 'backup_restore',
            'security_group_attach', 'security_group_detach',
            'nat_create', 'nat_delete',
            'virtual_media_mount', 'virtual_media_unmount', 'traffic_add'
        )
    );

ALTER TABLE plugin_idc_core_server_operation_execution
    DROP CONSTRAINT IF EXISTS ck_idc_server_operation_exec_original_runtime;
ALTER TABLE plugin_idc_core_server_operation_execution
    ADD CONSTRAINT ck_idc_server_operation_exec_original_runtime CHECK (
        "original_runtime_status_code" IN ('running', 'stopped', 'suspended')
    );

-- Purpose: Positive local proof that the current suspended runtime state was caused by traffic enforcement.
-- 用途：保存服务流量暂停相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_traffic_suspension (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "service_no" VARCHAR(40) NOT NULL,
    "source_operation_no" VARCHAR(40) NOT NULL,
    "active" BOOLEAN NOT NULL DEFAULT TRUE,
    "suspended_at" TIMESTAMPTZ NOT NULL,
    "cleared_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_traffic_suspension_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_traffic_suspension_state CHECK (
        "version" > 0
        AND length(btrim("source_operation_no")) BETWEEN 1 AND 40
        AND (("active" AND "cleared_at" IS NULL)
            OR (NOT "active" AND "cleared_at" IS NOT NULL
                AND "cleared_at" >= "suspended_at"))
    ),
    CONSTRAINT uk_idc_service_traffic_suspension_no UNIQUE (
        "service_no", "source_operation_no"
    ),
    CONSTRAINT fk_idc_service_traffic_suspension_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no")
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_service_traffic_suspension_active
    ON plugin_idc_core_service_traffic_suspension ("tenant_id", "service_no")
    WHERE "active";

COMMENT ON TABLE plugin_idc_core_service_traffic_suspension
    IS 'Positive local proof that the current suspended runtime state was caused by traffic enforcement';
