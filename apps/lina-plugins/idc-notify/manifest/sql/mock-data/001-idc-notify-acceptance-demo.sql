-- Local acceptance data only. No external delivery is performed by these rows.

INSERT INTO plugin_idc_notify_channel
  ("channel_code", "channel_kind", "provider_code", "display_name", "status_code",
   "balance_state", "configuration_generation", non_secret_configuration)
VALUES
  ('demo_email_disabled', 'email', '', '演示 · 邮件（已停用）', 'disabled', 'sufficient', 1, '{"environment":"demo"}'::jsonb),
  ('demo_sms_configuring', 'sms', 'aliyun_sms_v20170525', '演示 · 短信（配置中）', 'configuring', 'unknown', 1, '{"environment":"demo"}'::jsonb),
  ('demo_webhook_failed', 'signed_webhook', '', '演示 · Webhook（配置失败）', 'configuration_failed', 'sufficient', 1, '{"environment":"demo"}'::jsonb)
ON CONFLICT ("tenant_id", "channel_code") DO NOTHING;

INSERT INTO plugin_idc_notify_template_version
  ("template_no", "version", "event_code", "channel_code", "locale_code",
   "subject_template", "body_template", "provider_template_id",
   provider_sms_variable_order, provider_wechat_data_mapping,
   "status_code", "content_digest", "created_by", "provider_code",
   provider_sms_data_mapping, created_at)
