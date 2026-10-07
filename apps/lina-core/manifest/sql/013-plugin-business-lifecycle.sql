-- ============================================================
-- Purpose: Separates stopping new plugin business from disabling or
-- uninstalling a plugin while preserving existing-service access.
-- 用途：将插件停止新业务与停用、卸载分离，同时保留已有服务访问能力。
-- ============================================================

ALTER TABLE sys_plugin
    ADD COLUMN IF NOT EXISTS "new_business_state" VARCHAR(32) NOT NULL DEFAULT 'accepting';

ALTER TABLE sys_plugin
    ADD COLUMN IF NOT EXISTS "new_business_stopped_at" TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_sys_plugin_new_business_state'
          AND conrelid = 'sys_plugin'::regclass
    ) THEN
        ALTER TABLE sys_plugin
            ADD CONSTRAINT ck_sys_plugin_new_business_state
            CHECK ("new_business_state" IN ('accepting', 'stopped'));
    END IF;
END $$;

COMMENT ON COLUMN sys_plugin."new_business_state" IS
    'New business admission state: accepting/stopped; it does not control existing-service visibility';
COMMENT ON COLUMN sys_plugin."new_business_stopped_at" IS
    'Time when the plugin most recently stopped accepting new business';

CREATE INDEX IF NOT EXISTS idx_sys_plugin_new_business_candidates
    ON sys_plugin ("installed", "status", "new_business_state", "distribution");

INSERT INTO sys_menu (
    "parent_id", "menu_key", "name", "path", "component", "perms", "icon",
    "type", "sort", "visible", "status", "is_frame", "is_cache", "remark",
    "created_at", "updated_at"
)
SELECT
    parent."id", seed."menu_key", seed."name", '', '', seed."perms", '',
    'B', seed."sort", 1, 1, 0, 0, seed."remark", NOW(), NOW()
FROM (
    VALUES
        ('extension:plugin:business-impact', '插件影响检查', 'plugin:business-impact', 7, '查看插件停止新业务、停用和卸载影响'),
        ('extension:plugin:stop-new-business', '插件停止新业务', 'plugin:stop-new-business', 8, '停止插件接受新的业务绑定和购买'),
        ('extension:plugin:resume-new-business', '插件恢复新业务', 'plugin:resume-new-business', 9, '恢复插件接受新的业务绑定和购买')
) AS seed("menu_key", "name", "perms", "sort", "remark")
JOIN sys_menu AS parent ON parent."menu_key" = 'extension:plugin:list'
WHERE NOT EXISTS (
    SELECT 1 FROM sys_menu existing WHERE existing."menu_key" = seed."menu_key"
);
