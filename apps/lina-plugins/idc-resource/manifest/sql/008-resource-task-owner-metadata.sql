-- Separate resource-owner audit metadata from parameters sent to Providers.

ALTER TABLE plugin_idc_resource_task
    ADD COLUMN IF NOT EXISTS "owner_metadata" JSONB NOT NULL DEFAULT '{}'::JSONB;

DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_task_owner_metadata'
          AND conrelid = 'plugin_idc_resource_task'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_task
            ADD CONSTRAINT ck_plugin_idc_resource_task_owner_metadata
            CHECK (jsonb_typeof("owner_metadata") = 'object');
    END IF;
END;
$constraint$;

COMMENT ON COLUMN plugin_idc_resource_task."owner_metadata"
    IS 'Validated non-secret owner intent retained for replay checks and never sent to the Provider';
