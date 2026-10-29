-- Purpose: Persists business records for plugin_idc_compliance_verification_case.
-- 用途：持久化 plugin_idc_compliance_verification_case 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_verification_case (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "case_no" VARCHAR(64) NOT NULL,
    "customer_no" VARCHAR(64) NOT NULL,
    "principal_no" VARCHAR(64) NOT NULL,
    "subject_type_code" VARCHAR(16) NOT NULL,
    "provider_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'draft',
    "consent_no" VARCHAR(64) NOT NULL,
    "provider_case_ref" VARCHAR(256),
    "provider_subject_ref" VARCHAR(256),
    "legal_profile_snapshot_ref" VARCHAR(512),
    "legal_profile_sha256" CHAR(64),
    "masked_name" VARCHAR(128) NOT NULL DEFAULT '',
    "masked_identity_no" VARCHAR(128) NOT NULL DEFAULT '',
    "identity_lookup_digest" CHAR(64),
    "decision_reason_code" VARCHAR(128) NOT NULL DEFAULT '',
    "submitted_at" TIMESTAMPTZ,
    "verified_at" TIMESTAMPTZ,
    "rejected_at" TIMESTAMPTZ,
    "expires_at" TIMESTAMPTZ,
    "decided_by_principal_no" VARCHAR(64),
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_verification_case_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_subject
        CHECK ("subject_type_code" IN ('personal', 'enterprise')),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_provider
        CHECK (
            ("subject_type_code" = 'personal' AND "provider_code" = 'aliyun_face') OR
            ("subject_type_code" = 'enterprise' AND "provider_code" = 'manual_enterprise')
        ),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_status
        CHECK (
            "status_code" IN (
                'draft', 'pending', 'verifying', 'review_required', 'verified',
                'rejected', 'expired', 'canceled'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_legal_digest
        CHECK (
            ("legal_profile_snapshot_ref" IS NULL AND "legal_profile_sha256" IS NULL) OR
            (
                "legal_profile_snapshot_ref" IS NOT NULL AND
                "legal_profile_sha256" ~ '^[0-9a-f]{64}$'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_verification_identity_digest
        CHECK (
            "identity_lookup_digest" IS NULL OR
            "identity_lookup_digest" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_verification_terminal
        CHECK (
            ("status_code" = 'verified' AND "verified_at" IS NOT NULL AND "rejected_at" IS NULL) OR
            ("status_code" = 'rejected' AND "rejected_at" IS NOT NULL AND "verified_at" IS NULL) OR
            ("status_code" NOT IN ('verified', 'rejected'))
        ),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_expiry
        CHECK ("expires_at" IS NULL OR "verified_at" IS NULL OR "expires_at" > "verified_at"),
    CONSTRAINT ck_plugin_idc_compliance_verification_case_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_verification_case_no UNIQUE ("case_no"),
    CONSTRAINT uk_plugin_idc_compliance_verification_case_subject_no
        UNIQUE ("case_no", "subject_type_code")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_verification_case_active
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "customer_no", "subject_type_code"
    ) WHERE "status_code" IN ('draft', 'pending', 'verifying', 'review_required');
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_verification_case_customer
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "customer_no", "status_code", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_verification_case_review
    ON plugin_idc_compliance_verification_case (
        "tenant_id", "status_code", "submitted_at", "id"
    ) WHERE "status_code" IN ('pending', 'verifying', 'review_required');

COMMENT ON TABLE plugin_idc_compliance_verification_case IS
    'Compliance-owned personal or enterprise verification case; core stores only its opaque gate projection';
COMMENT ON COLUMN plugin_idc_compliance_verification_case."legal_profile_snapshot_ref" IS
    'Private encrypted object reference; identity documents, face photos, face videos, and raw provider payloads are forbidden in this table';
COMMENT ON COLUMN plugin_idc_compliance_verification_case."identity_lookup_digest" IS
    'Keyed lookup digest used for duplicate-risk checks; raw identity numbers and unsalted public hashes are forbidden';

-- Purpose: Persists business records for plugin_idc_compliance_enterprise_material.
-- 用途：持久化 plugin_idc_compliance_enterprise_material 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_enterprise_material (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "material_no" VARCHAR(64) NOT NULL,
    "case_no" VARCHAR(64) NOT NULL,
    "case_subject_type_code" VARCHAR(16) NOT NULL DEFAULT 'enterprise',
    "material_type_code" VARCHAR(40) NOT NULL,
    "sequence_no" INT NOT NULL DEFAULT 1,
    "private_object_ref" VARCHAR(512) NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "media_type" VARCHAR(128) NOT NULL,
    "byte_size" BIGINT NOT NULL,
    "encryption_state_code" VARCHAR(16) NOT NULL DEFAULT 'encrypted',
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'available',
    "retention_until" TIMESTAMPTZ,
    "legal_hold" BOOLEAN NOT NULL DEFAULT FALSE,
    "superseded_by_material_no" VARCHAR(64),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_subject
        CHECK ("case_subject_type_code" = 'enterprise'),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_type
        CHECK (
            "material_type_code" IN (
                'business_license', 'legal_representative_id', 'agent_id',
                'authorization_letter', 'organization_certificate', 'other'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_sequence CHECK ("sequence_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_digest
        CHECK ("content_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_size CHECK ("byte_size" > 0),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_encryption
        CHECK ("encryption_state_code" = 'encrypted'),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_status
        CHECK (
            "status_code" IN (
                'available', 'quarantined', 'superseded', 'deletion_pending', 'deleted', 'legal_hold'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_hold
        CHECK (("status_code" = 'legal_hold') = "legal_hold"),
    CONSTRAINT ck_plugin_idc_compliance_enterprise_material_replace
        CHECK (
            ("status_code" = 'superseded' AND "superseded_by_material_no" IS NOT NULL) OR
            ("status_code" <> 'superseded' AND "superseded_by_material_no" IS NULL)
        ),
    CONSTRAINT uk_plugin_idc_compliance_enterprise_material_no UNIQUE ("material_no"),
    CONSTRAINT uk_plugin_idc_compliance_enterprise_material_case_no
        UNIQUE ("material_no", "case_no"),
    CONSTRAINT uk_plugin_idc_compliance_enterprise_material_slot
        UNIQUE ("case_no", "material_type_code", "sequence_no"),
    CONSTRAINT fk_plugin_idc_compliance_enterprise_material_case
        FOREIGN KEY ("case_no", "case_subject_type_code")
        REFERENCES plugin_idc_compliance_verification_case ("case_no", "subject_type_code")
        ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_enterprise_material_replace
        FOREIGN KEY ("superseded_by_material_no", "case_no")
        REFERENCES plugin_idc_compliance_enterprise_material ("material_no", "case_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_enterprise_material_case
    ON plugin_idc_compliance_enterprise_material (
        "tenant_id", "case_no", "status_code", "material_type_code", "sequence_no"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_enterprise_material_retention
    ON plugin_idc_compliance_enterprise_material (
        "tenant_id", "status_code", "retention_until", "id"
    ) WHERE "retention_until" IS NOT NULL AND NOT "legal_hold";

COMMENT ON TABLE plugin_idc_compliance_enterprise_material IS
    'Private encrypted enterprise-review material metadata; document bytes are stored only in private object storage';

-- Purpose: Persists business records for plugin_idc_compliance_tax_invoice.
-- 用途：持久化 plugin_idc_compliance_tax_invoice 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_tax_invoice (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "request_sha256" CHAR(64) NOT NULL,
    "customer_no" VARCHAR(64) NOT NULL,
    "principal_no" VARCHAR(64) NOT NULL,
    "seller_snapshot_ref" VARCHAR(512) NOT NULL,
    "seller_snapshot_sha256" CHAR(64) NOT NULL,
    "buyer_snapshot_ref" VARCHAR(512) NOT NULL,
    "buyer_snapshot_sha256" CHAR(64) NOT NULL,
    "invoice_type_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'draft',
    "tax_currency_code" CHAR(3) NOT NULL,
    "gross_amount_minor" BIGINT NOT NULL,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL,
    "billing_currency_code" CHAR(3) NOT NULL,
    "exchange_rate_numerator" BIGINT,
    "exchange_rate_denominator" BIGINT,
    "exchange_rate_date" DATE,
    "exchange_rate_source_code" VARCHAR(64),
    "provider_code" VARCHAR(32),
    "external_invoice_ref" VARCHAR(256),
    "masked_invoice_number" VARCHAR(128) NOT NULL DEFAULT '',
    "invoice_number_lookup_digest" CHAR(64),
    "submitted_at" TIMESTAMPTZ,
    "issued_at" TIMESTAMPTZ,
    "delivered_at" TIMESTAMPTZ,
    "canceled_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_request_digest
        CHECK ("request_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_seller_digest
        CHECK ("seller_snapshot_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_buyer_digest
        CHECK ("buyer_snapshot_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_number_digest
        CHECK (
            "invoice_number_lookup_digest" IS NULL OR
            "invoice_number_lookup_digest" ~ '^[0-9a-f]{64}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_type
        CHECK (
            "invoice_type_code" IN (
                'vat_electronic_normal', 'vat_electronic_special', 'digital_electronic_normal',
                'digital_electronic_special'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_status
        CHECK (
            "status_code" IN (
                'draft', 'pending_review', 'queued', 'submitting', 'issued', 'delivered',
                'failed', 'result_unknown', 'red_letter_pending', 'red_lettered', 'canceled'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_currency
        CHECK (
            "tax_currency_code" ~ '^[A-Z]{3}$' AND
            "billing_currency_code" ~ '^[A-Z]{3}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_amount
        CHECK (
            "gross_amount_minor" > 0 AND "net_amount_minor" >= 0 AND "tax_amount_minor" >= 0 AND
            "gross_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_exchange_rate
        CHECK (
            (
                "tax_currency_code" = "billing_currency_code" AND
                "exchange_rate_numerator" IS NULL AND "exchange_rate_denominator" IS NULL AND
                "exchange_rate_date" IS NULL AND "exchange_rate_source_code" IS NULL
            ) OR
            (
                "tax_currency_code" <> "billing_currency_code" AND
                "exchange_rate_numerator" > 0 AND "exchange_rate_denominator" > 0 AND
                "exchange_rate_date" IS NOT NULL AND "exchange_rate_source_code" IS NOT NULL
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_issued
        CHECK (("status_code" IN ('issued', 'delivered', 'red_letter_pending', 'red_lettered') AND "issued_at" IS NOT NULL) OR
               "status_code" NOT IN ('issued', 'delivered', 'red_letter_pending', 'red_lettered')),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_no UNIQUE ("tax_invoice_no"),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_request UNIQUE ("request_sha256")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_tax_invoice_provider_ref
    ON plugin_idc_compliance_tax_invoice ("tenant_id", "provider_code", "external_invoice_ref")
    WHERE "external_invoice_ref" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_customer
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "customer_no", "status_code", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_work
    ON plugin_idc_compliance_tax_invoice (
        "tenant_id", "status_code", "submitted_at", "id"
    ) WHERE "status_code" IN ('pending_review', 'queued', 'submitting', 'result_unknown', 'red_letter_pending');

COMMENT ON TABLE plugin_idc_compliance_tax_invoice IS
    'Supplier-neutral tax invoice aggregate; distinct from core billing_invoice receivables';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice."seller_snapshot_ref" IS
    'Private immutable seller snapshot for the installation merchant, never the software platform';
COMMENT ON COLUMN plugin_idc_compliance_tax_invoice."invoice_number_lookup_digest" IS
    'Keyed lookup digest for exact invoice-number reconciliation; raw values and unsalted public hashes are forbidden';

-- Purpose: Persists business records for plugin_idc_compliance_tax_invoice_allocation.
-- 用途：持久化 plugin_idc_compliance_tax_invoice_allocation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_tax_invoice_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "allocation_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "billing_invoice_no" VARCHAR(64) NOT NULL,
    "order_no" VARCHAR(64) NOT NULL,
    "allocated_gross_minor" BIGINT NOT NULL,
    "allocated_net_minor" BIGINT NOT NULL,
    "allocated_tax_minor" BIGINT NOT NULL,
    "refunded_gross_minor" BIGINT NOT NULL DEFAULT 0,
    "red_lettered_gross_minor" BIGINT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_allocation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_allocation_amount
        CHECK (
            "allocated_gross_minor" > 0 AND "allocated_net_minor" >= 0 AND "allocated_tax_minor" >= 0 AND
            "allocated_gross_minor" = "allocated_net_minor" + "allocated_tax_minor" AND
            "refunded_gross_minor" >= 0 AND "red_lettered_gross_minor" >= 0 AND
            "refunded_gross_minor" <= "allocated_gross_minor" AND
            "red_lettered_gross_minor" <= "allocated_gross_minor"
        ),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_allocation_no UNIQUE ("allocation_no"),
    CONSTRAINT uk_plugin_idc_compliance_tax_alloc_invoice_no
        UNIQUE ("allocation_no", "tax_invoice_no"),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_allocation_source
        UNIQUE ("tax_invoice_no", "billing_invoice_no"),
    CONSTRAINT fk_plugin_idc_compliance_tax_invoice_allocation_invoice
        FOREIGN KEY ("tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice ("tax_invoice_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_alloc_billing
    ON plugin_idc_compliance_tax_invoice_allocation (
        "tenant_id", "billing_invoice_no", "tax_invoice_no", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_allocation_order
    ON plugin_idc_compliance_tax_invoice_allocation (
        "tenant_id", "order_no", "tax_invoice_no", "id"
    );

COMMENT ON COLUMN plugin_idc_compliance_tax_invoice_allocation."billing_invoice_no" IS
    'Opaque reference to a core receivable; no cross-plugin foreign key is permitted';

-- Purpose: Persists business records for plugin_idc_compliance_tax_invoice_item.
-- 用途：持久化 plugin_idc_compliance_tax_invoice_item 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_tax_invoice_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "item_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "line_no" INT NOT NULL,
    "item_snapshot_ref" VARCHAR(512) NOT NULL,
    "item_snapshot_sha256" CHAR(64) NOT NULL,
    "tax_classification_code" VARCHAR(64) NOT NULL,
    "tax_rate_ppm" INT NOT NULL,
    "gross_amount_minor" BIGINT NOT NULL,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_line CHECK ("line_no" > 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_digest
        CHECK ("item_snapshot_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_rate
        CHECK ("tax_rate_ppm" BETWEEN 0 AND 1000000),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_item_amount
        CHECK (
            "gross_amount_minor" > 0 AND "net_amount_minor" >= 0 AND "tax_amount_minor" >= 0 AND
            "gross_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        ),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_item_no UNIQUE ("item_no"),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_item_line UNIQUE ("tax_invoice_no", "line_no"),
    CONSTRAINT fk_plugin_idc_compliance_tax_invoice_item_invoice
        FOREIGN KEY ("tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice ("tax_invoice_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_item_invoice
    ON plugin_idc_compliance_tax_invoice_item (
        "tenant_id", "tax_invoice_no", "line_no"
    );

-- Purpose: Persists business records for plugin_idc_compliance_tax_invoice_file.
-- 用途：持久化 plugin_idc_compliance_tax_invoice_file 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_tax_invoice_file (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "file_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "file_type_code" VARCHAR(32) NOT NULL,
    "private_object_ref" VARCHAR(512) NOT NULL,
    "content_sha256" CHAR(64) NOT NULL,
    "media_type" VARCHAR(128) NOT NULL,
    "byte_size" BIGINT NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'staging',
    "immutable" BOOLEAN NOT NULL DEFAULT FALSE,
    "archived_at" TIMESTAMPTZ,
    "retention_until" TIMESTAMPTZ,
    "legal_hold" BOOLEAN NOT NULL DEFAULT FALSE,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_type
        CHECK ("file_type_code" IN ('invoice_pdf', 'invoice_ofd', 'invoice_xml', 'delivery_receipt', 'red_letter_file')),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_digest
        CHECK ("content_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_size CHECK ("byte_size" > 0),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_status
        CHECK (
            "status_code" IN (
                'staging', 'archived', 'failed', 'deletion_pending', 'deleted', 'legal_hold'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_archive
        CHECK (
            ("status_code" = 'archived' AND "immutable" AND "archived_at" IS NOT NULL) OR
            ("status_code" <> 'archived')
        ),
    CONSTRAINT ck_plugin_idc_compliance_tax_invoice_file_hold
        CHECK (("status_code" = 'legal_hold') = "legal_hold" AND (NOT "legal_hold" OR "immutable")),
    CONSTRAINT uk_plugin_idc_compliance_tax_invoice_file_no UNIQUE ("file_no"),
    CONSTRAINT fk_plugin_idc_compliance_tax_invoice_file_invoice
        FOREIGN KEY ("tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice ("tax_invoice_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_file_invoice
    ON plugin_idc_compliance_tax_invoice_file (
        "tenant_id", "tax_invoice_no", "file_type_code", "created_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_tax_invoice_file_retention
    ON plugin_idc_compliance_tax_invoice_file (
        "tenant_id", "status_code", "retention_until", "id"
    ) WHERE "retention_until" IS NOT NULL AND NOT "legal_hold";

COMMENT ON TABLE plugin_idc_compliance_tax_invoice_file IS
    'Private tax-invoice file metadata with database-enforced immutable archive semantics';

-- Purpose: Persists business records for plugin_idc_compliance_red_letter_task.
-- 用途：持久化 plugin_idc_compliance_red_letter_task 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_red_letter_task (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "task_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "request_sha256" CHAR(64) NOT NULL,
    "refund_operation_no" VARCHAR(64) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending_review',
    "gross_amount_minor" BIGINT NOT NULL,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL,
    "customer_confirmation_required" BOOLEAN NOT NULL DEFAULT FALSE,
    "customer_confirmed_at" TIMESTAMPTZ,
    "provider_code" VARCHAR(32),
    "external_task_ref" VARCHAR(256),
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "next_query_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_request_digest
        CHECK ("request_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_status
        CHECK (
            "status_code" IN (
                'pending_review', 'awaiting_confirmation', 'queued', 'submitting', 'processing',
                'completed', 'failed', 'result_unknown', 'manual_required', 'canceled'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_amount
        CHECK (
            "gross_amount_minor" > 0 AND "net_amount_minor" >= 0 AND "tax_amount_minor" >= 0 AND
            "gross_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        ),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_confirmation
        CHECK (
            ("customer_confirmation_required" AND
             ("status_code" = 'awaiting_confirmation' OR "customer_confirmed_at" IS NOT NULL)) OR
            (NOT "customer_confirmation_required" AND "customer_confirmed_at" IS NULL)
        ),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_complete
        CHECK (("status_code" = 'completed' AND "completed_at" IS NOT NULL) OR "status_code" <> 'completed'),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_task_version CHECK ("version" > 0),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_task_no UNIQUE ("task_no"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_task_invoice_no
        UNIQUE ("task_no", "tax_invoice_no"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_task_request UNIQUE ("request_sha256"),
    CONSTRAINT fk_plugin_idc_compliance_red_letter_task_invoice
        FOREIGN KEY ("tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice ("tax_invoice_no") ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_red_letter_task_active
    ON plugin_idc_compliance_red_letter_task ("tenant_id", "tax_invoice_no")
    WHERE "status_code" IN (
        'pending_review', 'awaiting_confirmation', 'queued', 'submitting', 'processing', 'result_unknown'
    );
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_red_letter_task_provider_ref
    ON plugin_idc_compliance_red_letter_task ("tenant_id", "provider_code", "external_task_ref")
    WHERE "external_task_ref" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_task_work
    ON plugin_idc_compliance_red_letter_task (
        "tenant_id", "status_code", "next_query_at", "available_at", "id"
    ) WHERE "status_code" IN (
        'pending_review', 'awaiting_confirmation', 'queued', 'submitting', 'processing', 'result_unknown'
    );

COMMENT ON TABLE plugin_idc_compliance_red_letter_task IS
    'Idempotent red-letter workflow linked to an opaque core refund operation reference';

-- Purpose: Persists business records for plugin_idc_compliance_red_letter_allocation.
-- 用途：持久化 plugin_idc_compliance_red_letter_allocation 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_red_letter_allocation (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "allocation_no" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "tax_invoice_no" VARCHAR(64) NOT NULL,
    "tax_invoice_allocation_no" VARCHAR(64) NOT NULL,
    "refund_allocation_no" VARCHAR(64) NOT NULL,
    "gross_amount_minor" BIGINT NOT NULL,
    "net_amount_minor" BIGINT NOT NULL,
    "tax_amount_minor" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_red_letter_allocation_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_red_letter_allocation_amount
        CHECK (
            "gross_amount_minor" > 0 AND "net_amount_minor" >= 0 AND "tax_amount_minor" >= 0 AND
            "gross_amount_minor" = "net_amount_minor" + "tax_amount_minor"
        ),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_allocation_no UNIQUE ("allocation_no"),
    CONSTRAINT uk_plugin_idc_compliance_red_letter_allocation_source
        UNIQUE ("tax_invoice_allocation_no", "refund_allocation_no"),
    CONSTRAINT fk_plugin_idc_compliance_red_letter_allocation_task
        FOREIGN KEY ("task_no", "tax_invoice_no")
        REFERENCES plugin_idc_compliance_red_letter_task ("task_no", "tax_invoice_no")
        ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_compliance_red_alloc_invoice_alloc
        FOREIGN KEY ("tax_invoice_allocation_no", "tax_invoice_no")
        REFERENCES plugin_idc_compliance_tax_invoice_allocation ("allocation_no", "tax_invoice_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_allocation_invoice
    ON plugin_idc_compliance_red_letter_allocation (
        "tenant_id", "tax_invoice_allocation_no", "task_no", "id"
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_red_letter_allocation_refund
    ON plugin_idc_compliance_red_letter_allocation (
        "tenant_id", "refund_allocation_no", "task_no", "id"
    );

COMMENT ON TABLE plugin_idc_compliance_red_letter_allocation IS
    'Immutable red-letter distribution linking one task to original tax-invoice and opaque core refund allocations';

-- Purpose: Persists business records for plugin_idc_compliance_external_evidence.
-- 用途：持久化 plugin_idc_compliance_external_evidence 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_external_evidence (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "evidence_no" VARCHAR(64) NOT NULL,
    "aggregate_type_code" VARCHAR(24) NOT NULL,
    "aggregate_no" VARCHAR(64) NOT NULL,
    "source_system_code" VARCHAR(64) NOT NULL,
    "source_event_ref" VARCHAR(256) NOT NULL,
    "evidence_type_code" VARCHAR(64) NOT NULL,
    "payload_sha256" CHAR(64) NOT NULL,
    "private_object_ref" VARCHAR(512),
    "media_type" VARCHAR(128),
    "byte_size" BIGINT,
    "disposition_code" VARCHAR(16) NOT NULL DEFAULT 'accepted',
    "occurred_at" TIMESTAMPTZ NOT NULL,
    "received_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_external_evidence_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_external_evidence_aggregate
        CHECK ("aggregate_type_code" IN ('verification', 'contract', 'tax_invoice', 'red_letter')),
    CONSTRAINT ck_plugin_idc_compliance_external_evidence_digest
        CHECK ("payload_sha256" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_plugin_idc_compliance_external_evidence_object
        CHECK (
            ("private_object_ref" IS NULL AND "media_type" IS NULL AND "byte_size" IS NULL) OR
            (
                "private_object_ref" IS NOT NULL AND "media_type" IS NOT NULL AND "byte_size" > 0
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_external_evidence_disposition
        CHECK ("disposition_code" IN ('accepted', 'duplicate', 'quarantined')),
    CONSTRAINT uk_plugin_idc_compliance_external_evidence_no UNIQUE ("evidence_no"),
    CONSTRAINT uk_plugin_idc_compliance_external_evidence_source
        UNIQUE ("tenant_id", "source_system_code", "source_event_ref")
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_external_evidence_aggregate
    ON plugin_idc_compliance_external_evidence (
        "tenant_id", "aggregate_type_code", "aggregate_no", "occurred_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_external_evidence_source
    ON plugin_idc_compliance_external_evidence (
        "tenant_id", "source_system_code", "received_at" DESC, "id" DESC
    );

COMMENT ON TABLE plugin_idc_compliance_external_evidence IS
    'Append-only digest and private-object metadata for external compliance evidence; raw payloads are forbidden';

CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_archived_file_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'compliance file metadata cannot be deleted';
    END IF;
    IF OLD."immutable" OR OLD."status_code" IN ('archived', 'legal_hold') OR OLD."legal_hold" THEN
        IF NEW."status_code" NOT IN ('archived', 'legal_hold', 'deletion_pending', 'deleted') THEN
            RAISE EXCEPTION 'archived compliance file cannot return to a mutable lifecycle';
        END IF;
        IF (
            to_jsonb(NEW) - ARRAY['status_code', 'retention_until', 'legal_hold', 'updated_at']
        ) IS DISTINCT FROM (
            to_jsonb(OLD) - ARRAY['status_code', 'retention_until', 'legal_hold', 'updated_at']
        ) THEN
            RAISE EXCEPTION 'archived compliance file content and ownership metadata are immutable';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_tax_invoice_file_immutable
    ON plugin_idc_compliance_tax_invoice_file;
CREATE TRIGGER trg_plugin_idc_compliance_tax_invoice_file_immutable
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_tax_invoice_file
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_archived_file_mutation();

CREATE OR REPLACE FUNCTION plugin_idc_compliance_reject_append_only_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'compliance history is append-only';
END;
$$;

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_red_alloc_append_only
    ON plugin_idc_compliance_red_letter_allocation;
CREATE TRIGGER trg_plugin_idc_compliance_red_alloc_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_red_letter_allocation
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_compliance_external_evidence_append_only
    ON plugin_idc_compliance_external_evidence;
CREATE TRIGGER trg_plugin_idc_compliance_external_evidence_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_external_evidence
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();
