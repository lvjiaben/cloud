-- Purpose: Store owner-only callback correlation without exposing callback secrets to WASM.
-- 用途：保存仅资源拥有方可用的回调关联，且不向 WASM 暴露回调秘密。
CREATE TABLE IF NOT EXISTS plugin_idc_resource_provider_correlation (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "task_id" BIGINT NOT NULL,
    "task_no" VARCHAR(64) NOT NULL,
    "plugin_id" VARCHAR(128) NOT NULL,
    "channel_no" VARCHAR(64) NOT NULL,
    "configuration_revision" VARCHAR(36) NOT NULL,
    "downstream_url" VARCHAR(2048) NOT NULL,
    "token_secret_ref" VARCHAR(255) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT plugin_idc_resource_provider_correlation_task_fk
        FOREIGN KEY ("task_id") REFERENCES plugin_idc_resource_task("id"),
    CONSTRAINT plugin_idc_resource_provider_correlation_task_unique UNIQUE ("tenant_id", "task_id"),
    CONSTRAINT plugin_idc_resource_provider_correlation_task_no_unique UNIQUE ("tenant_id", "task_no"),
    CONSTRAINT plugin_idc_resource_provider_correlation_revision_check
        CHECK ("configuration_revision" ~ '^rci_[0-9a-f]{32}$'),
    CONSTRAINT plugin_idc_resource_provider_correlation_url_check
        CHECK ("downstream_url" !~ '[[:cntrl:]]')
);

COMMENT ON TABLE plugin_idc_resource_provider_correlation IS
'Owner-only callback correlation. The random callback token remains an opaque secret reference and is never projected to WASM.';
