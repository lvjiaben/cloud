-- Purpose: Keeps staff numeric pages ordered within the same channel scope.
-- 用途：为后台渠道分页提供与渠道过滤及稳定排序匹配的索引。
CREATE INDEX IF NOT EXISTS idx_idc_channel_account_page
    ON plugin_idc_channel_account(updated_at DESC, channel_id);
CREATE INDEX IF NOT EXISTS idx_idc_channel_token_page
    ON plugin_idc_channel_token(channel_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_channel_publication_page
    ON plugin_idc_channel_publication(channel_id, id DESC);
CREATE INDEX IF NOT EXISTS idx_idc_channel_request_page
    ON plugin_idc_channel_request(channel_id, id DESC);
