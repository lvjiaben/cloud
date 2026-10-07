-- IDC migration staging, reconciliation and cutover evidence tables.
-- Only migration-owned records are stored here; source credentials are never persisted.
-- Purpose: Stores migration batches, checkpoints and aggregate execution state.
-- 用途：保存迁移批次、检查点和整体执行状态。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_batch (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_no" VARCHAR(80) NOT NULL,
    source_system VARCHAR(40) NOT NULL,
    "source_version" VARCHAR(80) NOT NULL,
    mode VARCHAR(20) NOT NULL DEFAULT 'dry_run',
    "status" VARCHAR(30) NOT NULL DEFAULT 'preflight',
    watermark VARCHAR(160) NOT NULL DEFAULT '',
    blockers JSONB NOT NULL DEFAULT '[]'::jsonb,
    warnings JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_hash VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (source_system, "batch_no")
);
-- Purpose: Stores source-to-target record mappings within one migration batch.
-- 用途：保存单个迁移批次内的源记录到目标记录映射。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_mapping (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_id" BIGINT NOT NULL REFERENCES plugin_idc_zjmfmigrate_batch("id"),
    "source_type" VARCHAR(80) NOT NULL,
    "source_id" VARCHAR(160) NOT NULL,
    "target_type" VARCHAR(80) NOT NULL,
    "target_id" VARCHAR(160) NOT NULL DEFAULT '',
    "status" VARCHAR(30) NOT NULL DEFAULT 'staged',
    "error_code" VARCHAR(80) NOT NULL DEFAULT '',
    source_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("batch_id", "source_type", "source_id")
);
-- Purpose: Stores migration conflicts and manual resolution decisions.
-- 用途：保存迁移冲突及人工处理决定。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_conflict (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_id" BIGINT NOT NULL REFERENCES plugin_idc_zjmfmigrate_batch("id"),
    "object_type" VARCHAR(80) NOT NULL,
    "source_id" VARCHAR(160) NOT NULL,
    "summary" VARCHAR(500) NOT NULL,
    "decision" VARCHAR(30) NOT NULL DEFAULT 'pending',
    "decided_by" BIGINT NOT NULL DEFAULT 0,
    decided_at TIMESTAMPTZ,
    UNIQUE ("batch_id", "object_type", "source_id")
);
-- Purpose: Stores migrated attachment locations, checksums and retry state.
-- 用途：保存迁移附件的位置、校验值和重试状态。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_attachment (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_id" BIGINT NOT NULL REFERENCES plugin_idc_zjmfmigrate_batch("id"),
    "object_type" VARCHAR(80) NOT NULL,
    "source_id" VARCHAR(160) NOT NULL,
    "object_key" VARCHAR(500) NOT NULL,
    "media_type" VARCHAR(160) NOT NULL,
    "size_bytes" BIGINT NOT NULL DEFAULT 0,
    "sha256" VARCHAR(128) NOT NULL DEFAULT '',
    "status" VARCHAR(30) NOT NULL DEFAULT 'pending',
    "retry_count" INT NOT NULL DEFAULT 0,
    UNIQUE ("batch_id", "object_type", "source_id", "object_key")
);
-- Purpose: Stores source-to-target amount reconciliation results by batch and currency.
-- 用途：按批次和币种保存源端到目标端的金额对账结果。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_reconciliation (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_id" BIGINT NOT NULL REFERENCES plugin_idc_zjmfmigrate_batch("id"),
    "object_type" VARCHAR(80) NOT NULL,
    "currency" VARCHAR(16) NOT NULL DEFAULT '',
    "source_amount" NUMERIC(24,8) NOT NULL DEFAULT 0,
    "target_amount" NUMERIC(24,8) NOT NULL DEFAULT 0,
    "difference" NUMERIC(24,8) NOT NULL DEFAULT 0,
    "status" VARCHAR(30) NOT NULL DEFAULT 'pending',
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE ("batch_id", "object_type", "currency")
);
-- Purpose: Stores cutover checks and their final readiness decisions.
-- 用途：保存割接检查及其最终就绪结论。
CREATE TABLE IF NOT EXISTS plugin_idc_zjmfmigrate_cutover (
    "id" BIGSERIAL PRIMARY KEY,
    "batch_id" BIGINT NOT NULL REFERENCES plugin_idc_zjmfmigrate_batch("id"),
    "check_name" VARCHAR(120) NOT NULL,
    "status" VARCHAR(30) NOT NULL,
    "detail" VARCHAR(500) NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE ("batch_id", "check_name")
);
CREATE INDEX IF NOT EXISTS idx_idc_zjmfmigrate_mapping_batch_status ON plugin_idc_zjmfmigrate_mapping("batch_id", "status");
CREATE INDEX IF NOT EXISTS idx_idc_zjmfmigrate_conflict_batch_decision ON plugin_idc_zjmfmigrate_conflict("batch_id", "decision");
CREATE INDEX IF NOT EXISTS idx_idc_zjmfmigrate_reconcile_batch_status ON plugin_idc_zjmfmigrate_reconciliation("batch_id", "status");
