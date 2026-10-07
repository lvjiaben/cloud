-- Stage 12 sales-channel foundation. Mutable projections are separated from
-- append-only qualification, attribution, commission, settlement, and payout evidence.

-- Purpose: Stores Sales Agent Level records.
-- 用途：保存销售代理等级相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_level (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "level_no" VARCHAR(40) NOT NULL,
    "level_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "sort_order" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_level CHECK (
        "tenant_id" = 0 AND "status_code" IN ('active', 'inactive') AND "version" > 0
    ),
    CONSTRAINT uk_plugin_idc_core_agent_level_no UNIQUE ("level_no"),
    CONSTRAINT uk_plugin_idc_core_agent_level_code UNIQUE ("tenant_id", "level_code")
);

-- Purpose: Stores Sales Agent Application records.
-- 用途：保存销售代理申请相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_application (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "application_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "requested_level_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'submitted',
    "application_digest" CHAR(64) NOT NULL,
    "submitted_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "reviewed_by" INT,
    "review_reason_code" VARCHAR(64),
    "reviewed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_application CHECK (
        "tenant_id" = 0 AND "application_digest" ~ '^[0-9a-f]{64}$' AND "version" > 0
        AND "status_code" IN ('submitted', 'approved', 'rejected', 'cancelled')
        AND (("status_code" = 'submitted' AND "reviewed_by" IS NULL AND "reviewed_at" IS NULL)
          OR ("status_code" = 'cancelled' AND "reviewed_by" IS NULL
              AND "review_reason_code" IS NULL AND "reviewed_at" IS NULL)
          OR ("status_code" IN ('approved', 'rejected') AND "reviewed_by" IS NOT NULL
              AND "review_reason_code" IS NOT NULL AND "reviewed_at" IS NOT NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_agent_application_no UNIQUE ("application_no"),
    CONSTRAINT fk_plugin_idc_core_agent_application_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_application_level FOREIGN KEY ("requested_level_no")
        REFERENCES plugin_idc_core_sales_agent_level ("level_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_application_list
    ON plugin_idc_core_sales_agent_application
       ("tenant_id", "status_code", "submitted_at" DESC, "id" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_application_customer
    ON plugin_idc_core_sales_agent_application
       ("tenant_id", "customer_no", "submitted_at" DESC, "id" DESC);

-- Purpose: Stores Sales Agent Qualification records.
-- 用途：保存销售代理资格相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_qualification (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "qualification_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "level_no" VARCHAR(40) NOT NULL,
    "source_application_no" VARCHAR(40),
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'active',
    "effective_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_qualification CHECK (
        "tenant_id" = 0 AND "status_code" IN ('active', 'suspended', 'expired', 'revoked')
        AND ("expires_at" IS NULL OR "expires_at" > "effective_at") AND "version" > 0
    ),
    CONSTRAINT uk_plugin_idc_core_agent_qualification_no UNIQUE ("qualification_no"),
    CONSTRAINT fk_plugin_idc_core_agent_qualification_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_qualification_level FOREIGN KEY ("level_no")
        REFERENCES plugin_idc_core_sales_agent_level ("level_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_qualification_application FOREIGN KEY ("source_application_no")
        REFERENCES plugin_idc_core_sales_agent_application ("application_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_agent_active
    ON plugin_idc_core_sales_agent_qualification ("tenant_id", "customer_no")
    WHERE "status_code" IN ('active', 'suspended');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_qualification_list
    ON plugin_idc_core_sales_agent_qualification
       ("tenant_id", "level_no", "status_code", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Agent Qualification Event records.
-- 用途：保存销售代理资格事件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_qualification_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "event_no" VARCHAR(40) NOT NULL,
    "qualification_no" VARCHAR(40) NOT NULL,
    "event_sequence" BIGINT NOT NULL,
    "from_status_code" VARCHAR(24),
    "to_status_code" VARCHAR(24) NOT NULL,
    "from_level_no" VARCHAR(40),
    "to_level_no" VARCHAR(40) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "actor_reference" VARCHAR(128) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_event CHECK (
        "tenant_id" = 0 AND "event_sequence" > 0 AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND "to_status_code" IN ('active', 'suspended', 'expired', 'revoked')
        AND ("from_status_code" IS NULL OR "from_status_code" IN ('active', 'suspended', 'expired', 'revoked'))
    ),
    CONSTRAINT uk_plugin_idc_core_agent_event_no UNIQUE ("event_no"),
    CONSTRAINT uk_plugin_idc_core_agent_event_seq UNIQUE ("qualification_no", "event_sequence"),
    CONSTRAINT fk_plugin_idc_core_agent_event_qualification FOREIGN KEY ("qualification_no")
        REFERENCES plugin_idc_core_sales_agent_qualification ("qualification_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_event_from_level FOREIGN KEY ("from_level_no")
        REFERENCES plugin_idc_core_sales_agent_level ("level_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_event_to_level FOREIGN KEY ("to_level_no")
        REFERENCES plugin_idc_core_sales_agent_level ("level_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_event_history
    ON plugin_idc_core_sales_agent_qualification_event
       ("tenant_id", "qualification_no", "event_sequence" DESC);

-- Purpose: Stores Sales Agent Credit Limit records.
-- 用途：保存销售代理信用限制相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_credit_limit (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "limit_no" VARCHAR(40) NOT NULL,
    "qualification_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "limit_amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "effective_at" TIMESTAMPTZ NOT NULL,
    "expires_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_credit CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "limit_amount_minor" >= 0
        AND "status_code" IN ('active', 'superseded', 'expired', 'revoked')
        AND ("expires_at" IS NULL OR "expires_at" > "effective_at")
    ),
    CONSTRAINT uk_plugin_idc_core_agent_credit_no UNIQUE ("limit_no"),
    CONSTRAINT fk_plugin_idc_core_agent_credit_qualification FOREIGN KEY ("qualification_no")
        REFERENCES plugin_idc_core_sales_agent_qualification ("qualification_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_agent_credit_active
    ON plugin_idc_core_sales_agent_credit_limit
       ("tenant_id", "qualification_no", "currency_code") WHERE "status_code" = 'active';

-- Purpose: Stores Sales Agent Product Rule records.
-- 用途：保存销售代理商品规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_product_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "product_rule_no" VARCHAR(40) NOT NULL,
    "level_no" VARCHAR(40) NOT NULL,
    "product_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "effective_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "rule_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_product_rule CHECK (
        "tenant_id" = 0 AND "status_code" IN ('draft', 'published', 'superseded', 'retired')
        AND "version" > 0 AND "rule_digest" ~ '^[0-9a-f]{64}$'
        AND ("expires_at" IS NULL OR ("effective_at" IS NOT NULL AND "expires_at" > "effective_at"))
    ),
    CONSTRAINT uk_plugin_idc_core_agent_product_rule_no UNIQUE ("product_rule_no"),
    CONSTRAINT uk_plugin_idc_core_agent_product_rule_ver UNIQUE ("level_no", "product_no", "version"),
    CONSTRAINT fk_plugin_idc_core_agent_product_rule_level FOREIGN KEY ("level_no")
        REFERENCES plugin_idc_core_sales_agent_level ("level_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_agent_product_rule_product FOREIGN KEY ("product_no")
        REFERENCES plugin_idc_core_catalog_product ("product_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_product_lookup
    ON plugin_idc_core_sales_agent_product_rule
       ("tenant_id", "level_no", "product_no", "status_code", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Agent Price Rule records.
-- 用途：保存销售代理价格规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_agent_price_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "price_rule_no" VARCHAR(40) NOT NULL,
    "product_rule_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "adjustment_type_code" VARCHAR(24) NOT NULL,
    "fixed_price_minor" BIGINT,
    "discount_basis_points" INT,
    "minimum_margin_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "effective_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "rule_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_agent_price_rule CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "minimum_margin_minor" >= 0
        AND "status_code" IN ('draft', 'published', 'superseded', 'retired')
        AND "version" > 0 AND "rule_digest" ~ '^[0-9a-f]{64}$'
        AND (("adjustment_type_code" = 'fixed_price' AND "fixed_price_minor" IS NOT NULL
              AND "fixed_price_minor" >= 0 AND "discount_basis_points" IS NULL)
          OR ("adjustment_type_code" = 'discount_basis_points' AND "fixed_price_minor" IS NULL
              AND "discount_basis_points" BETWEEN 0 AND 10000))
        AND ("expires_at" IS NULL OR ("effective_at" IS NOT NULL AND "expires_at" > "effective_at"))
    ),
    CONSTRAINT uk_plugin_idc_core_agent_price_rule_no UNIQUE ("price_rule_no"),
    CONSTRAINT uk_plugin_idc_core_agent_price_rule_ver UNIQUE
        ("product_rule_no", "currency_code", "version"),
    CONSTRAINT fk_plugin_idc_core_agent_price_rule_product FOREIGN KEY ("product_rule_no")
        REFERENCES plugin_idc_core_sales_agent_product_rule ("product_rule_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_agent_price_lookup
    ON plugin_idc_core_sales_agent_price_rule
       ("tenant_id", "product_rule_no", "currency_code", "status_code", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Affiliate Link records.
-- 用途：保存销售推广关联相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_affiliate_link (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "link_no" VARCHAR(40) NOT NULL,
    "owner_customer_no" VARCHAR(40) NOT NULL,
    "code_digest" CHAR(64) NOT NULL,
    "attribution_policy_code" VARCHAR(16) NOT NULL,
    "attribution_window_seconds" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_affiliate_link CHECK (
        "tenant_id" = 0 AND "code_digest" ~ '^[0-9a-f]{64}$'
        AND "attribution_policy_code" IN ('first_touch', 'last_touch')
        AND "attribution_window_seconds" BETWEEN 60 AND 31536000
        AND "status_code" IN ('active', 'inactive', 'expired', 'revoked') AND "version" > 0
    ),
    CONSTRAINT uk_plugin_idc_core_affiliate_link_no UNIQUE ("link_no"),
    CONSTRAINT uk_plugin_idc_core_affiliate_link_owner UNIQUE ("link_no", "owner_customer_no"),
    CONSTRAINT uk_plugin_idc_core_affiliate_code UNIQUE ("tenant_id", "code_digest"),
    CONSTRAINT fk_plugin_idc_core_affiliate_owner FOREIGN KEY ("owner_customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_affiliate_link_owner
    ON plugin_idc_core_sales_affiliate_link
       ("tenant_id", "owner_customer_no", "status_code", "created_at" DESC, "id" DESC);

-- Purpose: Stores Sales Affiliate Touch records.
-- 用途：保存销售推广触达相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_affiliate_touch (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "touch_no" VARCHAR(40) NOT NULL,
    "link_no" VARCHAR(40) NOT NULL,
    "visitor_digest" CHAR(64) NOT NULL,
    "device_digest" CHAR(64),
    "ip_digest" CHAR(64),
    "evidence_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_plugin_idc_core_affiliate_touch CHECK (
        "tenant_id" = 0 AND "visitor_digest" ~ '^[0-9a-f]{64}$'
        AND ("device_digest" IS NULL OR "device_digest" ~ '^[0-9a-f]{64}$')
        AND ("ip_digest" IS NULL OR "ip_digest" ~ '^[0-9a-f]{64}$')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$' AND "expires_at" > "occurred_at"
    ),
    CONSTRAINT uk_plugin_idc_core_affiliate_touch_no UNIQUE ("touch_no"),
    CONSTRAINT uk_plugin_idc_core_affiliate_touch_link UNIQUE ("touch_no", "link_no"),
    CONSTRAINT fk_plugin_idc_core_affiliate_touch_link FOREIGN KEY ("link_no")
        REFERENCES plugin_idc_core_sales_affiliate_link ("link_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_affiliate_touch_lookup
    ON plugin_idc_core_sales_affiliate_touch
       ("tenant_id", "visitor_digest", "occurred_at" DESC, "id" DESC);

-- Purpose: Stores Sales Affiliate Attribution records.
-- 用途：保存销售推广归因相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_affiliate_attribution (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "attribution_no" VARCHAR(40) NOT NULL,
    "link_no" VARCHAR(40) NOT NULL,
    "touch_no" VARCHAR(40) NOT NULL,
    "referrer_customer_no" VARCHAR(40) NOT NULL,
    "referred_customer_no" VARCHAR(40) NOT NULL,
    "policy_code" VARCHAR(16) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'eligible',
    "risk_reason_code" VARCHAR(64),
    "evidence_digest" CHAR(64) NOT NULL,
    "attributed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ NOT NULL,
    CONSTRAINT ck_plugin_idc_core_affiliate_attribution CHECK (
        "tenant_id" = 0 AND "policy_code" IN ('first_touch', 'last_touch')
        AND "status_code" IN ('eligible', 'rejected_self_referral', 'rejected_risk', 'revoked')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$' AND "expires_at" > "attributed_at"
        AND (("status_code" = 'eligible' AND "referrer_customer_no" <> "referred_customer_no")
          OR ("status_code" <> 'eligible' AND "risk_reason_code" IS NOT NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_affiliate_attribution_no UNIQUE ("attribution_no"),
    CONSTRAINT fk_plugin_idc_core_affiliate_attribution_link FOREIGN KEY ("link_no", "referrer_customer_no")
        REFERENCES plugin_idc_core_sales_affiliate_link ("link_no", "owner_customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_affiliate_attribution_touch FOREIGN KEY ("touch_no", "link_no")
        REFERENCES plugin_idc_core_sales_affiliate_touch ("touch_no", "link_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_affiliate_referrer FOREIGN KEY ("referrer_customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_affiliate_referred FOREIGN KEY ("referred_customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_affiliate_attribution_owner
    ON plugin_idc_core_sales_affiliate_attribution
       ("tenant_id", "referrer_customer_no", "status_code", "attributed_at" DESC, "id" DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_affiliate_attribution_eligible
    ON plugin_idc_core_sales_affiliate_attribution ("referred_customer_no")
    WHERE "status_code" = 'eligible';

-- Purpose: Stores Sales Group records.
-- 用途：保存销售分组相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_group (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "group_no" VARCHAR(40) NOT NULL,
    "group_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_sales_group CHECK (
        "tenant_id" = 0 AND "status_code" IN ('active', 'inactive') AND "version" > 0
    ),
    CONSTRAINT uk_plugin_idc_core_sales_group_no UNIQUE ("group_no"),
    CONSTRAINT uk_plugin_idc_core_sales_group_code UNIQUE ("tenant_id", "group_code")
);

-- Purpose: Stores Sales Group Member records.
-- 用途：保存销售分组成员相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_group_member (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "membership_no" VARCHAR(40) NOT NULL,
    "group_no" VARCHAR(40) NOT NULL,
    "staff_user_id" INT NOT NULL,
    "role_code" VARCHAR(16) NOT NULL DEFAULT 'member',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "effective_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ended_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_sales_member CHECK (
        "tenant_id" = 0 AND "staff_user_id" > 0 AND "role_code" IN ('member', 'leader')
        AND "status_code" IN ('active', 'ended')
        AND (("status_code" = 'active' AND "ended_at" IS NULL)
          OR ("status_code" = 'ended' AND "ended_at" IS NOT NULL AND "ended_at" >= "effective_at"))
    ),
    CONSTRAINT uk_plugin_idc_core_sales_member_no UNIQUE ("membership_no"),
    CONSTRAINT fk_plugin_idc_core_sales_member_group FOREIGN KEY ("group_no")
        REFERENCES plugin_idc_core_sales_group ("group_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_sales_member_active
    ON plugin_idc_core_sales_group_member ("tenant_id", "group_no", "staff_user_id")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_sales_member_staff
    ON plugin_idc_core_sales_group_member
       ("tenant_id", "staff_user_id", "status_code", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Customer Assignment records.
-- 用途：保存销售客户分派相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_customer_assignment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "assignment_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "membership_no" VARCHAR(40) NOT NULL,
    "protection_expires_at" TIMESTAMPTZ,
    "effective_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ended_at" TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "source_code" VARCHAR(24) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_sales_assignment CHECK (
        "tenant_id" = 0 AND "source_code" IN ('manual', 'automatic', 'transfer', 'migration')
        AND "status_code" IN ('active', 'ended')
        AND (("status_code" = 'active' AND "ended_at" IS NULL)
          OR ("status_code" = 'ended' AND "ended_at" IS NOT NULL AND "ended_at" >= "effective_at"))
        AND ("protection_expires_at" IS NULL OR "protection_expires_at" > "effective_at")
    ),
    CONSTRAINT uk_plugin_idc_core_sales_assignment_no UNIQUE ("assignment_no"),
    CONSTRAINT fk_plugin_idc_core_sales_assignment_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_assignment_member FOREIGN KEY ("membership_no")
        REFERENCES plugin_idc_core_sales_group_member ("membership_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_sales_assignment_active
    ON plugin_idc_core_sales_customer_assignment ("tenant_id", "customer_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_sales_assignment_owner
    ON plugin_idc_core_sales_customer_assignment
       ("tenant_id", "membership_no", "status_code", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Customer Collaborator records.
-- 用途：保存销售客户协作人相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_customer_collaborator (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "collaboration_no" VARCHAR(40) NOT NULL,
    "assignment_no" VARCHAR(40) NOT NULL,
    "membership_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "effective_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ended_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_sales_collaboration CHECK (
        "tenant_id" = 0 AND "status_code" IN ('active', 'ended')
        AND (("status_code" = 'active' AND "ended_at" IS NULL)
          OR ("status_code" = 'ended' AND "ended_at" IS NOT NULL AND "ended_at" >= "effective_at"))
    ),
    CONSTRAINT uk_plugin_idc_core_sales_collaboration_no UNIQUE ("collaboration_no"),
    CONSTRAINT fk_plugin_idc_core_sales_collaboration_assignment FOREIGN KEY ("assignment_no")
        REFERENCES plugin_idc_core_sales_customer_assignment ("assignment_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_collaboration_member FOREIGN KEY ("membership_no")
        REFERENCES plugin_idc_core_sales_group_member ("membership_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_sales_collab_active
    ON plugin_idc_core_sales_customer_collaborator ("tenant_id", "assignment_no", "membership_no")
    WHERE "status_code" = 'active';

-- Purpose: Stores Sales Transfer Batch records.
-- 用途：保存销售转移批次相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_transfer_batch (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "batch_no" VARCHAR(40) NOT NULL,
    "source_membership_no" VARCHAR(40) NOT NULL,
    "target_membership_no" VARCHAR(40) NOT NULL,
    "requested_count" INT NOT NULL,
    "succeeded_count" INT NOT NULL DEFAULT 0,
    "failed_count" INT NOT NULL DEFAULT 0,
    "reason_code" VARCHAR(64) NOT NULL,
    "requested_by" INT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_core_sales_transfer_batch CHECK (
        "tenant_id" = 0 AND "source_membership_no" <> "target_membership_no"
        AND "requested_count" > 0 AND "succeeded_count" >= 0 AND "failed_count" >= 0
        AND "succeeded_count" + "failed_count" <= "requested_count" AND "requested_by" > 0
        AND "status_code" IN ('pending', 'processing', 'completed', 'partially_failed', 'failed')
        AND (("status_code" IN ('pending', 'processing') AND "completed_at" IS NULL)
          OR ("status_code" IN ('completed', 'partially_failed', 'failed') AND "completed_at" IS NOT NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_sales_transfer_batch_no UNIQUE ("batch_no"),
    CONSTRAINT fk_plugin_idc_core_sales_transfer_source FOREIGN KEY ("source_membership_no")
        REFERENCES plugin_idc_core_sales_group_member ("membership_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_transfer_target FOREIGN KEY ("target_membership_no")
        REFERENCES plugin_idc_core_sales_group_member ("membership_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_sales_transfer_list
    ON plugin_idc_core_sales_transfer_batch
       ("tenant_id", "status_code", "created_at" DESC, "id" DESC);

-- Purpose: Stores Sales Transfer Item records.
-- 用途：保存销售转移明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_transfer_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "item_no" VARCHAR(40) NOT NULL,
    "batch_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "old_assignment_no" VARCHAR(40) NOT NULL,
    "new_assignment_no" VARCHAR(40),
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "failure_reason_code" VARCHAR(64),
    "processed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_sales_transfer_item CHECK (
        "tenant_id" = 0 AND "status_code" IN ('pending', 'succeeded', 'failed')
        AND (("status_code" = 'pending' AND "processed_at" IS NULL AND "new_assignment_no" IS NULL)
          OR ("status_code" = 'succeeded' AND "processed_at" IS NOT NULL AND "new_assignment_no" IS NOT NULL)
          OR ("status_code" = 'failed' AND "processed_at" IS NOT NULL AND "failure_reason_code" IS NOT NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_sales_transfer_item_no UNIQUE ("item_no"),
    CONSTRAINT uk_plugin_idc_core_sales_transfer_customer UNIQUE ("batch_no", "customer_no"),
    CONSTRAINT fk_plugin_idc_core_sales_transfer_item_batch FOREIGN KEY ("batch_no")
        REFERENCES plugin_idc_core_sales_transfer_batch ("batch_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_transfer_item_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_transfer_item_old FOREIGN KEY ("old_assignment_no")
        REFERENCES plugin_idc_core_sales_customer_assignment ("assignment_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_sales_transfer_item_new FOREIGN KEY ("new_assignment_no")
        REFERENCES plugin_idc_core_sales_customer_assignment ("assignment_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_sales_transfer_items
    ON plugin_idc_core_sales_transfer_item ("tenant_id", "batch_no", "status_code", "id");

-- Purpose: Stores Sales Commission Rule records.
-- 用途：保存销售佣金规则相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_rule (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "rule_no" VARCHAR(40) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "operation_type_code" VARCHAR(16) NOT NULL,
    "product_no" VARCHAR(40),
    "currency_code" CHAR(3) NOT NULL,
    "calculation_type_code" VARCHAR(16) NOT NULL,
    "fixed_amount_minor" BIGINT,
    "rate_basis_points" INT,
    "cap_amount_minor" BIGINT,
    "maturity_days" INT NOT NULL DEFAULT 0,
    "priority" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'draft',
    "effective_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL,
    "rule_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_rule CHECK (
        "tenant_id" = 0 AND "beneficiary_type_code" IN ('agent', 'affiliate', 'sales')
        AND "operation_type_code" IN ('new_purchase', 'renewal', 'change', 'refund')
        AND "currency_code" ~ '^[A-Z]{3}$' AND "maturity_days" BETWEEN 0 AND 3650
        AND "status_code" IN ('draft', 'published', 'superseded', 'retired')
        AND "version" > 0 AND "rule_digest" ~ '^[0-9a-f]{64}$'
        AND ("cap_amount_minor" IS NULL OR "cap_amount_minor" >= 0)
        AND (("calculation_type_code" = 'fixed' AND "fixed_amount_minor" IS NOT NULL
              AND "fixed_amount_minor" >= 0 AND "rate_basis_points" IS NULL)
          OR ("calculation_type_code" = 'rate' AND "fixed_amount_minor" IS NULL
              AND "rate_basis_points" BETWEEN 0 AND 10000)
          OR ("calculation_type_code" = 'tiered' AND "fixed_amount_minor" IS NULL
              AND "rate_basis_points" IS NULL))
        AND ("expires_at" IS NULL OR ("effective_at" IS NOT NULL AND "expires_at" > "effective_at"))
    ),
    CONSTRAINT uk_plugin_idc_core_commission_rule_no UNIQUE ("rule_no"),
    CONSTRAINT fk_plugin_idc_core_commission_rule_product FOREIGN KEY ("product_no")
        REFERENCES plugin_idc_core_catalog_product ("product_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_commission_rule_global
    ON plugin_idc_core_sales_commission_rule
       ("beneficiary_type_code", "operation_type_code", "currency_code", "version")
    WHERE "product_no" IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_commission_rule_product
    ON plugin_idc_core_sales_commission_rule
       ("beneficiary_type_code", "operation_type_code", "product_no", "currency_code", "version")
    WHERE "product_no" IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_rule_lookup
    ON plugin_idc_core_sales_commission_rule
       ("tenant_id", "beneficiary_type_code", "operation_type_code", "product_no",
        "currency_code", "status_code", "priority", "effective_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Tier records.
-- 用途：保存销售佣金层级相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_tier (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "tier_no" VARCHAR(40) NOT NULL,
    "rule_no" VARCHAR(40) NOT NULL,
    "tier_order" INT NOT NULL,
    "lower_bound_minor" BIGINT NOT NULL,
    "upper_bound_minor" BIGINT,
    "rate_basis_points" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_tier CHECK (
        "tenant_id" = 0 AND "tier_order" > 0 AND "lower_bound_minor" >= 0
        AND ("upper_bound_minor" IS NULL OR "upper_bound_minor" > "lower_bound_minor")
        AND "rate_basis_points" BETWEEN 0 AND 10000
    ),
    CONSTRAINT uk_plugin_idc_core_commission_tier_no UNIQUE ("tier_no"),
    CONSTRAINT uk_plugin_idc_core_commission_tier_order UNIQUE ("rule_no", "tier_order"),
    CONSTRAINT uk_plugin_idc_core_commission_tier_lower UNIQUE ("rule_no", "lower_bound_minor"),
    CONSTRAINT fk_plugin_idc_core_commission_tier_rule FOREIGN KEY ("rule_no")
        REFERENCES plugin_idc_core_sales_commission_rule ("rule_no") ON DELETE RESTRICT
);

ALTER TABLE plugin_idc_core_commission_accrual
    ADD COLUMN IF NOT EXISTS "agent_qualification_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "affiliate_attribution_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "sales_assignment_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "commission_rule_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "commission_tier_no" VARCHAR(40),
    ADD COLUMN IF NOT EXISTS "operation_type_code" VARCHAR(16);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_plugin_idc_core_commission_currency'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT uk_plugin_idc_core_commission_currency UNIQUE ("commission_no", "currency_code");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uk_plugin_idc_core_commission_beneficiary'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT uk_plugin_idc_core_commission_beneficiary UNIQUE
            ("commission_no", "currency_code", "beneficiary_type_code", "beneficiary_reference");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plugin_idc_core_commission_agent'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT fk_plugin_idc_core_commission_agent FOREIGN KEY ("agent_qualification_no")
            REFERENCES plugin_idc_core_sales_agent_qualification ("qualification_no") ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plugin_idc_core_commission_affiliate'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT fk_plugin_idc_core_commission_affiliate FOREIGN KEY ("affiliate_attribution_no")
            REFERENCES plugin_idc_core_sales_affiliate_attribution ("attribution_no") ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plugin_idc_core_commission_sales'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT fk_plugin_idc_core_commission_sales FOREIGN KEY ("sales_assignment_no")
            REFERENCES plugin_idc_core_sales_customer_assignment ("assignment_no") ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plugin_idc_core_commission_rule'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT fk_plugin_idc_core_commission_rule FOREIGN KEY ("commission_rule_no")
            REFERENCES plugin_idc_core_sales_commission_rule ("rule_no") ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_plugin_idc_core_commission_tier'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT fk_plugin_idc_core_commission_tier FOREIGN KEY ("commission_tier_no")
            REFERENCES plugin_idc_core_sales_commission_tier ("tier_no") ON DELETE RESTRICT;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_plugin_idc_core_commission_source'
                   AND conrelid = 'plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual
            ADD CONSTRAINT ck_plugin_idc_core_commission_source CHECK (
                num_nonnulls("agent_qualification_no", "affiliate_attribution_no", "sales_assignment_no") <= 1
                AND ("operation_type_code" IS NULL OR "operation_type_code" IN ('new_purchase', 'renewal', 'change', 'refund'))
            );
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_maturity
    ON plugin_idc_core_commission_accrual
       ("tenant_id", "status_code", "matures_at", "currency_code", "id")
    WHERE "pending_amount_minor" > 0;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_beneficiary
    ON plugin_idc_core_commission_accrual
       ("tenant_id", "beneficiary_type_code", "beneficiary_reference", "currency_code", "status_code", "id");

-- Purpose: Independent per-currency commission projection; never a customer spending balance.
-- 用途：保存销售佣金账户相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_account (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "account_no" VARCHAR(40) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "credited_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "pending_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "available_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "frozen_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "paid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "reversed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_account CHECK (
        "tenant_id" = 0 AND "beneficiary_type_code" IN ('agent', 'affiliate', 'sales')
        AND length("beneficiary_reference") BETWEEN 1 AND 128
        AND "currency_code" ~ '^[A-Z]{3}$' AND "status_code" IN ('active', 'frozen', 'closed')
        AND "version" > 0 AND "credited_amount_minor" >= 0 AND "pending_amount_minor" >= 0
        AND "available_amount_minor" >= 0 AND "frozen_amount_minor" >= 0
        AND "paid_amount_minor" >= 0 AND "reversed_amount_minor" >= 0
        AND "pending_amount_minor" + "available_amount_minor" + "frozen_amount_minor"
            + "paid_amount_minor" + "reversed_amount_minor" = "credited_amount_minor"
    ),
    CONSTRAINT uk_plugin_idc_core_commission_account_no UNIQUE ("account_no"),
    CONSTRAINT uk_plugin_idc_core_commission_account_currency UNIQUE ("account_no", "currency_code"),
    CONSTRAINT uk_plugin_idc_core_commission_account_beneficiary UNIQUE
        ("account_no", "currency_code", "beneficiary_type_code", "beneficiary_reference"),
    CONSTRAINT uk_plugin_idc_core_commission_account_owner UNIQUE
        ("tenant_id", "beneficiary_type_code", "beneficiary_reference", "currency_code")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_account_list
    ON plugin_idc_core_sales_commission_account
       ("tenant_id", "beneficiary_type_code", "status_code", "currency_code", "updated_at" DESC, "id" DESC);

-- Purpose: Append-only commission bucket ledger with per-entry conservation proof.
-- 用途：保存销售佣金账户记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_account_entry (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entry_no" VARCHAR(40) NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "entry_sequence" BIGINT NOT NULL,
    "entry_type_code" VARCHAR(24) NOT NULL,
    "source_type_code" VARCHAR(32) NOT NULL,
    "source_reference" VARCHAR(128) NOT NULL,
    "source_digest" CHAR(64) NOT NULL,
    "delta_credited_minor" BIGINT NOT NULL,
    "delta_pending_minor" BIGINT NOT NULL,
    "delta_available_minor" BIGINT NOT NULL,
    "delta_frozen_minor" BIGINT NOT NULL,
    "delta_paid_minor" BIGINT NOT NULL,
    "delta_reversed_minor" BIGINT NOT NULL,
    "after_credited_minor" BIGINT NOT NULL,
    "after_pending_minor" BIGINT NOT NULL,
    "after_available_minor" BIGINT NOT NULL,
    "after_frozen_minor" BIGINT NOT NULL,
    "after_paid_minor" BIGINT NOT NULL,
    "after_reversed_minor" BIGINT NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_entry CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "entry_sequence" > 0
        AND "entry_type_code" IN ('accrual', 'maturity', 'reversal', 'settlement_freeze',
                                'settlement_release', 'payout', 'payout_recovery')
        AND "source_digest" ~ '^[0-9a-f]{64}$'
        AND "delta_pending_minor" + "delta_available_minor" + "delta_frozen_minor"
            + "delta_paid_minor" + "delta_reversed_minor" = "delta_credited_minor"
        AND "after_credited_minor" >= 0 AND "after_pending_minor" >= 0
        AND "after_available_minor" >= 0 AND "after_frozen_minor" >= 0
        AND "after_paid_minor" >= 0 AND "after_reversed_minor" >= 0
        AND "after_pending_minor" + "after_available_minor" + "after_frozen_minor"
            + "after_paid_minor" + "after_reversed_minor" = "after_credited_minor"
    ),
    CONSTRAINT uk_plugin_idc_core_commission_entry_no UNIQUE ("entry_no"),
    CONSTRAINT uk_plugin_idc_core_commission_entry_seq UNIQUE ("account_no", "entry_sequence"),
    CONSTRAINT uk_plugin_idc_core_commission_entry_source UNIQUE
        ("account_no", "source_type_code", "source_reference"),
    CONSTRAINT fk_plugin_idc_core_commission_entry_account FOREIGN KEY ("account_no", "currency_code")
        REFERENCES plugin_idc_core_sales_commission_account ("account_no", "currency_code") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_entry_history
    ON plugin_idc_core_sales_commission_account_entry
       ("tenant_id", "account_no", "entry_sequence" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_entry_source
    ON plugin_idc_core_sales_commission_account_entry
       ("tenant_id", "source_type_code", "source_reference", "id");

-- Purpose: Stores Sales Commission Maturity records.
-- 用途：保存销售佣金成熟相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_maturity (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "maturity_no" VARCHAR(40) NOT NULL,
    "commission_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "processed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_maturity CHECK (
        "tenant_id" = 0 AND "attempt_no" > 0 AND "amount_minor" > 0
        AND "status_code" IN ('matured', 'blocked', 'reversed')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_commission_maturity_no UNIQUE ("maturity_no"),
    CONSTRAINT uk_plugin_idc_core_commission_maturity_attempt UNIQUE ("commission_no", "attempt_no"),
    CONSTRAINT fk_plugin_idc_core_commission_maturity_accrual FOREIGN KEY
        ("commission_no", "currency_code", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_commission_accrual
        ("commission_no", "currency_code", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_commission_maturity_account FOREIGN KEY
        ("account_no", "currency_code", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_account
        ("account_no", "currency_code", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_commission_maturity_terminal
    ON plugin_idc_core_sales_commission_maturity ("commission_no")
    WHERE "status_code" IN ('matured', 'reversed');

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_maturity_list
    ON plugin_idc_core_sales_commission_maturity
       ("tenant_id", "status_code", "processed_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Payee Profile records.
-- 用途：保存销售佣金收款方资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_payee_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "profile_no" VARCHAR(40) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "payee_type_code" VARCHAR(24) NOT NULL,
    "protected_object_reference" VARCHAR(512) NOT NULL,
    "account_digest" CHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_verification',
    "version" BIGINT NOT NULL DEFAULT 1,
    "verified_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_payee CHECK (
        "tenant_id" = 0 AND "beneficiary_type_code" IN ('agent', 'affiliate', 'sales')
        AND "payee_type_code" IN ('bank_account', 'alipay', 'wechat_pay', 'manual')
        AND length("protected_object_reference") BETWEEN 1 AND 512
        AND "account_digest" ~ '^[0-9a-f]{64}$'
        AND "status_code" IN ('pending_verification', 'verified', 'suspended', 'revoked')
        AND "version" > 0
        AND (("status_code" = 'verified' AND "verified_at" IS NOT NULL) OR "status_code" <> 'verified')
    ),
    CONSTRAINT uk_plugin_idc_core_commission_payee_no UNIQUE ("profile_no"),
    CONSTRAINT uk_plugin_idc_core_commission_payee_beneficiary UNIQUE
        ("profile_no", "beneficiary_type_code", "beneficiary_reference")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_payee_owner
    ON plugin_idc_core_sales_commission_payee_profile
       ("tenant_id", "beneficiary_type_code", "beneficiary_reference", "status_code", "updated_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Settlement records.
-- 用途：保存销售佣金结算相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_settlement (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "settlement_no" VARCHAR(40) NOT NULL,
    "account_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "payee_profile_no" VARCHAR(40) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "requested_amount_minor" BIGINT NOT NULL,
    "frozen_amount_minor" BIGINT NOT NULL,
    "paid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "returned_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "minimum_amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    "request_digest" CHAR(64) NOT NULL,
    "requested_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_plugin_idc_core_commission_settlement CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$'
        AND "requested_amount_minor" > 0 AND "minimum_amount_minor" >= 0
        AND "requested_amount_minor" >= "minimum_amount_minor"
        AND "frozen_amount_minor" >= 0 AND "paid_amount_minor" >= 0 AND "returned_amount_minor" >= 0
        AND "frozen_amount_minor" + "paid_amount_minor" + "returned_amount_minor" = "requested_amount_minor"
        AND "status_code" IN ('pending_review', 'approved', 'batched', 'paid', 'rejected',
                            'failed', 'manual_review', 'cancelled')
        AND "request_digest" ~ '^[0-9a-f]{64}$' AND "version" > 0
        AND (("status_code" IN ('paid', 'rejected', 'failed', 'cancelled') AND "completed_at" IS NOT NULL)
          OR ("status_code" IN ('pending_review', 'approved', 'batched', 'manual_review') AND "completed_at" IS NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_no UNIQUE ("settlement_no"),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_currency UNIQUE ("settlement_no", "currency_code"),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_beneficiary UNIQUE
        ("settlement_no", "currency_code", "beneficiary_type_code", "beneficiary_reference"),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_request UNIQUE ("account_no", "request_digest"),
    CONSTRAINT fk_plugin_idc_core_commission_settlement_account FOREIGN KEY
        ("account_no", "currency_code", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_account
        ("account_no", "currency_code", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_commission_settlement_payee FOREIGN KEY
        ("payee_profile_no", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_payee_profile
        ("profile_no", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_settlement_list
    ON plugin_idc_core_sales_commission_settlement
       ("tenant_id", "status_code", "currency_code", "requested_at" DESC, "id" DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_settlement_account
    ON plugin_idc_core_sales_commission_settlement
       ("tenant_id", "account_no", "requested_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Settlement Item records.
-- 用途：保存销售佣金结算明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_settlement_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "item_no" VARCHAR(40) NOT NULL,
    "settlement_no" VARCHAR(40) NOT NULL,
    "commission_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "beneficiary_type_code" VARCHAR(16) NOT NULL,
    "beneficiary_reference" VARCHAR(128) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_settlement_item CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "amount_minor" > 0
    ),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_item_no UNIQUE ("item_no"),
    CONSTRAINT uk_plugin_idc_core_commission_settlement_accrual UNIQUE ("settlement_no", "commission_no"),
    CONSTRAINT fk_plugin_idc_core_commission_settlement_item_head FOREIGN KEY
        ("settlement_no", "currency_code", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_settlement
        ("settlement_no", "currency_code", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_commission_settlement_item_accrual FOREIGN KEY
        ("commission_no", "currency_code", "beneficiary_type_code", "beneficiary_reference")
        REFERENCES plugin_idc_core_commission_accrual
        ("commission_no", "currency_code", "beneficiary_type_code", "beneficiary_reference") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_settlement_items
    ON plugin_idc_core_sales_commission_settlement_item
       ("tenant_id", "settlement_no", "id");

-- Purpose: Stores Sales Commission Settlement Review records.
-- 用途：保存销售佣金结算复核相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_settlement_review (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "review_no" VARCHAR(40) NOT NULL,
    "settlement_no" VARCHAR(40) NOT NULL,
    "decision_code" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "reviewer_user_id" INT NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "reviewed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_review CHECK (
        "tenant_id" = 0 AND "decision_code" IN ('approved', 'rejected', 'manual_review')
        AND "reviewer_user_id" > 0 AND "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_commission_review_no UNIQUE ("review_no"),
    CONSTRAINT uk_plugin_idc_core_commission_review_settlement UNIQUE ("settlement_no"),
    CONSTRAINT fk_plugin_idc_core_commission_review_settlement FOREIGN KEY ("settlement_no")
        REFERENCES plugin_idc_core_sales_commission_settlement ("settlement_no") ON DELETE RESTRICT
);

-- Purpose: Stores Sales Commission Payout Batch records.
-- 用途：保存销售佣金结佣批次相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_payout_batch (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "batch_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "item_count" INT NOT NULL,
    "total_amount_minor" BIGINT NOT NULL,
    "succeeded_count" INT NOT NULL DEFAULT 0,
    "succeeded_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "failed_count" INT NOT NULL DEFAULT 0,
    "failed_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "created_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "completed_at" TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_core_commission_payout_batch CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "item_count" > 0
        AND "total_amount_minor" > 0 AND "succeeded_count" >= 0 AND "failed_count" >= 0
        AND "succeeded_amount_minor" >= 0 AND "failed_amount_minor" >= 0
        AND "succeeded_count" + "failed_count" <= "item_count"
        AND "succeeded_amount_minor" + "failed_amount_minor" <= "total_amount_minor"
        AND "status_code" IN ('pending', 'processing', 'succeeded', 'partially_failed', 'failed', 'manual_review')
        AND "created_by" > 0
        AND (("status_code" IN ('pending', 'processing', 'manual_review') AND "completed_at" IS NULL)
          OR ("status_code" IN ('succeeded', 'partially_failed', 'failed') AND "completed_at" IS NOT NULL))
    ),
    CONSTRAINT uk_plugin_idc_core_commission_payout_batch_no UNIQUE ("batch_no"),
    CONSTRAINT uk_plugin_idc_core_commission_payout_currency UNIQUE ("batch_no", "currency_code")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_payout_list
    ON plugin_idc_core_sales_commission_payout_batch
       ("tenant_id", "status_code", "currency_code", "created_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Payout Item records.
-- 用途：保存销售佣金结佣明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_payout_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "item_no" VARCHAR(40) NOT NULL,
    "batch_no" VARCHAR(40) NOT NULL,
    "settlement_no" VARCHAR(40) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "amount_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "attempt_no" INT NOT NULL DEFAULT 0,
    "failure_reason_code" VARCHAR(64),
    "processed_at" TIMESTAMPTZ,
    CONSTRAINT ck_plugin_idc_core_commission_payout_item CHECK (
        "tenant_id" = 0 AND "currency_code" ~ '^[A-Z]{3}$' AND "amount_minor" > 0
        AND "attempt_no" >= 0 AND "status_code" IN ('pending', 'processing', 'succeeded',
                                                'failed', 'result_unknown', 'manual_review')
        AND ("status_code" NOT IN ('failed', 'manual_review') OR "failure_reason_code" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_core_commission_payout_item_no UNIQUE ("item_no"),
    CONSTRAINT uk_plugin_idc_core_commission_payout_settlement UNIQUE ("settlement_no"),
    CONSTRAINT fk_plugin_idc_core_commission_payout_item_batch FOREIGN KEY ("batch_no", "currency_code")
        REFERENCES plugin_idc_core_sales_commission_payout_batch ("batch_no", "currency_code") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_commission_payout_item_settlement FOREIGN KEY ("settlement_no", "currency_code")
        REFERENCES plugin_idc_core_sales_commission_settlement ("settlement_no", "currency_code") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_payout_items
    ON plugin_idc_core_sales_commission_payout_item
       ("tenant_id", "batch_no", "status_code", "id");

-- Purpose: Append-only payout evidence containing only protected object references and digests.
-- 用途：保存销售佣金结佣证据相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_payout_evidence (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "evidence_no" VARCHAR(40) NOT NULL,
    "payout_item_no" VARCHAR(40) NOT NULL,
    "evidence_type_code" VARCHAR(24) NOT NULL,
    "protected_object_reference" VARCHAR(512),
    "evidence_digest" CHAR(64) NOT NULL,
    "provider_reference_digest" CHAR(64),
    "observed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_payout_evidence CHECK (
        "tenant_id" = 0 AND "evidence_type_code" IN ('receipt', 'provider_result', 'manual_proof', 'query_result')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND ("provider_reference_digest" IS NULL OR "provider_reference_digest" ~ '^[0-9a-f]{64}$')
        AND ("protected_object_reference" IS NULL OR length("protected_object_reference") BETWEEN 1 AND 512)
    ),
    CONSTRAINT uk_plugin_idc_core_commission_payout_evidence_no UNIQUE ("evidence_no"),
    CONSTRAINT fk_plugin_idc_core_commission_payout_evidence_item FOREIGN KEY ("payout_item_no")
        REFERENCES plugin_idc_core_sales_commission_payout_item ("item_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_payout_evidence
    ON plugin_idc_core_sales_commission_payout_evidence
       ("tenant_id", "payout_item_no", "observed_at" DESC, "id" DESC);

-- Purpose: Stores Sales Commission Payout Recovery records.
-- 用途：保存销售佣金结佣恢复相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_sales_commission_payout_recovery (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "recovery_no" VARCHAR(40) NOT NULL,
    "payout_item_no" VARCHAR(40) NOT NULL,
    "attempt_no" INT NOT NULL,
    "action_code" VARCHAR(24) NOT NULL,
    "result_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "next_attempt_at" TIMESTAMPTZ,
    "recorded_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_commission_payout_recovery CHECK (
        "tenant_id" = 0 AND "attempt_no" > 0
        AND "action_code" IN ('query', 'release', 'retry', 'manual_takeover')
        AND "result_code" IN ('pending', 'retry_ready', 'recovered', 'result_unknown', 'manual_review')
        AND "evidence_digest" ~ '^[0-9a-f]{64}$'
        AND ("result_code" <> 'retry_ready' OR "next_attempt_at" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_core_commission_recovery_no UNIQUE ("recovery_no"),
    CONSTRAINT uk_plugin_idc_core_commission_recovery_attempt UNIQUE ("payout_item_no", "attempt_no"),
    CONSTRAINT fk_plugin_idc_core_commission_recovery_item FOREIGN KEY ("payout_item_no")
        REFERENCES plugin_idc_core_sales_commission_payout_item ("item_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_commission_recovery_due
    ON plugin_idc_core_sales_commission_payout_recovery
       ("tenant_id", "result_code", "next_attempt_at", "id")
    WHERE "result_code" IN ('pending', 'retry_ready', 'result_unknown');

-- Normalize schemas that already executed an earlier draft of migration 072.
ALTER TABLE plugin_idc_core_sales_commission_maturity
    ADD COLUMN IF NOT EXISTS "beneficiary_type_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "beneficiary_reference" VARCHAR(128);
ALTER TABLE plugin_idc_core_sales_commission_settlement
    ADD COLUMN IF NOT EXISTS "beneficiary_type_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "beneficiary_reference" VARCHAR(128);
ALTER TABLE plugin_idc_core_sales_commission_settlement_item
    ADD COLUMN IF NOT EXISTS "beneficiary_type_code" VARCHAR(16),
    ADD COLUMN IF NOT EXISTS "beneficiary_reference" VARCHAR(128);
UPDATE plugin_idc_core_sales_commission_maturity m
   SET "beneficiary_type_code"=a."beneficiary_type_code",
       "beneficiary_reference"=a."beneficiary_reference"
  FROM plugin_idc_core_commission_accrual a
 WHERE m."commission_no"=a."commission_no"
   AND (m."beneficiary_type_code" IS NULL OR m."beneficiary_reference" IS NULL);
UPDATE plugin_idc_core_sales_commission_settlement s
   SET "beneficiary_type_code"=a."beneficiary_type_code",
       "beneficiary_reference"=a."beneficiary_reference"
  FROM plugin_idc_core_sales_commission_account a
 WHERE s."account_no"=a."account_no"
   AND (s."beneficiary_type_code" IS NULL OR s."beneficiary_reference" IS NULL);
UPDATE plugin_idc_core_sales_commission_settlement_item i
   SET "beneficiary_type_code"=s."beneficiary_type_code",
       "beneficiary_reference"=s."beneficiary_reference"
  FROM plugin_idc_core_sales_commission_settlement s
 WHERE i."settlement_no"=s."settlement_no"
   AND (i."beneficiary_type_code" IS NULL OR i."beneficiary_reference" IS NULL);
ALTER TABLE plugin_idc_core_sales_commission_maturity
    ALTER COLUMN "beneficiary_type_code" SET NOT NULL,
    ALTER COLUMN "beneficiary_reference" SET NOT NULL;
ALTER TABLE plugin_idc_core_sales_commission_settlement
    ALTER COLUMN "beneficiary_type_code" SET NOT NULL,
    ALTER COLUMN "beneficiary_reference" SET NOT NULL;
ALTER TABLE plugin_idc_core_sales_commission_settlement_item
    ALTER COLUMN "beneficiary_type_code" SET NOT NULL,
    ALTER COLUMN "beneficiary_reference" SET NOT NULL;

ALTER TABLE plugin_idc_core_sales_affiliate_attribution
    DROP CONSTRAINT IF EXISTS uk_plugin_idc_core_affiliate_attribution_customer,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_affiliate_attribution_link,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_affiliate_attribution_touch;
ALTER TABLE plugin_idc_core_sales_commission_maturity
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_maturity_accrual,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_maturity_account;
ALTER TABLE plugin_idc_core_sales_commission_settlement
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_settlement_account,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_settlement_payee;
ALTER TABLE plugin_idc_core_sales_commission_settlement_item
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_settlement_item_head,
    DROP CONSTRAINT IF EXISTS fk_plugin_idc_core_commission_settlement_item_accrual;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_affiliate_link_owner'
                   AND conrelid='plugin_idc_core_sales_affiliate_link'::regclass) THEN
        ALTER TABLE plugin_idc_core_sales_affiliate_link ADD CONSTRAINT
            uk_plugin_idc_core_affiliate_link_owner UNIQUE ("link_no","owner_customer_no");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_affiliate_touch_link'
                   AND conrelid='plugin_idc_core_sales_affiliate_touch'::regclass) THEN
        ALTER TABLE plugin_idc_core_sales_affiliate_touch ADD CONSTRAINT
            uk_plugin_idc_core_affiliate_touch_link UNIQUE ("touch_no","link_no");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_commission_beneficiary'
                   AND conrelid='plugin_idc_core_commission_accrual'::regclass) THEN
        ALTER TABLE plugin_idc_core_commission_accrual ADD CONSTRAINT
            uk_plugin_idc_core_commission_beneficiary UNIQUE
            ("commission_no","currency_code","beneficiary_type_code","beneficiary_reference");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_commission_account_beneficiary'
                   AND conrelid='plugin_idc_core_sales_commission_account'::regclass) THEN
        ALTER TABLE plugin_idc_core_sales_commission_account ADD CONSTRAINT
            uk_plugin_idc_core_commission_account_beneficiary UNIQUE
            ("account_no","currency_code","beneficiary_type_code","beneficiary_reference");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_commission_payee_beneficiary'
                   AND conrelid='plugin_idc_core_sales_commission_payee_profile'::regclass) THEN
        ALTER TABLE plugin_idc_core_sales_commission_payee_profile ADD CONSTRAINT
            uk_plugin_idc_core_commission_payee_beneficiary UNIQUE
            ("profile_no","beneficiary_type_code","beneficiary_reference");
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname='uk_plugin_idc_core_commission_settlement_beneficiary'
                   AND conrelid='plugin_idc_core_sales_commission_settlement'::regclass) THEN
        ALTER TABLE plugin_idc_core_sales_commission_settlement ADD CONSTRAINT
            uk_plugin_idc_core_commission_settlement_beneficiary UNIQUE
            ("settlement_no","currency_code","beneficiary_type_code","beneficiary_reference");
    END IF;
END $$;

ALTER TABLE plugin_idc_core_sales_affiliate_attribution
    ADD CONSTRAINT fk_plugin_idc_core_affiliate_attribution_link FOREIGN KEY
        ("link_no","referrer_customer_no") REFERENCES plugin_idc_core_sales_affiliate_link
        ("link_no","owner_customer_no") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_plugin_idc_core_affiliate_attribution_touch FOREIGN KEY
        ("touch_no","link_no") REFERENCES plugin_idc_core_sales_affiliate_touch
        ("touch_no","link_no") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_sales_commission_maturity
    ADD CONSTRAINT fk_plugin_idc_core_commission_maturity_accrual FOREIGN KEY
        ("commission_no","currency_code","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_commission_accrual
        ("commission_no","currency_code","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_plugin_idc_core_commission_maturity_account FOREIGN KEY
        ("account_no","currency_code","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_account
        ("account_no","currency_code","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_sales_commission_settlement
    ADD CONSTRAINT fk_plugin_idc_core_commission_settlement_account FOREIGN KEY
        ("account_no","currency_code","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_account
        ("account_no","currency_code","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_plugin_idc_core_commission_settlement_payee FOREIGN KEY
        ("payee_profile_no","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_payee_profile
        ("profile_no","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT;
ALTER TABLE plugin_idc_core_sales_commission_settlement_item
    ADD CONSTRAINT fk_plugin_idc_core_commission_settlement_item_head FOREIGN KEY
        ("settlement_no","currency_code","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_sales_commission_settlement
        ("settlement_no","currency_code","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT,
    ADD CONSTRAINT fk_plugin_idc_core_commission_settlement_item_accrual FOREIGN KEY
        ("commission_no","currency_code","beneficiary_type_code","beneficiary_reference")
        REFERENCES plugin_idc_core_commission_accrual
        ("commission_no","currency_code","beneficiary_type_code","beneficiary_reference") ON DELETE RESTRICT;

-- Evidence and ledger rows are immutable. Corrections are new rows that point
-- at the original fact; PostgreSQL 14 supports these ordinary trigger forms.
CREATE OR REPLACE FUNCTION plugin_idc_core_sales_reject_append_only_mutation()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only', TG_TABLE_NAME USING ERRCODE = '55000';
END $$;

DO $$
DECLARE
    table_name TEXT;
    trigger_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'plugin_idc_core_sales_agent_qualification_event',
        'plugin_idc_core_sales_affiliate_touch',
        'plugin_idc_core_sales_affiliate_attribution',
        'plugin_idc_core_sales_commission_account_entry',
        'plugin_idc_core_sales_commission_maturity',
        'plugin_idc_core_sales_commission_settlement_item',
        'plugin_idc_core_sales_commission_settlement_review',
        'plugin_idc_core_sales_commission_payout_evidence',
        'plugin_idc_core_sales_commission_payout_recovery'
    ] LOOP
        trigger_name := replace(table_name, 'plugin_idc_core_sales_', 'trg_idc_sales_') || '_append_only';
        EXECUTE format('DROP TRIGGER IF EXISTS %I ON %I', trigger_name, table_name);
        EXECUTE format(
            'CREATE TRIGGER %I BEFORE UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_reject_append_only_mutation()',
            trigger_name, table_name
        );
    END LOOP;
END $$;

-- An entry owns the only legal projection advance. The account row lock makes
-- sequence and after-values authoritative under concurrent writers.
CREATE OR REPLACE FUNCTION plugin_idc_core_sales_apply_commission_entry()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    account_row plugin_idc_core_sales_commission_account%ROWTYPE;
    previous_sequence BIGINT;
BEGIN
    SELECT * INTO account_row
      FROM plugin_idc_core_sales_commission_account
     WHERE "account_no" = NEW."account_no" AND "currency_code" = NEW."currency_code"
     FOR UPDATE;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'commission account not found' USING ERRCODE = '23503';
    END IF;
    SELECT COALESCE(MAX("entry_sequence"), 0) INTO previous_sequence
      FROM plugin_idc_core_sales_commission_account_entry
     WHERE "account_no" = NEW."account_no" AND "id" <> NEW."id";
    IF NEW."entry_sequence" <> previous_sequence + 1 THEN
        RAISE EXCEPTION 'commission entry sequence is not continuous' USING ERRCODE = '23514';
    END IF;
    IF NEW."after_credited_minor" <> account_row."credited_amount_minor" + NEW."delta_credited_minor"
       OR NEW."after_pending_minor" <> account_row."pending_amount_minor" + NEW."delta_pending_minor"
       OR NEW."after_available_minor" <> account_row."available_amount_minor" + NEW."delta_available_minor"
       OR NEW."after_frozen_minor" <> account_row."frozen_amount_minor" + NEW."delta_frozen_minor"
       OR NEW."after_paid_minor" <> account_row."paid_amount_minor" + NEW."delta_paid_minor"
       OR NEW."after_reversed_minor" <> account_row."reversed_amount_minor" + NEW."delta_reversed_minor" THEN
        RAISE EXCEPTION 'commission entry after-values do not continue the account projection' USING ERRCODE = '23514';
    END IF;
    UPDATE plugin_idc_core_sales_commission_account
       SET "credited_amount_minor" = NEW."after_credited_minor",
           "pending_amount_minor" = NEW."after_pending_minor",
           "available_amount_minor" = NEW."after_available_minor",
           "frozen_amount_minor" = NEW."after_frozen_minor",
           "paid_amount_minor" = NEW."after_paid_minor",
           "reversed_amount_minor" = NEW."after_reversed_minor",
           "version" = "version" + 1,
           "updated_at" = CURRENT_TIMESTAMP
     WHERE "account_no" = NEW."account_no" AND "currency_code" = NEW."currency_code";
    RETURN NULL;
END $$;

CREATE OR REPLACE FUNCTION plugin_idc_core_sales_guard_commission_projection()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE latest plugin_idc_core_sales_commission_account_entry%ROWTYPE;
BEGIN
    IF (NEW."credited_amount_minor", NEW."pending_amount_minor", NEW."available_amount_minor",
        NEW."frozen_amount_minor", NEW."paid_amount_minor", NEW."reversed_amount_minor")
       IS NOT DISTINCT FROM
       (OLD."credited_amount_minor", OLD."pending_amount_minor", OLD."available_amount_minor",
        OLD."frozen_amount_minor", OLD."paid_amount_minor", OLD."reversed_amount_minor") THEN
        RETURN NEW;
    END IF;
    SELECT * INTO latest
      FROM plugin_idc_core_sales_commission_account_entry
     WHERE "account_no" = NEW."account_no" ORDER BY "entry_sequence" DESC LIMIT 1;
    IF NOT FOUND OR
       (NEW."credited_amount_minor", NEW."pending_amount_minor", NEW."available_amount_minor",
        NEW."frozen_amount_minor", NEW."paid_amount_minor", NEW."reversed_amount_minor")
       IS DISTINCT FROM
       (latest."after_credited_minor", latest."after_pending_minor", latest."after_available_minor",
        latest."after_frozen_minor", latest."after_paid_minor", latest."after_reversed_minor") THEN
        RAISE EXCEPTION 'commission projection must match the latest append-only entry' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;

DROP TRIGGER IF EXISTS trg_idc_sales_commission_entry_apply
    ON plugin_idc_core_sales_commission_account_entry;
CREATE TRIGGER trg_idc_sales_commission_entry_apply
    AFTER INSERT ON plugin_idc_core_sales_commission_account_entry
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_apply_commission_entry();
DROP TRIGGER IF EXISTS trg_idc_sales_commission_projection_guard
    ON plugin_idc_core_sales_commission_account;
CREATE TRIGGER trg_idc_sales_commission_projection_guard
    BEFORE UPDATE OF "credited_amount_minor", "pending_amount_minor", "available_amount_minor",
        "frozen_amount_minor", "paid_amount_minor", "reversed_amount_minor"
    ON plugin_idc_core_sales_commission_account
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_guard_commission_projection();

CREATE OR REPLACE FUNCTION plugin_idc_core_sales_check_settlement_total()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    "target_no" VARCHAR(40);
    frozen_amount BIGINT;
    settlement_status VARCHAR(24);
    item_amount BIGINT;
BEGIN
    "target_no" := CASE WHEN TG_TABLE_NAME LIKE '%settlement_item' THEN COALESCE(NEW."settlement_no", OLD."settlement_no")
                      ELSE COALESCE(NEW."settlement_no", OLD."settlement_no") END;
    SELECT "frozen_amount_minor", "status_code" INTO frozen_amount, settlement_status
      FROM plugin_idc_core_sales_commission_settlement WHERE "settlement_no" = "target_no";
    IF NOT FOUND OR settlement_status NOT IN ('pending_review', 'approved', 'batched') THEN RETURN NULL; END IF;
    SELECT COALESCE(SUM("amount_minor"), 0) INTO item_amount
      FROM plugin_idc_core_sales_commission_settlement_item WHERE "settlement_no" = "target_no";
    IF item_amount <> frozen_amount THEN
        RAISE EXCEPTION 'settlement items do not equal frozen amount' USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END $$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_sales_commission_settlement'::regclass
           AND conname = 'trg_idc_sales_settlement_total'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_sales_commission_settlement'::regclass
               AND constraint_row.conname = 'trg_idc_sales_settlement_total'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_sales_settlement_total'
               AND trigger_row.tgfoid = 'plugin_idc_core_sales_check_settlement_total()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_sales_settlement_total catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_sales_settlement_total
        AFTER INSERT OR UPDATE ON plugin_idc_core_sales_commission_settlement
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
        EXECUTE FUNCTION plugin_idc_core_sales_check_settlement_total();
    END IF;
END;
$constraint_trigger$;
DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_sales_commission_settlement_item'::regclass
           AND conname = 'trg_idc_sales_settlement_item_total'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_sales_commission_settlement_item'::regclass
               AND constraint_row.conname = 'trg_idc_sales_settlement_item_total'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_sales_settlement_item_total'
               AND trigger_row.tgfoid = 'plugin_idc_core_sales_check_settlement_total()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_sales_settlement_item_total catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_sales_settlement_item_total
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_sales_commission_settlement_item
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
        EXECUTE FUNCTION plugin_idc_core_sales_check_settlement_total();
    END IF;
END;
$constraint_trigger$;

CREATE OR REPLACE FUNCTION plugin_idc_core_sales_check_payout_item_amount()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE settlement_amount BIGINT;
BEGIN
    SELECT "requested_amount_minor" INTO settlement_amount
      FROM plugin_idc_core_sales_commission_settlement
     WHERE "settlement_no" = NEW."settlement_no" AND "currency_code" = NEW."currency_code";
    IF NOT FOUND OR NEW."amount_minor" <> settlement_amount THEN
        RAISE EXCEPTION 'payout item does not equal settlement payable amount' USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION plugin_idc_core_sales_guard_payout_item_identity()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF (NEW."batch_no", NEW."settlement_no", NEW."currency_code", NEW."amount_minor")
       IS DISTINCT FROM (OLD."batch_no", OLD."settlement_no", OLD."currency_code", OLD."amount_minor") THEN
        RAISE EXCEPTION 'payout item financial identity is immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS trg_idc_sales_payout_item_identity
    ON plugin_idc_core_sales_commission_payout_item;
CREATE TRIGGER trg_idc_sales_payout_item_identity
    BEFORE UPDATE ON plugin_idc_core_sales_commission_payout_item
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_guard_payout_item_identity();
DROP TRIGGER IF EXISTS trg_idc_sales_payout_item_amount
    ON plugin_idc_core_sales_commission_payout_item;
CREATE TRIGGER trg_idc_sales_payout_item_amount
    BEFORE INSERT OR UPDATE OF "settlement_no", "currency_code", "amount_minor"
    ON plugin_idc_core_sales_commission_payout_item
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_sales_check_payout_item_amount();

CREATE OR REPLACE FUNCTION plugin_idc_core_sales_check_payout_batch_terminal()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE "target_no" VARCHAR(40); "summary" RECORD;
BEGIN
    "target_no" := COALESCE(NEW."batch_no", OLD."batch_no");
    SELECT b."status_code", b."item_count", b."total_amount_minor",
           b."succeeded_count", b."succeeded_amount_minor", b."failed_count", b."failed_amount_minor",
           COUNT(i."id") AS actual_count, COALESCE(SUM(i."amount_minor"),0) AS actual_amount,
           COUNT(i."id") FILTER (WHERE i."status_code"='succeeded') AS actual_success_count,
           COALESCE(SUM(i."amount_minor") FILTER (WHERE i."status_code"='succeeded'),0) AS actual_success_amount,
           COUNT(i."id") FILTER (WHERE i."status_code"='failed') AS actual_failed_count,
           COALESCE(SUM(i."amount_minor") FILTER (WHERE i."status_code"='failed'),0) AS actual_failed_amount
      INTO "summary"
      FROM plugin_idc_core_sales_commission_payout_batch b
      LEFT JOIN plugin_idc_core_sales_commission_payout_item i ON i."batch_no"=b."batch_no"
     WHERE b."batch_no"="target_no" GROUP BY b."id";
    IF NOT FOUND OR "summary"."status_code" NOT IN ('succeeded','partially_failed','failed') THEN RETURN NULL; END IF;
    IF "summary"."item_count" <> "summary".actual_count OR "summary"."total_amount_minor" <> "summary".actual_amount
       OR "summary"."succeeded_count" <> "summary".actual_success_count
       OR "summary"."succeeded_amount_minor" <> "summary".actual_success_amount
       OR "summary"."failed_count" <> "summary".actual_failed_count
       OR "summary"."failed_amount_minor" <> "summary".actual_failed_amount
       OR ("summary"."status_code"='succeeded' AND "summary".actual_success_count <> "summary".actual_count)
       OR ("summary"."status_code"='failed' AND "summary".actual_failed_count <> "summary".actual_count)
       OR ("summary"."status_code"='partially_failed' AND ("summary".actual_success_count=0 OR "summary".actual_failed_count=0)) THEN
        RAISE EXCEPTION 'terminal payout batch summary does not equal its items' USING ERRCODE = '23514';
    END IF;
    RETURN NULL;
END $$;
DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_sales_commission_payout_batch'::regclass
           AND conname = 'trg_idc_sales_payout_batch_terminal'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_sales_commission_payout_batch'::regclass
               AND constraint_row.conname = 'trg_idc_sales_payout_batch_terminal'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_sales_payout_batch_terminal'
               AND trigger_row.tgfoid = 'plugin_idc_core_sales_check_payout_batch_terminal()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_sales_payout_batch_terminal catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_sales_payout_batch_terminal
        AFTER INSERT OR UPDATE ON plugin_idc_core_sales_commission_payout_batch
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
        EXECUTE FUNCTION plugin_idc_core_sales_check_payout_batch_terminal();
    END IF;
END;
$constraint_trigger$;
DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_sales_commission_payout_item'::regclass
           AND conname = 'trg_idc_sales_payout_item_terminal'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_sales_commission_payout_item'::regclass
               AND constraint_row.conname = 'trg_idc_sales_payout_item_terminal'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_idc_sales_payout_item_terminal'
               AND trigger_row.tgfoid = 'plugin_idc_core_sales_check_payout_batch_terminal()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_idc_sales_payout_item_terminal catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_idc_sales_payout_item_terminal
        AFTER INSERT OR UPDATE OR DELETE ON plugin_idc_core_sales_commission_payout_item
        DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
        EXECUTE FUNCTION plugin_idc_core_sales_check_payout_batch_terminal();
    END IF;
END;
$constraint_trigger$;

INSERT INTO sys_dict_type
    ("tenant_id","name","type","status","is_builtin","allow_tenant_override","remark","created_at","updated_at")
VALUES
    (0,'IDC 代理申请状态','idc_sales_agent_application_status',1,1,FALSE,'代理申请审核状态',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 代理资格状态','idc_sales_agent_qualification_status',1,1,FALSE,'代理资格生命周期状态',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 推荐归因状态','idc_sales_affiliate_attribution_status',1,1,FALSE,'AFF 推荐归因结果',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 佣金账户状态','idc_sales_commission_account_status',1,1,FALSE,'逐币种佣金账户状态',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 佣金成熟状态','idc_sales_commission_maturity_status',1,1,FALSE,'佣金成熟处理结果',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 佣金结算状态','idc_sales_commission_settlement_status',1,1,FALSE,'佣金结算申请状态',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'IDC 佣金打款状态','idc_sales_commission_payout_status',1,1,FALSE,'佣金打款处理状态',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id","type") DO NOTHING;

INSERT INTO sys_dict_data
    ("tenant_id","dict_type","label","value","sort","tag_style","status","is_builtin","created_at","updated_at")
VALUES
    (0,'idc_sales_agent_application_status','待审核','submitted',1,'processing',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_application_status','已批准','approved',2,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_application_status','已拒绝','rejected',3,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_application_status','已取消','cancelled',4,'default',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_qualification_status','有效','active',1,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_qualification_status','已暂停','suspended',2,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_qualification_status','已到期','expired',3,'default',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_agent_qualification_status','已撤销','revoked',4,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_affiliate_attribution_status','有效归因','eligible',1,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_affiliate_attribution_status','自荐已拒绝','rejected_self_referral',2,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_affiliate_attribution_status','风险已拒绝','rejected_risk',3,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_affiliate_attribution_status','已撤销','revoked',4,'default',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_account_status','正常','active',1,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_account_status','已冻结','frozen',2,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_account_status','已关闭','closed',3,'default',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_maturity_status','已成熟','matured',1,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_maturity_status','已阻止','blocked',2,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_maturity_status','已冲回','reversed',3,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','待审核','pending_review',1,'processing',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','已批准','approved',2,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','已入批次','batched',3,'processing',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','已打款','paid',4,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','已拒绝','rejected',5,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','打款失败','failed',6,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','待人工处理','manual_review',7,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_settlement_status','已取消','cancelled',8,'default',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','待处理','pending',1,'processing',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','处理中','processing',2,'processing',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','成功','succeeded',3,'success',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','部分失败','partially_failed',4,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','失败','failed',5,'error',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP),
    (0,'idc_sales_commission_payout_status','待人工处理','manual_review',6,'warning',1,1,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id","dict_type","value") DO NOTHING;

COMMENT ON TABLE plugin_idc_core_sales_commission_account IS
    'Independent per-currency commission projection; never a customer spending balance';
COMMENT ON TABLE plugin_idc_core_sales_commission_account_entry IS
    'Append-only commission bucket ledger with per-entry conservation proof';
COMMENT ON TABLE plugin_idc_core_sales_commission_payout_evidence IS
    'Append-only payout evidence containing only protected object references and digests';
