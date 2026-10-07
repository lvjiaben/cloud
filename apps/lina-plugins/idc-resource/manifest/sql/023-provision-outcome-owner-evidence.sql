-- 023: Freeze the structured owner evidence needed by the entitlement sink.
--
-- The one-time backfill reads the old normalized task payload only while
-- upgrading a development database. Production ownership lookups use these
-- formal columns and never scan JSON.

ALTER TABLE plugin_idc_resource_provision_outcome_receipt
    ADD COLUMN IF NOT EXISTS "operation_no" VARCHAR(40) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "customer_no" VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS "configuration_snapshot_no" VARCHAR(64) NOT NULL DEFAULT '';

UPDATE plugin_idc_resource_provision_outcome_receipt receipt
SET "operation_no" = COALESCE(task.owner_metadata ->> 'operationNo', ''),
    "customer_no" = COALESCE(task.owner_metadata ->> 'customerNo', ''),
    "configuration_snapshot_no" = COALESCE(
        task.owner_metadata ->> 'configurationSnapshotNo',
        ''
    )
FROM plugin_idc_resource_task task
WHERE task."tenant_id" = receipt."tenant_id"
  AND task."task_no" = receipt."task_no"
  AND (
      receipt."operation_no" = '' OR receipt."customer_no" = '' OR
      receipt."configuration_snapshot_no" = ''
  );

ALTER TABLE plugin_idc_resource_provision_outcome_receipt
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_resource_provision_outcome_owner;
ALTER TABLE plugin_idc_resource_provision_outcome_receipt
    ADD CONSTRAINT ck_plugin_idc_resource_provision_outcome_owner CHECK (
        "operation_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,39}$' AND
        "customer_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$' AND
        "configuration_snapshot_no" ~ '^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$'
    );

CREATE UNIQUE INDEX IF NOT EXISTS
    uk_plugin_idc_resource_provision_outcome_owner_reference
    ON plugin_idc_resource_provision_outcome_receipt (
        "tenant_id", "service_no", "operation_no", "customer_no",
        "configuration_snapshot_no"
    );

COMMENT ON COLUMN plugin_idc_resource_provision_outcome_receipt."customer_no"
    IS 'Frozen opaque customer owner copied from the validated provision request';
COMMENT ON COLUMN plugin_idc_resource_provision_outcome_receipt."operation_no"
    IS 'Frozen opaque fulfillment operation copied from the validated provision request';
COMMENT ON COLUMN plugin_idc_resource_provision_outcome_receipt."configuration_snapshot_no"
    IS 'Frozen configuration snapshot copied from the validated provision request';
