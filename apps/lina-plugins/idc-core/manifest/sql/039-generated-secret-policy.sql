-- Extend the authenticated secret request fence for envelope-owned generated
-- credentials. Reapplying the migration converges both constraints and the
-- generated model description without changing existing encrypted payloads.

ALTER TABLE plugin_idc_core_secret_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_secret_request_operation;
ALTER TABLE plugin_idc_core_secret_version
    ADD CONSTRAINT ck_plugin_idc_core_secret_request_operation
    CHECK ("request_operation" IN ('create', 'generate_u256', 'update'));

ALTER TABLE plugin_idc_core_secret_version
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_secret_request_fence;
ALTER TABLE plugin_idc_core_secret_version
    ADD CONSTRAINT ck_plugin_idc_core_secret_request_fence CHECK (
        (
            "request_operation" IN ('create', 'generate_u256') AND "version" = 1 AND
            "request_parent_version" = 0 AND "request_expected_updated_at" IS NULL
        ) OR
        (
            "request_operation" = 'update' AND "request_parent_version" > 0 AND
            "version" = "request_parent_version" + 1 AND
            "request_expected_updated_at" IS NOT NULL
        )
    );

COMMENT ON COLUMN plugin_idc_core_secret_version."request_operation"
    IS 'Authenticated write operation discriminator: create, generate_u256, or update';
