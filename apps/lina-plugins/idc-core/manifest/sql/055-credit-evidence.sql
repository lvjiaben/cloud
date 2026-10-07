-- Private credit-application evidence uploads. Objects are never exposed by a public URL.
-- Purpose: Stores Credit Evidence Upload records.
-- 用途：保存信用证据上传相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_credit_evidence_upload (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "upload_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "uploader_user_id" BIGINT NOT NULL,
    "application_no" VARCHAR(40),
    "file_name" VARCHAR(255) NOT NULL,
    "media_type" VARCHAR(80),
    "declared_size_bytes" BIGINT NOT NULL,
    "sha256_hex" CHAR(64),
    "object_ref" VARCHAR(500) NOT NULL,
    "status_code" VARCHAR(20) NOT NULL DEFAULT 'reserved',
    "expires_at" TIMESTAMPTZ NOT NULL,
    "completed_at" TIMESTAMPTZ,
    "consumed_at" TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_idc_credit_evidence_upload_no UNIQUE ("upload_no"),
    CONSTRAINT fk_idc_credit_evidence_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no"),
    CONSTRAINT fk_idc_credit_evidence_application FOREIGN KEY ("application_no")
        REFERENCES plugin_idc_core_credit_application ("application_no"),
    CONSTRAINT ck_idc_credit_evidence_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_credit_evidence_size CHECK ("declared_size_bytes" BETWEEN 1 AND 20971520),
    CONSTRAINT ck_idc_credit_evidence_status CHECK ("status_code" IN ('reserved','ready','consumed','expired','rejected')),
    CONSTRAINT ck_idc_credit_evidence_hash CHECK ("sha256_hex" IS NULL OR "sha256_hex" ~ '^[0-9a-f]{64}$'),
    CONSTRAINT ck_idc_credit_evidence_ready CHECK (
        "status_code" NOT IN ('ready','consumed') OR
        (length("media_type") > 0 AND length("sha256_hex") = 64 AND "completed_at" IS NOT NULL)
    ),
    CONSTRAINT ck_idc_credit_evidence_binding CHECK (
        ("status_code" = 'consumed' AND "application_no" IS NOT NULL AND "consumed_at" IS NOT NULL) OR
        ("status_code" <> 'consumed' AND "application_no" IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_idc_credit_evidence_customer_ready
    ON plugin_idc_core_credit_evidence_upload ("tenant_id", "customer_no", "status_code", "expires_at", "id");
CREATE INDEX IF NOT EXISTS idx_idc_credit_evidence_application
    ON plugin_idc_core_credit_evidence_upload ("tenant_id", "application_no", "id")
    WHERE "application_no" IS NOT NULL;