VALUES
  ('demo_tpl_payment_email', 1, 'billing.payment_result', 'demo_email_disabled', 'zh-CN', '账单支付结果', '账单 {{invoice_no}} 当前状态：{{status}}。', '', '[]'::jsonb, '{}'::jsonb, 'active', repeat('1',64), 1, '', '{}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '30 days'),
  ('demo_tpl_payment_sms', 1, 'billing.payment_result', 'demo_sms_configuring', 'zh-CN', '', '账单${invoice_no}状态${status}', 'DEMO_SMS_001', '["invoice_no","status"]'::jsonb, '{}'::jsonb, 'draft', repeat('2',64), 1, 'aliyun_sms_v20170525', '{"invoice_no":"invoice_no","status":"status"}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_tpl_webhook', 1, 'billing.payment_result', 'demo_webhook_failed', 'en-US', '', '{"invoice":"{{invoice_no}}","status":"{{status}}"}', '', '[]'::jsonb, '{}'::jsonb, 'disabled', repeat('3',64), 1, '', '{}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '10 days')
ON CONFLICT ("tenant_id", "event_code", "channel_code", "locale_code", "version") DO NOTHING;

INSERT INTO plugin_idc_notify_delivery_chain
  ("chain_no", "root_task_no", "tail_task_no", "manual_resend_count",
   last_manual_resend_at, created_at, updated_at)
VALUES
  ('demo_chain_queued', 'demo_notify_queued', 'demo_notify_queued', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_chain_sending', 'demo_notify_sending', 'demo_notify_sending', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '1 minute'),
  ('demo_chain_retry', 'demo_notify_retry', 'demo_notify_retry', 1, CURRENT_TIMESTAMP - INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_chain_delivered', 'demo_notify_delivered', 'demo_notify_delivered', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_chain_failed', 'demo_notify_failed', 'demo_notify_failed', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_chain_unknown', 'demo_notify_unknown', 'demo_notify_unknown', 2, CURRENT_TIMESTAMP - INTERVAL '20 minutes', CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
  ('demo_chain_blocked', 'demo_notify_blocked', 'demo_notify_blocked', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '4 days')
ON CONFLICT ("tenant_id", "root_task_no") DO NOTHING;

INSERT INTO plugin_idc_notify_delivery_task
  ("task_no", "chain_no", "event_code", "topic_code", "channel_code", "locale_code",
   "owner_user_id", "customer_no", "contact_no", "recipient_mask",
   "business_object_type", "business_object_no", "business_object_version",
   "subject_content", "body_content", provider_variables, "content_digest",
   "status_code", "result_class", external_boundary_crossed,
   "provider_message_ref", provider_query_deadline,
   "provider_query_deadline_source", "retry_sequence", next_attempt_at,
   queued_at, completed_at, updated_at)
VALUES
  ('demo_notify_queued', 'demo_chain_queued', 'billing.payment_result', 'billing', 'in_app', 'zh-CN', 1, 'CUS_10000000000000000000000000000001', 'demo_contact_001', '用户中心消息', 'invoice', 'demo_invoice_pending', 1, '待付款提醒', '您的演示账单等待支付。', '{}'::jsonb, repeat('1',64), 'queued', 'none', false, NULL, NULL, '', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP - INTERVAL '5 minutes', NULL, CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_notify_sending', 'demo_chain_sending', 'billing.payment_result', 'billing', 'demo_email_disabled', 'zh-CN', 1, 'CUS_10000000000000000000000000000005', 'demo_contact_005', 'o***@yunsan-demo.example.com', 'invoice', 'demo_invoice_paid', 1, '支付结果通知', '演示邮件正在处理中。', '{}'::jsonb, repeat('2',64), 'sending', 'none', false, NULL, NULL, '', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes', NULL, CURRENT_TIMESTAMP - INTERVAL '1 minute'),
  ('demo_notify_retry', 'demo_chain_retry', 'billing.payment_result', 'billing', 'demo_sms_configuring', 'zh-CN', 1, 'CUS_10000000000000000000000000000006', 'demo_contact_006', '138****8006', 'invoice', 'demo_invoice_bad_debt', 1, '', '演示短信等待重试。', '{"invoice_no":"demo_invoice_bad_debt","status":"bad_debt"}'::jsonb, repeat('3',64), 'retry_wait', 'definitive_not_received_temporary', true, 'demo-provider-message-003', CURRENT_TIMESTAMP + INTERVAL '20 minutes', 'provider_policy', 1, CURRENT_TIMESTAMP + INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '2 hours', NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_notify_delivered', 'demo_chain_delivered', 'billing.payment_result', 'billing', 'in_app', 'zh-CN', 1, 'CUS_10000000000000000000000000000001', 'demo_contact_001', '用户中心消息', 'invoice', 'demo_invoice_overdue', 1, '逾期提醒', '演示站内信已送达。', '{}'::jsonb, repeat('4',64), 'delivered', 'delivered', false, 'demo-in-app-message-004', NULL, '', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day' + INTERVAL '1 minute', CURRENT_TIMESTAMP - INTERVAL '1 day' + INTERVAL '1 minute'),
  ('demo_notify_failed', 'demo_chain_failed', 'billing.payment_result', 'billing', 'demo_webhook_failed', 'en-US', 1, 'CUS_10000000000000000000000000000003', 'demo_contact_003', 'https://demo.invalid/***', 'invoice', 'demo_invoice_overdue', 1, '', 'Demo webhook delivery failed.', '{}'::jsonb, repeat('5',64), 'failed', 'configuration_error', false, NULL, NULL, '', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days' + INTERVAL '1 minute', CURRENT_TIMESTAMP - INTERVAL '2 days' + INTERVAL '1 minute'),
  ('demo_notify_unknown', 'demo_chain_unknown', 'billing.payment_result', 'billing', 'demo_sms_configuring', 'zh-CN', 1, 'CUS_10000000000000000000000000000003', 'demo_contact_003', '138****8003', 'invoice', 'demo_invoice_overdue', 1, '', '演示短信结果暂不确定。', '{}'::jsonb, repeat('6',64), 'result_unknown', 'result_unknown', true, 'demo-provider-message-006', CURRENT_TIMESTAMP + INTERVAL '30 minutes', 'provider_policy', 2, CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '3 hours', NULL, CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
  ('demo_notify_blocked', 'demo_chain_blocked', 'billing.payment_result', 'billing', 'demo_email_disabled', 'zh-CN', 1, 'CUS_10000000000000000000000000000004', 'demo_contact_004', 'w***@example.com', 'invoice', 'demo_invoice_cancelled', 1, '通知未发送', '演示：客户偏好阻止了本次通知。', '{}'::jsonb, repeat('7',64), 'preference_blocked', 'unsubscribed', false, NULL, NULL, '', 0, NULL, CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '4 days')
ON CONFLICT ("tenant_id", "task_no") DO NOTHING;
