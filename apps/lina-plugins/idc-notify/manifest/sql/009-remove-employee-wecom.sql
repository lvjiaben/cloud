-- Retire employee WeCom delivery and remove the deleted self-service binding data.
-- Historical channel, template, and delivery rows remain as foreign-keyed evidence.
DELETE FROM plugin_idc_notify_event_policy WHERE channel_code = 'wechat_work';

UPDATE plugin_idc_notify_channel
SET status_code = 'disabled'
WHERE channel_code = 'wechat_work' AND status_code <> 'disabled';

UPDATE plugin_idc_notify_template_version
SET status_code = 'disabled'
WHERE channel_code = 'wechat_work' AND status_code <> 'disabled';

UPDATE plugin_idc_notify_channel_intent
SET status_code = 'failed', last_error_code = 'channel_removed',
    lease_token = NULL, lease_expires_at = NULL
WHERE channel_code = 'wechat_work' AND status_code IN ('prepared', 'secret_applied');

UPDATE plugin_idc_notify_delivery_task
SET status_code = 'cancelled', attempt_lease = NULL, attempt_lease_until = NULL
WHERE channel_code = 'wechat_work' AND external_boundary_crossed = FALSE
  AND status_code IN ('queued', 'retry_wait', 'sending');

-- The deleted adapter cannot query old provider results. Fence already-crossed
-- work out of the send worker without declaring it cancelled, failed, or sent.
WITH retired AS (
    SELECT tenant_id, task_no, status_code
    FROM plugin_idc_notify_delivery_task
    WHERE channel_code = 'wechat_work' AND external_boundary_crossed = TRUE
      AND status_code IN ('queued', 'retry_wait', 'sending', 'submitted', 'result_unknown')
    FOR UPDATE
), transitioned AS (
    UPDATE plugin_idc_notify_delivery_task task
    SET status_code = 'manual_review', attempt_lease = NULL, attempt_lease_until = NULL,
        next_attempt_at = NULL, updated_at = CURRENT_TIMESTAMP, version = task.version + 1
    FROM retired
    WHERE task.tenant_id = retired.tenant_id AND task.task_no = retired.task_no
    RETURNING task.tenant_id, task.task_no, retired.status_code AS previous_status, task.version
)
INSERT INTO plugin_idc_notify_delivery_audit
    (tenant_id, audit_no, task_no, action_code, actor_type, reason_code,
     previous_status, current_status, task_version)
SELECT tenant_id, 'nda_retired_wecom_' || md5(task_no), task_no, 'query_manual_review', 'system',
       'channel_removed', previous_status, 'manual_review', version
FROM transitioned
ON CONFLICT DO NOTHING;

-- Keep immutable operation evidence, detaching only its retired identity mapping.
-- Other constraints and foreign keys on the audit table remain unchanged.
DO $$
DECLARE
    binding_reference RECORD;
BEGIN
    FOR binding_reference IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = to_regclass('plugin_idc_notify_wecom_binding_audit')
          AND confrelid = to_regclass('plugin_idc_notify_wecom_employee_binding')
          AND contype = 'f'
    LOOP
        EXECUTE format('ALTER TABLE plugin_idc_notify_wecom_binding_audit DROP CONSTRAINT %I', binding_reference.conname);
    END LOOP;
END $$;

DROP TABLE IF EXISTS plugin_idc_notify_wecom_employee_binding;
DROP TABLE IF EXISTS plugin_idc_notify_wecom_oauth_intent;
