-- 004: Exact database-clock lower bound for resource observation leases
--
-- This business timestamp is distinct from GoFrame's automatically maintained
-- updated_at value. It is written once per resource-observation claim and
-- returned unchanged to the owner worker as the snapshot freshness boundary.

ALTER TABLE plugin_idc_resource_task
    ADD COLUMN IF NOT EXISTS "observation_claimed_at" TIMESTAMPTZ;

COMMENT ON COLUMN plugin_idc_resource_task."observation_claimed_at"
    IS 'PostgreSQL time of the latest QueryResource observation claim; never derived from updated_at';

