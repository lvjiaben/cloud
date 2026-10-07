-- 056: Add core-owned, secret-free execution intent and convergence state for
-- customer-visible server operations. Provider identities remain resource-owned.

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_type;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_type CHECK (
        "operation_type_code" IN (
            'new_purchase', 'renewal', 'change', 'ip_change', 'addon',
            'refund', 'suspend', 'resume', 'terminate', 'start', 'stop',
            'reboot', 'reset_password', 'console', 'reinstall', 'resize'
        ) AND "currency_code" ~ '^[A-Z]{3}$'
    );

ALTER TABLE plugin_idc_core_business_operation
    DROP CONSTRAINT IF EXISTS ck_idc_business_operation_financial;
ALTER TABLE plugin_idc_core_business_operation
    ADD CONSTRAINT ck_idc_business_operation_financial CHECK (
        (("financial_settlement_required" AND "operation_type_code" IN (
                'new_purchase', 'renewal', 'change', 'ip_change', 'addon', 'resize'
            )) OR (NOT "financial_settlement_required"))
        AND ("status_code" NOT IN (
                'financially_settled', 'ready', 'executing', 'succeeded',
                'failed', 'compensating', 'refunding'
            ) OR NOT "financial_settlement_required"
            OR "financially_settled_at" IS NOT NULL)
    );

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
                'new_purchase', 'start', 'stop', 'reboot', 'reset_password',
                'console', 'reinstall', 'resize', 'ip_change'
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
        ("operation_type_code" IN ('new_purchase', 'renewal', 'change', 'addon')
            AND "price_snapshot_no" IS NOT NULL)
        OR ("operation_type_code" IN ('ip_change', 'resize')
            AND (("financial_settlement_required" AND "price_snapshot_no" IS NOT NULL)
                OR (NOT "financial_settlement_required" AND "price_snapshot_no" IS NULL)))
        OR ("operation_type_code" IN (
                'refund', 'suspend', 'resume', 'terminate', 'start', 'stop',
                'reboot', 'reset_password', 'console', 'reinstall'
            ) AND "price_snapshot_no" IS NULL)
    );

