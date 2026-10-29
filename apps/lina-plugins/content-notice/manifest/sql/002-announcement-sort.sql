-- 按排序值升序、编号倒序稳定排列公告。
ALTER TABLE plugin_content_notice ADD COLUMN IF NOT EXISTS sort_order INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_plugin_content_notice_tenant_sort ON plugin_content_notice (tenant_id, sort_order, id DESC) WHERE deleted_at IS NULL;
