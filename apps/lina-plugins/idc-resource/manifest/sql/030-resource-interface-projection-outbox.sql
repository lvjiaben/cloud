-- 030: persist frozen connector evidence beside the pfi-to-channel binding and
-- deliver secret-free interface projections through a retryable outbox.

ALTER TABLE plugin_idc_resource_interface_channel_binding
    ADD COLUMN IF NOT EXISTS "display_name" VARCHAR(128),
    ADD COLUMN IF NOT EXISTS "resource_plugin_id" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "resource_plugin_release_version" VARCHAR(32),
    ADD COLUMN IF NOT EXISTS "resource_contract_version" VARCHAR(32),
    ADD COLUMN IF NOT EXISTS "resource_authorized_methods" TEXT[],
    ADD COLUMN IF NOT EXISTS "resource_release_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "resource_declaration_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "product_plugin_id" VARCHAR(64),
    ADD COLUMN IF NOT EXISTS "product_plugin_release_version" VARCHAR(32),
    ADD COLUMN IF NOT EXISTS "product_contract_version" VARCHAR(32),
    ADD COLUMN IF NOT EXISTS "product_authorized_methods" TEXT[],
    ADD COLUMN IF NOT EXISTS "product_release_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "product_declaration_digest" CHAR(64),
    ADD COLUMN IF NOT EXISTS "new_business_state" VARCHAR(16);

ALTER TABLE plugin_idc_resource_interface_channel_binding
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_interface_channel_projection;
ALTER TABLE plugin_idc_resource_interface_channel_binding
    ADD CONSTRAINT ck_plugin_idc_resource_interface_channel_projection CHECK (
        (
            "resource_plugin_id" IS NULL AND "resource_plugin_release_version" IS NULL AND
            "resource_contract_version" IS NULL AND "resource_authorized_methods" IS NULL AND
            "resource_release_digest" IS NULL AND "resource_declaration_digest" IS NULL AND
            "product_plugin_id" IS NULL AND "product_plugin_release_version" IS NULL AND
            "product_contract_version" IS NULL AND "product_authorized_methods" IS NULL AND
            "product_release_digest" IS NULL AND "product_declaration_digest" IS NULL AND
            "new_business_state" IS NULL AND "display_name" IS NULL
        ) OR
        (
            "display_name" = btrim("display_name") AND length("display_name") > 0 AND
            "resource_plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND
            length("resource_plugin_release_version") > 0 AND length("resource_contract_version") > 0 AND
            cardinality("resource_authorized_methods") BETWEEN 1 AND 16 AND
            "resource_release_digest" ~ '^[0-9a-f]{64}$' AND
            "resource_declaration_digest" ~ '^[0-9a-f]{64}$' AND
            "product_plugin_id" = "resource_plugin_id" AND
            "product_plugin_release_version" = "resource_plugin_release_version" AND
            "product_contract_version" = 'v1' AND
            cardinality("product_authorized_methods") BETWEEN 1 AND 16 AND
            "product_release_digest" = "resource_release_digest" AND
            "product_declaration_digest" ~ '^[0-9a-f]{64}$' AND
            "new_business_state" IN ('accepting', 'stopped')
        )
    );

-- Purpose: Persists retryable, secret-free resource interface projections.
-- 用途：持久化可重试且不含密钥的服务器供应商接口投影事件。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_interface_projection_outbox (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "outbox_no" VARCHAR(64) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "projection_sequence" BIGINT NOT NULL,
    "projection_payload" JSONB NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'pending',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "available_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "locked_by" VARCHAR(128) NOT NULL DEFAULT '',
    "lease_expires_at" TIMESTAMPTZ,
    "delivered_at" TIMESTAMPTZ,
    "last_failure_code" VARCHAR(128) NOT NULL DEFAULT '',
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_no CHECK (
        "outbox_no" ~ '^RIO_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_interface CHECK (
        "interface_instance_no" ~ '^pfi_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_sequence CHECK (
        "projection_sequence" > 0
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_payload CHECK (
        jsonb_typeof("projection_payload") = 'object'
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_status CHECK (
        "status_code" IN ('pending', 'delivering', 'delivered', 'manual_attention')
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_attempt CHECK (
        "attempt_count" BETWEEN 0 AND 10
    ),
    CONSTRAINT ck_plugin_idc_resource_interface_outbox_delivery CHECK (
        ("status_code" = 'delivered' AND "delivered_at" IS NOT NULL AND "lease_expires_at" IS NULL) OR
        ("status_code" = 'delivering' AND "delivered_at" IS NULL AND "lease_expires_at" IS NOT NULL) OR
        ("status_code" IN ('pending','manual_attention') AND "delivered_at" IS NULL AND "lease_expires_at" IS NULL)
    ),
    CONSTRAINT uk_plugin_idc_resource_interface_outbox_no UNIQUE ("outbox_no"),
    CONSTRAINT uk_plugin_idc_resource_interface_outbox_sequence UNIQUE (
        "tenant_id", "interface_instance_no", "projection_sequence"
    ),
    CONSTRAINT fk_plugin_idc_resource_interface_outbox_interface FOREIGN KEY (
        "tenant_id", "interface_instance_no"
    ) REFERENCES plugin_idc_resource_interface_channel_binding (
        "tenant_id", "interface_instance_no"
    ) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_resource_interface_outbox_claim
    ON plugin_idc_resource_interface_projection_outbox (
        "tenant_id", "status_code", "available_at", "id"
    ) WHERE "status_code" IN ('pending', 'delivering');

COMMENT ON TABLE plugin_idc_resource_interface_projection_outbox IS
    'Retryable owner outbox for secret-free product interface projections';
COMMENT ON COLUMN plugin_idc_resource_interface_projection_outbox."projection_payload" IS
    'Whitelisted pfi, display name, channel reference and frozen connector evidence only; endpoint and credential material are forbidden';