-- Purpose: Core-owned secret-free server action intent and resource task convergence state.
-- 用途：保存服务器操作执行相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_operation_execution (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "execution_no" VARCHAR(40) NOT NULL,
    "operation_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "resource_owner_code" VARCHAR(64) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "pool_no" VARCHAR(64) NOT NULL,
    "expected_service_version" BIGINT NOT NULL,
    "original_runtime_status_code" VARCHAR(24) NOT NULL,
    "parameters" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "secret_bindings" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'ready',
    "provider_task_no" VARCHAR(64),
    "request_digest" CHAR(64) NOT NULL,
    "capability_evidence_no" VARCHAR(64),
    "capability_evidence_digest" CHAR(64),
    "task_evidence_digest" CHAR(64),
    "last_failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "output_bindings" JSONB NOT NULL DEFAULT '[]'::JSONB,
    "observed_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_operation_exec_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_operation_exec_action CHECK (
        "action_code" IN (
            'start', 'stop', 'reboot', 'reset_password', 'console',
            'reinstall', 'resize', 'ip_change'
        )
    ),
    CONSTRAINT ck_idc_server_operation_exec_original_runtime CHECK (
        "original_runtime_status_code" IN ('running', 'stopped')
    ),
    CONSTRAINT ck_idc_server_operation_exec_owner CHECK (
        "resource_owner_code" = 'idc-resource'
        AND length(btrim("channel_no")) BETWEEN 1 AND 64
        AND length(btrim("pool_no")) BETWEEN 1 AND 64
    ),
    CONSTRAINT ck_idc_server_operation_exec_payload CHECK (
        jsonb_typeof("parameters") = 'object'
        AND octet_length("parameters"::TEXT) <= 16384
        AND jsonb_typeof("secret_bindings") = 'array'
        AND jsonb_array_length("secret_bindings") <= 8
        AND jsonb_typeof("output_bindings") = 'array'
        AND jsonb_array_length("output_bindings") <= 8
    ),
    CONSTRAINT ck_idc_server_operation_exec_status CHECK (
        "status_code" IN (
            'ready', 'pending', 'running', 'succeeded', 'failed',
            'result_unknown', 'manual_review'
        )
    ),
    CONSTRAINT ck_idc_server_operation_exec_task CHECK (
        ("status_code" IN ('ready', 'manual_review')
            AND "provider_task_no" IS NULL
            AND "capability_evidence_no" IS NULL
            AND "capability_evidence_digest" IS NULL
            AND "task_evidence_digest" IS NULL
            AND "observed_at" IS NULL)
        OR ("status_code" <> 'ready'
            AND length(btrim("provider_task_no")) BETWEEN 1 AND 64
            AND length(btrim("capability_evidence_no")) BETWEEN 1 AND 64
            AND "capability_evidence_digest" ~ '^[0-9a-f]{64}$'
            AND "task_evidence_digest" ~ '^[0-9a-f]{64}$'
            AND "observed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_server_operation_exec_result CHECK (
        "expected_service_version" > 0
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND ("last_failure_code" = ''
            OR "last_failure_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
        AND (("status_code" IN ('succeeded', 'failed', 'manual_review')
                AND "completed_at" IS NOT NULL)
            OR ("status_code" NOT IN ('succeeded', 'failed', 'manual_review')
                AND "completed_at" IS NULL))
        AND ("completed_at" IS NULL OR "completed_at" >= "created_at")
        AND "version" > 0
    ),
    CONSTRAINT uk_idc_server_operation_exec_no UNIQUE ("execution_no"),
    CONSTRAINT uk_idc_server_operation_exec_operation UNIQUE ("operation_no"),
    CONSTRAINT uk_idc_server_operation_exec_task UNIQUE ("provider_task_no"),
    CONSTRAINT fk_idc_server_operation_exec_operation FOREIGN KEY (
        "operation_no", "service_no"
    ) REFERENCES plugin_idc_core_business_operation (
        "operation_no", "service_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_operation_exec_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no")
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_server_operation_exec_active_service
    ON plugin_idc_core_server_operation_execution (
        "tenant_id", "service_no"
    ) WHERE "status_code" IN ('ready', 'pending', 'running', 'result_unknown');

CREATE INDEX IF NOT EXISTS idx_idc_server_operation_exec_work
    ON plugin_idc_core_server_operation_execution (
        "tenant_id", "status_code", "updated_at", "id"
    ) WHERE "status_code" IN ('ready', 'pending', 'running', 'result_unknown');

CREATE INDEX IF NOT EXISTS idx_idc_server_operation_exec_history
    ON plugin_idc_core_server_operation_execution (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Merchant-local server operation security policy; high-risk recent login defaults enabled.
-- 用途：保存服务器操作策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_operation_policy (
    "tenant_id" INT PRIMARY KEY DEFAULT 0,
    "require_high_risk_reauth" BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_operation_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_operation_policy_version CHECK ("version" > 0)
);

INSERT INTO plugin_idc_core_server_operation_policy
    ("tenant_id", "require_high_risk_reauth")
VALUES (0, TRUE)
ON CONFLICT ("tenant_id") DO NOTHING;

COMMENT ON TABLE plugin_idc_core_server_operation_policy
    IS 'Merchant-local server operation security policy; high-risk recent login defaults enabled';

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_server_operation_intent()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'server operation executions cannot be deleted';
    END IF;
    IF ROW(
        OLD."tenant_id", OLD."execution_no", OLD."operation_no", OLD."service_no",
        OLD."action_code", OLD."resource_owner_code", OLD."channel_no", OLD."pool_no",
        OLD."expected_service_version", OLD."original_runtime_status_code",
        OLD."parameters", OLD."secret_bindings",
        OLD."request_digest", OLD."created_at"
    ) IS DISTINCT FROM ROW(
        NEW."tenant_id", NEW."execution_no", NEW."operation_no", NEW."service_no",
        NEW."action_code", NEW."resource_owner_code", NEW."channel_no", NEW."pool_no",
        NEW."expected_service_version", NEW."original_runtime_status_code",
        NEW."parameters", NEW."secret_bindings",
        NEW."request_digest", NEW."created_at"
    ) THEN
        RAISE EXCEPTION 'server operation intent is immutable';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_idc_server_operation_intent_immutable
    ON plugin_idc_core_server_operation_execution;
CREATE TRIGGER trg_idc_server_operation_intent_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_server_operation_execution
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_server_operation_intent();

COMMENT ON TABLE plugin_idc_core_server_operation_execution
    IS 'Core-owned secret-free server action intent and resource task convergence state';
COMMENT ON COLUMN plugin_idc_core_server_operation_execution."secret_bindings"
    IS 'Opaque input secret references only; plaintext is forbidden';
COMMENT ON COLUMN plugin_idc_core_server_operation_execution."output_bindings"
    IS 'Opaque confidential result references with one-time or short-lived delivery policy';

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_business_operation_type', '开机', 'start', 10, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '关机', 'stop', 11, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '重启', 'reboot', 12, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '重置密码', 'reset_password', 13, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '控制台', 'console', 14, 'info', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '重装系统', 'reinstall', 15, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_business_operation_type', '调整配置', 'resize', 16, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
