-- 032: Immutable service lifecycle policies and task-only expiry scheduling.

ALTER TABLE plugin_idc_core_service_instance
    ADD COLUMN IF NOT EXISTS "current_lifecycle_policy_no" VARCHAR(40);
ALTER TABLE plugin_idc_core_service_instance
    ADD COLUMN IF NOT EXISTS "current_lifecycle_policy_version" BIGINT;

-- Purpose: Append-only per-service expiry, grace, suspension, and termination policy revision.
-- 用途：保存服务生命周期策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_lifecycle_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "reminder_count" INT NOT NULL DEFAULT 0,
    "grace_seconds" BIGINT NOT NULL DEFAULT 0,
    "suspend_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
    "suspend_after_seconds" BIGINT,
    "terminate_enabled" BOOLEAN NOT NULL DEFAULT FALSE,
    "terminate_after_seconds" BIGINT,
    "configured_by" INT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_lifecycle_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_lifecycle_policy_values CHECK (
        "policy_version" > 0
        AND "reminder_count" BETWEEN 0 AND 16
        AND "grace_seconds" BETWEEN 0 AND 2678400
        AND "configured_by" > 0
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND ((NOT "suspend_enabled" AND "suspend_after_seconds" IS NULL)
            OR ("suspend_enabled" AND "suspend_after_seconds" BETWEEN "grace_seconds" AND 31536000))
        AND ((NOT "terminate_enabled" AND "terminate_after_seconds" IS NULL)
            OR ("terminate_enabled" AND "terminate_after_seconds" BETWEEN "grace_seconds" AND 31536000))
        AND (NOT "suspend_enabled" OR NOT "terminate_enabled"
            OR "terminate_after_seconds" > "suspend_after_seconds")
    ),
    CONSTRAINT uk_idc_service_lifecycle_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_service_lifecycle_policy_no_version UNIQUE (
        "policy_no", "policy_version"
    ),
    CONSTRAINT uk_idc_service_lifecycle_policy_owner_version UNIQUE (
        "policy_no", "policy_version", "service_no"
    ),
    CONSTRAINT uk_idc_service_lifecycle_policy_service_version UNIQUE (
        "service_no", "policy_version"
    ),
    CONSTRAINT uk_idc_service_lifecycle_policy_service_no UNIQUE (
        "policy_no", "service_no"
    ),
    CONSTRAINT fk_idc_service_lifecycle_policy_service FOREIGN KEY ("service_no")
        REFERENCES plugin_idc_core_service_instance ("service_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_policy_current
    ON plugin_idc_core_service_lifecycle_policy (
        "tenant_id", "service_no", "policy_version" DESC, "id" DESC
    );

-- Purpose: Append-only normalized reminder time relative to one frozen entitlement expiry.
-- 用途：保存服务生命周期提醒规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_lifecycle_reminder_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reminder_rule_no" VARCHAR(40) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "reminder_sequence" INT NOT NULL,
    "relative_seconds" BIGINT NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_lifecycle_reminder_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_lifecycle_reminder_values CHECK (
        "reminder_sequence" BETWEEN 1 AND 16
        AND "relative_seconds" BETWEEN -31536000 AND 31536000
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_lifecycle_reminder_no UNIQUE ("reminder_rule_no"),
    CONSTRAINT uk_idc_service_lifecycle_reminder_sequence UNIQUE (
        "policy_no", "policy_version", "reminder_sequence"
    ),
    CONSTRAINT uk_idc_service_lifecycle_reminder_offset UNIQUE (
        "policy_no", "policy_version", "relative_seconds"
    ),
    CONSTRAINT fk_idc_service_lifecycle_reminder_policy FOREIGN KEY (
        "policy_no", "policy_version", "service_no"
    ) REFERENCES plugin_idc_core_service_lifecycle_policy (
        "policy_no", "policy_version", "service_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_reminder_order
    ON plugin_idc_core_service_lifecycle_reminder_rule (
        "tenant_id", "policy_no", "policy_version", "relative_seconds", "id"
    );

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_service_lifecycle_policy_pointer'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT ck_idc_service_lifecycle_policy_pointer CHECK (
                ("current_lifecycle_policy_no" IS NULL
                    AND "current_lifecycle_policy_version" IS NULL)
                OR ("current_lifecycle_policy_no" IS NOT NULL
                    AND "current_lifecycle_policy_version" > 0)
            );
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'fk_idc_service_lifecycle_policy_pointer'
          AND conrelid = 'plugin_idc_core_service_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_instance
            ADD CONSTRAINT fk_idc_service_lifecycle_policy_pointer FOREIGN KEY (
                "current_lifecycle_policy_no", "current_lifecycle_policy_version", "service_no"
            ) REFERENCES plugin_idc_core_service_lifecycle_policy (
                "policy_no", "policy_version", "service_no"
            ) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED;
    END IF;
END;
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_schedule
    ON plugin_idc_core_service_instance (
        "tenant_id", "entitlement_expires_at", "id"
    ) WHERE "current_lifecycle_policy_no" IS NOT NULL
      AND "entitlement_status_code" IN ('active', 'grace', 'expired');

-- Purpose: Version-fenced task-only expiry schedule; scheduler never overwrites service state.
-- 用途：保存服务生命周期任务相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_lifecycle_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "reminder_sequence" INT,
    "expected_service_version" BIGINT NOT NULL,
    "expected_entitlement_expires_at" TIMESTAMPTZ NOT NULL,
    "due_at" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "business_operation_no" VARCHAR(40),
    "previous_runtime_status_code" VARCHAR(24),
    "last_error_code" VARCHAR(128) NOT NULL DEFAULT '',
    "idempotency_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "claimed_at" TIMESTAMPTZ,
    "dispatched_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_lifecycle_task_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_lifecycle_task_action CHECK (
        "action_code" IN (
            'reminder', 'enter_grace', 'mark_expired',
            'suspend', 'resume', 'terminate'
        )
        AND (("action_code" = 'reminder' AND "reminder_sequence" BETWEEN 1 AND 16)
            OR ("action_code" <> 'reminder' AND "reminder_sequence" IS NULL))
    ),
    CONSTRAINT ck_idc_service_lifecycle_task_status CHECK (
        "status_code" IN (
            'pending', 'claimed', 'dispatched', 'succeeded',
            'failed', 'cancelled', 'manual_review'
        )
    ),
    CONSTRAINT ck_idc_service_lifecycle_task_values CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
        AND "expected_service_version" > 0 AND "policy_version" > 0
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
        AND "version" > 0
        AND ("last_error_code" = ''
            OR "last_error_code" ~ '^[a-z][a-z0-9_.-]{0,127}$')
    ),
    CONSTRAINT ck_idc_service_lifecycle_task_previous_runtime CHECK (
        "previous_runtime_status_code" IS NULL
        OR "previous_runtime_status_code" IN (
            'pending_provision', 'provisioning', 'running', 'stopped',
            'suspended', 'resuming', 'changing', 'terminating',
            'terminated', 'unknown'
        )
    ),
    CONSTRAINT ck_idc_service_lifecycle_task_timestamps CHECK (
        ("claimed_at" IS NULL OR "claimed_at" >= "created_at")
        AND ("dispatched_at" IS NULL OR "dispatched_at" >= COALESCE("claimed_at", "created_at"))
        AND ("completed_at" IS NULL OR "completed_at" >= "created_at")
        AND ("status_code" NOT IN ('succeeded', 'failed', 'cancelled', 'manual_review')
            OR "completed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_service_lifecycle_task_no UNIQUE ("task_no"),
    CONSTRAINT uk_idc_service_lifecycle_task_service_no UNIQUE (
        "task_no", "service_no"
    ),
    CONSTRAINT uk_idc_service_lifecycle_task_digest UNIQUE (
        "service_no", "action_code", "idempotency_digest"
    ),
    CONSTRAINT uk_idc_service_lifecycle_task_owner UNIQUE (
        "task_no", "service_no", "customer_no", "currency_code"
    ),
    CONSTRAINT fk_idc_service_lifecycle_task_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_lifecycle_task_policy FOREIGN KEY (
        "policy_no", "policy_version", "service_no"
    ) REFERENCES plugin_idc_core_service_lifecycle_policy (
        "policy_no", "policy_version", "service_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_service_lifecycle_task_operation FOREIGN KEY (
        "business_operation_no", "service_no"
    ) REFERENCES plugin_idc_core_business_operation (
        "operation_no", "service_no"
    ) ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_service_lifecycle_task
    ADD COLUMN IF NOT EXISTS "previous_runtime_status_code" VARCHAR(24);

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_service_lifecycle_task_previous_runtime'
          AND conrelid = 'plugin_idc_core_service_lifecycle_task'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_service_lifecycle_task
            ADD CONSTRAINT ck_idc_service_lifecycle_task_previous_runtime CHECK (
                "previous_runtime_status_code" IS NULL
                OR "previous_runtime_status_code" IN (
                    'pending_provision', 'provisioning', 'running', 'stopped',
                    'suspended', 'resuming', 'changing', 'terminating',
                    'terminated', 'unknown'
                )
            );
    END IF;
END;
$constraint$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_service_lifecycle_task_identity
    ON plugin_idc_core_service_lifecycle_task (
        "service_no", "policy_no", "policy_version",
        "expected_entitlement_expires_at", "action_code",
        COALESCE("reminder_sequence", 0)
    );

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_task_work
    ON plugin_idc_core_service_lifecycle_task (
        "tenant_id", "status_code", "due_at", "id"
    ) WHERE "status_code" IN ('pending', 'claimed', 'dispatched');

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_task_service_history
    ON plugin_idc_core_service_lifecycle_task (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Append-only service lifecycle task transition evidence.
-- 用途：保存服务生命周期任务事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_service_lifecycle_task_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "task_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "event_sequence" INT NOT NULL,
    "from_status_code" VARCHAR(24),
    "to_status_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(128) NOT NULL,
    "actor_type_code" VARCHAR(16) NOT NULL,
    "actor_reference" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "content_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_service_lifecycle_event_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_service_lifecycle_event_values CHECK (
        "event_sequence" BETWEEN 1 AND 1000000
        AND ("from_status_code" IS NULL OR "from_status_code" IN (
            'pending', 'claimed', 'dispatched', 'succeeded',
            'failed', 'cancelled', 'manual_review'
        ))
        AND "to_status_code" IN (
            'pending', 'claimed', 'dispatched', 'succeeded',
            'failed', 'cancelled', 'manual_review'
        )
        AND "reason_code" ~ '^[a-z][a-z0-9_.-]{0,127}$'
        AND "actor_type_code" IN ('system', 'staff')
        AND length(btrim("actor_reference")) BETWEEN 1 AND 64
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "content_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_service_lifecycle_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_idc_service_lifecycle_event_sequence UNIQUE (
        "task_no", "event_sequence"
    ),
    CONSTRAINT fk_idc_service_lifecycle_event_task FOREIGN KEY (
        "task_no", "service_no"
    ) REFERENCES plugin_idc_core_service_lifecycle_task (
        "task_no", "service_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_service_lifecycle_event_history
    ON plugin_idc_core_service_lifecycle_task_event (
        "tenant_id", "task_no", "event_sequence", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_lifecycle_reminders()
RETURNS TRIGGER AS $$
DECLARE
    affected_policy_no VARCHAR(40);
    affected_policy_version BIGINT;
    expected_count INT;
    actual_count INT;
BEGIN
    affected_policy_no := COALESCE(NEW."policy_no", OLD."policy_no");
    affected_policy_version := COALESCE(NEW."policy_version", OLD."policy_version");
    SELECT "reminder_count" INTO expected_count
    FROM plugin_idc_core_service_lifecycle_policy
    WHERE "policy_no" = affected_policy_no AND "policy_version" = affected_policy_version;
    IF expected_count IS NULL THEN
        RETURN NULL;
    END IF;
    SELECT COUNT(*)::INT INTO actual_count
    FROM plugin_idc_core_service_lifecycle_reminder_rule
    WHERE "policy_no" = affected_policy_no AND "policy_version" = affected_policy_version;
    IF actual_count <> expected_count THEN
        RAISE EXCEPTION 'service lifecycle reminder count mismatch';
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_lifecycle_task()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW."task_no" IS DISTINCT FROM OLD."task_no"
        OR NEW."service_no" IS DISTINCT FROM OLD."service_no"
        OR NEW."customer_no" IS DISTINCT FROM OLD."customer_no"
        OR NEW."currency_code" IS DISTINCT FROM OLD."currency_code"
        OR NEW."policy_no" IS DISTINCT FROM OLD."policy_no"
        OR NEW."policy_version" IS DISTINCT FROM OLD."policy_version"
        OR NEW."action_code" IS DISTINCT FROM OLD."action_code"
        OR NEW."reminder_sequence" IS DISTINCT FROM OLD."reminder_sequence"
        OR NEW."expected_service_version" IS DISTINCT FROM OLD."expected_service_version"
        OR NEW."expected_entitlement_expires_at" IS DISTINCT FROM OLD."expected_entitlement_expires_at"
        OR NEW."due_at" IS DISTINCT FROM OLD."due_at"
        OR NEW."idempotency_digest" IS DISTINCT FROM OLD."idempotency_digest"
        OR NEW."content_digest" IS DISTINCT FROM OLD."content_digest"
        OR NEW."created_at" IS DISTINCT FROM OLD."created_at" THEN
        RAISE EXCEPTION 'service lifecycle task identity is immutable';
    END IF;
    IF OLD."previous_runtime_status_code" IS NOT NULL
        AND NEW."previous_runtime_status_code" IS DISTINCT FROM OLD."previous_runtime_status_code" THEN
        RAISE EXCEPTION 'service lifecycle task previous runtime state is immutable once frozen';
    END IF;
    IF NEW."version" <> OLD."version" + 1 THEN
        RAISE EXCEPTION 'service lifecycle task version must advance by one';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_service_lifecycle_policy_immutable
    ON plugin_idc_core_service_lifecycle_policy;
CREATE TRIGGER trg_idc_service_lifecycle_policy_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_lifecycle_policy
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_lifecycle_reminder_immutable
    ON plugin_idc_core_service_lifecycle_reminder_rule;
CREATE TRIGGER trg_idc_service_lifecycle_reminder_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_lifecycle_reminder_rule
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_lifecycle_event_immutable
    ON plugin_idc_core_service_lifecycle_task_event;
CREATE TRIGGER trg_idc_service_lifecycle_event_immutable
BEFORE UPDATE OR DELETE ON plugin_idc_core_service_lifecycle_task_event
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_reject_service_history_mutation();

DROP TRIGGER IF EXISTS trg_idc_service_lifecycle_task_guard
    ON plugin_idc_core_service_lifecycle_task;
CREATE TRIGGER trg_idc_service_lifecycle_task_guard
BEFORE UPDATE ON plugin_idc_core_service_lifecycle_task
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_lifecycle_task();

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_service_lifecycle_policy'::regclass
           AND conname = 'trg_idc_service_lifecycle_policy_reminder_count'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_service_lifecycle_policy'::regclass
               AND constraint_row.conname = 'trg_idc_service_lifecycle_policy_reminder_count'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_service_lifecycle_policy_reminder_count'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_lifecycle_reminders()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_service_lifecycle_policy_reminder_count catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_service_lifecycle_policy_reminder_count
        AFTER INSERT ON plugin_idc_core_service_lifecycle_policy
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_lifecycle_reminders();
    END IF;
END;
$constraint_trigger$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_service_lifecycle_reminder_rule'::regclass
           AND conname = 'trg_idc_service_lifecycle_rule_reminder_count'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_service_lifecycle_reminder_rule'::regclass
               AND constraint_row.conname = 'trg_idc_service_lifecycle_rule_reminder_count'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_service_lifecycle_rule_reminder_count'
               AND trigger_row.tgfoid = 'plugin_idc_core_validate_lifecycle_reminders()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_service_lifecycle_rule_reminder_count catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_service_lifecycle_rule_reminder_count
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_service_lifecycle_reminder_rule
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_lifecycle_reminders();
    END IF;
END;
$constraint_trigger$;

INSERT INTO sys_dict_type
    ("tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override",
     "remark", "created_at", "updated_at")
VALUES
    (0, 'IDC 服务生命周期任务动作', 'idc_service_lifecycle_action', 1, 1, FALSE,
     '到期提醒、宽限、暂停、恢复和终止调度动作', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 服务生命周期任务状态', 'idc_service_lifecycle_task_status', 1, 1, FALSE,
     '服务生命周期任务处理状态', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id", "dict_type", "label", "value", "sort", "tag_style",
     "status", "is_builtin", "created_at", "updated_at")
VALUES
    (0, 'idc_service_lifecycle_action', '到期提醒', 'reminder', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_action', '进入宽限', 'enter_grace', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_action', '标记到期', 'mark_expired', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_action', '暂停服务', 'suspend', 4, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_action', '恢复服务', 'resume', 5, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_action', '终止服务', 'terminate', 6, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '等待处理', 'pending', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '已领取', 'claimed', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '已派发', 'dispatched', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '成功', 'succeeded', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '失败', 'failed', 5, 'error', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '已取消', 'cancelled', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_service_lifecycle_task_status', '人工处理', 'manual_review', 7, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;

COMMENT ON TABLE plugin_idc_core_service_lifecycle_policy
    IS 'Append-only per-service expiry, grace, suspension, and termination policy revision';
COMMENT ON TABLE plugin_idc_core_service_lifecycle_reminder_rule
    IS 'Append-only normalized reminder time relative to one frozen entitlement expiry';
COMMENT ON TABLE plugin_idc_core_service_lifecycle_task
    IS 'Version-fenced task-only expiry schedule; scheduler never overwrites service state';
COMMENT ON TABLE plugin_idc_core_service_lifecycle_task_event
    IS 'Append-only service lifecycle task transition evidence';
