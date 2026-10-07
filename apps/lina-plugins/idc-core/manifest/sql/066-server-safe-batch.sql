-- 066: Persist bounded service batches, immutable per-service previews, safe
-- metadata, and reminder delivery evidence. Dangerous execution never stores
-- recent-authentication proof material and always advances one item at a time.

-- Purpose: Stores Server Service Tag records.
-- 用途：保存服务器服务标签相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_service_tag (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "tag_code" VARCHAR(40) NOT NULL,
    "created_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_service_tag_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_service_tag_code CHECK (
        "tag_code" ~ '^[a-z0-9][a-z0-9._-]{0,39}$'
    ),
    CONSTRAINT uk_idc_server_service_tag UNIQUE ("service_no", "tag_code"),
    CONSTRAINT fk_idc_server_service_tag_service FOREIGN KEY (
        "service_no", "customer_no"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_service_tag_search
    ON plugin_idc_core_server_service_tag (
        "tenant_id", "tag_code", "service_no"
    );

-- Purpose: Stores Server Classification records.
-- 用途：保存服务器分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_classification (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "classification_no" VARCHAR(40) NOT NULL,
    "name_zh_cn" VARCHAR(80) NOT NULL,
    "name_en_us" VARCHAR(80) NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_classification_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_classification_name CHECK (
        length(btrim("name_zh_cn")) BETWEEN 1 AND 80
        AND length(btrim("name_en_us")) BETWEEN 1 AND 80
    ),
    CONSTRAINT ck_idc_server_classification_status CHECK (
        "status_code" IN ('active', 'disabled')
    ),
    CONSTRAINT ck_idc_server_classification_version CHECK ("version" > 0),
    CONSTRAINT uk_idc_server_classification_no UNIQUE ("classification_no")
);

CREATE INDEX IF NOT EXISTS idx_idc_server_classification_directory
    ON plugin_idc_core_server_classification (
        "tenant_id", "status_code", "name_zh_cn", "id"
    );

-- Purpose: Stores Server Service Classification records.
-- 用途：保存服务器服务分类相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_service_classification (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "classification_no" VARCHAR(40) NOT NULL,
    "assigned_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_service_classification_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_idc_server_service_classification UNIQUE ("service_no"),
    CONSTRAINT fk_idc_server_service_classification_service FOREIGN KEY (
        "service_no", "customer_no"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no"
    ) ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_service_classification_directory FOREIGN KEY (
        "classification_no"
    ) REFERENCES plugin_idc_core_server_classification (
        "classification_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_service_classification_search
    ON plugin_idc_core_server_service_classification (
        "tenant_id", "classification_no", "service_no"
    );

-- Purpose: Stores Server Service Note records.
-- 用途：保存服务器服务备注相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_service_note (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "note_no" VARCHAR(40) NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "content" VARCHAR(1000) NOT NULL,
    "created_by" BIGINT NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_service_note_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_service_note_content CHECK (
        length(btrim("content")) BETWEEN 1 AND 1000
    ),
    CONSTRAINT ck_idc_server_service_note_digest CHECK (
        "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_server_service_note_no UNIQUE ("note_no"),
    CONSTRAINT uk_idc_server_service_note_request UNIQUE (
        "created_by", "service_no", "idempotency_digest"
    ),
    CONSTRAINT fk_idc_server_service_note_service FOREIGN KEY (
        "service_no", "customer_no"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_service_note_history
    ON plugin_idc_core_server_service_note (
        "tenant_id", "service_no", "created_at" DESC, "id" DESC
    );

-- Purpose: Bounded staff service batch with an immutable exact-target preview digest.
-- 用途：保存服务器服务批次相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_service_batch (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "batch_no" VARCHAR(40) NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "dangerous" BOOLEAN NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'previewed',
    "item_count" INT NOT NULL,
    "preview_digest" CHAR(64) NOT NULL,
    "idempotency_digest" CHAR(64) NOT NULL,
    "request_digest" CHAR(64) NOT NULL,
    "requested_by" BIGINT NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "confirmed_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_service_batch_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_service_batch_action CHECK (
        "action_code" IN (
            'tag_add', 'tag_remove', 'classification_set', 'note_append', 'renewal_reminder',
            'suspend', 'reboot', 'reinstall', 'ip_change', 'terminate'
        )
        AND "dangerous" = ("action_code" IN (
            'suspend', 'reboot', 'reinstall', 'ip_change', 'terminate'
        ))
    ),
    CONSTRAINT ck_idc_server_service_batch_status CHECK (
        "status_code" IN ('previewed', 'executing', 'completed')
    ),
    CONSTRAINT ck_idc_server_service_batch_count CHECK (
        "item_count" BETWEEN 1 AND 20
    ),
    CONSTRAINT ck_idc_server_service_batch_digest CHECK (
        "preview_digest" ~ '^[0-9a-f]{64}$'
        AND "idempotency_digest" ~ '^[0-9a-f]{64}$'
        AND "request_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_idc_server_service_batch_times CHECK (
        ("status_code" = 'previewed' AND "confirmed_at" IS NULL AND "completed_at" IS NULL)
        OR ("status_code" = 'executing' AND "confirmed_at" IS NOT NULL AND "completed_at" IS NULL)
        OR ("status_code" = 'completed' AND "confirmed_at" IS NOT NULL AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT uk_idc_server_service_batch_no UNIQUE ("batch_no"),
    CONSTRAINT uk_idc_server_service_batch_request UNIQUE (
        "requested_by", "idempotency_digest"
    )
);

CREATE INDEX IF NOT EXISTS idx_idc_server_service_batch_operator
    ON plugin_idc_core_server_service_batch (
        "tenant_id", "requested_by", "created_at" DESC, "id" DESC
    );

-- Purpose: Per-service impact and independently converged idempotent batch outcome.
-- 用途：保存服务器服务批次明细相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_server_service_batch_item (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "batch_no" VARCHAR(40) NOT NULL,
    "line_number" INT NOT NULL,
    "service_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "service_version" BIGINT NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "entitlement_status_code" VARCHAR(24) NOT NULL,
    "runtime_status_code" VARCHAR(24) NOT NULL,
    "provider_task_status_code" VARCHAR(24) NOT NULL,
    "entitlement_expires_at" TIMESTAMPTZ,
    "unpaid_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "action_amount_minor" BIGINT NOT NULL DEFAULT 0,
    "resource_summary" VARCHAR(256) NOT NULL,
    "irreversible" BOOLEAN NOT NULL DEFAULT FALSE,
    "parameters" JSONB NOT NULL DEFAULT '{}'::JSONB,
    "preview_status_code" VARCHAR(24) NOT NULL,
    "failure_code" VARCHAR(64) NOT NULL DEFAULT '',
    "execution_status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "operation_no" VARCHAR(40),
    "notification_event_no" VARCHAR(64),
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "executed_at" TIMESTAMPTZ,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_server_service_batch_item_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_server_service_batch_item_line CHECK ("line_number" BETWEEN 1 AND 20),
    CONSTRAINT ck_idc_server_service_batch_item_values CHECK (
        "service_version" > 0 AND "currency_code" ~ '^[A-Z]{3}$'
        AND "unpaid_amount_minor" >= 0 AND "action_amount_minor" >= 0
        AND jsonb_typeof("parameters") = 'object'
        AND length("resource_summary") BETWEEN 1 AND 256
    ),
    CONSTRAINT ck_idc_server_service_batch_item_preview CHECK (
        "preview_status_code" IN ('eligible', 'ineligible')
    ),
    CONSTRAINT ck_idc_server_service_batch_item_execution CHECK (
        "execution_status_code" IN (
            'pending', 'succeeded', 'failed', 'manual_review', 'skipped'
        )
        AND (("execution_status_code" = 'pending' AND "executed_at" IS NULL)
          OR ("execution_status_code" <> 'pending' AND "executed_at" IS NOT NULL))
    ),
    CONSTRAINT uk_idc_server_service_batch_item_line UNIQUE (
        "batch_no", "line_number"
    ),
    CONSTRAINT uk_idc_server_service_batch_item_service UNIQUE (
        "batch_no", "service_no"
    ),
    CONSTRAINT fk_idc_server_service_batch_item_batch FOREIGN KEY ("batch_no")
        REFERENCES plugin_idc_core_server_service_batch ("batch_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_server_service_batch_item_service FOREIGN KEY (
        "service_no", "customer_no", "currency_code"
    ) REFERENCES plugin_idc_core_service_instance (
        "service_no", "customer_no", "currency_code"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_server_service_batch_item_work
    ON plugin_idc_core_server_service_batch_item (
        "tenant_id", "batch_no", "execution_status_code", "line_number"
    );

COMMENT ON TABLE plugin_idc_core_server_service_batch
    IS 'Bounded staff service batch with an immutable exact-target preview digest';
COMMENT ON TABLE plugin_idc_core_server_service_batch_item
    IS 'Per-service impact and independently converged idempotent batch outcome';
