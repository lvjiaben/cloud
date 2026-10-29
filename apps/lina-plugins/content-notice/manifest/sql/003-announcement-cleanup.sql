-- 公告简化：删除类型、附件字段及站内投递类型，所有发布公告统一可读。
DROP INDEX IF EXISTS idx_plugin_content_notice_tenant_type;
ALTER TABLE plugin_content_notice DROP COLUMN IF EXISTS type;
ALTER TABLE plugin_content_notice DROP COLUMN IF EXISTS file_ids;
ALTER TABLE plugin_content_notice DROP COLUMN IF EXISTS remark;
-- 公告标题可以重复，避免给简单的内容发布附加业务限制。
DROP INDEX IF EXISTS uk_plugin_content_notice_tenant_title;
ALTER TABLE plugin_content_notice ALTER COLUMN title TYPE TEXT;
CREATE INDEX IF NOT EXISTS idx_plugin_content_notice_tenant_sort ON plugin_content_notice (tenant_id, sort_order, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_plugin_content_notice_published_sort ON plugin_content_notice (status, sort_order, id DESC) WHERE deleted_at IS NULL;
CREATE TABLE IF NOT EXISTS plugin_content_notice_read (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    notice_id BIGINT NOT NULL REFERENCES plugin_content_notice(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, notice_id)
);
COMMENT ON TABLE plugin_content_notice_read IS 'Announcement read state for the current user';
COMMENT ON COLUMN plugin_content_notice_read.notice_id IS 'Announcement ID';
COMMENT ON COLUMN plugin_content_notice_read.user_id IS 'Reader user ID';
COMMENT ON COLUMN plugin_content_notice_read.created_at IS 'Read time';
DELETE FROM sys_dict_data WHERE dict_type='sys_notice_type';
DELETE FROM sys_dict_type WHERE type='sys_notice_type';

-- 受控的一次性跨插件迁移：旧泛内容公告搬到唯一公告源后删除，运行期不会跨插件查私有表。
DO $$
DECLARE child_table TEXT;
BEGIN
    IF to_regclass('plugin_idc_core_content_item') IS NOT NULL THEN
        INSERT INTO plugin_content_notice (tenant_id,title,content,status,created_by,updated_by,created_at,updated_at,deleted_at)
        SELECT tenant_id,title,body,CASE WHEN status_code='published' THEN 1 ELSE 0 END,created_by,updated_by,created_at,updated_at,deleted_at
        FROM plugin_idc_core_content_item WHERE content_type='announcement';
        FOREACH child_table IN ARRAY ARRAY[
            'plugin_idc_core_content_download_ticket',
            'plugin_idc_core_content_download_authorization',
            'plugin_idc_core_content_file',
            'plugin_idc_core_content_upload_session',
            'plugin_idc_core_content_action_history',
            'plugin_idc_core_content_customer_scope',
            'plugin_idc_core_content_group_scope',
            'plugin_idc_core_content_service_scope'
        ] LOOP
            IF to_regclass(child_table) IS NOT NULL THEN
                EXECUTE format('DELETE FROM %I WHERE content_no IN (SELECT content_no FROM plugin_idc_core_content_item WHERE content_type=''announcement'')', child_table);
            END IF;
        END LOOP;
        DELETE FROM plugin_idc_core_content_item WHERE content_type='announcement';
        ALTER TABLE plugin_idc_core_content_item DROP CONSTRAINT IF EXISTS ck_idc_content_item_type;
        ALTER TABLE plugin_idc_core_content_item ADD CONSTRAINT ck_idc_content_item_type CHECK (content_type IN ('news','knowledge','download'));
        ALTER TABLE plugin_idc_core_content_item DROP CONSTRAINT IF EXISTS ck_idc_content_item_category;
        ALTER TABLE plugin_idc_core_content_item DROP CONSTRAINT IF EXISTS ck_idc_content_item_visibility;
        ALTER TABLE plugin_idc_core_content_item ADD CONSTRAINT ck_idc_content_item_visibility CHECK (visibility_code IN ('public','all_logged_in','customer_group','internal'));
    END IF;
END $$;

DROP TABLE IF EXISTS plugin_idc_core_content_customer_scope;
DROP TABLE IF EXISTS plugin_idc_core_content_service_scope;
