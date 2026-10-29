-- 037: Retain failed provisioning receipts across staff-authorized successors.
-- One successful receipt remains authoritative for each frozen service owner.
DROP INDEX IF EXISTS uk_plugin_idc_resource_provision_outcome_owner_reference;
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_resource_successful_provision_owner
 ON plugin_idc_resource_provision_outcome_receipt (
  tenant_id,service_no,operation_no,customer_no,configuration_snapshot_no
 ) WHERE status_code='succeeded';
CREATE INDEX IF NOT EXISTS idx_idc_resource_provision_owner_history
 ON plugin_idc_resource_provision_outcome_receipt (
  tenant_id,service_no,operation_no,customer_no,configuration_snapshot_no,created_at DESC,id DESC
 );
