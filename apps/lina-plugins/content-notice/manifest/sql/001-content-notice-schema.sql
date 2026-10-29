-- 001: content-notice schema
-- 001：content-notice 数据结构

-- Purpose: Stores tenant-scoped announcements, including publication status, content, and audit fields.
-- 用途：存储租户级公告，包括发布状态、正文与审计字段。
CREATE TABLE IF NOT EXISTS plugin_content_notice (
    "id"          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id"   INT          NOT NULL DEFAULT 0,
    "title"       TEXT         NOT NULL DEFAULT '',
    "content"     TEXT         NOT NULL,
    "status"      SMALLINT     NOT NULL DEFAULT 0,
    "created_by"  BIGINT       NOT NULL DEFAULT 0,
    "updated_by"  BIGINT       NOT NULL DEFAULT 0,
    "created_at"  TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at"  TIMESTAMPTZ    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "deleted_at"  TIMESTAMPTZ    NULL DEFAULT NULL
);

COMMENT ON TABLE plugin_content_notice IS 'Notice table';
COMMENT ON COLUMN plugin_content_notice."id" IS 'Notice ID';
COMMENT ON COLUMN plugin_content_notice."tenant_id" IS 'Owning tenant ID, 0 means PLATFORM';
COMMENT ON COLUMN plugin_content_notice."title" IS 'Notice title';
COMMENT ON COLUMN plugin_content_notice."content" IS 'Notice content';
COMMENT ON COLUMN plugin_content_notice."status" IS 'Notice status: 0=draft, 1=published';
COMMENT ON COLUMN plugin_content_notice."created_by" IS 'Creator';
COMMENT ON COLUMN plugin_content_notice."updated_by" IS 'Updater';
COMMENT ON COLUMN plugin_content_notice."created_at" IS 'Creation time';
COMMENT ON COLUMN plugin_content_notice."updated_at" IS 'Update time';
COMMENT ON COLUMN plugin_content_notice."deleted_at" IS 'Deletion time';

CREATE INDEX IF NOT EXISTS idx_plugin_content_notice_tenant_status ON plugin_content_notice ("tenant_id", "status");

INSERT INTO sys_dict_type ("name", "type", "status", "is_builtin", "remark", "created_at", "updated_at")
VALUES ('公告状态', 'sys_notice_status', 1, 1, '公告状态列表', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING;
INSERT INTO sys_dict_data ("dict_type", "label", "value", "sort", "tag_style", "status", "is_builtin", "created_at", "updated_at")
VALUES ('sys_notice_status', '草稿', '0', 1, 'default', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), ('sys_notice_status', '已发布', '1', 2, 'success', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT DO NOTHING;
