-- Customer account profile and CRM foundation for the single-merchant IDC installation.
-- Ordinary customer PII is anonymized through an explicit workflow; taxonomy
-- records are archived and evidence/history records are never normally deleted.

ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "search_name" VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "status_reason_code" VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "status_reason" VARCHAR(512) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "status_changed_by" INT NOT NULL DEFAULT 0;
ALTER TABLE plugin_idc_core_customer_subject
    ADD COLUMN IF NOT EXISTS "status_changed_at" TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_customer_search_name
    ON plugin_idc_core_customer_subject (
        "tenant_id", "status_code", "search_name" varchar_pattern_ops, "id"
    );
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_customer_no_type
    ON plugin_idc_core_customer_subject ("customer_no", "customer_type_code");

COMMENT ON COLUMN plugin_idc_core_customer_subject."search_name" IS 'Service-normalized display-name projection for indexed prefix search';
COMMENT ON COLUMN plugin_idc_core_customer_subject."status_reason_code" IS 'Stable reason code for the current account restriction or lifecycle state';
COMMENT ON COLUMN plugin_idc_core_customer_subject."status_reason" IS 'Bounded operator explanation presented through localized business copy';

-- Purpose: Individual customer legal identity projection without raw identity documents.
-- 用途：保存客户个人资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_individual_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL DEFAULT 'individual',
    "legal_name" VARCHAR(256) NOT NULL DEFAULT '',
    "legal_name_masked" VARCHAR(256) NOT NULL DEFAULT '',
    "identity_identifier_masked" VARCHAR(128) NOT NULL DEFAULT '',
    "identity_lookup_digest" CHAR(64) NOT NULL DEFAULT '',
    "identity_case_no" VARCHAR(40) NOT NULL DEFAULT '',
    "identity_status_code" VARCHAR(24) NOT NULL DEFAULT 'unverified',
    "identity_verified_at" TIMESTAMPTZ,
    "identity_expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_ind_profile_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_ind_profile_type CHECK ("customer_type_code" = 'individual'),
    CONSTRAINT ck_plugin_idc_core_ind_profile_status CHECK (
        "identity_status_code" IN ('unverified', 'pending', 'verified', 'rejected', 'expired', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_ind_profile_digest CHECK (
        ("identity_identifier_masked" = '' AND "identity_lookup_digest" = '') OR
        ("identity_identifier_masked" <> '' AND "identity_lookup_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_plugin_idc_core_ind_profile_verified CHECK (
        "identity_status_code" <> 'verified' OR
        (
            "legal_name" <> '' AND
            "legal_name_masked" <> '' AND
            "identity_identifier_masked" <> '' AND
            "identity_lookup_digest" ~ '^[0-9a-f]{64}$' AND
            "identity_case_no" <> '' AND
            "identity_verified_at" IS NOT NULL
        )
    ),
    CONSTRAINT ck_plugin_idc_core_ind_profile_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_ind_profile_customer UNIQUE ("customer_no"),
    CONSTRAINT fk_plugin_idc_core_ind_profile_customer FOREIGN KEY ("customer_no", "customer_type_code")
        REFERENCES plugin_idc_core_customer_subject ("customer_no", "customer_type_code") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_ind_profile_identity
    ON plugin_idc_core_customer_individual_profile (
        "tenant_id", "identity_status_code", "identity_lookup_digest", "customer_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_ind_profile_digest
    ON plugin_idc_core_customer_individual_profile (
        "tenant_id", "identity_lookup_digest", "customer_no"
    ) WHERE "identity_lookup_digest" <> '';

COMMENT ON TABLE plugin_idc_core_customer_individual_profile IS 'Individual customer legal identity projection without raw identity documents';
COMMENT ON COLUMN plugin_idc_core_customer_individual_profile."legal_name" IS 'Legal name protected by customer-profile access controls';
COMMENT ON COLUMN plugin_idc_core_customer_individual_profile."identity_identifier_masked" IS 'Display-safe masked identity identifier; the raw identifier is not stored in this table';
COMMENT ON COLUMN plugin_idc_core_customer_individual_profile."identity_lookup_digest" IS 'Keyed lookup digest; never a raw identity number or unsalted public hash';
COMMENT ON COLUMN plugin_idc_core_customer_individual_profile."identity_case_no" IS 'Opaque compliance-owned verification case reference without a database foreign key';

-- Purpose: Single-operator enterprise customer legal profile and verification projection.
-- 用途：保存客户企业资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_enterprise_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL DEFAULT 'enterprise',
    "legal_name" VARCHAR(512) NOT NULL DEFAULT '',
    "registration_country_code" CHAR(2) NOT NULL DEFAULT 'CN',
    "registration_identifier" VARCHAR(128) NOT NULL DEFAULT '',
    "registration_identifier_masked" VARCHAR(128) NOT NULL DEFAULT '',
    "registration_lookup_digest" CHAR(64) NOT NULL DEFAULT '',
    "industry_code" VARCHAR(64) NOT NULL DEFAULT '',
    "verification_case_no" VARCHAR(40) NOT NULL DEFAULT '',
    "verification_status_code" VARCHAR(24) NOT NULL DEFAULT 'unverified',
    "verified_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_ent_profile_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_ent_profile_type CHECK ("customer_type_code" = 'enterprise'),
    CONSTRAINT ck_plugin_idc_core_ent_profile_country CHECK (
        "registration_country_code" ~ '^[A-Z]{2}$'
    ),
    CONSTRAINT ck_plugin_idc_core_ent_profile_status CHECK (
        "verification_status_code" IN ('unverified', 'pending', 'verified', 'rejected', 'expired', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_ent_profile_digest CHECK (
        ("registration_identifier" = '' AND "registration_identifier_masked" = '' AND "registration_lookup_digest" = '') OR
        ("registration_identifier" <> '' AND "registration_identifier_masked" <> '' AND
            "registration_lookup_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_plugin_idc_core_ent_profile_verified CHECK (
        "verification_status_code" <> 'verified' OR
        (
            "legal_name" <> '' AND
            "registration_identifier" <> '' AND
            "registration_identifier_masked" <> '' AND
            "registration_lookup_digest" ~ '^[0-9a-f]{64}$' AND
            "verification_case_no" <> '' AND
            "verified_at" IS NOT NULL
        )
    ),
    CONSTRAINT ck_plugin_idc_core_ent_profile_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_ent_profile_customer UNIQUE ("customer_no"),
    CONSTRAINT fk_plugin_idc_core_ent_profile_customer FOREIGN KEY ("customer_no", "customer_type_code")
        REFERENCES plugin_idc_core_customer_subject ("customer_no", "customer_type_code") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_ent_profile_registration
    ON plugin_idc_core_customer_enterprise_profile (
        "tenant_id", "verification_status_code", "registration_lookup_digest", "customer_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_ent_profile_digest
    ON plugin_idc_core_customer_enterprise_profile (
        "tenant_id", "registration_lookup_digest", "customer_no"
    ) WHERE "registration_lookup_digest" <> '';

COMMENT ON TABLE plugin_idc_core_customer_enterprise_profile IS 'Single-operator enterprise customer legal profile and verification projection';
COMMENT ON COLUMN plugin_idc_core_customer_enterprise_profile."registration_identifier" IS 'Enterprise registration identifier protected as customer legal profile data';
COMMENT ON COLUMN plugin_idc_core_customer_enterprise_profile."registration_lookup_digest" IS 'Keyed lookup digest used for duplicate review without query-time normalization';
COMMENT ON COLUMN plugin_idc_core_customer_enterprise_profile."verification_case_no" IS 'Opaque compliance-owned verification case reference without a database foreign key';

-- Purpose: Notification-only customer contact; it never grants login or business-operation authority.
-- 用途：保存客户联系人相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_contact (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "contact_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "display_name" VARCHAR(256) NOT NULL,
    "email" VARCHAR(320) NOT NULL DEFAULT '',
    "email_lookup_digest" CHAR(64) NOT NULL DEFAULT '',
    "email_verified_at" TIMESTAMPTZ,
    "phone_country_code" VARCHAR(8) NOT NULL DEFAULT '',
    "phone" VARCHAR(32) NOT NULL DEFAULT '',
    "phone_lookup_digest" CHAR(64) NOT NULL DEFAULT '',
    "phone_verified_at" TIMESTAMPTZ,
    "preferred_locale" VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_contact_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_contact_name CHECK (
        length(btrim("display_name")) BETWEEN 1 AND 256
    ),
    CONSTRAINT ck_plugin_idc_core_contact_destination CHECK ("email" <> '' OR "phone" <> ''),
    CONSTRAINT ck_plugin_idc_core_contact_email_digest CHECK (
        ("email" = '' AND "email_lookup_digest" = '') OR
        ("email" <> '' AND "email_lookup_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_phone_digest CHECK (
        ("phone" = '' AND "phone_country_code" = '' AND "phone_lookup_digest" = '') OR
        ("phone" <> '' AND "phone_country_code" <> '' AND "phone_lookup_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_status CHECK ("status_code" IN ('active', 'archived')),
    CONSTRAINT ck_plugin_idc_core_contact_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_contact_no UNIQUE ("contact_no"),
    CONSTRAINT uk_plugin_idc_core_contact_customer_no UNIQUE ("customer_no", "contact_no"),
    CONSTRAINT fk_plugin_idc_core_contact_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_contact_customer
    ON plugin_idc_core_customer_contact ("tenant_id", "customer_no", "status_code", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_contact_email
    ON plugin_idc_core_customer_contact ("tenant_id", "email_lookup_digest", "customer_no")
    WHERE "email_lookup_digest" <> '';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_contact_phone
    ON plugin_idc_core_customer_contact ("tenant_id", "phone_lookup_digest", "customer_no")
    WHERE "phone_lookup_digest" <> '';

COMMENT ON TABLE plugin_idc_core_customer_contact IS 'Notification-only customer contact; it never grants login or business-operation authority';
COMMENT ON COLUMN plugin_idc_core_customer_contact."email_lookup_digest" IS 'Keyed normalized email lookup digest';
COMMENT ON COLUMN plugin_idc_core_customer_contact."phone_lookup_digest" IS 'Keyed E.164-style phone lookup digest';

-- Purpose: Role assignment for notification-only customer contacts.
-- 用途：保存客户联系人角色相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_contact_role (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "role_code" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "assigned_by" INT NOT NULL DEFAULT 0,
    "assigned_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_by" INT NOT NULL DEFAULT 0,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_contact_role_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_contact_role_code CHECK (
        "role_code" IN ('primary', 'billing', 'technical', 'operations')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_role_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_role_revoked CHECK (
        ("status_code" = 'active' AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT fk_plugin_idc_core_contact_role_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_contact_role_contact FOREIGN KEY ("customer_no", "contact_no")
        REFERENCES plugin_idc_core_customer_contact ("customer_no", "contact_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_contact_role_active
    ON plugin_idc_core_customer_contact_role ("customer_no", "contact_no", "role_code")
    WHERE "status_code" = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_contact_primary
    ON plugin_idc_core_customer_contact_role ("customer_no", "role_code")
    WHERE "status_code" = 'active' AND "role_code" = 'primary';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_contact_role_customer
    ON plugin_idc_core_customer_contact_role ("tenant_id", "customer_no", "status_code", "role_code", "id");

COMMENT ON TABLE plugin_idc_core_customer_contact_role IS 'Role assignment for notification-only customer contacts';

-- Purpose: Per-contact notification preference; security and mandatory service notices remain policy controlled.
-- 用途：保存客户联系人偏好相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_contact_preference (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "contact_no" VARCHAR(40) NOT NULL,
    "topic_code" VARCHAR(32) NOT NULL,
    "channel_code" VARCHAR(24) NOT NULL,
    "enabled" BOOLEAN NOT NULL DEFAULT TRUE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_contact_pref_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_contact_pref_topic CHECK (
        "topic_code" IN ('account', 'billing', 'service', 'security', 'support', 'marketing')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_pref_channel CHECK (
        "channel_code" IN ('in_app', 'email', 'sms', 'wechat_official', 'wechat_work')
    ),
    CONSTRAINT ck_plugin_idc_core_contact_pref_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_contact_pref UNIQUE ("contact_no", "topic_code", "channel_code"),
    CONSTRAINT fk_plugin_idc_core_contact_pref_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_contact_pref_contact FOREIGN KEY ("customer_no", "contact_no")
        REFERENCES plugin_idc_core_customer_contact ("customer_no", "contact_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_contact_pref_delivery
    ON plugin_idc_core_customer_contact_preference (
        "tenant_id", "customer_no", "topic_code", "enabled", "channel_code", "contact_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_contact_preference IS 'Per-contact notification preference; security and mandatory service notices remain policy controlled';

-- Purpose: Customer-owned contact, billing, registered, or service address.
-- 用途：保存客户地址相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_address (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "address_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "address_type_code" VARCHAR(24) NOT NULL,
    "label" VARCHAR(128) NOT NULL DEFAULT '',
    "recipient_name" VARCHAR(256) NOT NULL DEFAULT '',
    "recipient_phone" VARCHAR(32) NOT NULL DEFAULT '',
    "country_code" CHAR(2) NOT NULL,
    "province" VARCHAR(128) NOT NULL DEFAULT '',
    "city" VARCHAR(128) NOT NULL DEFAULT '',
    "district" VARCHAR(128) NOT NULL DEFAULT '',
    "address_line1" VARCHAR(512) NOT NULL,
    "address_line2" VARCHAR(512) NOT NULL DEFAULT '',
    "postal_code" VARCHAR(32) NOT NULL DEFAULT '',
    "is_default" BOOLEAN NOT NULL DEFAULT FALSE,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_address_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_address_type CHECK (
        "address_type_code" IN ('contact', 'billing', 'registered', 'service')
    ),
    CONSTRAINT ck_plugin_idc_core_address_country CHECK ("country_code" ~ '^[A-Z]{2}$'),
    CONSTRAINT ck_plugin_idc_core_address_line CHECK (
        length(btrim("address_line1")) BETWEEN 1 AND 512
    ),
    CONSTRAINT ck_plugin_idc_core_address_status CHECK ("status_code" IN ('active', 'archived')),
    CONSTRAINT ck_plugin_idc_core_address_default CHECK (
        "status_code" = 'active' OR "is_default" = FALSE
    ),
    CONSTRAINT ck_plugin_idc_core_address_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_address_no UNIQUE ("address_no"),
    CONSTRAINT fk_plugin_idc_core_address_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_address_default
    ON plugin_idc_core_customer_address ("customer_no", "address_type_code")
    WHERE "status_code" = 'active' AND "is_default" = TRUE;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_address_customer
    ON plugin_idc_core_customer_address (
        "tenant_id", "customer_no", "status_code", "address_type_code", "is_default" DESC, "id"
    );

COMMENT ON TABLE plugin_idc_core_customer_address IS 'Customer-owned contact, billing, registered, or service address';

-- Purpose: Customer billing-header profile; actual tax invoices are owned by the compliance plugin.
-- 用途：保存客户账单资料相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_invoice_profile (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "profile_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "title_type_code" VARCHAR(16) NOT NULL,
    "invoice_title" VARCHAR(512) NOT NULL,
    "tax_registration_no" VARCHAR(128) NOT NULL DEFAULT '',
    "registered_address" VARCHAR(512) NOT NULL DEFAULT '',
    "registered_phone" VARCHAR(32) NOT NULL DEFAULT '',
    "bank_name" VARCHAR(256) NOT NULL DEFAULT '',
    "bank_account_masked" VARCHAR(128) NOT NULL DEFAULT '',
    "bank_account_secret_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "is_default" BOOLEAN NOT NULL DEFAULT FALSE,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_invoice_profile_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_type CHECK (
        "title_type_code" IN ('individual', 'enterprise')
    ),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_title CHECK (
        length(btrim("invoice_title")) BETWEEN 1 AND 512
    ),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_status CHECK (
        "status_code" IN ('active', 'archived')
    ),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_default CHECK (
        "status_code" = 'active' OR "is_default" = FALSE
    ),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_bank CHECK (
        ("bank_account_masked" = '' AND "bank_account_secret_ref" = '') OR
        ("bank_account_masked" <> '' AND "bank_account_secret_ref" <> '')
    ),
    CONSTRAINT ck_plugin_idc_core_invoice_profile_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_invoice_profile_no UNIQUE ("profile_no"),
    CONSTRAINT fk_plugin_idc_core_invoice_profile_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_invoice_profile_default
    ON plugin_idc_core_customer_invoice_profile ("customer_no")
    WHERE "status_code" = 'active' AND "is_default" = TRUE;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_invoice_profile_customer
    ON plugin_idc_core_customer_invoice_profile ("tenant_id", "customer_no", "status_code", "id");

COMMENT ON TABLE plugin_idc_core_customer_invoice_profile IS 'Customer billing-header profile; actual tax invoices are owned by the compliance plugin';
COMMENT ON COLUMN plugin_idc_core_customer_invoice_profile."bank_account_secret_ref" IS 'Opaque core secret-envelope reference; never returned by ordinary APIs or logs';

-- Purpose: Merchant-defined customer level used by pricing and segmentation.
-- 用途：保存客户等级相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_level (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "level_no" VARCHAR(40) NOT NULL,
    "level_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "priority" INT NOT NULL DEFAULT 0,
    "is_default" BOOLEAN NOT NULL DEFAULT FALSE,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_level_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_level_code CHECK (
        "level_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_core_level_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_level_status CHECK ("status_code" IN ('active', 'archived')),
    CONSTRAINT ck_plugin_idc_core_level_default CHECK (
        "status_code" = 'active' OR "is_default" = FALSE
    ),
    CONSTRAINT ck_plugin_idc_core_level_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_level_no UNIQUE ("level_no"),
    CONSTRAINT uk_plugin_idc_core_level_code UNIQUE ("level_code")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_level_default
    ON plugin_idc_core_customer_level ("tenant_id")
    WHERE "status_code" = 'active' AND "is_default" = TRUE;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_level_list
    ON plugin_idc_core_customer_level ("tenant_id", "status_code", "priority" DESC, "id");

COMMENT ON TABLE plugin_idc_core_customer_level IS 'Merchant-defined customer level used by pricing and segmentation';

-- Purpose: Auditable current and historical customer-level assignment.
-- 用途：保存客户等级分派相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_level_assignment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "level_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL DEFAULT 'manual',
    "reason" VARCHAR(512) NOT NULL DEFAULT '',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "assigned_by" INT NOT NULL DEFAULT 0,
    "assigned_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_by" INT NOT NULL DEFAULT 0,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_level_assign_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_level_assign_source CHECK (
        "source_code" IN ('manual', 'rule', 'migration')
    ),
    CONSTRAINT ck_plugin_idc_core_level_assign_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_level_assign_revoked CHECK (
        ("status_code" = 'active' AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT fk_plugin_idc_core_level_assign_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_level_assign_level FOREIGN KEY ("level_no")
        REFERENCES plugin_idc_core_customer_level ("level_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_level_assign_active
    ON plugin_idc_core_customer_level_assignment ("customer_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_level_assign_segment
    ON plugin_idc_core_customer_level_assignment (
        "tenant_id", "level_no", "status_code", "customer_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_level_assignment IS 'Auditable current and historical customer-level assignment';

-- Purpose: Merchant-defined customer group for operations and policy selection.
-- 用途：保存客户分组相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_group (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "group_no" VARCHAR(40) NOT NULL,
    "group_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "description_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "description_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_group_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_group_code CHECK (
        "group_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_core_group_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_group_status CHECK ("status_code" IN ('active', 'archived')),
    CONSTRAINT ck_plugin_idc_core_group_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_group_no UNIQUE ("group_no"),
    CONSTRAINT uk_plugin_idc_core_group_code UNIQUE ("group_code")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_group_list
    ON plugin_idc_core_customer_group ("tenant_id", "status_code", "name_zh_cn", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_group_list_en
    ON plugin_idc_core_customer_group ("tenant_id", "status_code", "name_en_us", "id");

COMMENT ON TABLE plugin_idc_core_customer_group IS 'Merchant-defined customer group for operations and policy selection';

-- Purpose: Auditable many-to-many customer group assignment.
-- 用途：保存客户分组分派相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_group_assignment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "group_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL DEFAULT 'manual',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "assigned_by" INT NOT NULL DEFAULT 0,
    "assigned_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_by" INT NOT NULL DEFAULT 0,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_group_assign_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_group_assign_source CHECK (
        "source_code" IN ('manual', 'rule', 'migration')
    ),
    CONSTRAINT ck_plugin_idc_core_group_assign_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_group_assign_revoked CHECK (
        ("status_code" = 'active' AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT fk_plugin_idc_core_group_assign_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_group_assign_group FOREIGN KEY ("group_no")
        REFERENCES plugin_idc_core_customer_group ("group_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_group_assign_active
    ON plugin_idc_core_customer_group_assignment ("customer_no", "group_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_group_assign_segment
    ON plugin_idc_core_customer_group_assignment (
        "tenant_id", "group_no", "status_code", "customer_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_group_assign_customer
    ON plugin_idc_core_customer_group_assignment (
        "tenant_id", "customer_no", "status_code", "group_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_group_assignment IS 'Auditable many-to-many customer group assignment';

-- Purpose: Merchant-defined customer tag with semantic color token instead of arbitrary CSS.
-- 用途：保存客户标签相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_tag (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "tag_no" VARCHAR(40) NOT NULL,
    "tag_code" VARCHAR(64) NOT NULL,
    "name_zh_cn" VARCHAR(128) NOT NULL,
    "name_en_us" VARCHAR(128) NOT NULL,
    "color_token" VARCHAR(32) NOT NULL DEFAULT 'default',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_tag_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_tag_code CHECK (
        "tag_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_core_tag_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("name_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_tag_color CHECK (
        "color_token" ~ '^[a-z][a-z0-9_-]{0,31}$'
    ),
    CONSTRAINT ck_plugin_idc_core_tag_status CHECK ("status_code" IN ('active', 'archived')),
    CONSTRAINT ck_plugin_idc_core_tag_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_tag_no UNIQUE ("tag_no"),
    CONSTRAINT uk_plugin_idc_core_tag_code UNIQUE ("tag_code")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_tag_list
    ON plugin_idc_core_customer_tag ("tenant_id", "status_code", "name_zh_cn", "id");
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_tag_list_en
    ON plugin_idc_core_customer_tag ("tenant_id", "status_code", "name_en_us", "id");

COMMENT ON TABLE plugin_idc_core_customer_tag IS 'Merchant-defined customer tag with semantic color token instead of arbitrary CSS';

-- Purpose: Auditable many-to-many customer tag assignment.
-- 用途：保存客户标签分派相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_tag_assignment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "tag_no" VARCHAR(40) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL DEFAULT 'manual',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "assigned_by" INT NOT NULL DEFAULT 0,
    "assigned_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "revoked_by" INT NOT NULL DEFAULT 0,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_tag_assign_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_tag_assign_source CHECK (
        "source_code" IN ('manual', 'rule', 'migration')
    ),
    CONSTRAINT ck_plugin_idc_core_tag_assign_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_tag_assign_revoked CHECK (
        ("status_code" = 'active' AND "revoked_at" IS NULL) OR
        ("status_code" = 'revoked' AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT fk_plugin_idc_core_tag_assign_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_tag_assign_tag FOREIGN KEY ("tag_no")
        REFERENCES plugin_idc_core_customer_tag ("tag_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_tag_assign_active
    ON plugin_idc_core_customer_tag_assignment ("customer_no", "tag_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_tag_assign_segment
    ON plugin_idc_core_customer_tag_assignment (
        "tenant_id", "tag_no", "status_code", "customer_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_tag_assign_customer
    ON plugin_idc_core_customer_tag_assignment (
        "tenant_id", "customer_no", "status_code", "tag_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_tag_assignment IS 'Auditable many-to-many customer tag assignment';

-- Purpose: Typed customer custom-field definition with bilingual natural-language labels.
-- 用途：保存客户自定义字段相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_custom_field (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "field_no" VARCHAR(40) NOT NULL,
    "field_key" VARCHAR(64) NOT NULL,
    "label_zh_cn" VARCHAR(128) NOT NULL,
    "label_en_us" VARCHAR(128) NOT NULL,
    "help_zh_cn" VARCHAR(512) NOT NULL DEFAULT '',
    "help_en_us" VARCHAR(512) NOT NULL DEFAULT '',
    "data_type_code" VARCHAR(24) NOT NULL,
    "customer_type_code" VARCHAR(16) NOT NULL DEFAULT 'all',
    "visibility_code" VARCHAR(24) NOT NULL DEFAULT 'admin_only',
    "sensitivity_code" VARCHAR(16) NOT NULL DEFAULT 'ordinary',
    "required" BOOLEAN NOT NULL DEFAULT FALSE,
    "customer_editable" BOOLEAN NOT NULL DEFAULT FALSE,
    "minimum_length" INT,
    "maximum_length" INT,
    "minimum_number" NUMERIC(30, 9),
    "maximum_number" NUMERIC(30, 9),
    "validation_pattern" VARCHAR(512) NOT NULL DEFAULT '',
    "priority" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_custom_field_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_custom_field_key CHECK (
        "field_key" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_label CHECK (
        length(btrim("label_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("label_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_type CHECK (
        "data_type_code" IN (
            'text', 'integer', 'decimal', 'boolean', 'date', 'datetime', 'single_select', 'multi_select'
        )
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_customer CHECK (
        "customer_type_code" IN ('all', 'individual', 'enterprise')
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_visibility CHECK (
        "visibility_code" IN ('admin_only', 'customer_read', 'customer_edit')
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_sensitive CHECK (
        "sensitivity_code" IN ('ordinary', 'personal', 'sensitive')
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_editable CHECK (
        "customer_editable" = FALSE OR "visibility_code" = 'customer_edit'
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_length CHECK (
        ("minimum_length" IS NULL OR "minimum_length" >= 0) AND
        ("maximum_length" IS NULL OR "maximum_length" >= 0) AND
        ("minimum_length" IS NULL OR "maximum_length" IS NULL OR "minimum_length" <= "maximum_length")
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_number CHECK (
        "minimum_number" IS NULL OR "maximum_number" IS NULL OR "minimum_number" <= "maximum_number"
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_status CHECK (
        "status_code" IN ('active', 'archived')
    ),
    CONSTRAINT ck_plugin_idc_core_custom_field_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_custom_field_no UNIQUE ("field_no"),
    CONSTRAINT uk_plugin_idc_core_custom_field_key UNIQUE ("field_key")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_custom_field_form
    ON plugin_idc_core_customer_custom_field (
        "tenant_id", "status_code", "customer_type_code", "visibility_code", "priority" DESC, "id"
    );

COMMENT ON TABLE plugin_idc_core_customer_custom_field IS 'Typed customer custom-field definition with bilingual natural-language labels';
COMMENT ON COLUMN plugin_idc_core_customer_custom_field."validation_pattern" IS 'Bounded service-validated pattern; never evaluated by an unbounded backtracking engine';
COMMENT ON COLUMN plugin_idc_core_customer_custom_field."sensitivity_code" IS 'Controls masking, export, audit, and AI exclusion policy';

-- Purpose: Stable localized option set for select custom fields.
-- 用途：保存客户自定义字段选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_custom_field_option (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "option_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "option_value" VARCHAR(128) NOT NULL,
    "label_zh_cn" VARCHAR(128) NOT NULL,
    "label_en_us" VARCHAR(128) NOT NULL,
    "priority" INT NOT NULL DEFAULT 0,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_field_option_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_field_option_value CHECK (
        length(btrim("option_value")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_field_option_label CHECK (
        length(btrim("label_zh_cn")) BETWEEN 1 AND 128 AND
        length(btrim("label_en_us")) BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_plugin_idc_core_field_option_status CHECK (
        "status_code" IN ('active', 'archived')
    ),
    CONSTRAINT uk_plugin_idc_core_field_option_no UNIQUE ("option_no"),
    CONSTRAINT uk_plugin_idc_core_field_option_value UNIQUE ("field_no", "option_value"),
    CONSTRAINT uk_plugin_idc_core_field_option_pair UNIQUE ("field_no", "option_no"),
    CONSTRAINT fk_plugin_idc_core_field_option_field FOREIGN KEY ("field_no")
        REFERENCES plugin_idc_core_customer_custom_field ("field_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_option_list
    ON plugin_idc_core_customer_custom_field_option (
        "tenant_id", "field_no", "status_code", "priority" DESC, "id"
    );

COMMENT ON TABLE plugin_idc_core_customer_custom_field_option IS 'Stable localized option set for select custom fields';

-- Purpose: One active typed custom-field value per customer and definition.
-- 用途：保存客户自定义字段值相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_custom_field_value (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "value_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "text_value" VARCHAR(4096),
    "text_lookup_digest" CHAR(64) NOT NULL DEFAULT '',
    "secret_value_ref" VARCHAR(256) NOT NULL DEFAULT '',
    "integer_value" BIGINT,
    "decimal_value" NUMERIC(30, 9),
    "boolean_value" BOOLEAN,
    "date_value" DATE,
    "datetime_value" TIMESTAMPTZ,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "updated_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_field_value_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_field_value_scalar CHECK (
        num_nonnulls(
            "text_value", NULLIF("secret_value_ref", ''), "integer_value", "decimal_value",
            "boolean_value", "date_value", "datetime_value"
        ) <= 1
    ),
    CONSTRAINT ck_plugin_idc_core_field_value_digest CHECK (
        ("text_value" IS NULL AND "text_lookup_digest" = '') OR
        ("text_value" IS NOT NULL AND "text_lookup_digest" ~ '^[0-9a-f]{64}$')
    ),
    CONSTRAINT ck_plugin_idc_core_field_value_status CHECK (
        "status_code" IN ('active', 'revoked')
    ),
    CONSTRAINT ck_plugin_idc_core_field_value_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_core_field_value_no UNIQUE ("value_no"),
    CONSTRAINT uk_plugin_idc_core_field_value_pair UNIQUE ("field_no", "value_no"),
    CONSTRAINT uk_plugin_idc_core_field_value_owner UNIQUE ("customer_no", "field_no", "value_no"),
    CONSTRAINT fk_plugin_idc_core_field_value_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_field_value_field FOREIGN KEY ("field_no")
        REFERENCES plugin_idc_core_customer_custom_field ("field_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_field_value_active
    ON plugin_idc_core_customer_custom_field_value ("customer_no", "field_no")
    WHERE "status_code" = 'active';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_customer
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "customer_no", "status_code", "field_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_text
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "text_lookup_digest", "customer_no"
    ) WHERE "status_code" = 'active' AND "text_value" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_integer
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "integer_value", "customer_no"
    ) WHERE "status_code" = 'active' AND "integer_value" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_decimal
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "decimal_value", "customer_no"
    ) WHERE "status_code" = 'active' AND "decimal_value" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_boolean
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "boolean_value", "customer_no"
    ) WHERE "status_code" = 'active' AND "boolean_value" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_date
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "date_value", "customer_no"
    ) WHERE "status_code" = 'active' AND "date_value" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_field_value_datetime
    ON plugin_idc_core_customer_custom_field_value (
        "tenant_id", "field_no", "datetime_value", "customer_no"
    ) WHERE "status_code" = 'active' AND "datetime_value" IS NOT NULL;

COMMENT ON TABLE plugin_idc_core_customer_custom_field_value IS 'One active typed custom-field value per customer and definition';
COMMENT ON COLUMN plugin_idc_core_customer_custom_field_value."text_lookup_digest" IS 'Keyed normalized digest for exact filtering without indexing long plaintext';
COMMENT ON COLUMN plugin_idc_core_customer_custom_field_value."secret_value_ref" IS 'Opaque core secret-envelope reference used instead of plaintext for sensitive custom values';

-- Purpose: Normalized selected options for single-select and multi-select customer fields.
-- 用途：保存客户自定义字段值选项相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_custom_field_value_option (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "customer_no" VARCHAR(40) NOT NULL,
    "field_no" VARCHAR(40) NOT NULL,
    "value_no" VARCHAR(40) NOT NULL,
    "option_no" VARCHAR(40) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_value_option_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_plugin_idc_core_value_option UNIQUE ("value_no", "option_no"),
    CONSTRAINT fk_plugin_idc_core_value_option_value FOREIGN KEY ("customer_no", "field_no", "value_no")
        REFERENCES plugin_idc_core_customer_custom_field_value ("customer_no", "field_no", "value_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_value_option_option FOREIGN KEY ("field_no", "option_no")
        REFERENCES plugin_idc_core_customer_custom_field_option ("field_no", "option_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_value_option_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_value_option_segment
    ON plugin_idc_core_customer_custom_field_value_option (
        "tenant_id", "field_no", "option_no", "customer_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_value_option_customer
    ON plugin_idc_core_customer_custom_field_value_option (
        "tenant_id", "customer_no", "field_no", "value_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_custom_field_value_option IS 'Normalized selected options for single-select and multi-select customer fields';

-- Purpose: Append-only customer consent grant or withdrawal evidence.
-- 用途：保存客户同意相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_consent (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "consent_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "consent_type_code" VARCHAR(32) NOT NULL,
    "policy_reference" VARCHAR(256) NOT NULL,
    "policy_version" VARCHAR(64) NOT NULL,
    "action_code" VARCHAR(16) NOT NULL,
    "source_code" VARCHAR(24) NOT NULL,
    "locale" VARCHAR(16) NOT NULL,
    "evidence_digest" CHAR(64) NOT NULL,
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "recorded_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_consent_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_consent_type CHECK (
        "consent_type_code" IN (
            'terms', 'privacy', 'sensitive_personal_information', 'marketing', 'data_export', 'account_closure'
        )
    ),
    CONSTRAINT ck_plugin_idc_core_consent_policy CHECK (
        length(btrim("policy_reference")) BETWEEN 1 AND 256 AND
        length(btrim("policy_version")) BETWEEN 1 AND 64
    ),
    CONSTRAINT ck_plugin_idc_core_consent_action CHECK ("action_code" IN ('granted', 'withdrawn')),
    CONSTRAINT ck_plugin_idc_core_consent_source CHECK (
        "source_code" IN ('portal', 'admin', 'migration', 'system')
    ),
    CONSTRAINT ck_plugin_idc_core_consent_digest CHECK (
        "evidence_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_plugin_idc_core_consent_no UNIQUE ("consent_no"),
    CONSTRAINT fk_plugin_idc_core_consent_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_consent_latest
    ON plugin_idc_core_customer_consent (
        "tenant_id", "customer_no", "consent_type_code", "occurred_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_consent_policy
    ON plugin_idc_core_customer_consent (
        "tenant_id", "consent_type_code", "policy_reference", "policy_version", "action_code", "customer_no"
    );

COMMENT ON TABLE plugin_idc_core_customer_consent IS 'Append-only customer consent grant or withdrawal evidence';
COMMENT ON COLUMN plugin_idc_core_customer_consent."evidence_digest" IS 'Digest of canonical consent evidence; raw request metadata remains outside ordinary fields';

-- Purpose: Internal-only customer note history; edits create a superseding note instead of erasing evidence.
-- 用途：保存客户备注相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_note (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "note_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "category_code" VARCHAR(24) NOT NULL DEFAULT 'general',
    "content" VARCHAR(8192) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "supersedes_note_no" VARCHAR(40) NOT NULL DEFAULT '',
    "author_user_id" INT NOT NULL,
    "withdrawn_by" INT NOT NULL DEFAULT 0,
    "withdrawn_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_note_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_note_category CHECK (
        "category_code" IN ('general', 'billing', 'service', 'sales', 'risk', 'compliance')
    ),
    CONSTRAINT ck_plugin_idc_core_note_content CHECK (
        length(btrim("content")) BETWEEN 1 AND 8192
    ),
    CONSTRAINT ck_plugin_idc_core_note_status CHECK ("status_code" IN ('active', 'withdrawn')),
    CONSTRAINT ck_plugin_idc_core_note_withdrawn CHECK (
        ("status_code" = 'active' AND "withdrawn_at" IS NULL) OR
        ("status_code" = 'withdrawn' AND "withdrawn_at" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_core_note_no UNIQUE ("note_no"),
    CONSTRAINT fk_plugin_idc_core_note_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_note_timeline
    ON plugin_idc_core_customer_note (
        "tenant_id", "customer_no", "status_code", "created_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_customer_note IS 'Internal-only customer note history; edits create a superseding note instead of erasing evidence';

-- Purpose: Customer follow-up plans and completed activity history.
-- 用途：保存客户跟进跟进相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_follow_up (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "follow_up_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "kind_code" VARCHAR(16) NOT NULL,
    "channel_code" VARCHAR(24) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL,
    "subject" VARCHAR(512) NOT NULL,
    "detail" VARCHAR(8192) NOT NULL DEFAULT '',
    "outcome_code" VARCHAR(32) NOT NULL DEFAULT '',
    "assigned_user_id" INT NOT NULL DEFAULT 0,
    "planned_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "cancelled_at" TIMESTAMPTZ,
    "created_by" INT NOT NULL,
    "updated_by" INT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_follow_up_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_follow_up_kind CHECK ("kind_code" IN ('plan', 'activity')),
    CONSTRAINT ck_plugin_idc_core_follow_up_channel CHECK (
        "channel_code" IN ('phone', 'email', 'sms', 'wechat', 'meeting', 'ticket', 'other')
    ),
    CONSTRAINT ck_plugin_idc_core_follow_up_status CHECK (
        "status_code" IN ('planned', 'completed', 'cancelled')
    ),
    CONSTRAINT ck_plugin_idc_core_follow_up_subject CHECK (
        length(btrim("subject")) BETWEEN 1 AND 512
    ),
    CONSTRAINT ck_plugin_idc_core_follow_up_time CHECK (
        ("status_code" = 'planned' AND "planned_at" IS NOT NULL AND "completed_at" IS NULL AND "cancelled_at" IS NULL) OR
        ("status_code" = 'completed' AND "completed_at" IS NOT NULL AND "cancelled_at" IS NULL) OR
        ("status_code" = 'cancelled' AND "cancelled_at" IS NOT NULL AND "completed_at" IS NULL)
    ),
    CONSTRAINT uk_plugin_idc_core_follow_up_no UNIQUE ("follow_up_no"),
    CONSTRAINT fk_plugin_idc_core_follow_up_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_follow_up_due
    ON plugin_idc_core_customer_follow_up (
        "tenant_id", "status_code", "planned_at", "assigned_user_id", "id"
    ) WHERE "status_code" = 'planned';
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_follow_up_timeline
    ON plugin_idc_core_customer_follow_up (
        "tenant_id", "customer_no", "created_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_core_customer_follow_up IS 'Customer follow-up plans and completed activity history';

-- Purpose: Private customer attachment metadata; bytes remain in private object storage.
-- 用途：保存客户附件相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_attachment (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "attachment_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "category_code" VARCHAR(32) NOT NULL,
    "visibility_code" VARCHAR(16) NOT NULL DEFAULT 'admin_only',
    "object_key" VARCHAR(1024) NOT NULL,
    "original_filename" VARCHAR(512) NOT NULL,
    "media_type" VARCHAR(255) NOT NULL,
    "size_bytes" BIGINT NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "scan_status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "retention_until" TIMESTAMPTZ,
    "uploaded_by" INT NOT NULL DEFAULT 0,
    "available_at" TIMESTAMPTZ,
    "removed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_attachment_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_attachment_category CHECK (
        "category_code" IN ('general', 'customer_provided', 'sales', 'support', 'risk', 'migration')
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_visibility CHECK (
        "visibility_code" IN ('admin_only', 'customer_visible')
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_size CHECK (
        "size_bytes" BETWEEN 1 AND 104857600
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_digest CHECK (
        "content_sha256" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_scan CHECK (
        "scan_status_code" IN ('pending', 'clean', 'infected', 'failed')
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_status CHECK (
        "status_code" IN ('pending', 'available', 'quarantined', 'removed')
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_available CHECK (
        "status_code" <> 'available' OR ("scan_status_code" = 'clean' AND "available_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_core_attachment_removed CHECK (
        "status_code" <> 'removed' OR "removed_at" IS NOT NULL
    ),
    CONSTRAINT uk_plugin_idc_core_attachment_no UNIQUE ("attachment_no"),
    CONSTRAINT fk_plugin_idc_core_attachment_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_attachment_customer
    ON plugin_idc_core_customer_attachment (
        "tenant_id", "customer_no", "status_code", "category_code", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_attachment_scan
    ON plugin_idc_core_customer_attachment (
        "tenant_id", "scan_status_code", "status_code", "created_at", "id"
    ) WHERE "scan_status_code" IN ('pending', 'failed');

COMMENT ON TABLE plugin_idc_core_customer_attachment IS 'Private customer attachment metadata; bytes remain in private object storage';
COMMENT ON COLUMN plugin_idc_core_customer_attachment."object_key" IS 'Internal private object key, never a public or long-lived download URL';

-- Purpose: Scoped, approved, expiring customer business restriction evidence.
-- 用途：保存客户黑名单记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_blacklist_entry (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "entry_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "scope_code" VARCHAR(32) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "reason_detail" VARCHAR(2048) NOT NULL DEFAULT '',
    "evidence_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "risk_level_code" VARCHAR(16) NOT NULL DEFAULT 'medium',
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "starts_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "expires_at" TIMESTAMPTZ,
    "created_by" INT NOT NULL,
    "approved_by" INT NOT NULL DEFAULT 0,
    "approved_at" TIMESTAMPTZ,
    "revoked_by" INT NOT NULL DEFAULT 0,
    "revoked_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_blacklist_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_blacklist_scope CHECK (
        "scope_code" IN ('registration', 'purchase', 'refund', 'resource_operation', 'all_business')
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_reason CHECK (
        "reason_code" ~ '^[a-z][a-z0-9_.-]{0,63}$'
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_risk CHECK (
        "risk_level_code" IN ('low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_status CHECK (
        "status_code" IN ('pending', 'active', 'rejected', 'revoked', 'expired')
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_time CHECK (
        "expires_at" IS NULL OR "expires_at" > "starts_at"
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_approval CHECK (
        "status_code" <> 'active' OR ("approved_by" > 0 AND "approved_at" IS NOT NULL)
    ),
    CONSTRAINT ck_plugin_idc_core_blacklist_revoked CHECK (
        "status_code" <> 'revoked' OR ("revoked_by" > 0 AND "revoked_at" IS NOT NULL)
    ),
    CONSTRAINT uk_plugin_idc_core_blacklist_no UNIQUE ("entry_no"),
    CONSTRAINT fk_plugin_idc_core_blacklist_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_blacklist_gate
    ON plugin_idc_core_customer_blacklist_entry (
        "tenant_id", "customer_no", "scope_code", "status_code", "starts_at", "expires_at", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_blacklist_review
    ON plugin_idc_core_customer_blacklist_entry (
        "tenant_id", "status_code", "risk_level_code", "created_at", "id"
    ) WHERE "status_code" = 'pending';

COMMENT ON TABLE plugin_idc_core_customer_blacklist_entry IS 'Scoped, approved, expiring customer business restriction evidence';

-- Purpose: Auditable customer risk signal and review outcome without raw external evidence.
-- 用途：保存客户风险记录相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_customer_risk_record (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "risk_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "category_code" VARCHAR(32) NOT NULL,
    "source_code" VARCHAR(32) NOT NULL,
    "level_code" VARCHAR(16) NOT NULL,
    "score_basis_points" INT,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'open',
    "summary" VARCHAR(1024) NOT NULL,
    "evidence_reference" VARCHAR(256) NOT NULL DEFAULT '',
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "acknowledged_by" INT NOT NULL DEFAULT 0,
    "acknowledged_at" TIMESTAMPTZ,
    "resolved_by" INT NOT NULL DEFAULT 0,
    "resolved_at" TIMESTAMPTZ,
    "resolution" VARCHAR(2048) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_risk_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_core_risk_category CHECK (
        "category_code" IN ('identity', 'payment', 'refund', 'abuse', 'security', 'resource', 'compliance', 'other')
    ),
    CONSTRAINT ck_plugin_idc_core_risk_source CHECK (
        "source_code" IN ('manual', 'rule', 'provider', 'payment', 'compliance', 'migration')
    ),
    CONSTRAINT ck_plugin_idc_core_risk_level CHECK (
        "level_code" IN ('info', 'low', 'medium', 'high', 'critical')
    ),
    CONSTRAINT ck_plugin_idc_core_risk_score CHECK (
        "score_basis_points" IS NULL OR "score_basis_points" BETWEEN 0 AND 10000
    ),
    CONSTRAINT ck_plugin_idc_core_risk_status CHECK (
        "status_code" IN ('open', 'acknowledged', 'resolved', 'dismissed')
    ),
    CONSTRAINT ck_plugin_idc_core_risk_summary CHECK (
        length(btrim("summary")) BETWEEN 1 AND 1024
    ),
    CONSTRAINT ck_plugin_idc_core_risk_acknowledged CHECK (
        "status_code" NOT IN ('acknowledged', 'resolved', 'dismissed') OR "acknowledged_at" IS NOT NULL
    ),
    CONSTRAINT ck_plugin_idc_core_risk_resolved CHECK (
        "status_code" NOT IN ('resolved', 'dismissed') OR "resolved_at" IS NOT NULL
    ),
    CONSTRAINT uk_plugin_idc_core_risk_no UNIQUE ("risk_no"),
    CONSTRAINT fk_plugin_idc_core_risk_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_risk_customer
    ON plugin_idc_core_customer_risk_record (
        "tenant_id", "customer_no", "status_code", "level_code", "occurred_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_risk_queue
    ON plugin_idc_core_customer_risk_record (
        "tenant_id", "status_code", "level_code", "occurred_at", "id"
    ) WHERE "status_code" IN ('open', 'acknowledged');

COMMENT ON TABLE plugin_idc_core_customer_risk_record IS 'Auditable customer risk signal and review outcome without raw external evidence';

-- Stable customer-domain enum values are registered in the host dictionary
-- module. Similar value sets remain independent when their business meaning
-- or future lifecycle can evolve separately.
INSERT INTO sys_dict_type (
    "tenant_id", "name", "type", "status", "is_builtin", "allow_tenant_override", "remark", "created_at", "updated_at"
)
VALUES
    (0, 'IDC 客户地址类型', 'idc_address_type', 1, 1, FALSE, '客户地址用途类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户关系分配来源', 'idc_assignment_source', 1, 1, FALSE, '客户等级、分组和标签等关系的分配来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户关系分配状态', 'idc_assignment_status', 1, 1, FALSE, '客户联系人角色、等级、分组、标签和字段值关系生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 附件扫描状态', 'idc_attachment_scan_status', 1, 1, FALSE, '私有客户附件安全扫描生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 附件状态', 'idc_attachment_status', 1, 1, FALSE, '私有客户附件可用性生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 黑名单限制范围', 'idc_blacklist_scope', 1, 1, FALSE, '客户黑名单业务限制范围', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 黑名单风险等级', 'idc_blacklist_risk_level', 1, 1, FALSE, '客户黑名单限制的风险严重程度', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 黑名单状态', 'idc_blacklist_status', 1, 1, FALSE, '客户黑名单审核与生效生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 同意动作', 'idc_consent_action', 1, 1, FALSE, '客户同意的授予与撤回动作', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 同意来源', 'idc_consent_source', 1, 1, FALSE, '客户同意证据的采集来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 同意类型', 'idc_consent_type', 1, 1, FALSE, '客户条款、隐私与敏感信息等同意类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 联系人角色', 'idc_contact_role', 1, 1, FALSE, '客户主体内的通知联系人角色', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 自定义字段客户范围', 'idc_custom_field_customer_scope', 1, 1, FALSE, '客户自定义字段适用的客户类型范围', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 自定义字段数据类型', 'idc_custom_field_data_type', 1, 1, FALSE, '客户自定义字段的稳定数据类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 自定义字段可见性', 'idc_custom_field_visibility', 1, 1, FALSE, '客户自定义字段的后台与客户侧可见性', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户附件分类', 'idc_customer_attachment_category', 1, 1, FALSE, '客户附件业务分类', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户附件可见性', 'idc_customer_attachment_visibility', 1, 1, FALSE, '客户附件的后台与客户侧可见性', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户备注分类', 'idc_customer_note_category', 1, 1, FALSE, '客户内部备注业务分类', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户备注状态', 'idc_customer_note_status', 1, 1, FALSE, '客户内部备注生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户风险分类', 'idc_customer_risk_category', 1, 1, FALSE, '客户风险信号业务分类', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户风险等级', 'idc_customer_risk_level', 1, 1, FALSE, '包含提示级别的客户风险严重程度', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户风险来源', 'idc_customer_risk_source', 1, 1, FALSE, '客户风险信号来源', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户风险状态', 'idc_customer_risk_status', 1, 1, FALSE, '客户风险复核与处置生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 数据敏感级别', 'idc_data_sensitivity', 1, 1, FALSE, '客户自定义字段的数据敏感级别', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户跟进渠道', 'idc_follow_up_channel', 1, 1, FALSE, '客户跟进活动使用的联系渠道', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户跟进类型', 'idc_follow_up_kind', 1, 1, FALSE, '客户跟进计划与活动类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户跟进状态', 'idc_follow_up_status', 1, 1, FALSE, '客户跟进计划生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 发票抬头类型', 'idc_invoice_title_type', 1, 1, FALSE, '客户开票资料的抬头主体类型', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 通知渠道', 'idc_notification_channel', 1, 1, FALSE, '客户通知偏好支持的渠道', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 通知主题', 'idc_notification_topic', 1, 1, FALSE, '客户通知偏好的业务主题', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 通用归档状态', 'idc_record_status', 1, 1, FALSE, '可归档客户配置与资料记录的生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'IDC 客户认证状态', 'idc_verification_status', 1, 1, FALSE, '个人与企业客户认证生命周期', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "type") DO NOTHING;

INSERT INTO sys_dict_data (
    "tenant_id", "dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at"
)
VALUES
    (0, 'idc_address_type', '联系地址', 'contact', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_address_type', '账单地址', 'billing', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_address_type', '注册地址', 'registered', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_address_type', '服务地址', 'service', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_assignment_source', '手工分配', 'manual', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_assignment_source', '规则分配', 'rule', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_assignment_source', '数据迁移', 'migration', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_assignment_status', '生效', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_assignment_status', '已撤销', 'revoked', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_scan_status', '待扫描', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_scan_status', '安全', 'clean', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_scan_status', '检出威胁', 'infected', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_scan_status', '扫描失败', 'failed', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_status', '待处理', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_status', '可用', 'available', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_status', '已隔离', 'quarantined', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_attachment_status', '已移除', 'removed', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_scope', '注册', 'registration', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_scope', '购买', 'purchase', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_scope', '退款', 'refund', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_scope', '资源操作', 'resource_operation', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_scope', '全部业务', 'all_business', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_risk_level', '低', 'low', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_risk_level', '中', 'medium', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_risk_level', '高', 'high', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_risk_level', '严重', 'critical', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_status', '待审核', 'pending', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_status', '生效', 'active', 2, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_status', '已驳回', 'rejected', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_status', '已撤销', 'revoked', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_blacklist_status', '已过期', 'expired', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_action', '已同意', 'granted', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_action', '已撤回', 'withdrawn', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_source', '客户门户', 'portal', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_source', '管理后台', 'admin', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_source', '数据迁移', 'migration', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_source', '系统', 'system', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '服务条款', 'terms', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '隐私政策', 'privacy', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '敏感个人信息', 'sensitive_personal_information', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '营销', 'marketing', 4, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '数据导出', 'data_export', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_consent_type', '账户注销', 'account_closure', 6, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_role', '主要联系人', 'primary', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_role', '账务联系人', 'billing', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_role', '技术联系人', 'technical', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_contact_role', '运维联系人', 'operations', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_customer_scope', '全部客户', 'all', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_customer_scope', '个人客户', 'individual', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_customer_scope', '企业客户', 'enterprise', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '文本', 'text', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '整数', 'integer', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '小数', 'decimal', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '布尔值', 'boolean', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '日期', 'date', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '日期时间', 'datetime', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '单选', 'single_select', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_data_type', '多选', 'multi_select', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_visibility', '仅后台', 'admin_only', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_visibility', '客户只读', 'customer_read', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_custom_field_visibility', '客户可编辑', 'customer_edit', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '常规', 'general', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '客户提供', 'customer_provided', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '销售', 'sales', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '支持', 'support', 4, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '风险', 'risk', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_category', '数据迁移', 'migration', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_visibility', '仅后台', 'admin_only', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_attachment_visibility', '客户可见', 'customer_visible', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '常规', 'general', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '财务', 'billing', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '服务', 'service', 3, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '销售', 'sales', 4, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '风险', 'risk', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_category', '合规', 'compliance', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_status', '生效', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_note_status', '已撤回', 'withdrawn', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '身份', 'identity', 1, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '支付', 'payment', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '退款', 'refund', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '滥用', 'abuse', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '安全', 'security', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '资源', 'resource', 6, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '合规', 'compliance', 7, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_category', '其他', 'other', 8, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_level', '提示', 'info', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_level', '低', 'low', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_level', '中', 'medium', 3, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_level', '高', 'high', 4, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_level', '严重', 'critical', 5, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '人工', 'manual', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '规则', 'rule', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '供应商', 'provider', 3, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '支付', 'payment', 4, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '合规', 'compliance', 5, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_source', '数据迁移', 'migration', 6, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_status', '待处理', 'open', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_status', '已确认', 'acknowledged', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_status', '已解决', 'resolved', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_customer_risk_status', '已忽略', 'dismissed', 4, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_data_sensitivity', '普通', 'ordinary', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_data_sensitivity', '个人信息', 'personal', 2, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_data_sensitivity', '敏感信息', 'sensitive', 3, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '电话', 'phone', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '电子邮箱', 'email', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '短信', 'sms', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '微信', 'wechat', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '会议', 'meeting', 5, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '工单', 'ticket', 6, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_channel', '其他', 'other', 7, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_kind', '计划', 'plan', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_kind', '跟进记录', 'activity', 2, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_status', '计划中', 'planned', 1, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_status', '已完成', 'completed', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_follow_up_status', '已取消', 'cancelled', 3, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_invoice_title_type', '个人', 'individual', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_invoice_title_type', '企业', 'enterprise', 2, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_channel', '站内信', 'in_app', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_channel', '电子邮箱', 'email', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_channel', '短信', 'sms', 3, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_channel', '微信公众号', 'wechat_official', 4, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_channel', '企业微信', 'wechat_work', 5, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '账户', 'account', 1, 'primary', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '财务', 'billing', 2, 'cyan', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '服务', 'service', 3, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '安全', 'security', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '支持', 'support', 5, 'purple', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_notification_topic', '营销', 'marketing', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_record_status', '生效', 'active', 1, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_record_status', '已归档', 'archived', 2, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '未认证', 'unverified', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '认证中', 'pending', 2, 'processing', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '已通过', 'verified', 3, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '已拒绝', 'rejected', 4, 'danger', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '已过期', 'expired', 5, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (0, 'idc_verification_status', '已撤销', 'revoked', 6, 'warning', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT ("tenant_id", "dict_type", "value") DO NOTHING;
