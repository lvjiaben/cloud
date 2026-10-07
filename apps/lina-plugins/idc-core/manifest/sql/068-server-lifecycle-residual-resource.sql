ALTER TABLE plugin_idc_core_service_lifecycle_task
    ADD COLUMN IF NOT EXISTS "residual_resources" JSONB NOT NULL DEFAULT '[]'::JSONB;

ALTER TABLE plugin_idc_core_service_lifecycle_task
    DROP CONSTRAINT IF EXISTS ck_idc_service_lifecycle_task_residual_resources;
ALTER TABLE plugin_idc_core_service_lifecycle_task
    ADD CONSTRAINT ck_idc_service_lifecycle_task_residual_resources CHECK (
        jsonb_typeof("residual_resources") = 'array'
        AND jsonb_array_length("residual_resources") <= 16
        AND (
            jsonb_array_length("residual_resources") = 0
            OR ("action_code" = 'terminate' AND "status_code" = 'manual_review')
        )
    );

COMMENT ON COLUMN plugin_idc_core_service_lifecycle_task."residual_resources" IS
    'At most 16 bounded non-secret resource kind, opaque number, safe status and stable reason projections that still require reclamation';
