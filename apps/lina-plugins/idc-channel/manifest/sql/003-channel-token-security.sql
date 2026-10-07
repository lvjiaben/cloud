-- One bounded, database-authoritative rate window per channel token.
-- Purpose: Stores the database-authoritative request counter for each channel token window.
-- 用途：保存每个渠道令牌时间窗口的数据库权威请求计数。
CREATE TABLE IF NOT EXISTS plugin_idc_channel_token_rate_window (
    "token_id" BIGINT PRIMARY KEY,
    window_started_at TIMESTAMPTZ NOT NULL,
    "request_count" INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_plugin_idc_channel_token_rate_window_token
        FOREIGN KEY ("token_id") REFERENCES plugin_idc_channel_token("id") ON DELETE CASCADE,
    CONSTRAINT ck_plugin_idc_channel_token_rate_window_count
        CHECK ("request_count" >= 0)
);

COMMENT ON TABLE plugin_idc_channel_token_rate_window IS
    'Database-authoritative fixed-minute request counter for one downstream channel token';
