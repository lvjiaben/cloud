-- Purpose: Persists business records for plugin_staff_access_grant.
-- 用途：持久化 plugin_staff_access_grant 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_staff_access_grant (
    "id" BIGSERIAL PRIMARY KEY,
    "tenant_id" BIGINT NOT NULL DEFAULT 0,
    "grant_no" VARCHAR(64) NOT NULL,
    "user_id" BIGINT NOT NULL,
    "capability_code" VARCHAR(128) NOT NULL,
    "scope_type" VARCHAR(32) NOT NULL,
    "scope_key" VARCHAR(128) NOT NULL DEFAULT '',
    "status" SMALLINT NOT NULL DEFAULT 1,
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" BIGINT NOT NULL,
    "updated_by" BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_plugin_staff_access_grant_no UNIQUE ("grant_no"),
    CONSTRAINT uq_plugin_staff_access_grant_target UNIQUE ("tenant_id", "user_id", "capability_code", "scope_type", "scope_key"),
    CONSTRAINT ck_plugin_staff_access_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_staff_access_scope CHECK (
        ("scope_type" = 'global' AND "scope_key" = '') OR
        ("scope_type" = 'department' AND "scope_key" <> '')
    ),
    CONSTRAINT ck_plugin_staff_access_status CHECK ("status" IN (0, 1)),
    CONSTRAINT ck_plugin_staff_access_validity CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from),
    CONSTRAINT ck_plugin_staff_access_version CHECK ("version" > 0)
);

ALTER TABLE plugin_staff_access_grant
    ADD COLUMN IF NOT EXISTS valid_from TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_staff_access_validity'
          AND conrelid = 'plugin_staff_access_grant'::regclass
    ) THEN
        ALTER TABLE plugin_staff_access_grant
            ADD CONSTRAINT ck_plugin_staff_access_validity
            CHECK (valid_until IS NULL OR valid_from IS NULL OR valid_until > valid_from);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_plugin_staff_access_lookup
    ON plugin_staff_access_grant ("tenant_id", "capability_code", "scope_type", "scope_key", "status", "user_id");

CREATE INDEX IF NOT EXISTS idx_plugin_staff_access_user
    ON plugin_staff_access_grant ("tenant_id", "user_id", "status", "capability_code");

-- Purpose: Persists business records for plugin_staff_access_event.
-- 用途：持久化 plugin_staff_access_event 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_staff_access_event (
    "id" BIGSERIAL PRIMARY KEY,
    "event_no" VARCHAR(64) NOT NULL,
    "grant_no" VARCHAR(64) NOT NULL,
    "action_code" VARCHAR(32) NOT NULL,
    "actor_user_id" BIGINT NOT NULL,
    "user_id" BIGINT NOT NULL,
    "capability_code" VARCHAR(128) NOT NULL,
    "scope_type" VARCHAR(32) NOT NULL,
    "scope_key" VARCHAR(128) NOT NULL DEFAULT '',
    "reason" VARCHAR(500) NOT NULL,
    "grant_version" BIGINT NOT NULL,
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_plugin_staff_access_event_no UNIQUE ("event_no"),
    CONSTRAINT ck_plugin_staff_access_event_action CHECK ("action_code" IN ('grant', 'enable', 'disable')),
    CONSTRAINT ck_plugin_staff_access_event_version CHECK ("grant_version" > 0)
);

ALTER TABLE plugin_staff_access_event
    ADD COLUMN IF NOT EXISTS valid_from TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS valid_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_plugin_staff_access_event_grant
    ON plugin_staff_access_event ("grant_no", occurred_at DESC, "id" DESC);
