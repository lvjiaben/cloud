-- External customer notifications now use email and SMS only.
-- Keep historical delivery evidence and the internal employee renewal inbox.
DELETE FROM plugin_idc_notify_event_policy
WHERE channel_code NOT IN ('email', 'sms')
  AND NOT (channel_code = 'in_app' AND recipient_kind = 'staff_user'
           AND event_code = 'billing.accessory_renewal_attention');

UPDATE plugin_idc_notify_template_version
SET status_code = 'disabled'
WHERE channel_code NOT IN ('email', 'sms') AND status_code <> 'disabled'
  AND NOT (channel_code = 'in_app' AND event_code = 'billing.accessory_renewal_attention');

UPDATE plugin_idc_notify_channel
SET status_code = 'disabled', non_secret_configuration = '{}'::jsonb
WHERE channel_code NOT IN ('in_app', 'email', 'sms')
  AND (status_code <> 'disabled' OR non_secret_configuration <> '{}'::jsonb);

UPDATE plugin_idc_notify_channel_intent
SET status_code = 'failed', last_error_code = 'channel_removed',
    lease_token = NULL, lease_expires_at = NULL
WHERE channel_code NOT IN ('email', 'sms')
  AND status_code IN ('prepared', 'secret_applied');

-- Never claim delivery success or cancellation after a provider was called.
WITH retired AS (
    SELECT tenant_id, task_no, status_code, external_boundary_crossed
    FROM plugin_idc_notify_delivery_task
    WHERE channel_code NOT IN ('email', 'sms')
      AND NOT (channel_code = 'in_app' AND event_code = 'billing.accessory_renewal_attention'
               AND owner_user_id IS NOT NULL)
      AND status_code IN ('queued', 'retry_wait', 'sending', 'submitted', 'result_unknown')
    FOR UPDATE
), transitioned AS (
    UPDATE plugin_idc_notify_delivery_task task
    SET status_code = CASE WHEN retired.external_boundary_crossed THEN 'manual_review' ELSE 'cancelled' END,
        attempt_lease = NULL, attempt_lease_until = NULL, next_attempt_at = NULL,
        completed_at = CASE WHEN retired.external_boundary_crossed THEN task.completed_at ELSE CURRENT_TIMESTAMP END,
        updated_at = CURRENT_TIMESTAMP, version = task.version + 1
    FROM retired
    WHERE task.tenant_id = retired.tenant_id AND task.task_no = retired.task_no
    RETURNING task.tenant_id, task.task_no, retired.status_code AS previous_status,
              task.status_code AS current_status, task.version
)
INSERT INTO plugin_idc_notify_delivery_audit
    (tenant_id, audit_no, task_no, action_code, actor_type, reason_code,
     previous_status, current_status, task_version)
SELECT tenant_id, 'nda_email_sms_' || md5(task_no), task_no, 'delivery_result', 'system',
       'channel_removed', previous_status, current_status, version
FROM transitioned
ON CONFLICT DO NOTHING;
