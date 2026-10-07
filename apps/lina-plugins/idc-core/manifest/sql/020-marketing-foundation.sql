-- 020: Typed marketing segments, promotion assets, quotas, points, and non-cash value grants.

-- Purpose: Typed static or dynamic customer segment definition; existing customer groups remain independent.
-- 用途：保存营销分群相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_segment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "segment_no" VARCHAR(40) NOT NULL,
    "segment_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "mode_code" VARCHAR(16) NOT NULL,
    "refresh_interval_minutes" INT,
    "definition_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_segment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_segment_code CHECK (
        "segment_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_idc_marketing_segment_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_segment_mode CHECK ("mode_code" IN ('static', 'dynamic')),
    CONSTRAINT ck_idc_marketing_segment_refresh CHECK (
        ("mode_code" = 'static' AND "refresh_interval_minutes" IS NULL) OR
        ("mode_code" = 'dynamic' AND "refresh_interval_minutes" BETWEEN 5 AND 10080)
    ),
    CONSTRAINT ck_idc_marketing_segment_digest CHECK (
        "definition_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_segment_status CHECK (
        "status_code" IN ('draft', 'active', 'paused', 'archived')
    ),
    CONSTRAINT ck_idc_marketing_segment_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_segment_no UNIQUE ("segment_no"),
    CONSTRAINT uk_idc_marketing_segment_code UNIQUE ("segment_code")
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_list
    ON plugin_idc_core_marketing_segment (
        "tenant_id", "status_code", "name_zh_cn", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_list_en
    ON plugin_idc_core_marketing_segment (
        "tenant_id", "status_code", "name_en_us", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_workbench
    ON plugin_idc_core_marketing_segment (
        "tenant_id", "status_code", "mode_code", "updated_at" DESC, "id" DESC
    );

-- Purpose: Typed segment predicate; OR groups contain ordered AND conditions without arbitrary executable JSON.
-- 用途：保存营销分群条件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_segment_condition (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "condition_no" VARCHAR(40) NOT NULL,
    "segment_no" VARCHAR(40) NOT NULL,
    "or_group_no" INT NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "fact_code" VARCHAR(32) NOT NULL,
    "operator_code" VARCHAR(16) NOT NULL,
    "value_kind_code" VARCHAR(16) NOT NULL,
    "value_text" VARCHAR(128),
    "value_integer_lower" BIGINT,
    "value_integer_upper" BIGINT,
    "value_boolean" BOOLEAN,
    "currency_code" CHAR(3),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_condition_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_condition_group CHECK ("or_group_no" BETWEEN 1 AND 128),
    CONSTRAINT ck_idc_marketing_condition_sort CHECK ("sort_order" BETWEEN 0 AND 1000000),
    CONSTRAINT ck_idc_marketing_condition_fact CHECK (
        "fact_code" IN (
            'customer_level', 'customer_tag', 'customer_type', 'identity_status',
            'region_country', 'registered_age_days', 'paid_amount',
            'active_service_count', 'renewal_due_days', 'ticket_count',
            'acquisition_channel', 'risk_state'
        )
    ),
    CONSTRAINT ck_idc_marketing_condition_operator CHECK (
        "operator_code" IN (
            'eq', 'neq', 'in', 'not_in', 'gte', 'lte', 'between', 'exists', 'not_exists'
        )
    ),
    CONSTRAINT ck_idc_marketing_condition_kind CHECK (
        "value_kind_code" IN ('none', 'text', 'integer', 'boolean', 'currency_amount')
    ),
    CONSTRAINT ck_idc_marketing_condition_currency CHECK (
        ("value_kind_code" = 'currency_amount' AND "currency_code" ~ '^[A-Z]{3}$') OR
        ("value_kind_code" <> 'currency_amount' AND "currency_code" IS NULL)
    ),
    CONSTRAINT ck_idc_marketing_condition_value CHECK (
        (
            "operator_code" IN ('exists', 'not_exists') AND "value_kind_code" = 'none' AND
            "value_text" IS NULL AND "value_integer_lower" IS NULL AND
            "value_integer_upper" IS NULL AND "value_boolean" IS NULL
        ) OR (
            "operator_code" IN ('in', 'not_in') AND "value_kind_code" IN ('text', 'integer') AND
            "value_text" IS NULL AND "value_integer_lower" IS NULL AND
            "value_integer_upper" IS NULL AND "value_boolean" IS NULL
        ) OR (
            "operator_code" IN ('eq', 'neq') AND "value_kind_code" = 'text' AND
            length(btrim("value_text")) BETWEEN 1 AND 128 AND
            "value_integer_lower" IS NULL AND "value_integer_upper" IS NULL AND
            "value_boolean" IS NULL
        ) OR (
            "operator_code" IN ('eq', 'neq') AND "value_kind_code" = 'boolean' AND
            "value_boolean" IS NOT NULL AND "value_text" IS NULL AND
            "value_integer_lower" IS NULL AND "value_integer_upper" IS NULL
        ) OR (
            "operator_code" IN ('eq', 'neq', 'gte', 'lte') AND
            "value_kind_code" IN ('integer', 'currency_amount') AND
            "value_integer_lower" IS NOT NULL AND "value_integer_upper" IS NULL AND
            "value_text" IS NULL AND "value_boolean" IS NULL
        ) OR (
            "operator_code" = 'between' AND
            "value_kind_code" IN ('integer', 'currency_amount') AND
            "value_integer_lower" IS NOT NULL AND
            "value_integer_upper" IS NOT NULL AND
            "value_integer_upper" >= "value_integer_lower" AND
            "value_text" IS NULL AND "value_boolean" IS NULL
        )
    ),
    CONSTRAINT uk_idc_marketing_condition_no UNIQUE ("condition_no"),
    CONSTRAINT fk_idc_marketing_condition_segment FOREIGN KEY ("segment_no")
        REFERENCES plugin_idc_core_marketing_segment ("segment_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_condition_order
    ON plugin_idc_core_marketing_segment_condition (
        "segment_no", "or_group_no", "sort_order", "condition_no"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_condition_segment
    ON plugin_idc_core_marketing_segment_condition (
        "tenant_id", "segment_no", "or_group_no", "sort_order", "id"
    );

-- Purpose: Stores Marketing Condition Value records.
-- 用途：保存营销条件值相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_condition_value (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "condition_no" VARCHAR(40) NOT NULL,
    "value_text" VARCHAR(128),
    "value_integer" BIGINT,
    "sort_order" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_condition_value_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_condition_value_one CHECK (
        ("value_text" IS NOT NULL AND length(btrim("value_text")) BETWEEN 1 AND 128 AND
            "value_integer" IS NULL) OR
        ("value_text" IS NULL AND "value_integer" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_marketing_condition_value_sort CHECK ("sort_order" BETWEEN 0 AND 1000000),
    CONSTRAINT fk_idc_marketing_condition_value_parent FOREIGN KEY ("condition_no")
        REFERENCES plugin_idc_core_marketing_segment_condition ("condition_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_condition_value_text
    ON plugin_idc_core_marketing_condition_value ("condition_no", "value_text")
    WHERE "value_text" IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_condition_value_int
    ON plugin_idc_core_marketing_condition_value ("condition_no", "value_integer")
    WHERE "value_integer" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_marketing_condition_value_list
    ON plugin_idc_core_marketing_condition_value (
        "tenant_id", "condition_no", "sort_order", "id"
    );

-- Purpose: Stores Marketing Segment Member records.
-- 用途：保存营销分群成员相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_segment_member (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "segment_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(16) NOT NULL,
    "segment_version" BIGINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "evaluated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_segment_member_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_segment_member_source CHECK (
        "source_code" IN ('manual', 'rule', 'migration')
    ),
    CONSTRAINT ck_idc_marketing_segment_member_version CHECK ("segment_version" > 0),
    CONSTRAINT ck_idc_marketing_segment_member_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_idc_marketing_segment_member_revoked CHECK (
        ("status_code" = 'active' AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT fk_idc_marketing_segment_member_segment FOREIGN KEY ("segment_no")
        REFERENCES plugin_idc_core_marketing_segment ("segment_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_segment_member_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_segment_member_active
    ON plugin_idc_core_marketing_segment_member ("segment_no", "customer_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_member_list
    ON plugin_idc_core_marketing_segment_member (
        "tenant_id", "segment_no", "status_code", "customer_no", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_member_customer
    ON plugin_idc_core_marketing_segment_member (
        "tenant_id", "customer_no", "status_code", "segment_no", "id"
    );

-- Purpose: Immutable target snapshot header used to freeze campaign audiences.
-- 用途：保存营销分群快照相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_segment_snapshot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "segment_no" VARCHAR(40) NOT NULL,
    "segment_version" BIGINT NOT NULL,
    "definition_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'building',
    "member_count" BIGINT NOT NULL DEFAULT 0,
    "content_digest" CHAR(64),
    "started_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_segment_snapshot_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_segment_snapshot_version CHECK ("segment_version" > 0),
    CONSTRAINT ck_idc_marketing_segment_snapshot_digest CHECK (
        "definition_digest" ~ '^[0-9a-f]{64}$' AND
        ("content_digest" IS NULL OR "content_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_idc_marketing_segment_snapshot_status CHECK (
        "status_code" IN ('building', 'complete', 'failed', 'superseded')
    ),
    CONSTRAINT ck_idc_marketing_segment_snapshot_count CHECK ("member_count" >= 0),
    CONSTRAINT ck_idc_marketing_segment_snapshot_terminal CHECK (
        ("status_code" = 'building' AND "completed_at" IS NULL AND "content_digest" IS NULL) OR
        ("status_code" = 'complete' AND "completed_at" >= "started_at" AND
            "content_digest" IS NOT NULL) OR
        ("status_code" IN ('failed', 'superseded') AND "completed_at" >= "started_at")
    ),
    CONSTRAINT uk_idc_marketing_segment_snapshot_no UNIQUE ("snapshot_no"),
    CONSTRAINT uk_idc_marketing_segment_snapshot_owner UNIQUE ("snapshot_no", "segment_no"),
    CONSTRAINT fk_idc_marketing_segment_snapshot_segment FOREIGN KEY ("segment_no")
        REFERENCES plugin_idc_core_marketing_segment ("segment_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_segment_snapshot_active
    ON plugin_idc_core_marketing_segment_snapshot ("segment_no")
    WHERE "status_code" = 'building';
CREATE INDEX IF NOT EXISTS idx_idc_marketing_segment_snapshot_latest
    ON plugin_idc_core_marketing_segment_snapshot (
        "tenant_id", "segment_no", "status_code", "started_at" DESC, "id" DESC
    );

-- Purpose: Stores Marketing Snapshot Member records.
-- 用途：保存营销快照成员相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_snapshot_member (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "snapshot_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_snapshot_member_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_idc_marketing_snapshot_member UNIQUE ("snapshot_no", "customer_no"),
    CONSTRAINT fk_idc_marketing_snapshot_member_snapshot FOREIGN KEY ("snapshot_no")
        REFERENCES plugin_idc_core_marketing_segment_snapshot ("snapshot_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_snapshot_member_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_snapshot_member_list
    ON plugin_idc_core_marketing_snapshot_member (
        "tenant_id", "snapshot_no", "customer_no", "id"
    );

-- Purpose: Versioned promotion campaign with database-enforced global reservation and consumption quota.
-- 用途：保存营销活动相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_campaign (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "campaign_no" VARCHAR(40) NOT NULL,
    "campaign_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "campaign_type_code" VARCHAR(24) NOT NULL,
    "target_segment_no" VARCHAR(40),
    "target_snapshot_no" VARCHAR(40),
    "starts_at" TIMESTAMPTZ NOT NULL,
    "ends_at" TIMESTAMPTZ NOT NULL,
    "priority" INT NOT NULL DEFAULT 1000,
    "stack_policy_code" VARCHAR(24) NOT NULL DEFAULT 'exclusive',
    "stack_group_code" VARCHAR(64) NOT NULL DEFAULT 'default',
    "global_quota_limit" BIGINT,
    "global_reserved_count" BIGINT NOT NULL DEFAULT 0,
    "global_consumed_count" BIGINT NOT NULL DEFAULT 0,
    "per_customer_quota_limit" INT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_campaign_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_campaign_code CHECK (
        "campaign_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_idc_marketing_campaign_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_campaign_type CHECK (
        "campaign_type_code" IN (
            'discount', 'coupon', 'voucher', 'points', 'recharge_bonus',
            'cashback', 'customer_price'
        )
    ),
    CONSTRAINT ck_idc_marketing_campaign_snapshot CHECK (
        "target_snapshot_no" IS NULL OR "target_segment_no" IS NOT NULL
    ),
    CONSTRAINT ck_idc_marketing_campaign_window CHECK ("ends_at" > "starts_at"),
    CONSTRAINT ck_idc_marketing_campaign_priority CHECK ("priority" BETWEEN 0 AND 1000000),
    CONSTRAINT ck_idc_marketing_campaign_stack CHECK (
        "stack_policy_code" IN ('exclusive', 'stackable', 'same_group_exclusive') AND
        "stack_group_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_idc_marketing_campaign_global_quota CHECK (
        "global_reserved_count" >= 0 AND "global_consumed_count" >= 0 AND
        (
            ("global_quota_limit" IS NULL AND "global_reserved_count" = 0 AND
                "global_consumed_count" = 0) OR
            ("global_quota_limit" > 0 AND
                "global_reserved_count" + "global_consumed_count" <= "global_quota_limit")
        )
    ),
    CONSTRAINT ck_idc_marketing_campaign_customer_quota CHECK (
        "per_customer_quota_limit" IS NULL OR "per_customer_quota_limit" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_idc_marketing_campaign_status CHECK (
        "status_code" IN ('draft', 'scheduled', 'active', 'paused', 'ended', 'archived')
    ),
    CONSTRAINT ck_idc_marketing_campaign_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_campaign_no UNIQUE ("campaign_no"),
    CONSTRAINT uk_idc_marketing_campaign_code UNIQUE ("campaign_code"),
    CONSTRAINT uk_idc_marketing_campaign_type UNIQUE ("campaign_no", "campaign_type_code"),
    CONSTRAINT uk_idc_marketing_campaign_customer_limit UNIQUE (
        "campaign_no", "per_customer_quota_limit"
    ),
    CONSTRAINT fk_idc_marketing_campaign_segment FOREIGN KEY ("target_segment_no")
        REFERENCES plugin_idc_core_marketing_segment ("segment_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_campaign_snapshot FOREIGN KEY (
        "target_snapshot_no", "target_segment_no"
    ) REFERENCES plugin_idc_core_marketing_segment_snapshot (
        "snapshot_no", "segment_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_list
    ON plugin_idc_core_marketing_campaign (
        "tenant_id", "status_code", "starts_at", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_active
    ON plugin_idc_core_marketing_campaign (
        "tenant_id", "campaign_type_code", "status_code", "starts_at", "ends_at", "priority", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_workbench
    ON plugin_idc_core_marketing_campaign (
        "tenant_id", "status_code", "campaign_type_code", "updated_at" DESC, "id" DESC
    );

-- Purpose: Stores Marketing Campaign Scope records.
-- 用途：保存营销活动范围相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_campaign_scope (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "campaign_no" VARCHAR(40) NOT NULL,
    "scope_kind_code" VARCHAR(24) NOT NULL,
    "match_mode_code" VARCHAR(16) NOT NULL DEFAULT 'include',
    "reference_code" VARCHAR(128) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_campaign_scope_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_campaign_scope_kind CHECK (
        "scope_kind_code" IN ('all_products', 'product', 'category')
    ),
    CONSTRAINT ck_idc_marketing_campaign_scope_match CHECK (
        "match_mode_code" IN ('include', 'exclude')
    ),
    CONSTRAINT ck_idc_marketing_campaign_scope_reference CHECK (
        ("scope_kind_code" = 'all_products' AND "match_mode_code" = 'include' AND "reference_code" = '') OR
        ("scope_kind_code" <> 'all_products' AND
            length(btrim("reference_code")) BETWEEN 1 AND 128)
    ),
    CONSTRAINT uk_idc_marketing_campaign_scope UNIQUE (
        "campaign_no", "scope_kind_code", "match_mode_code", "reference_code"
    ),
    CONSTRAINT fk_idc_marketing_campaign_scope_campaign FOREIGN KEY ("campaign_no")
        REFERENCES plugin_idc_core_marketing_campaign ("campaign_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_scope_list
    ON plugin_idc_core_marketing_campaign_scope (
        "tenant_id", "campaign_no", "scope_kind_code", "id"
    );

-- Purpose: Typed per-currency monetary or points benefit semantics.
-- 用途：保存营销权益规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_benefit_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "benefit_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "campaign_type_code" VARCHAR(24) NOT NULL,
    "benefit_type_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3),
    "amount_minor" BIGINT,
    "rate_basis_points" INT,
    "maximum_discount_minor" BIGINT,
    "minimum_spend_minor" BIGINT,
    "points_quantity" BIGINT,
    "purchase_phase_code" VARCHAR(16) NOT NULL DEFAULT 'both',
    "cycle_scope_code" VARCHAR(16) NOT NULL DEFAULT 'first',
    "cycle_count" INT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_benefit_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_benefit_type CHECK (
        "benefit_type_code" IN (
            'fixed_discount', 'percentage_discount', 'voucher', 'points',
            'recharge_bonus', 'cashback', 'customer_price'
        )
    ),
    CONSTRAINT ck_idc_marketing_benefit_campaign_type CHECK (
        ("campaign_type_code" IN ('discount', 'coupon') AND
            "benefit_type_code" IN ('fixed_discount', 'percentage_discount')) OR
        ("campaign_type_code" = 'voucher' AND "benefit_type_code" = 'voucher') OR
        ("campaign_type_code" = 'points' AND "benefit_type_code" = 'points') OR
        ("campaign_type_code" = 'recharge_bonus' AND
            "benefit_type_code" = 'recharge_bonus') OR
        ("campaign_type_code" = 'cashback' AND "benefit_type_code" = 'cashback') OR
        ("campaign_type_code" = 'customer_price' AND
            "benefit_type_code" = 'customer_price')
    ),
    CONSTRAINT ck_idc_marketing_benefit_currency CHECK (
        ("benefit_type_code" = 'points' AND "currency_code" IS NULL) OR
        ("benefit_type_code" <> 'points' AND "currency_code" ~ '^[A-Z]{3}$')
    ),
    CONSTRAINT ck_idc_marketing_benefit_value CHECK (
        (
            "benefit_type_code" IN (
                'fixed_discount', 'voucher', 'recharge_bonus', 'cashback', 'customer_price'
            ) AND "amount_minor" > 0 AND "rate_basis_points" IS NULL AND
            "points_quantity" IS NULL AND "maximum_discount_minor" IS NULL
        ) OR (
            "benefit_type_code" = 'percentage_discount' AND "amount_minor" IS NULL AND
            "rate_basis_points" BETWEEN 1 AND 10000 AND "points_quantity" IS NULL AND
            ("maximum_discount_minor" IS NULL OR "maximum_discount_minor" > 0)
        ) OR (
            "benefit_type_code" = 'points' AND "amount_minor" IS NULL AND
            "rate_basis_points" IS NULL AND "maximum_discount_minor" IS NULL AND
            "minimum_spend_minor" IS NULL AND "points_quantity" > 0
        )
    ),
    CONSTRAINT ck_idc_marketing_benefit_minimum CHECK (
        "minimum_spend_minor" IS NULL OR "minimum_spend_minor" >= 0
    ),
    CONSTRAINT ck_idc_marketing_benefit_phase CHECK (
        "purchase_phase_code" IN ('new_purchase', 'renewal', 'both')
    ),
    CONSTRAINT ck_idc_marketing_benefit_cycle CHECK (
        ("cycle_scope_code" IN ('first', 'all') AND "cycle_count" IS NULL) OR
        ("cycle_scope_code" = 'limited' AND "cycle_count" BETWEEN 1 AND 1000000)
    ),
    CONSTRAINT ck_idc_marketing_benefit_status CHECK (
        "status_code" IN ('active', 'inactive')
    ),
    CONSTRAINT ck_idc_marketing_benefit_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_benefit_no UNIQUE ("benefit_no"),
    CONSTRAINT uk_idc_marketing_benefit_campaign UNIQUE ("campaign_no", "benefit_no"),
    CONSTRAINT uk_idc_marketing_benefit_type UNIQUE (
        "campaign_no", "benefit_no", "benefit_type_code"
    ),
    CONSTRAINT fk_idc_marketing_benefit_campaign FOREIGN KEY (
        "campaign_no", "campaign_type_code"
    ) REFERENCES plugin_idc_core_marketing_campaign (
        "campaign_no", "campaign_type_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_benefit_campaign
    ON plugin_idc_core_marketing_benefit_rule (
        "tenant_id", "campaign_no", "status_code", "currency_code", "id"
    );

-- Purpose: Stores Marketing Customer Quota records.
-- 用途：保存营销客户额度相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_customer_quota (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "campaign_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "quota_limit" INT NOT NULL,
    "reserved_count" INT NOT NULL DEFAULT 0,
    "consumed_count" INT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_customer_quota_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_customer_quota_count CHECK (
        "quota_limit" BETWEEN 1 AND 1000000 AND "reserved_count" >= 0 AND
        "consumed_count" >= 0 AND "reserved_count" + "consumed_count" <= "quota_limit"
    ),
    CONSTRAINT ck_idc_marketing_customer_quota_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_customer_quota UNIQUE ("campaign_no", "customer_no"),
    CONSTRAINT fk_idc_marketing_customer_quota_campaign FOREIGN KEY ("campaign_no", "quota_limit")
        REFERENCES plugin_idc_core_marketing_campaign (
            "campaign_no", "per_customer_quota_limit"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_customer_quota_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_customer_quota_customer
    ON plugin_idc_core_marketing_customer_quota (
        "tenant_id", "customer_no", "campaign_no", "id"
    );

-- Purpose: Exact quote-scoped campaign benefit reservation used for later consume, release, or expiry.
-- 用途：保存营销活动预留相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_campaign_reservation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reservation_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "benefit_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "quote_reference" VARCHAR(128) NOT NULL,
    "request_snapshot_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "reserved_units" INT NOT NULL DEFAULT 1,
    "quoted_value_minor" BIGINT NOT NULL DEFAULT 0,
    "benefit_snapshot_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_campaign_res_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_campaign_res_quote CHECK (
        length(btrim("quote_reference")) BETWEEN 8 AND 128
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_request CHECK (
        "request_snapshot_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_currency CHECK (
        "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_value CHECK (
        "reserved_units" BETWEEN 1 AND 1000000 AND "quoted_value_minor" >= 0 AND
        "benefit_snapshot_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_status CHECK (
        "status_code" IN ('active', 'consumed', 'released', 'expired')
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_terminal CHECK (
        ("status_code" = 'active' AND "consumed_at" IS NULL AND "released_at" IS NULL) OR
        ("status_code" = 'consumed' AND "consumed_at" >= "created_at" AND
            "released_at" IS NULL) OR
        ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL AND
            "released_at" >= "created_at")
    ),
    CONSTRAINT ck_idc_marketing_campaign_res_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT uk_idc_marketing_campaign_res_no UNIQUE ("reservation_no"),
    CONSTRAINT uk_idc_marketing_campaign_res_quote UNIQUE (
        "campaign_no", "customer_no", "quote_reference"
    ),
    CONSTRAINT fk_idc_marketing_campaign_res_benefit FOREIGN KEY (
        "campaign_no", "benefit_no"
    ) REFERENCES plugin_idc_core_marketing_benefit_rule (
        "campaign_no", "benefit_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_campaign_res_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_res_expiry
    ON plugin_idc_core_marketing_campaign_reservation (
        "tenant_id", "status_code", "expires_at", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_campaign_res_customer
    ON plugin_idc_core_marketing_campaign_reservation (
        "tenant_id", "customer_no", "status_code", "created_at" DESC, "id" DESC
    );

-- Purpose: Customer coupon or voucher with HMAC claim-code digest and bounded concurrent usage counters.
-- 用途：保存营销促销资产相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_promotion_asset (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "asset_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "benefit_no" VARCHAR(40) NOT NULL,
    "benefit_type_code" VARCHAR(24) NOT NULL,
    "asset_type_code" VARCHAR(16) NOT NULL,
    "customer_no" VARCHAR(40),
    "claim_code_digest" BYTEA,
    "benefit_snapshot_digest" CHAR(64) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "face_amount_minor" BIGINT,
    "rate_basis_points" INT,
    "usage_limit" INT NOT NULL DEFAULT 1,
    "reserved_uses" INT NOT NULL DEFAULT 0,
    "consumed_uses" INT NOT NULL DEFAULT 0,
    "valid_from" TIMESTAMPTZ NOT NULL,
    "valid_until" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'issued',
    "claimed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_asset_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_asset_type CHECK ("asset_type_code" IN ('coupon', 'voucher')),
    CONSTRAINT ck_idc_marketing_asset_benefit_type CHECK (
        ("asset_type_code" = 'coupon' AND
            "benefit_type_code" IN ('fixed_discount', 'percentage_discount')) OR
        ("asset_type_code" = 'voucher' AND "benefit_type_code" = 'voucher')
    ),
    CONSTRAINT ck_idc_marketing_asset_claim_digest CHECK (
        "claim_code_digest" IS NULL OR octet_length("claim_code_digest") = 32
    ),
    CONSTRAINT ck_idc_marketing_asset_snapshot CHECK (
        "benefit_snapshot_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_asset_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_marketing_asset_value CHECK (
        ("asset_type_code" = 'voucher' AND "face_amount_minor" > 0 AND
            "rate_basis_points" IS NULL) OR
        ("asset_type_code" = 'coupon' AND (
            ("face_amount_minor" > 0 AND "rate_basis_points" IS NULL) OR
            ("face_amount_minor" IS NULL AND "rate_basis_points" BETWEEN 1 AND 10000)
        ))
    ),
    CONSTRAINT ck_idc_marketing_asset_usage CHECK (
        "usage_limit" BETWEEN 1 AND 1000000 AND "reserved_uses" >= 0 AND
        "consumed_uses" >= 0 AND "reserved_uses" + "consumed_uses" <= "usage_limit"
    ),
    CONSTRAINT ck_idc_marketing_asset_window CHECK ("valid_until" > "valid_from"),
    CONSTRAINT ck_idc_marketing_asset_status CHECK (
        "status_code" IN ('issued', 'active', 'exhausted', 'expired', 'revoked')
    ),
    CONSTRAINT ck_idc_marketing_asset_lifecycle CHECK (
        ("status_code" = 'issued' AND "claim_code_digest" IS NOT NULL AND
            "reserved_uses" = 0 AND "consumed_uses" = 0) OR
        ("status_code" = 'active' AND
            ("reserved_uses" > 0 OR "consumed_uses" < "usage_limit")) OR
        ("status_code" = 'exhausted' AND "reserved_uses" = 0 AND
            "consumed_uses" = "usage_limit") OR
        ("status_code" IN ('expired', 'revoked') AND "reserved_uses" = 0)
    ),
    CONSTRAINT ck_idc_marketing_asset_customer CHECK (
        ("status_code" = 'issued' AND "customer_no" IS NULL AND "claimed_at" IS NULL) OR
        ("status_code" IN ('active', 'exhausted') AND
            "customer_no" IS NOT NULL AND "claimed_at" IS NOT NULL) OR
        ("status_code" IN ('expired', 'revoked') AND (
            ("customer_no" IS NULL AND "claimed_at" IS NULL) OR
            ("customer_no" IS NOT NULL AND "claimed_at" IS NOT NULL)
        ))
    ),
    CONSTRAINT uk_idc_marketing_asset_no UNIQUE ("asset_no"),
    CONSTRAINT uk_idc_marketing_asset_owner UNIQUE ("asset_no", "customer_no", "currency_code"),
    CONSTRAINT fk_idc_marketing_asset_benefit FOREIGN KEY (
        "campaign_no", "benefit_no", "benefit_type_code"
    )
        REFERENCES plugin_idc_core_marketing_benefit_rule (
            "campaign_no", "benefit_no", "benefit_type_code"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_asset_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_marketing_asset_claim_digest
    ON plugin_idc_core_marketing_promotion_asset ("claim_code_digest")
    WHERE "claim_code_digest" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_customer
    ON plugin_idc_core_marketing_promotion_asset (
        "tenant_id", "customer_no", "status_code", "valid_until", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_campaign
    ON plugin_idc_core_marketing_promotion_asset (
        "tenant_id", "campaign_no", "status_code", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_workbench
    ON plugin_idc_core_marketing_promotion_asset (
        "tenant_id", "status_code", "asset_type_code", "currency_code",
        "updated_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_customer_workbench
    ON plugin_idc_core_marketing_promotion_asset (
        "tenant_id", "customer_no", "status_code", "updated_at" DESC, "id" DESC
    );

-- Purpose: Stores Marketing Asset Reservation records.
-- 用途：保存营销资产预留相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_asset_reservation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "reservation_no" VARCHAR(40) NOT NULL,
    "asset_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "quote_reference" VARCHAR(128) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "reserved_uses" INT NOT NULL DEFAULT 1,
    "discount_amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "consumed_at" TIMESTAMPTZ,
    "released_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_asset_res_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_asset_res_quote CHECK (
        length(btrim("quote_reference")) BETWEEN 8 AND 128
    ),
    CONSTRAINT ck_idc_marketing_asset_res_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_marketing_asset_res_value CHECK (
        "reserved_uses" BETWEEN 1 AND 1000000 AND "discount_amount_minor" >= 0
    ),
    CONSTRAINT ck_idc_marketing_asset_res_status CHECK (
        "status_code" IN ('active', 'consumed', 'released', 'expired')
    ),
    CONSTRAINT ck_idc_marketing_asset_res_terminal CHECK (
        ("status_code" = 'active' AND "consumed_at" IS NULL AND "released_at" IS NULL) OR
        ("status_code" = 'consumed' AND "consumed_at" >= "created_at" AND
            "released_at" IS NULL) OR
        ("status_code" IN ('released', 'expired') AND "consumed_at" IS NULL AND
            "released_at" >= "created_at")
    ),
    CONSTRAINT ck_idc_marketing_asset_res_expiry CHECK ("expires_at" > "created_at"),
    CONSTRAINT uk_idc_marketing_asset_res_no UNIQUE ("reservation_no"),
    CONSTRAINT uk_idc_marketing_asset_res_quote UNIQUE ("asset_no", "quote_reference"),
    CONSTRAINT fk_idc_marketing_asset_res_asset FOREIGN KEY (
        "asset_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_marketing_promotion_asset (
        "asset_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_asset_res_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_asset_res_expiry
    ON plugin_idc_core_marketing_asset_reservation (
        "tenant_id", "status_code", "expires_at", "id"
    );

-- Purpose: Points account projection; points are not legal tender, customer balance, or withdrawable commission.
-- 用途：保存营销积分账户相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_points_account (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "available_points" BIGINT NOT NULL DEFAULT 0,
    "frozen_points" BIGINT NOT NULL DEFAULT 0,
    "debt_points" BIGINT NOT NULL DEFAULT 0,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_points_account_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_points_account_balance CHECK (
        "available_points" >= 0 AND "frozen_points" >= 0 AND "debt_points" >= 0
    ),
    CONSTRAINT ck_idc_marketing_points_account_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_marketing_points_account_customer UNIQUE ("customer_no"),
    CONSTRAINT fk_idc_marketing_points_account_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

-- Purpose: Expiring points lot with exact nonnegative conservation across available, frozen, consumed, expired, and reversed buckets.
-- 用途：保存营销积分批次相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_points_lot (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "lot_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "campaign_no" VARCHAR(40),
    "campaign_type_code" VARCHAR(24),
    "source_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "original_points" BIGINT NOT NULL,
    "available_points" BIGINT NOT NULL,
    "frozen_points" BIGINT NOT NULL DEFAULT 0,
    "consumed_points" BIGINT NOT NULL DEFAULT 0,
    "expired_points" BIGINT NOT NULL DEFAULT 0,
    "reversed_points" BIGINT NOT NULL DEFAULT 0,
    "reversal_shortfall_points" BIGINT NOT NULL DEFAULT 0,
    "expires_at" TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'available',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_points_lot_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_points_lot_source CHECK (
        "source_type_code" IN ('order', 'activity', 'manual', 'migration', 'reversal') AND
        length(btrim("source_reference")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_points_lot_campaign CHECK (
        ("campaign_no" IS NULL AND "campaign_type_code" IS NULL) OR
        ("campaign_no" IS NOT NULL AND "campaign_type_code" = 'points')
    ),
    CONSTRAINT ck_idc_marketing_points_lot_conservation CHECK (
        "original_points" > 0 AND "available_points" >= 0 AND "frozen_points" >= 0 AND
        "consumed_points" >= 0 AND "expired_points" >= 0 AND "reversed_points" >= 0 AND
        "reversal_shortfall_points" >= 0 AND
        "available_points" + "frozen_points" + "consumed_points" +
            "expired_points" + "reversed_points" = "original_points"
    ),
    CONSTRAINT ck_idc_marketing_points_lot_reversal_shortfall CHECK (
        "reversal_shortfall_points" <= "consumed_points" AND
        "reversed_points" + "reversal_shortfall_points" <= "original_points"
    ),
    CONSTRAINT ck_idc_marketing_points_lot_status CHECK (
        ("status_code" = 'available' AND "available_points" + "frozen_points" > 0) OR
        ("status_code" = 'exhausted' AND "available_points" = 0 AND
            "frozen_points" = 0 AND "consumed_points" > 0) OR
        ("status_code" = 'expired' AND "available_points" = 0 AND
            "frozen_points" = 0 AND "expired_points" > 0) OR
        ("status_code" = 'reversed' AND "available_points" = 0 AND
            "frozen_points" = 0 AND "reversed_points" > 0)
    ),
    CONSTRAINT uk_idc_marketing_points_lot_no UNIQUE ("lot_no"),
    CONSTRAINT uk_idc_marketing_points_lot_owner UNIQUE ("lot_no", "customer_no"),
    CONSTRAINT uk_idc_marketing_points_lot_source UNIQUE (
        "customer_no", "source_type_code", "source_reference"
    ),
    CONSTRAINT fk_idc_marketing_points_lot_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_points_lot_campaign FOREIGN KEY (
        "campaign_no", "campaign_type_code"
    ) REFERENCES plugin_idc_core_marketing_campaign (
        "campaign_no", "campaign_type_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_points_account_workbench
    ON plugin_idc_core_marketing_points_account (
        "tenant_id", "updated_at" DESC, "id" DESC
    );

CREATE INDEX IF NOT EXISTS idx_idc_marketing_points_lot_spend
    ON plugin_idc_core_marketing_points_lot (
        "tenant_id", "customer_no", "status_code", "expires_at", "created_at", "id"
    );

-- Purpose: Stores Marketing Points Entry records.
-- 用途：保存营销积分记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_points_entry (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entry_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "lot_no" VARCHAR(40),
    "entry_type_code" VARCHAR(24) NOT NULL,
    "available_delta" BIGINT NOT NULL DEFAULT 0,
    "frozen_delta" BIGINT NOT NULL DEFAULT 0,
    "debt_delta" BIGINT NOT NULL DEFAULT 0,
    "source_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "entry_line_no" INT NOT NULL DEFAULT 1,
    "reversal_of_entry_no" VARCHAR(40),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_points_entry_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_points_entry_type CHECK (
        "entry_type_code" IN (
            'earn', 'freeze', 'release', 'spend', 'expire', 'reverse', 'debt', 'debt_repay'
        )
    ),
    CONSTRAINT ck_idc_marketing_points_entry_delta CHECK (
        "available_delta" <> 0 OR "frozen_delta" <> 0 OR "debt_delta" <> 0
    ),
    CONSTRAINT ck_idc_marketing_points_entry_semantics CHECK (
        ("entry_type_code" = 'earn' AND "available_delta" > 0 AND
            "frozen_delta" = 0 AND "debt_delta" = 0) OR
        ("entry_type_code" = 'freeze' AND "available_delta" < 0 AND
            "frozen_delta" > 0 AND "available_delta" + "frozen_delta" = 0 AND
            "debt_delta" = 0) OR
        ("entry_type_code" = 'release' AND "available_delta" > 0 AND
            "frozen_delta" < 0 AND "available_delta" + "frozen_delta" = 0 AND
            "debt_delta" = 0) OR
        ("entry_type_code" IN ('spend', 'expire') AND "debt_delta" = 0 AND
            "available_delta" <= 0 AND "frozen_delta" <= 0 AND
            "available_delta" + "frozen_delta" < 0) OR
        ("entry_type_code" = 'debt' AND "available_delta" = 0 AND
            "frozen_delta" = 0 AND "debt_delta" > 0) OR
        ("entry_type_code" = 'debt_repay' AND "available_delta" = 0 AND
            "frozen_delta" = 0 AND "debt_delta" < 0) OR
        "entry_type_code" = 'reverse'
    ),
    CONSTRAINT ck_idc_marketing_points_entry_source CHECK (
        "source_type_code" IN (
            'order', 'refund', 'activity', 'redemption', 'manual', 'migration', 'expiry'
        ) AND length(btrim("source_reference")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_points_entry_line CHECK (
        "entry_line_no" BETWEEN 1 AND 1000000
    ),
    CONSTRAINT ck_idc_marketing_points_entry_reversal CHECK (
        ("entry_type_code" = 'reverse' AND "reversal_of_entry_no" IS NOT NULL) OR
        ("entry_type_code" <> 'reverse' AND "reversal_of_entry_no" IS NULL)
    ),
    CONSTRAINT uk_idc_marketing_points_entry_no UNIQUE ("entry_no"),
    CONSTRAINT uk_idc_marketing_points_entry_owner UNIQUE ("entry_no", "customer_no"),
    CONSTRAINT uk_idc_marketing_points_entry_source UNIQUE (
        "customer_no", "source_type_code", "source_reference", "entry_type_code", "entry_line_no"
    ),
    CONSTRAINT fk_idc_marketing_points_entry_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_points_entry_lot FOREIGN KEY ("lot_no", "customer_no")
        REFERENCES plugin_idc_core_marketing_points_lot (
            "lot_no", "customer_no"
        ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_points_entry_reversal FOREIGN KEY (
        "reversal_of_entry_no", "customer_no"
    ) REFERENCES plugin_idc_core_marketing_points_entry (
        "entry_no", "customer_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_points_entry_customer
    ON plugin_idc_core_marketing_points_entry (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_points_entry_reversal
    ON plugin_idc_core_marketing_points_entry (
        "tenant_id", "reversal_of_entry_no", "created_at", "id"
    ) WHERE "reversal_of_entry_no" IS NOT NULL;

-- Purpose: Marketing entitlement for recharge bonus or cashback; financial ledger posting remains externally owned.
-- 用途：保存营销值发放相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_value_grant (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "grant_no" VARCHAR(40) NOT NULL,
    "grant_type_code" VARCHAR(24) NOT NULL,
    "campaign_no" VARCHAR(40) NOT NULL,
    "campaign_type_code" VARCHAR(24) NOT NULL,
    "benefit_no" VARCHAR(40) NOT NULL,
    "benefit_type_code" VARCHAR(24) NOT NULL,
    "benefit_snapshot_digest" CHAR(64) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "original_amount_minor" BIGINT NOT NULL,
    "pending_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "available_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "frozen_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "consumed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "expired_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "reversed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "withdrawable" BOOLEAN NOT NULL DEFAULT FALSE,
    "global_quota_reserved" BOOLEAN NOT NULL DEFAULT FALSE,
    "customer_quota_reserved" BOOLEAN NOT NULL DEFAULT FALSE,
    "expires_at" TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "financial_posting_reference" VARCHAR(128),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_value_grant_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_value_grant_type CHECK (
        "grant_type_code" IN ('recharge_bonus', 'cashback') AND
        "campaign_type_code" = "grant_type_code" AND
        "benefit_type_code" = "grant_type_code"
    ),
    CONSTRAINT ck_idc_marketing_value_grant_digest CHECK (
        "benefit_snapshot_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_marketing_value_grant_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_marketing_value_grant_source CHECK (
        length(btrim("source_reference")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_value_grant_conservation CHECK (
        "original_amount_minor" > 0 AND "pending_amount_minor" >= 0 AND
        "available_amount_minor" >= 0 AND
        "frozen_amount_minor" >= 0 AND "consumed_amount_minor" >= 0 AND
        "expired_amount_minor" >= 0 AND "reversed_amount_minor" >= 0 AND
        "pending_amount_minor" + "available_amount_minor" + "frozen_amount_minor" +
            "consumed_amount_minor" + "expired_amount_minor" +
            "reversed_amount_minor" = "original_amount_minor"
    ),
    CONSTRAINT ck_idc_marketing_value_grant_withdrawal CHECK (
        "grant_type_code" <> 'recharge_bonus' OR "withdrawable" = FALSE
    ),
    CONSTRAINT ck_idc_marketing_value_grant_status CHECK (
        ("status_code" = 'pending' AND "pending_amount_minor" = "original_amount_minor") OR
        ("status_code" = 'available' AND "pending_amount_minor" = 0 AND
            "available_amount_minor" + "frozen_amount_minor" > 0) OR
        ("status_code" = 'exhausted' AND "pending_amount_minor" = 0 AND
            "available_amount_minor" = 0 AND "frozen_amount_minor" = 0 AND
            "consumed_amount_minor" > 0) OR
        ("status_code" = 'expired' AND "pending_amount_minor" = 0 AND
            "available_amount_minor" = 0 AND "frozen_amount_minor" = 0 AND
            "expired_amount_minor" > 0) OR
        ("status_code" = 'reversed' AND "pending_amount_minor" = 0 AND
            "available_amount_minor" = 0 AND "frozen_amount_minor" = 0 AND
            "reversed_amount_minor" > 0) OR
        ("status_code" = 'cancelled' AND "pending_amount_minor" = 0 AND
            "available_amount_minor" = 0 AND "frozen_amount_minor" = 0 AND
            "consumed_amount_minor" = 0 AND "expired_amount_minor" = 0 AND
            "reversed_amount_minor" = "original_amount_minor")
    ),
    CONSTRAINT ck_idc_marketing_value_grant_posting CHECK (
        ("status_code" = 'pending' AND "financial_posting_reference" IS NULL AND
            "pending_amount_minor" = "original_amount_minor") OR
        ("status_code" = 'cancelled' AND "financial_posting_reference" IS NULL AND
            "pending_amount_minor" = 0) OR
        ("status_code" NOT IN ('pending', 'cancelled') AND
            "pending_amount_minor" = 0 AND
            length(btrim("financial_posting_reference")) BETWEEN 1 AND 128)
    ),
    CONSTRAINT uk_idc_marketing_value_grant_no UNIQUE ("grant_no"),
    CONSTRAINT uk_idc_marketing_value_grant_owner UNIQUE (
        "grant_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_marketing_value_grant_source UNIQUE (
        "grant_type_code", "customer_no", "source_reference"
    ),
    CONSTRAINT fk_idc_marketing_value_grant_campaign FOREIGN KEY (
        "campaign_no", "campaign_type_code"
    ) REFERENCES plugin_idc_core_marketing_campaign (
        "campaign_no", "campaign_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_value_grant_benefit FOREIGN KEY (
        "campaign_no", "benefit_no", "benefit_type_code"
    ) REFERENCES plugin_idc_core_marketing_benefit_rule (
        "campaign_no", "benefit_no", "benefit_type_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_value_grant_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_value_grant_spend
    ON plugin_idc_core_marketing_value_grant (
        "tenant_id", "customer_no", "currency_code", "status_code", "expires_at", "id"
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_value_grant_workbench
    ON plugin_idc_core_marketing_value_grant (
        "tenant_id", "status_code", "grant_type_code", "currency_code",
        "updated_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_value_grant_customer_workbench
    ON plugin_idc_core_marketing_value_grant (
        "tenant_id", "customer_no", "status_code", "currency_code",
        "updated_at" DESC, "id" DESC
    );

-- Purpose: Stores Marketing Value Entry records.
-- 用途：保存营销值记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_marketing_value_entry (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entry_no" VARCHAR(40) NOT NULL,
    "grant_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "entry_type_code" VARCHAR(24) NOT NULL,
    "pending_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "available_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "frozen_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "consumed_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "expired_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "reversed_delta_minor" BIGINT NOT NULL DEFAULT 0,
    "source_type_code" VARCHAR(24) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "reversal_of_entry_no" VARCHAR(40),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_marketing_value_entry_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_marketing_value_entry_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_idc_marketing_value_entry_type CHECK (
        "entry_type_code" IN (
            'post', 'freeze', 'release', 'consume', 'expire', 'reverse', 'cancel'
        )
    ),
    CONSTRAINT ck_idc_marketing_value_entry_delta CHECK (
        "pending_delta_minor" <> 0 OR "available_delta_minor" <> 0 OR
            "frozen_delta_minor" <> 0 OR "consumed_delta_minor" <> 0 OR
            "expired_delta_minor" <> 0 OR "reversed_delta_minor" <> 0
    ),
    CONSTRAINT ck_idc_marketing_value_entry_semantics CHECK (
        ("entry_type_code" = 'post' AND "pending_delta_minor" < 0 AND
            "available_delta_minor" > 0 AND "frozen_delta_minor" = 0 AND
            "consumed_delta_minor" = 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" = 0 AND
            "pending_delta_minor" + "available_delta_minor" = 0) OR
        ("entry_type_code" = 'freeze' AND "pending_delta_minor" = 0 AND
            "available_delta_minor" < 0 AND "frozen_delta_minor" > 0 AND
            "consumed_delta_minor" = 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" = 0 AND
            "available_delta_minor" + "frozen_delta_minor" = 0) OR
        ("entry_type_code" = 'release' AND "pending_delta_minor" = 0 AND
            "available_delta_minor" > 0 AND "frozen_delta_minor" < 0 AND
            "consumed_delta_minor" = 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" = 0 AND
            "available_delta_minor" + "frozen_delta_minor" = 0) OR
        ("entry_type_code" = 'consume' AND "pending_delta_minor" = 0 AND
            "available_delta_minor" = 0 AND "frozen_delta_minor" < 0 AND
            "consumed_delta_minor" > 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" = 0 AND
            "frozen_delta_minor" + "consumed_delta_minor" = 0) OR
        ("entry_type_code" = 'expire' AND "pending_delta_minor" = 0 AND
            "available_delta_minor" <= 0 AND "frozen_delta_minor" <= 0 AND
            "consumed_delta_minor" = 0 AND "expired_delta_minor" > 0 AND
            "reversed_delta_minor" = 0 AND
            "available_delta_minor" + "frozen_delta_minor" +
                "expired_delta_minor" = 0) OR
        ("entry_type_code" = 'reverse' AND "pending_delta_minor" = 0 AND
            "available_delta_minor" <= 0 AND "frozen_delta_minor" <= 0 AND
            "consumed_delta_minor" <= 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" > 0 AND
            "available_delta_minor" + "frozen_delta_minor" +
                "consumed_delta_minor" + "reversed_delta_minor" = 0) OR
        ("entry_type_code" = 'cancel' AND "pending_delta_minor" < 0 AND
            "available_delta_minor" = 0 AND "frozen_delta_minor" = 0 AND
            "consumed_delta_minor" = 0 AND "expired_delta_minor" = 0 AND
            "reversed_delta_minor" > 0 AND
            "pending_delta_minor" + "reversed_delta_minor" = 0)
    ),
    CONSTRAINT ck_idc_marketing_value_entry_source CHECK (
        "source_type_code" IN ('order', 'refund', 'activity', 'manual', 'migration', 'expiry') AND
        length(btrim("source_reference")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_marketing_value_entry_reversal CHECK (
        ("entry_type_code" = 'reverse' AND "reversal_of_entry_no" IS NOT NULL) OR
        ("entry_type_code" <> 'reverse' AND "reversal_of_entry_no" IS NULL)
    ),
    CONSTRAINT uk_idc_marketing_value_entry_no UNIQUE ("entry_no"),
    CONSTRAINT uk_idc_marketing_value_entry_owner UNIQUE (
        "entry_no", "grant_no", "customer_no", "currency_code"
    ),
    CONSTRAINT uk_idc_marketing_value_entry_source UNIQUE (
        "grant_no", "source_type_code", "source_reference"
    ),
    CONSTRAINT fk_idc_marketing_value_entry_grant FOREIGN KEY (
        "grant_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_marketing_value_grant (
        "grant_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_value_entry_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_marketing_value_entry_reversal FOREIGN KEY (
        "reversal_of_entry_no", "grant_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_marketing_value_entry (
        "entry_no", "grant_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_marketing_value_entry_customer
    ON plugin_idc_core_marketing_value_entry (
        "tenant_id", "customer_no", "currency_code", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_idc_marketing_value_entry_reversal
    ON plugin_idc_core_marketing_value_entry (
        "tenant_id", "reversal_of_entry_no", "created_at", "id"
    ) WHERE "reversal_of_entry_no" IS NOT NULL;

COMMENT ON TABLE plugin_idc_core_marketing_segment
    IS 'Typed static or dynamic customer segment definition; existing customer groups remain independent';
COMMENT ON TABLE plugin_idc_core_marketing_segment_condition
    IS 'Typed segment predicate; OR groups contain ordered AND conditions without arbitrary executable JSON';
COMMENT ON TABLE plugin_idc_core_marketing_segment_snapshot
    IS 'Immutable target snapshot header used to freeze campaign audiences';
COMMENT ON TABLE plugin_idc_core_marketing_campaign
    IS 'Versioned promotion campaign with database-enforced global reservation and consumption quota';
COMMENT ON TABLE plugin_idc_core_marketing_campaign_reservation
    IS 'Exact quote-scoped campaign benefit reservation used for later consume, release, or expiry';
COMMENT ON TABLE plugin_idc_core_marketing_benefit_rule
    IS 'Typed per-currency monetary or points benefit semantics';
COMMENT ON TABLE plugin_idc_core_marketing_promotion_asset
    IS 'Customer coupon or voucher with HMAC claim-code digest and bounded concurrent usage counters';
COMMENT ON TABLE plugin_idc_core_marketing_points_account
    IS 'Points account projection; points are not legal tender, customer balance, or withdrawable commission';
COMMENT ON TABLE plugin_idc_core_marketing_points_lot
    IS 'Expiring points lot with exact nonnegative conservation across available, frozen, consumed, expired, and reversed buckets';
COMMENT ON COLUMN plugin_idc_core_marketing_points_lot."reversal_shortfall_points"
    IS 'Cumulative refunded award points already consumed and therefore represented as account debt';
COMMENT ON TABLE plugin_idc_core_marketing_value_grant
    IS 'Marketing entitlement for recharge bonus or cashback; financial ledger posting remains externally owned';
