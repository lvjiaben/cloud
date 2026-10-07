-- 011: Durable read-only observation backoff for Provider task reconciliation
--
-- The counter is independent from Provider write attempts. It may only delay
-- QueryTask/QueryResource reads and must never make an external write replayable.

ALTER TABLE plugin_idc_resource_task
    ADD COLUMN IF NOT EXISTS "observation_attempt_count" INT NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_task_observation_attempt'
          AND conrelid = 'plugin_idc_resource_task'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_task
            ADD CONSTRAINT ck_plugin_idc_resource_task_observation_attempt
            CHECK ("observation_attempt_count" >= 0);
    END IF;
END $$;

COMMENT ON COLUMN plugin_idc_resource_task."observation_attempt_count"
    IS 'Consecutive read-only reconciliation attempts; never authorizes Provider write replay';
