-- 003: Durable provider_capacity page continuation and bounded publication
--
-- Collecting observations persist exact continuation state so process restart,
-- duplicate delivery, and page skipping cannot publish a partial provider_capacity.

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "page_size" INT NOT NULL DEFAULT 0;

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "next_cursor" VARCHAR(1024) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_resource_provider_capacity_observation
    ADD COLUMN IF NOT EXISTS "page_count" INT NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_resource_provider_capacity_collecting
    ON plugin_idc_resource_provider_capacity_observation (
        "tenant_id", "channel_no", "capacity_kind"
    ) WHERE "status_code" = 'collecting';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_page_size'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_page_size
            CHECK ("page_size" = 0 OR "page_size" BETWEEN 1 AND 500);
    END IF;
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'ck_plugin_idc_resource_provider_capacity_page_count'
          AND conrelid = 'plugin_idc_resource_provider_capacity_observation'::regclass
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            ADD CONSTRAINT ck_plugin_idc_resource_provider_capacity_page_count
            CHECK ("page_count" BETWEEN 0 AND 200);
    END IF;
END $$;

COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."page_size"
    IS 'Fixed Provider page size for the exact collecting observation';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."next_cursor"
    IS 'Exact opaque Provider cursor required for the next staged page';
COMMENT ON COLUMN plugin_idc_resource_provider_capacity_observation."page_count"
    IS 'Number of validated pages durably staged into this observation';
