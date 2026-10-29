-- 042: Core-owned support desk and public/customer content facts.
-- All tables are single-installation tenant facts. Replies, notes, files,
-- assignment history, SLA history, and action history are append-only at the
-- application boundary; stable constraints prevent state-name drift.

-- Purpose: Stores Support Department records.
-- 用途：保存支持部门相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_department (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "department_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "parent_department_no" VARCHAR(40),
    "assignment_mode" VARCHAR(32) NOT NULL DEFAULT 'manual_claim',
    "round_robin_cursor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_department_no UNIQUE ("department_no"),
    CONSTRAINT ck_idc_support_department_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_department_assignment CHECK (
        "assignment_mode" IN ('manual_claim', 'round_robin', 'least_unresolved')
    ),
    CONSTRAINT ck_idc_support_department_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_department_version CHECK ("version" > 0),
    CONSTRAINT ck_idc_support_department_parent CHECK (
        "parent_department_no" IS NULL OR "parent_department_no" <> "department_no"
    )
);

CREATE INDEX IF NOT EXISTS idx_idc_support_department_parent
    ON plugin_idc_core_support_department (
        "tenant_id", "parent_department_no", "status_code", "department_no"
    );

-- Purpose: Stores Support Department Member records.
-- 用途：保存支持部门成员相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_department_member (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "department_no" VARCHAR(40) NOT NULL,
    "user_id" BIGINT NOT NULL,
    "role_code" VARCHAR(16) NOT NULL DEFAULT 'agent',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_department_member UNIQUE ("department_no", "user_id"),
    CONSTRAINT ck_idc_support_department_member_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_department_member_user CHECK ("user_id" > 0),
    CONSTRAINT ck_idc_support_department_member_role CHECK ("role_code" IN ('agent', 'leader')),
    CONSTRAINT ck_idc_support_department_member_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_department_member_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_department_member_department FOREIGN KEY ("department_no")
        REFERENCES plugin_idc_core_support_department ("department_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_department_member_user
    ON plugin_idc_core_support_department_member (
        "tenant_id", "user_id", "status_code", "department_no", "role_code"
    );

-- Purpose: Stores Support Issue Category records.
-- 用途：保存支持问题分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_issue_category (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "category_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "department_no" VARCHAR(40),
    "priority_code" VARCHAR(16) NOT NULL DEFAULT 'normal',
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_issue_category_no UNIQUE ("category_no"),
    CONSTRAINT ck_idc_support_issue_category_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_issue_category_priority CHECK (
        "priority_code" IN ('low', 'normal', 'high', 'urgent')
    ),
    CONSTRAINT ck_idc_support_issue_category_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_issue_category_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_issue_category_department FOREIGN KEY ("department_no")
        REFERENCES plugin_idc_core_support_department ("department_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_issue_category_directory
    ON plugin_idc_core_support_issue_category (
        "tenant_id", "status_code", "sort_order", "id"
    );

-- Purpose: Stores Support Custom Status records.
-- 用途：保存支持自定义状态相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_custom_status (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "status_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "base_stage" VARCHAR(32) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_custom_status_no UNIQUE ("status_no"),
    CONSTRAINT ck_idc_support_custom_status_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_custom_status_stage CHECK (
        "base_stage" IN ('new', 'in_progress', 'waiting_customer', 'waiting_upstream', 'resolved', 'closed')
    ),
    CONSTRAINT ck_idc_support_custom_status_state CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_custom_status_version CHECK ("version" > 0)
);

-- Purpose: Stores Support Custom Field Definition records.
-- 用途：保存支持自定义字段定义相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_custom_field_definition (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "field_no" VARCHAR(40) NOT NULL,
    "field_type" VARCHAR(24) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "applies_presales" BOOLEAN NOT NULL DEFAULT FALSE,
    "applies_customer" BOOLEAN NOT NULL DEFAULT FALSE,
    "applies_internal" BOOLEAN NOT NULL DEFAULT FALSE,
    "visibility_code" VARCHAR(24) NOT NULL,
    "required" BOOLEAN NOT NULL DEFAULT FALSE,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_custom_field_no UNIQUE ("field_no"),
    CONSTRAINT ck_idc_support_custom_field_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_custom_field_type CHECK (
        "field_type" IN (
            'single_line', 'multi_line', 'number', 'single_select',
            'multi_select', 'date', 'boolean'
        )
    ),
    CONSTRAINT ck_idc_support_custom_field_names CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_support_custom_field_scope CHECK (
        ("applies_presales" OR "applies_customer" OR "applies_internal") AND
        (("visibility_code" = 'customer_visible' AND
          ("applies_presales" OR "applies_customer") AND NOT "applies_internal") OR
         ("visibility_code" = 'internal_only' AND
          "applies_internal" AND NOT "applies_presales" AND NOT "applies_customer"))
    ),
    CONSTRAINT ck_idc_support_custom_field_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_custom_field_sort CHECK ("sort_order" >= 0),
    CONSTRAINT ck_idc_support_custom_field_version CHECK ("version" > 0)
);

CREATE INDEX IF NOT EXISTS idx_idc_support_custom_field_directory
    ON plugin_idc_core_support_custom_field_definition (
        "tenant_id", "status_code", "visibility_code", "sort_order", "id"
    );

-- Purpose: Stores Support Custom Field Option records.
-- 用途：保存支持自定义字段选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_custom_field_option (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "option_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_custom_field_option_no UNIQUE ("option_no"),
    CONSTRAINT ck_idc_support_custom_field_option_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_custom_field_option_names CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_support_custom_field_option_sort CHECK ("sort_order" >= 0),
    CONSTRAINT ck_idc_support_custom_field_option_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_custom_field_option_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_custom_field_option_definition FOREIGN KEY ("field_no")
        REFERENCES plugin_idc_core_support_custom_field_definition ("field_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_custom_field_option_directory
    ON plugin_idc_core_support_custom_field_option (
        "tenant_id", "field_no", "status_code", "sort_order", "id"
    );

-- Purpose: Stores Support Sla Calendar records.
-- 用途：保存支持服务等级日历相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_sla_calendar (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "calendar_no" VARCHAR(40) NOT NULL,
    "name" VARCHAR(128) NOT NULL,
    "mode_code" VARCHAR(24) NOT NULL DEFAULT 'continuous',
    "timezone_name" VARCHAR(128),
    "workdays" SMALLINT[] NOT NULL DEFAULT ARRAY[1,2,3,4,5]::SMALLINT[],
    "work_start_minute" INT,
    "work_end_minute" INT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_sla_calendar_no UNIQUE ("calendar_no"),
    CONSTRAINT ck_idc_support_sla_calendar_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_sla_calendar_mode CHECK ("mode_code" IN ('continuous', 'business_hours')),
    CONSTRAINT ck_idc_support_sla_calendar_window CHECK (
        ("mode_code" = 'continuous' AND "timezone_name" IS NULL AND "work_start_minute" IS NULL AND "work_end_minute" IS NULL)
        OR
        ("mode_code" = 'business_hours' AND length("timezone_name") > 0 AND
         "work_start_minute" BETWEEN 0 AND 1439 AND "work_end_minute" BETWEEN 1 AND 1440 AND
         "work_start_minute" < "work_end_minute" AND cardinality("workdays") BETWEEN 1 AND 7)
    ),
    CONSTRAINT ck_idc_support_sla_calendar_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_sla_calendar_version CHECK ("version" > 0)
);

-- Purpose: Stores Support Sla Holiday records.
-- 用途：保存支持服务等级节假日相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_sla_holiday (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "calendar_no" VARCHAR(40) NOT NULL,
    "holiday_date" DATE NOT NULL,
    "name" VARCHAR(128) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_sla_holiday UNIQUE ("calendar_no", "holiday_date"),
    CONSTRAINT ck_idc_support_sla_holiday_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT fk_idc_support_sla_holiday_calendar FOREIGN KEY ("calendar_no")
        REFERENCES plugin_idc_core_support_sla_calendar ("calendar_no")
);

-- Purpose: Stores Support Sla Rule records.
-- 用途：保存支持服务等级规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_sla_rule (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "rule_no" VARCHAR(40) NOT NULL,
    "department_no" VARCHAR(40) NOT NULL,
    "category_no" VARCHAR(40),
    "priority_code" VARCHAR(16) NOT NULL,
    "calendar_no" VARCHAR(40) NOT NULL,
    "first_response_minutes" INT,
    "resolution_minutes" INT,
    "near_due_minutes" INT[] NOT NULL DEFAULT ARRAY[]::INT[],
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_sla_rule_no UNIQUE ("rule_no"),
    CONSTRAINT uk_idc_support_sla_rule_scope UNIQUE ("department_no", "category_no", "priority_code"),
    CONSTRAINT ck_idc_support_sla_rule_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_sla_rule_priority CHECK ("priority_code" IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT ck_idc_support_sla_rule_limits CHECK (
        ("first_response_minutes" IS NULL OR "first_response_minutes" > 0) AND
        ("resolution_minutes" IS NULL OR "resolution_minutes" > 0)
    ),
    CONSTRAINT ck_idc_support_sla_rule_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_sla_rule_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_sla_rule_department FOREIGN KEY ("department_no")
        REFERENCES plugin_idc_core_support_department ("department_no"),
    CONSTRAINT fk_idc_support_sla_rule_category FOREIGN KEY ("category_no")
        REFERENCES plugin_idc_core_support_issue_category ("category_no"),
    CONSTRAINT fk_idc_support_sla_rule_calendar FOREIGN KEY ("calendar_no")
        REFERENCES plugin_idc_core_support_sla_calendar ("calendar_no")
);

-- Purpose: Core-owned ticket head with six immutable base stages and bounded current projections.
-- 用途：保存支持工单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "ticket_type" VARCHAR(16) NOT NULL,
    "visibility_code" VARCHAR(24) NOT NULL,
    "customer_no" VARCHAR(40),
    "related_service_no" VARCHAR(40),
    "related_service_version" BIGINT,
    "related_service_name_zh_cn" VARCHAR(256),
    "related_service_name_en_us" VARCHAR(256),
    "related_service_entitlement_status" VARCHAR(24),
    "related_service_runtime_status" VARCHAR(24),
    "anonymous_destination_digest" VARCHAR(64),
    "anonymous_destination_kind" VARCHAR(16),
    "category_no" VARCHAR(40),
    "department_no" VARCHAR(40),
    "assignee_user_id" BIGINT,
    "subject" VARCHAR(256) NOT NULL,
    "base_stage" VARCHAR(32) NOT NULL DEFAULT 'new',
    "custom_status_no" VARCHAR(40),
    "priority_code" VARCHAR(16) NOT NULL DEFAULT 'normal',
    "current_round" INT NOT NULL DEFAULT 1,
    "follow_up_required" BOOLEAN NOT NULL DEFAULT FALSE,
    "sla_risk_code" VARCHAR(16) NOT NULL DEFAULT 'none',
    "resolved_at" TIMESTAMPTZ,
    "closed_at" TIMESTAMPTZ,
    "closed_reason_code" VARCHAR(32),
    "closed_by_type" VARCHAR(16),
    "customer_last_replied_at" TIMESTAMPTZ,
    "last_activity_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_no UNIQUE ("ticket_no"),
    CONSTRAINT ck_idc_support_ticket_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_type CHECK ("ticket_type" IN ('presales', 'customer', 'internal')),
    CONSTRAINT ck_idc_support_ticket_visibility CHECK ("visibility_code" IN ('anonymous', 'customer', 'internal')),
    CONSTRAINT ck_idc_support_ticket_owner CHECK (
        ("ticket_type" = 'presales' AND "visibility_code" = 'anonymous' AND "customer_no" IS NULL AND
         length("anonymous_destination_digest") = 64 AND "anonymous_destination_kind" IN ('email', 'sms'))
        OR
        ("ticket_type" = 'customer' AND "visibility_code" = 'customer' AND length("customer_no") > 0 AND
         "anonymous_destination_digest" IS NULL AND "anonymous_destination_kind" IS NULL)
        OR
        ("ticket_type" = 'internal' AND "visibility_code" = 'internal' AND
         "anonymous_destination_digest" IS NULL AND "anonymous_destination_kind" IS NULL)
    ),
    CONSTRAINT ck_idc_support_ticket_service_link CHECK (
        ("related_service_no" IS NULL AND "related_service_version" IS NULL AND
         "related_service_name_zh_cn" IS NULL AND "related_service_name_en_us" IS NULL AND
         "related_service_entitlement_status" IS NULL AND "related_service_runtime_status" IS NULL)
        OR
        ("ticket_type" = 'customer' AND "customer_no" IS NOT NULL AND
         "related_service_no" ~ '^[A-Za-z0-9_-]{1,40}$' AND "related_service_version" > 0 AND
         length(btrim("related_service_name_zh_cn")) > 0 AND
         length(btrim("related_service_name_en_us")) > 0 AND
         "related_service_entitlement_status" NOT IN ('terminated', 'cancelled') AND
         length("related_service_runtime_status") > 0)
    ),
    CONSTRAINT ck_idc_support_ticket_stage CHECK (
        "base_stage" IN ('new', 'in_progress', 'waiting_customer', 'waiting_upstream', 'resolved', 'closed')
    ),
    CONSTRAINT ck_idc_support_ticket_priority CHECK ("priority_code" IN ('low', 'normal', 'high', 'urgent')),
    CONSTRAINT ck_idc_support_ticket_sla_risk CHECK ("sla_risk_code" IN ('none', 'at_risk', 'breached')),
    CONSTRAINT ck_idc_support_ticket_round CHECK ("current_round" > 0),
    CONSTRAINT ck_idc_support_ticket_assignee CHECK ("assignee_user_id" IS NULL OR "assignee_user_id" > 0),
    CONSTRAINT ck_idc_support_ticket_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_ticket_category FOREIGN KEY ("category_no")
        REFERENCES plugin_idc_core_support_issue_category ("category_no"),
    CONSTRAINT fk_idc_support_ticket_department FOREIGN KEY ("department_no")
        REFERENCES plugin_idc_core_support_department ("department_no"),
    CONSTRAINT fk_idc_support_ticket_custom_status FOREIGN KEY ("custom_status_no")
        REFERENCES plugin_idc_core_support_custom_status ("status_no")
);

ALTER TABLE plugin_idc_core_support_ticket
    ADD COLUMN IF NOT EXISTS "related_service_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "related_service_version" BIGINT,
    ADD COLUMN IF NOT EXISTS "related_service_name_zh_cn" VARCHAR(256),
    ADD COLUMN IF NOT EXISTS "related_service_name_en_us" VARCHAR(256),
    ADD COLUMN IF NOT EXISTS "related_service_entitlement_status" VARCHAR(24),
    ADD COLUMN IF NOT EXISTS "related_service_runtime_status" VARCHAR(24);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_support_ticket'::regclass
          AND conname = 'ck_idc_support_ticket_service_link'
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket
            ADD CONSTRAINT ck_idc_support_ticket_service_link CHECK (
                ("related_service_no" IS NULL AND "related_service_version" IS NULL AND
                 "related_service_name_zh_cn" IS NULL AND "related_service_name_en_us" IS NULL AND
                 "related_service_entitlement_status" IS NULL AND "related_service_runtime_status" IS NULL)
                OR
                ("ticket_type" = 'customer' AND "customer_no" IS NOT NULL AND
                 "related_service_no" ~ '^[A-Za-z0-9_-]{1,40}$' AND "related_service_version" > 0 AND
                 length(btrim("related_service_name_zh_cn")) > 0 AND
                 length(btrim("related_service_name_en_us")) > 0 AND
                 "related_service_entitlement_status" NOT IN ('terminated', 'cancelled') AND
                 length("related_service_runtime_status") > 0)
            );
    END IF;
END $$;

-- The service link intentionally has no foreign key to service_instance. The
-- creation transaction revalidates current ownership and non-terminal status,
-- then the immutable snapshot survives later service archival or deletion.
ALTER TABLE plugin_idc_core_support_ticket
    DROP CONSTRAINT IF EXISTS fk_idc_support_ticket_related_service;

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_queue
    ON plugin_idc_core_support_ticket (
        "tenant_id", "department_no", "assignee_user_id", "base_stage", "priority_code", "last_activity_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_customer
    ON plugin_idc_core_support_ticket (
        "tenant_id", "customer_no", "last_activity_at" DESC, "id" DESC
    ) WHERE "customer_no" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_service
    ON plugin_idc_core_support_ticket (
        "tenant_id", "related_service_no", "last_activity_at" DESC, "id" DESC
    ) WHERE "related_service_no" IS NOT NULL;

CREATE OR REPLACE FUNCTION plugin_idc_core_support_ticket_service_link_immutable()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."related_service_no" IS DISTINCT FROM NEW."related_service_no" OR
       OLD."related_service_version" IS DISTINCT FROM NEW."related_service_version" OR
       OLD."related_service_name_zh_cn" IS DISTINCT FROM NEW."related_service_name_zh_cn" OR
       OLD."related_service_name_en_us" IS DISTINCT FROM NEW."related_service_name_en_us" OR
       OLD."related_service_entitlement_status" IS DISTINCT FROM NEW."related_service_entitlement_status" OR
       OLD."related_service_runtime_status" IS DISTINCT FROM NEW."related_service_runtime_status" THEN
        RAISE EXCEPTION 'support ticket service link is immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_support_ticket_service_link_immutable
    ON plugin_idc_core_support_ticket;
CREATE TRIGGER trg_idc_support_ticket_service_link_immutable
    BEFORE UPDATE ON plugin_idc_core_support_ticket
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_support_ticket_service_link_immutable();

-- Purpose: Stores Support Ticket Custom Field Value records.
-- 用途：保存支持工单自定义字段值相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_custom_field_value (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "definition_version" BIGINT NOT NULL,
    "field_type" VARCHAR(24) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "visibility_code" VARCHAR(24) NOT NULL,
    "required" BOOLEAN NOT NULL,
    "sort_order" INT NOT NULL,
    "normalized_value" TEXT NOT NULL,
    "selected_option_count" SMALLINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_custom_field UNIQUE ("ticket_no", "field_no"),
    CONSTRAINT ck_idc_support_ticket_custom_field_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_custom_field_type CHECK (
        "field_type" IN (
            'single_line', 'multi_line', 'number', 'single_select',
            'multi_select', 'date', 'boolean'
        )
    ),
    CONSTRAINT ck_idc_support_ticket_custom_field_visibility CHECK (
        "visibility_code" IN ('customer_visible', 'internal_only')
    ),
    CONSTRAINT ck_idc_support_ticket_custom_field_value CHECK (
        octet_length("normalized_value") BETWEEN 1 AND 65536 AND
        (("field_type" IN ('single_select', 'multi_select') AND "selected_option_count" > 0) OR
         ("field_type" NOT IN ('single_select', 'multi_select') AND "selected_option_count" = 0))
    ),
    CONSTRAINT ck_idc_support_ticket_custom_field_snapshot CHECK (
        "definition_version" > 0 AND "sort_order" >= 0 AND
        length(btrim("name_zh_cn")) > 0 AND length(btrim("name_en_us")) > 0
    ),
    CONSTRAINT fk_idc_support_ticket_custom_field_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no"),
    CONSTRAINT fk_idc_support_ticket_custom_field_definition FOREIGN KEY ("field_no")
        REFERENCES plugin_idc_core_support_custom_field_definition ("field_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_custom_field_read
    ON plugin_idc_core_support_ticket_custom_field_value (
        "tenant_id", "ticket_no", "visibility_code", "sort_order", "id"
    );

-- Preserve the custom-field option snapshots created under the formerly
-- overlong physical identifier. PostgreSQL truncated that table name before
-- the one-time debranding migration, so rename the exact verified relation
-- instead of creating a second table beside the historical data.
DO $support_custom_field_option_table$
DECLARE
    legacy_table REGCLASS := to_regclass('plugin_idc_core_support_ticket_custom_field_value_optio');
    current_table REGCLASS := to_regclass('plugin_idc_core_support_ticket_custom_field_value_option');
    expected_columns TEXT[] := ARRAY[
        'id', 'tenant_id', 'ticket_no', 'field_no', 'option_no',
        'name_zh_cn', 'name_en_us', 'sort_order', 'created_at'
    ];
    observed_columns TEXT[];
BEGIN
    IF legacy_table IS NOT NULL AND current_table IS NOT NULL THEN
        RAISE EXCEPTION 'support custom field option table rename is ambiguous'
            USING ERRCODE = '23514';
    END IF;

    IF legacy_table IS NOT NULL THEN
        SELECT array_agg(attribute_row.attname ORDER BY attribute_row.attnum)
          INTO observed_columns
          FROM pg_attribute AS attribute_row
         WHERE attribute_row.attrelid = legacy_table
           AND attribute_row.attnum > 0
           AND NOT attribute_row.attisdropped;
        IF observed_columns IS DISTINCT FROM expected_columns THEN
            RAISE EXCEPTION 'support custom field option legacy table contract is invalid'
                USING ERRCODE = '23514';
        END IF;
        ALTER TABLE plugin_idc_core_support_ticket_custom_field_value_optio
            RENAME TO plugin_idc_core_support_ticket_custom_field_value_option;
    END IF;
END;
$support_custom_field_option_table$;

-- Purpose: Stores Support Ticket Custom Field Value Option records.
-- 用途：保存支持工单自定义字段值选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_custom_field_value_option (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "option_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_custom_field_option UNIQUE ("ticket_no", "field_no", "option_no"),
    CONSTRAINT ck_idc_support_ticket_custom_field_option_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_custom_field_option_snapshot CHECK (
        "sort_order" >= 0 AND length(btrim("name_zh_cn")) > 0 AND length(btrim("name_en_us")) > 0
    ),
    CONSTRAINT fk_idc_support_ticket_custom_field_option_value FOREIGN KEY ("ticket_no", "field_no")
        REFERENCES plugin_idc_core_support_ticket_custom_field_value ("ticket_no", "field_no"),
    CONSTRAINT fk_idc_support_ticket_custom_field_option_definition FOREIGN KEY ("option_no")
        REFERENCES plugin_idc_core_support_custom_field_option ("option_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_custom_field_option_read
    ON plugin_idc_core_support_ticket_custom_field_value_option (
        "tenant_id", "ticket_no", "field_no", "sort_order", "id"
    );

CREATE OR REPLACE FUNCTION plugin_idc_core_support_ticket_custom_field_immutable()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'support ticket custom field snapshots are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_support_ticket_custom_field_immutable
    ON plugin_idc_core_support_ticket_custom_field_value;
CREATE TRIGGER trg_idc_support_ticket_custom_field_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_core_support_ticket_custom_field_value
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_support_ticket_custom_field_immutable();

DROP TRIGGER IF EXISTS trg_idc_support_ticket_custom_field_option_immutable
    ON plugin_idc_core_support_ticket_custom_field_value_option;
CREATE TRIGGER trg_idc_support_ticket_custom_field_option_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_core_support_ticket_custom_field_value_option
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_support_ticket_custom_field_immutable();

-- Purpose: Stores Support Ticket Follower records.
-- 用途：保存支持工单关注人相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_follower (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "user_id" BIGINT NOT NULL,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_follower UNIQUE ("ticket_no", "user_id"),
    CONSTRAINT ck_idc_support_ticket_follower_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_follower_user CHECK ("user_id" > 0),
    CONSTRAINT fk_idc_support_ticket_follower_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Assignment History records.
-- 用途：保存支持工单分派历史相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_assignment_history (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "history_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "from_department_no" VARCHAR(40),
    "to_department_no" VARCHAR(40),
    "from_user_id" BIGINT,
    "to_user_id" BIGINT,
    "actor_user_id" BIGINT NOT NULL,
    "reason_code" VARCHAR(64),
    "reason_text" VARCHAR(512),
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_assignment_history_no UNIQUE ("history_no"),
    CONSTRAINT ck_idc_support_assignment_history_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_assignment_history_action CHECK (
        "action_code" IN ('created', 'auto_assigned', 'claimed', 'transferred', 'assignee_cleared')
    ),
    CONSTRAINT ck_idc_support_assignment_history_actor CHECK ("actor_user_id" >= 0),
    CONSTRAINT fk_idc_support_assignment_history_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_assignment_history_ticket
    ON plugin_idc_core_support_ticket_assignment_history ("tenant_id", "ticket_no", "occurred_at", "id");

-- Purpose: Stores Support Ticket Status History records.
-- 用途：保存支持工单状态历史相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_status_history (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "history_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "from_base_stage" VARCHAR(32),
    "to_base_stage" VARCHAR(32) NOT NULL,
    "custom_status_no" VARCHAR(40),
    "source_code" VARCHAR(24) NOT NULL,
    "actor_user_id" BIGINT NOT NULL DEFAULT 0,
    "reason_code" VARCHAR(64),
    "reason_text" VARCHAR(512),
    "rule_version" BIGINT,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_status_history_no UNIQUE ("history_no"),
    CONSTRAINT ck_idc_support_status_history_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_status_history_from CHECK (
        "from_base_stage" IS NULL OR "from_base_stage" IN ('new', 'in_progress', 'waiting_customer', 'waiting_upstream', 'resolved', 'closed')
    ),
    CONSTRAINT ck_idc_support_status_history_to CHECK (
        "to_base_stage" IN ('new', 'in_progress', 'waiting_customer', 'waiting_upstream', 'resolved', 'closed')
    ),
    CONSTRAINT ck_idc_support_status_history_source CHECK (
        "source_code" IN ('customer', 'anonymous', 'agent', 'leader', 'admin', 'automation', 'merge')
    ),
    CONSTRAINT fk_idc_support_status_history_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_status_history_ticket
    ON plugin_idc_core_support_ticket_status_history ("tenant_id", "ticket_no", "occurred_at", "id");

-- Purpose: Frozen SLA rule and result for one ticket resolution round.
-- 用途：保存支持工单服务等级轮次相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_sla_round (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "round_no" INT NOT NULL,
    "rule_no" VARCHAR(40),
    "rule_version" BIGINT,
    "calendar_no" VARCHAR(40),
    "calendar_version" BIGINT,
    "calendar_mode" VARCHAR(24),
    "timezone_name" VARCHAR(128),
    "workdays" SMALLINT[],
    "work_start_minute" INT,
    "work_end_minute" INT,
    "holiday_dates" DATE[],
    "first_response_minutes" INT,
    "resolution_minutes" INT,
    "near_due_minutes" INT[] NOT NULL DEFAULT ARRAY[]::INT[],
    "first_response_due_at" TIMESTAMPTZ,
    "resolution_due_at" TIMESTAMPTZ,
    "first_responded_at" TIMESTAMPTZ,
    "resolved_at" TIMESTAMPTZ,
    "stopped_at" TIMESTAMPTZ,
    "paused_seconds" BIGINT NOT NULL DEFAULT 0,
    "first_response_breached" BOOLEAN NOT NULL DEFAULT FALSE,
    "resolution_breached" BOOLEAN NOT NULL DEFAULT FALSE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_sla_round UNIQUE ("ticket_no", "round_no"),
    CONSTRAINT ck_idc_support_sla_round_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_sla_round_number CHECK ("round_no" > 0),
    CONSTRAINT ck_idc_support_sla_round_limits CHECK (
        ("first_response_minutes" IS NULL OR "first_response_minutes" > 0) AND
        ("resolution_minutes" IS NULL OR "resolution_minutes" > 0) AND
        "paused_seconds" >= 0
    ),
    CONSTRAINT ck_idc_support_sla_round_snapshot CHECK (
        ("rule_no" IS NULL AND "rule_version" IS NULL AND "calendar_no" IS NULL AND "calendar_version" IS NULL AND
         "calendar_mode" IS NULL AND "timezone_name" IS NULL AND "workdays" IS NULL AND
         "work_start_minute" IS NULL AND "work_end_minute" IS NULL AND "holiday_dates" IS NULL AND
         "first_response_minutes" IS NULL AND "resolution_minutes" IS NULL AND cardinality("near_due_minutes") = 0)
        OR
        (length("rule_no") > 0 AND "rule_version" > 0 AND length("calendar_no") > 0 AND "calendar_version" > 0 AND
         "calendar_mode" IN ('continuous', 'business_hours') AND
         (("calendar_mode" = 'continuous' AND "timezone_name" IS NULL AND "workdays" IS NULL AND
           "work_start_minute" IS NULL AND "work_end_minute" IS NULL AND "holiday_dates" IS NULL)
          OR
          ("calendar_mode" = 'business_hours' AND length("timezone_name") > 0 AND cardinality("workdays") BETWEEN 1 AND 7 AND
           "work_start_minute" BETWEEN 0 AND 1439 AND "work_end_minute" BETWEEN 1 AND 1440 AND
           "work_start_minute" < "work_end_minute" AND "holiday_dates" IS NOT NULL)))
    ),
    CONSTRAINT fk_idc_support_sla_round_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Sla Pause records.
-- 用途：保存支持工单服务等级暂停相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_sla_pause (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "round_no" INT NOT NULL,
    "started_at" TIMESTAMPTZ NOT NULL,
    "ended_at" TIMESTAMPTZ,
    "reason_code" VARCHAR(32) NOT NULL DEFAULT 'waiting_customer',
    CONSTRAINT ck_idc_support_sla_pause_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_sla_pause_reason CHECK ("reason_code" = 'waiting_customer'),
    CONSTRAINT ck_idc_support_sla_pause_window CHECK ("ended_at" IS NULL OR "ended_at" >= "started_at"),
    CONSTRAINT fk_idc_support_sla_pause_round FOREIGN KEY ("ticket_no", "round_no")
        REFERENCES plugin_idc_core_support_ticket_sla_round ("ticket_no", "round_no")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_support_sla_active_pause
    ON plugin_idc_core_support_ticket_sla_pause ("ticket_no", "round_no")
    WHERE "ended_at" IS NULL;

-- Purpose: Stores Support Ticket Sla Reminder records.
-- 用途：保存支持工单服务等级提醒相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_sla_reminder (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "reminder_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "round_no" INT NOT NULL,
    "timer_code" VARCHAR(24) NOT NULL,
    "node_minutes" INT NOT NULL,
    "event_no" VARCHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_sla_reminder_no UNIQUE ("reminder_no"),
    CONSTRAINT uk_idc_support_sla_reminder_node UNIQUE ("ticket_no", "round_no", "timer_code", "node_minutes"),
    CONSTRAINT ck_idc_support_sla_reminder_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_sla_reminder_timer CHECK (
        "timer_code" IN ('first_response_near', 'first_response_breached', 'resolution_near', 'resolution_breached')
    ),
    CONSTRAINT ck_idc_support_sla_reminder_node_minutes CHECK ("node_minutes" >= 0),
    CONSTRAINT fk_idc_support_sla_reminder_round FOREIGN KEY ("ticket_no", "round_no")
        REFERENCES plugin_idc_core_support_ticket_sla_round ("ticket_no", "round_no")
);

-- Purpose: Append-only public replies, internal notes, upstream replies, and corrections.
-- 用途：保存支持工单消息相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_message (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "message_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "message_type" VARCHAR(24) NOT NULL,
    "visibility_code" VARCHAR(24) NOT NULL,
    "author_type" VARCHAR(16) NOT NULL,
    "author_user_id" BIGINT NOT NULL DEFAULT 0,
    "body" TEXT NOT NULL,
    "automated" BOOLEAN NOT NULL DEFAULT FALSE,
    "rule_version" BIGINT,
    "hidden_at" TIMESTAMPTZ,
    "hidden_by" BIGINT,
    "hidden_reason" VARCHAR(512),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_message_no UNIQUE ("message_no"),
    CONSTRAINT ck_idc_support_ticket_message_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_message_type CHECK (
        "message_type" IN ('initial', 'public_reply', 'internal_note', 'upstream_reply', 'correction', 'automatic_receipt')
    ),
    CONSTRAINT ck_idc_support_ticket_message_visibility CHECK (
        "visibility_code" IN ('customer_visible', 'internal_only')
    ),
    CONSTRAINT ck_idc_support_ticket_message_author CHECK (
        "author_type" IN ('customer', 'anonymous', 'staff', 'system', 'upstream')
    ),
    CONSTRAINT ck_idc_support_ticket_message_body CHECK (length("body") BETWEEN 1 AND 65536),
    CONSTRAINT ck_idc_support_ticket_message_hidden CHECK (
        ("hidden_at" IS NULL AND "hidden_by" IS NULL AND "hidden_reason" IS NULL)
        OR
        ("hidden_at" IS NOT NULL AND "hidden_by" > 0 AND length("hidden_reason") > 0)
    ),
    CONSTRAINT fk_idc_support_ticket_message_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_message_timeline
    ON plugin_idc_core_support_ticket_message ("tenant_id", "ticket_no", "created_at", "id");

-- Purpose: Stores Support Ticket Attachment Upload records.
-- 用途：保存支持工单附件上传相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_attachment_upload (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40),
    "uploader_user_id" BIGINT NOT NULL,
    "uploader_type" VARCHAR(16) NOT NULL,
    "context_code" VARCHAR(24) NOT NULL,
    "file_name" VARCHAR(255) NOT NULL,
    "declared_size_bytes" BIGINT NOT NULL,
    "media_type" VARCHAR(128),
    "sha256_hex" VARCHAR(64),
    "temporary_object_ref" VARCHAR(512) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'reserved',
	"expires_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "consumed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_attachment_upload_no UNIQUE ("upload_no"),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_user CHECK (
        ("uploader_type" = 'anonymous' AND "uploader_user_id" = 0) OR
        ("uploader_type" IN ('customer', 'staff') AND "uploader_user_id" > 0)
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_type CHECK (
        "uploader_type" IN ('anonymous', 'customer', 'staff')
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_context CHECK (
        ("uploader_type" = 'customer' AND "context_code" IN ('customer_initial', 'customer_reply')) OR
        ("uploader_type" = 'staff' AND "context_code" IN ('internal_initial', 'public_reply', 'internal_note')) OR
        ("uploader_type" = 'anonymous' AND "context_code" = 'anonymous_reply')
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_binding CHECK (
        ("context_code" IN ('customer_initial', 'internal_initial') AND (
            ("status_code" IN ('reserved', 'ready', 'cancelled', 'expired', 'rejected') AND "ticket_no" IS NULL) OR
            ("status_code" = 'consumed' AND length("ticket_no") > 0)
        )) OR
        ("context_code" NOT IN ('customer_initial', 'internal_initial') AND length("ticket_no") > 0)
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_size CHECK (
        "declared_size_bytes" BETWEEN 1 AND 20971520
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_hash CHECK (
        "sha256_hex" IS NULL OR "sha256_hex" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_status CHECK (
        "status_code" IN ('reserved', 'ready', 'consumed', 'cancelled', 'expired', 'rejected')
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_ready CHECK (
        "status_code" NOT IN ('ready', 'consumed') OR
        (length("media_type") > 0 AND length("sha256_hex") = 64 AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_upload_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_ticket_attachment_upload_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Anonymous presales attachment uploads are bound by a live two-hour access
-- token in the service layer. The token is never persisted; user zero is only
-- valid for the explicitly anonymous uploader/context pair.
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_attachment_upload_user;
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    ADD CONSTRAINT ck_idc_support_ticket_attachment_upload_user CHECK (
        ("uploader_type" = 'anonymous' AND "uploader_user_id" = 0) OR
        ("uploader_type" IN ('customer', 'staff') AND "uploader_user_id" > 0)
    );
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_attachment_upload_type;
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    ADD CONSTRAINT ck_idc_support_ticket_attachment_upload_type CHECK (
        "uploader_type" IN ('anonymous', 'customer', 'staff')
    );
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_attachment_upload_context;
ALTER TABLE plugin_idc_core_support_ticket_attachment_upload
    ADD CONSTRAINT ck_idc_support_ticket_attachment_upload_context CHECK (
        ("uploader_type" = 'customer' AND "context_code" IN ('customer_initial', 'customer_reply')) OR
        ("uploader_type" = 'staff' AND "context_code" IN ('internal_initial', 'public_reply', 'internal_note')) OR
        ("uploader_type" = 'anonymous' AND "context_code" = 'anonymous_reply')
    );

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_attachment_upload_cleanup
    ON plugin_idc_core_support_ticket_attachment_upload (
        "tenant_id", "status_code", "expires_at", "id"
    );

-- Purpose: Stores Support Ticket Attachment records.
-- 用途：保存支持工单附件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_attachment (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "attachment_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "message_no" VARCHAR(40),
    "visibility_code" VARCHAR(24) NOT NULL,
    "file_name" VARCHAR(255) NOT NULL,
    "media_type" VARCHAR(128) NOT NULL,
    "size_bytes" BIGINT NOT NULL,
    "sha256_hex" VARCHAR(64) NOT NULL,
    "object_ref" VARCHAR(512) NOT NULL,
    "inspection_code" VARCHAR(24) NOT NULL,
    "hidden_at" TIMESTAMPTZ,
    "hidden_by" BIGINT,
    "hidden_reason" VARCHAR(512),
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_attachment_no UNIQUE ("attachment_no"),
    CONSTRAINT ck_idc_support_ticket_attachment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_attachment_visibility CHECK (
        "visibility_code" IN ('customer_visible', 'internal_only')
    ),
    CONSTRAINT ck_idc_support_ticket_attachment_size CHECK ("size_bytes" BETWEEN 1 AND 20971520),
    CONSTRAINT ck_idc_support_ticket_attachment_hash CHECK ("sha256_hex" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_support_ticket_attachment_inspection CHECK ("inspection_code" = 'passed'),
    CONSTRAINT ck_idc_support_ticket_attachment_hidden CHECK (
        ("hidden_at" IS NULL AND "hidden_by" IS NULL AND "hidden_reason" IS NULL)
        OR
        ("hidden_at" IS NOT NULL AND "hidden_by" > 0 AND length("hidden_reason") > 0)
    ),
    CONSTRAINT fk_idc_support_ticket_attachment_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no"),
    CONSTRAINT fk_idc_support_ticket_attachment_message FOREIGN KEY ("message_no")
        REFERENCES plugin_idc_core_support_ticket_message ("message_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_attachment_timeline
    ON plugin_idc_core_support_ticket_attachment ("tenant_id", "ticket_no", "created_at", "id");

-- Purpose: Stores Support Ticket Access Link records.
-- 用途：保存支持工单访问关联相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_access_link (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "link_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "token_digest" VARCHAR(64) NOT NULL,
    "destination_digest" VARCHAR(64) NOT NULL,
    "destination_kind" VARCHAR(16) NOT NULL,
    "operation_digest" VARCHAR(64),
    "request_digest" VARCHAR(64),
    "token_secret_ref" VARCHAR(256),
    "expires_at" TIMESTAMPTZ NOT NULL,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_access_link_no UNIQUE ("link_no"),
    CONSTRAINT uk_idc_support_ticket_access_link_token UNIQUE ("token_digest"),
    CONSTRAINT ck_idc_support_ticket_access_link_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_access_link_digest CHECK (
        "token_digest" ~ '^[0-9a-f]{64}$' AND "destination_digest" ~ '^[0-9a-f]{64}$'
    ),
	CONSTRAINT ck_idc_support_ticket_access_link_kind CHECK ("destination_kind" IN ('email', 'sms')),
	CONSTRAINT ck_idc_support_ticket_access_link_operation CHECK (
		("operation_digest" IS NULL AND "request_digest" IS NULL AND "token_secret_ref" IS NULL)
		OR
		("operation_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$' AND
		 length("token_secret_ref") BETWEEN 1 AND 256)
	),
	CONSTRAINT ck_idc_support_ticket_access_link_duration CHECK ("expires_at" = "created_at" + INTERVAL '2 hours'),
    CONSTRAINT fk_idc_support_ticket_access_link_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

ALTER TABLE plugin_idc_core_support_ticket_access_link
    ADD COLUMN IF NOT EXISTS "operation_digest" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "request_digest" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "token_secret_ref" VARCHAR(256);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_idc_support_ticket_access_link_operation'
          AND conrelid = 'plugin_idc_core_support_ticket_access_link'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_support_ticket_access_link
            ADD CONSTRAINT ck_idc_support_ticket_access_link_operation CHECK (
                ("operation_digest" IS NULL AND "request_digest" IS NULL AND "token_secret_ref" IS NULL)
                OR
                ("operation_digest" ~ '^[0-9a-f]{64}$' AND "request_digest" ~ '^[0-9a-f]{64}$' AND
                 length("token_secret_ref") BETWEEN 1 AND 256)
            );
    END IF;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_support_ticket_access_link_operation
    ON plugin_idc_core_support_ticket_access_link ("operation_digest")
    WHERE "operation_digest" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_idc_support_ticket_access_link_current
    ON plugin_idc_core_support_ticket_access_link ("tenant_id", "ticket_no", "expires_at" DESC)
    WHERE "revoked_at" IS NULL;

-- Purpose: Stores Support Ticket Rating records.
-- 用途：保存支持工单评价相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_rating (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "rating_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "round_no" INT NOT NULL,
    "score" SMALLINT NOT NULL,
    "comment" VARCHAR(2000),
    "author_type" VARCHAR(16) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_rating_no UNIQUE ("rating_no"),
    CONSTRAINT uk_idc_support_ticket_rating_round UNIQUE ("ticket_no", "round_no"),
    CONSTRAINT ck_idc_support_ticket_rating_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_rating_score CHECK ("score" BETWEEN 1 AND 5),
    CONSTRAINT ck_idc_support_ticket_rating_author CHECK ("author_type" IN ('customer', 'anonymous')),
    CONSTRAINT fk_idc_support_ticket_rating_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Relation records.
-- 用途：保存支持工单关系相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_relation (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "relation_no" VARCHAR(40) NOT NULL,
    "relation_type" VARCHAR(16) NOT NULL,
    "source_ticket_no" VARCHAR(40) NOT NULL,
    "target_ticket_no" VARCHAR(40) NOT NULL,
    "actor_user_id" BIGINT NOT NULL,
    "reason" VARCHAR(512) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_relation_no UNIQUE ("relation_no"),
    CONSTRAINT uk_idc_support_ticket_relation_pair UNIQUE ("relation_type", "source_ticket_no", "target_ticket_no"),
    CONSTRAINT ck_idc_support_ticket_relation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_relation_type CHECK ("relation_type" IN ('merged_into', 'split_from')),
    CONSTRAINT ck_idc_support_ticket_relation_distinct CHECK ("source_ticket_no" <> "target_ticket_no"),
    CONSTRAINT fk_idc_support_ticket_relation_source FOREIGN KEY ("source_ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no"),
    CONSTRAINT fk_idc_support_ticket_relation_target FOREIGN KEY ("target_ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Escalation records.
-- 用途：保存支持工单升级相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_escalation (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "escalation_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "from_user_id" BIGINT NOT NULL,
    "to_user_id" BIGINT NOT NULL,
    "reason" VARCHAR(512) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_ticket_escalation_no UNIQUE ("escalation_no"),
    CONSTRAINT ck_idc_support_ticket_escalation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_ticket_escalation_users CHECK (
        "from_user_id" > 0 AND "to_user_id" > 0 AND "from_user_id" <> "to_user_id"
    ),
    CONSTRAINT fk_idc_support_ticket_escalation_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Upstream Forward records.
-- 用途：保存支持工单上游转发相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_upstream_forward (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "forward_no" VARCHAR(40) NOT NULL,
    "ticket_no" VARCHAR(40) NOT NULL,
    "provider_id" VARCHAR(64) NOT NULL,
    "provider_ticket_reference" VARCHAR(128),
    "selection_digest" VARCHAR(64) NOT NULL,
    "preview_digest" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(32) NOT NULL DEFAULT 'prepared',
    "result_classification" VARCHAR(32),
    "retry_eligible" BOOLEAN NOT NULL DEFAULT FALSE,
    "confirmed_by" BIGINT,
    "confirmed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_upstream_forward_no UNIQUE ("forward_no"),
    CONSTRAINT ck_idc_support_upstream_forward_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_upstream_forward_digest CHECK (
        "selection_digest" ~ '^[0-9a-f]{64}$' AND "preview_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_support_upstream_forward_status CHECK (
        "status_code" IN ('prepared', 'submitted', 'succeeded', 'definitive_failure', 'result_unknown', 'manual_review')
    ),
    CONSTRAINT ck_idc_support_upstream_forward_result CHECK (
        "result_classification" IS NULL OR
        "result_classification" IN ('accepted', 'definitive_not_received', 'result_unknown')
    ),
    CONSTRAINT ck_idc_support_upstream_forward_retry CHECK (
        NOT "retry_eligible" OR ("status_code" = 'definitive_failure' AND "result_classification" = 'definitive_not_received')
    ),
    CONSTRAINT ck_idc_support_upstream_forward_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_upstream_forward_ticket FOREIGN KEY ("ticket_no")
        REFERENCES plugin_idc_core_support_ticket ("ticket_no")
);

-- Purpose: Stores Support Ticket Upstream Item records.
-- 用途：保存支持工单上游明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_ticket_upstream_item (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "forward_no" VARCHAR(40) NOT NULL,
    "item_type" VARCHAR(16) NOT NULL,
    "item_no" VARCHAR(40) NOT NULL,
    "sort_order" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_upstream_item UNIQUE ("forward_no", "item_type", "item_no"),
    CONSTRAINT ck_idc_support_upstream_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_upstream_item_type CHECK ("item_type" IN ('message', 'attachment')),
    CONSTRAINT fk_idc_support_upstream_item_forward FOREIGN KEY ("forward_no")
        REFERENCES plugin_idc_core_support_ticket_upstream_forward ("forward_no")
);

-- Purpose: Stores Support Reply Preset records.
-- 用途：保存支持回复预设相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_reply_preset (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "preset_no" VARCHAR(40) NOT NULL,
    "department_no" VARCHAR(40) NOT NULL,
    "locale" VARCHAR(16) NOT NULL,
    "title" VARCHAR(128) NOT NULL,
    "body" TEXT NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_reply_preset_no UNIQUE ("preset_no"),
    CONSTRAINT ck_idc_support_reply_preset_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_reply_preset_locale CHECK ("locale" IN ('zh-CN', 'en-US')),
    CONSTRAINT ck_idc_support_reply_preset_body CHECK (length("body") BETWEEN 1 AND 65536),
    CONSTRAINT ck_idc_support_reply_preset_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_support_reply_preset_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_support_reply_preset_department FOREIGN KEY ("department_no")
        REFERENCES plugin_idc_core_support_department ("department_no")
);

-- Purpose: Stores Support Automation Policy records.
-- 用途：保存支持自动化策略相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_support_automation_policy (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "auto_close_days" INT NOT NULL DEFAULT 3,
    "reopen_days" INT NOT NULL DEFAULT 7,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_support_automation_policy_no UNIQUE ("policy_no"),
    CONSTRAINT ck_idc_support_automation_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_support_automation_policy_days CHECK (
		"auto_close_days" BETWEEN 1 AND 365 AND "reopen_days" BETWEEN 1 AND 365
    ),
    CONSTRAINT ck_idc_support_automation_policy_version CHECK ("version" > 0)
);

INSERT INTO plugin_idc_core_support_automation_policy (
	"tenant_id", "policy_no", "auto_close_days", "reopen_days"
) VALUES (0, 'support-default', 3, 7)
ON CONFLICT ("policy_no") DO NOTHING;

-- Purpose: Stores Content Category records.
-- 用途：保存内容分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_category (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "category_no" VARCHAR(40) NOT NULL,
    "content_type" VARCHAR(16) NOT NULL,
    "name" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_category_no UNIQUE ("category_no"),
    CONSTRAINT ck_idc_content_category_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_category_type CHECK ("content_type" IN ('news', 'knowledge', 'download')),
    CONSTRAINT ck_idc_content_category_status CHECK ("status_code" IN ('active', 'disabled')),
    CONSTRAINT ck_idc_content_category_version CHECK ("version" > 0)
);

CREATE INDEX IF NOT EXISTS idx_idc_content_category_directory
    ON plugin_idc_core_content_category ("tenant_id", "content_type", "status_code", "sort_order", "id");

-- Purpose: News, knowledge, and download content with stable public path and authoritative validity version.
-- 用途：保存内容明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_item (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "content_no" VARCHAR(40) NOT NULL,
    "content_type" VARCHAR(16) NOT NULL,
    "category_no" VARCHAR(40),
    "visibility_code" VARCHAR(24) NOT NULL,
    "title" VARCHAR(256) NOT NULL,
    "summary" VARCHAR(2000),
    "body" TEXT NOT NULL DEFAULT '',
    "seo_title" VARCHAR(256),
    "seo_description" VARCHAR(2000),
    "public_path" VARCHAR(160) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "validity_version" BIGINT NOT NULL DEFAULT 1,
    "current_file_no" VARCHAR(40),
    "published_at" TIMESTAMPTZ,
    "offline_at" TIMESTAMPTZ,
    "deleted_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "updated_by" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_item_no UNIQUE ("content_no"),
    CONSTRAINT uk_idc_content_item_public_path UNIQUE ("public_path"),
    CONSTRAINT ck_idc_content_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_item_type CHECK ("content_type" IN ('news', 'knowledge', 'download')),
    CONSTRAINT ck_idc_content_item_visibility CHECK ("visibility_code" IN ('public','all_logged_in','customer_group','internal')),
    CONSTRAINT ck_idc_content_item_title CHECK (length("title") BETWEEN 1 AND 256),
    CONSTRAINT ck_idc_content_item_public_path CHECK ("public_path" ~ '^/[a-z0-9][a-z0-9_/-]{0,158}$'),
    CONSTRAINT ck_idc_content_item_status CHECK ("status_code" IN ('draft', 'published', 'offline', 'deleted')),
    CONSTRAINT ck_idc_content_item_validity CHECK ("validity_version" > 0),
    CONSTRAINT ck_idc_content_item_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_content_item_category FOREIGN KEY ("category_no")
        REFERENCES plugin_idc_core_content_category ("category_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_content_item_admin
    ON plugin_idc_core_content_item (
        "tenant_id", "content_type", "status_code", "updated_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_content_item_public
    ON plugin_idc_core_content_item (
        "tenant_id", "visibility_code", "status_code", "published_at" DESC, "id" DESC
    );

ALTER TABLE plugin_idc_core_content_item
    DROP CONSTRAINT IF EXISTS ck_idc_content_item_public_path;
ALTER TABLE plugin_idc_core_content_item
    ADD CONSTRAINT ck_idc_content_item_public_path CHECK ("public_path" ~ '^/[a-z0-9][a-z0-9_/-]{0,158}$');

-- Purpose: Stores Content Group Scope records.
-- 用途：保存内容分组范围相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_group_scope (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "content_no" VARCHAR(40) NOT NULL,
    "group_no" VARCHAR(40) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_group_scope UNIQUE ("content_no", "group_no"),
    CONSTRAINT ck_idc_content_group_scope_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT fk_idc_content_group_scope_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_content_group_scope_group
    ON plugin_idc_core_content_group_scope ("tenant_id", "group_no", "content_no");

-- Purpose: Stores Content Action History records.
-- 用途：保存内容动作历史相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_action_history (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "action_no" VARCHAR(40) NOT NULL,
    "content_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "actor_user_id" BIGINT NOT NULL,
    "content_version" BIGINT NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_action_history_no UNIQUE ("action_no"),
    CONSTRAINT ck_idc_content_action_history_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_action_history_action CHECK (
        "action_code" IN ('draft_saved', 'published', 'modified', 'offline', 'republished', 'deleted', 'file_enabled')
    ),
    CONSTRAINT ck_idc_content_action_history_actor CHECK ("actor_user_id" > 0),
    CONSTRAINT ck_idc_content_action_history_version CHECK ("content_version" > 0),
    CONSTRAINT fk_idc_content_action_history_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_content_action_history_item
    ON plugin_idc_core_content_action_history ("tenant_id", "content_no", "occurred_at", "id");

-- Purpose: Stores Content Upload Session records.
-- 用途：保存内容上传会话相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_upload_session (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "content_no" VARCHAR(40) NOT NULL,
    "uploader_user_id" BIGINT NOT NULL,
    "file_name" VARCHAR(255) NOT NULL,
    "declared_size_bytes" BIGINT NOT NULL,
    "persisted_size_bytes" BIGINT NOT NULL DEFAULT 0,
    "temporary_object_ref" VARCHAR(512) NOT NULL,
    "transfer_mode" VARCHAR(24) NOT NULL,
    "provider_upload_id" VARCHAR(512),
    "chunk_size_bytes" INT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'uploading',
    "last_data_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ NOT NULL,
	"cleanup_lease_expires_at" TIMESTAMPTZ,
	"cleanup_completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_upload_session_no UNIQUE ("upload_no"),
    CONSTRAINT ck_idc_content_upload_session_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_upload_session_user CHECK ("uploader_user_id" > 0),
    CONSTRAINT ck_idc_content_upload_session_size CHECK (
        "declared_size_bytes" BETWEEN 1 AND 5368709120 AND
        "persisted_size_bytes" BETWEEN 0 AND "declared_size_bytes"
    ),
    CONSTRAINT ck_idc_content_upload_session_status CHECK (
        "status_code" IN ('uploading', 'completed', 'cancelled', 'expired')
    ),
    CONSTRAINT ck_idc_content_upload_session_transfer CHECK (
        ("transfer_mode" = 'provider_multipart' AND length("provider_upload_id") > 0) OR
        ("transfer_mode" = 'chunk_objects' AND "provider_upload_id" IS NULL)
    ),
    CONSTRAINT ck_idc_content_upload_session_chunk_size CHECK (
        "chunk_size_bytes" BETWEEN 5242880 AND 67108864
    ),
    CONSTRAINT ck_idc_content_upload_session_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_content_upload_session_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no")
);

ALTER TABLE plugin_idc_core_content_upload_session
	ADD COLUMN IF NOT EXISTS "cleanup_lease_expires_at" TIMESTAMPTZ;
ALTER TABLE plugin_idc_core_content_upload_session
	ADD COLUMN IF NOT EXISTS "cleanup_completed_at" TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_idc_content_upload_session_cleanup
    ON plugin_idc_core_content_upload_session ("tenant_id", "status_code", "expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_idc_content_upload_session_cleanup_pending
	ON plugin_idc_core_content_upload_session ("tenant_id", "status_code", "cleanup_lease_expires_at", "id")
	WHERE "cleanup_completed_at" IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_content_upload_session_active
    ON plugin_idc_core_content_upload_session ("content_no")
    WHERE "status_code" = 'uploading';

-- Purpose: Stores Content Upload Part records.
-- 用途：保存内容上传分片相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_upload_part (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "part_number" INT NOT NULL,
    "offset_bytes" BIGINT NOT NULL,
    "size_bytes" INT NOT NULL,
    "etag" VARCHAR(512),
    "object_ref" VARCHAR(512),
    "sha256_hex" VARCHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_upload_part UNIQUE ("upload_no", "part_number"),
    CONSTRAINT uk_idc_content_upload_part_offset UNIQUE ("upload_no", "offset_bytes"),
    CONSTRAINT ck_idc_content_upload_part_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_upload_part_number CHECK ("part_number" BETWEEN 1 AND 1024),
    CONSTRAINT ck_idc_content_upload_part_offset CHECK ("offset_bytes" >= 0),
    CONSTRAINT ck_idc_content_upload_part_size CHECK ("size_bytes" BETWEEN 1 AND 67108864),
    CONSTRAINT ck_idc_content_upload_part_location CHECK (
        (("etag" IS NOT NULL)::INT + ("object_ref" IS NOT NULL)::INT) = 1
    ),
    CONSTRAINT ck_idc_content_upload_part_hash CHECK ("sha256_hex" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT fk_idc_content_upload_part_session FOREIGN KEY ("upload_no")
        REFERENCES plugin_idc_core_content_upload_session ("upload_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_content_upload_part_sequence
    ON plugin_idc_core_content_upload_part ("tenant_id", "upload_no", "part_number");

-- Purpose: Single-current and single-candidate download file facts; dangerous types require a local passed scan.
-- 用途：保存内容文件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_file (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "file_no" VARCHAR(40) NOT NULL,
    "content_no" VARCHAR(40) NOT NULL,
    "upload_no" VARCHAR(40) NOT NULL,
    "candidate_generation" BIGINT NOT NULL,
    "role_code" VARCHAR(16) NOT NULL DEFAULT 'candidate',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'checking',
    "file_name" VARCHAR(255) NOT NULL,
    "media_type" VARCHAR(128) NOT NULL,
    "content_kind" VARCHAR(32) NOT NULL,
    "size_bytes" BIGINT NOT NULL,
    "sha256_hex" VARCHAR(64) NOT NULL,
    "object_ref" VARCHAR(512),
    "scan_required" BOOLEAN NOT NULL DEFAULT FALSE,
    "scan_status" VARCHAR(24) NOT NULL DEFAULT 'not_configured',
    "gate_completed_at" TIMESTAMPTZ,
    "retention_expires_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "enabled_at" TIMESTAMPTZ,
    "object_deleted_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_file_no UNIQUE ("file_no"),
    CONSTRAINT uk_idc_content_file_generation UNIQUE ("content_no", "candidate_generation"),
    CONSTRAINT uk_idc_content_file_no_generation UNIQUE ("file_no", "candidate_generation"),
    CONSTRAINT ck_idc_content_file_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_file_generation CHECK ("candidate_generation" > 0),
    CONSTRAINT ck_idc_content_file_role CHECK ("role_code" IN ('candidate', 'current', 'retired')),
    CONSTRAINT ck_idc_content_file_status CHECK (
        "status_code" IN ('checking', 'ready', 'active', 'cancelled', 'expired', 'risk_rejected', 'replaced')
    ),
    CONSTRAINT ck_idc_content_file_kind CHECK (
        "content_kind" IN ('image', 'pdf', 'office', 'zip', '7z', 'exe', 'msi', 'iso')
    ),
    CONSTRAINT ck_idc_content_file_size CHECK ("size_bytes" BETWEEN 1 AND 5368709120),
    CONSTRAINT ck_idc_content_file_hash CHECK ("sha256_hex" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_content_file_scan CHECK (
        "scan_status" IN ('not_configured', 'scanning', 'passed', 'risk_found', 'failed')
    ),
    CONSTRAINT ck_idc_content_file_publishable CHECK (
        "status_code" NOT IN ('ready', 'active') OR
        (NOT "scan_required" OR "scan_status" = 'passed')
    ),
    CONSTRAINT ck_idc_content_file_object CHECK (
        ("status_code" IN ('risk_rejected', 'cancelled', 'expired') AND "object_ref" IS NULL)
        OR "status_code" NOT IN ('risk_rejected', 'cancelled', 'expired')
    ),
    CONSTRAINT ck_idc_content_file_object_deleted CHECK (
        "object_deleted_at" IS NULL OR "status_code" IN ('risk_rejected', 'cancelled', 'expired', 'replaced')
    ),
    CONSTRAINT ck_idc_content_file_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_content_file_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no"),
    CONSTRAINT fk_idc_content_file_upload FOREIGN KEY ("upload_no")
        REFERENCES plugin_idc_core_content_upload_session ("upload_no")
);

ALTER TABLE plugin_idc_core_content_file
    ADD COLUMN IF NOT EXISTS "object_deleted_at" TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_content_file_active_candidate
    ON plugin_idc_core_content_file ("content_no")
    WHERE "role_code" = 'candidate' AND "status_code" IN ('checking', 'ready');
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_content_file_current
    ON plugin_idc_core_content_file ("content_no")
    WHERE "role_code" = 'current' AND "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_idc_content_file_cleanup
    ON plugin_idc_core_content_file ("tenant_id", "status_code", "retention_expires_at", "object_deleted_at", "id");

ALTER TABLE plugin_idc_core_content_file
    DROP CONSTRAINT IF EXISTS ck_idc_content_file_object_deleted;
ALTER TABLE plugin_idc_core_content_file
    ADD CONSTRAINT ck_idc_content_file_object_deleted CHECK (
        "object_deleted_at" IS NULL OR "status_code" IN ('risk_rejected', 'cancelled', 'expired', 'replaced')
    );
CREATE INDEX IF NOT EXISTS idx_idc_content_file_scan_cleanup
    ON plugin_idc_core_content_file ("tenant_id", "status_code", "scan_status", "retention_expires_at", "object_deleted_at", "id");

-- Purpose: Stores Content Scan Attempt records.
-- 用途：保存内容扫描尝试相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_scan_attempt (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "attempt_no" VARCHAR(40) NOT NULL,
    "file_no" VARCHAR(40) NOT NULL,
    "candidate_generation" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "queued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "started_at" TIMESTAMPTZ,
    "lease_expires_at" TIMESTAMPTZ,
    "finished_at" TIMESTAMPTZ,
    "risk_code" VARCHAR(64),
    "created_by" BIGINT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uk_idc_content_scan_attempt_no UNIQUE ("attempt_no"),
    CONSTRAINT ck_idc_content_scan_attempt_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_scan_attempt_status CHECK (
        "status_code" IN ('pending', 'scanning', 'passed', 'risk_found', 'failed')
    ),
    CONSTRAINT ck_idc_content_scan_attempt_window CHECK (
        "started_at" IS NULL OR "started_at" >= "queued_at"
    ),
    CONSTRAINT ck_idc_content_scan_attempt_finished_window CHECK (
        "finished_at" IS NULL OR ("started_at" IS NOT NULL AND "finished_at" >= "started_at")
    ),
    CONSTRAINT ck_idc_content_scan_attempt_state CHECK (
        ("status_code" = 'pending' AND "started_at" IS NULL AND "lease_expires_at" IS NULL AND "finished_at" IS NULL)
        OR ("status_code" = 'scanning' AND "started_at" IS NOT NULL AND "lease_expires_at" > "started_at" AND "finished_at" IS NULL)
        OR ("status_code" IN ('passed', 'risk_found', 'failed') AND "started_at" IS NOT NULL AND "lease_expires_at" IS NULL AND "finished_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_content_scan_attempt_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_content_scan_attempt_file FOREIGN KEY ("file_no", "candidate_generation")
        REFERENCES plugin_idc_core_content_file ("file_no", "candidate_generation")
);

ALTER TABLE plugin_idc_core_content_scan_attempt
    ADD COLUMN IF NOT EXISTS "queued_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS "lease_expires_at" TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS "version" BIGINT NOT NULL DEFAULT 1;
ALTER TABLE plugin_idc_core_content_scan_attempt
    ALTER COLUMN "started_at" DROP NOT NULL;
UPDATE plugin_idc_core_content_scan_attempt
SET "status_code" = 'failed',
    "finished_at" = COALESCE("finished_at", CURRENT_TIMESTAMP),
    "lease_expires_at" = NULL,
    "version" = "version" + 1
WHERE "status_code" = 'scanning';
ALTER TABLE plugin_idc_core_content_scan_attempt
    DROP CONSTRAINT IF EXISTS ck_idc_content_scan_attempt_status,
    DROP CONSTRAINT IF EXISTS ck_idc_content_scan_attempt_window,
    DROP CONSTRAINT IF EXISTS ck_idc_content_scan_attempt_finished_window,
    DROP CONSTRAINT IF EXISTS ck_idc_content_scan_attempt_state,
    DROP CONSTRAINT IF EXISTS ck_idc_content_scan_attempt_version;
ALTER TABLE plugin_idc_core_content_scan_attempt
    ADD CONSTRAINT ck_idc_content_scan_attempt_status CHECK (
        "status_code" IN ('pending', 'scanning', 'passed', 'risk_found', 'failed')
    ),
    ADD CONSTRAINT ck_idc_content_scan_attempt_window CHECK (
        "started_at" IS NULL OR "started_at" >= "queued_at"
    ),
    ADD CONSTRAINT ck_idc_content_scan_attempt_finished_window CHECK (
        "finished_at" IS NULL OR ("started_at" IS NOT NULL AND "finished_at" >= "started_at")
    ),
    ADD CONSTRAINT ck_idc_content_scan_attempt_state CHECK (
        ("status_code" = 'pending' AND "started_at" IS NULL AND "lease_expires_at" IS NULL AND "finished_at" IS NULL)
        OR ("status_code" = 'scanning' AND "started_at" IS NOT NULL AND "lease_expires_at" > "started_at" AND "finished_at" IS NULL)
        OR ("status_code" IN ('passed', 'risk_found', 'failed') AND "started_at" IS NOT NULL AND "lease_expires_at" IS NULL AND "finished_at" IS NOT NULL)
    ),
    ADD CONSTRAINT ck_idc_content_scan_attempt_version CHECK ("version" > 0);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_content_scan_attempt_active
    ON plugin_idc_core_content_scan_attempt ("file_no", "candidate_generation")
    WHERE "status_code" IN ('pending', 'scanning');
CREATE INDEX IF NOT EXISTS idx_idc_content_scan_attempt_claim
    ON plugin_idc_core_content_scan_attempt ("tenant_id", "status_code", "queued_at", "id");
CREATE INDEX IF NOT EXISTS idx_idc_content_scan_attempt_lease
    ON plugin_idc_core_content_scan_attempt ("tenant_id", "status_code", "lease_expires_at", "id");

-- Purpose: Stores Content Download Authorization records.
-- 用途：保存内容下载授权相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_download_authorization (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "content_no" VARCHAR(40) NOT NULL,
    "user_id" BIGINT NOT NULL,
    "revision" BIGINT NOT NULL DEFAULT 1,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_content_download_authorization UNIQUE ("content_no", "user_id"),
    CONSTRAINT ck_idc_content_download_authorization_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_download_authorization_user CHECK ("user_id" > 0),
    CONSTRAINT ck_idc_content_download_authorization_revision CHECK ("revision" > 0),
    CONSTRAINT fk_idc_content_download_authorization_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no")
);

-- Purpose: Ten-minute authoritative public or session-bound download authorization.
-- 用途：保存内容下载工单相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_content_download_ticket (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "ticket_no" VARCHAR(40) NOT NULL,
    "token_digest" VARCHAR(64) NOT NULL,
    "content_no" VARCHAR(40) NOT NULL,
    "file_no" VARCHAR(40) NOT NULL,
    "content_validity_version" BIGINT NOT NULL,
    "visibility_code" VARCHAR(24) NOT NULL,
    "user_id" BIGINT,
    "session_reference_digest" VARCHAR(64),
    "authorization_revision" BIGINT,
    "issued_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ NOT NULL,
    "revoked_at" TIMESTAMPTZ,
    CONSTRAINT uk_idc_content_download_ticket_no UNIQUE ("ticket_no"),
    CONSTRAINT uk_idc_content_download_ticket_digest UNIQUE ("token_digest"),
    CONSTRAINT ck_idc_content_download_ticket_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_content_download_ticket_digest CHECK ("token_digest" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_content_download_ticket_window CHECK (
        "expires_at" = "issued_at" + INTERVAL '10 minutes'
    ),
    CONSTRAINT ck_idc_content_download_ticket_binding CHECK (
        ("visibility_code" = 'public' AND "user_id" IS NULL AND "session_reference_digest" IS NULL AND "authorization_revision" IS NULL)
        OR
        ("visibility_code" <> 'public' AND "user_id" > 0 AND
         "session_reference_digest" ~ '^[0-9a-f]{64}$' AND "authorization_revision" > 0)
    ),
    CONSTRAINT fk_idc_content_download_ticket_item FOREIGN KEY ("content_no")
        REFERENCES plugin_idc_core_content_item ("content_no"),
    CONSTRAINT fk_idc_content_download_ticket_file FOREIGN KEY ("file_no")
        REFERENCES plugin_idc_core_content_file ("file_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_content_download_ticket_current
    ON plugin_idc_core_content_download_ticket (
        "tenant_id", "content_no", "file_no", "expires_at"
    ) WHERE "revoked_at" IS NULL;

-- Purpose: Stores Notification Preference Profile records.
-- 用途：保存通知偏好资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_notification_preference_profile (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "version" BIGINT NOT NULL DEFAULT 1,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_notification_preference_profile UNIQUE ("customer_no"),
    CONSTRAINT ck_idc_notification_preference_profile_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_notification_preference_profile_version CHECK ("version" > 0),
    CONSTRAINT fk_idc_notification_preference_profile_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
);

-- Purpose: Stores Notification Preference Command records.
-- 用途：保存通知偏好指令相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_notification_preference_command (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "command_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "idempotency_digest" VARCHAR(64) NOT NULL,
    "request_fingerprint" VARCHAR(64) NOT NULL,
    "evidence_digest" VARCHAR(64) NOT NULL,
    "result_version" BIGINT NOT NULL,
    "applied_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT uk_idc_notification_preference_command_no UNIQUE ("command_no"),
    CONSTRAINT uk_idc_notification_preference_idempotency UNIQUE ("customer_no", "idempotency_digest"),
    CONSTRAINT ck_idc_notification_preference_command_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_notification_preference_command_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$' AND
        "request_fingerprint" ~ '^[0-9a-f]{64}$' AND
        "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_notification_preference_command_version CHECK ("result_version" > 0),
    CONSTRAINT fk_idc_notification_preference_command_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_notification_preference_history
    ON plugin_idc_core_notification_preference_command (
        "tenant_id", "customer_no", "applied_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_support_ticket IS
    'Core-owned ticket head with six immutable base stages and bounded current projections';
COMMENT ON COLUMN plugin_idc_core_support_ticket."related_service_no" IS
    'Immutable optional customer service reference revalidated at creation; intentionally not a foreign key so ticket history survives service archival';
COMMENT ON TABLE plugin_idc_core_support_ticket_message IS
    'Append-only public replies, internal notes, upstream replies, and corrections';
COMMENT ON TABLE plugin_idc_core_support_ticket_sla_round IS
    'Frozen SLA rule and result for one ticket resolution round';
COMMENT ON TABLE plugin_idc_core_content_item IS
    'News, knowledge, and download content with stable public path and authoritative validity version';
COMMENT ON TABLE plugin_idc_core_content_file IS
    'Single-current and single-candidate download file facts; dangerous types require a local passed scan';
COMMENT ON TABLE plugin_idc_core_content_download_ticket IS
    'Ten-minute authoritative public or session-bound download authorization';
