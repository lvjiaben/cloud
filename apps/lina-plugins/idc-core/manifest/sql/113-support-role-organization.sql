-- Role-based queues bind explicitly to organization departments. Existing queues
-- remain unbound until an administrator selects the real department; no name guessing.
ALTER TABLE plugin_idc_core_support_department ADD COLUMN IF NOT EXISTS org_dept_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_support_department_org ON plugin_idc_core_support_department (tenant_id, org_dept_id, status_code);

-- Queue escalation preserves old personal history while allowing no target user.
ALTER TABLE plugin_idc_core_support_ticket_escalation ALTER COLUMN to_user_id DROP NOT NULL;
ALTER TABLE plugin_idc_core_support_ticket_escalation DROP CONSTRAINT IF EXISTS ck_idc_support_ticket_escalation_users;
ALTER TABLE plugin_idc_core_support_ticket_escalation ADD CONSTRAINT ck_idc_support_ticket_escalation_users CHECK (from_user_id > 0 AND (to_user_id IS NULL OR (to_user_id > 0 AND from_user_id <> to_user_id)));
