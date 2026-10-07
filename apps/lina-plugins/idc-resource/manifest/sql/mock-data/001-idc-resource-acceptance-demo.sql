-- Local acceptance data only. No demo channel contains real credentials.

INSERT INTO plugin_idc_resource_channel
  ("channel_no", "provider_id", "provider_plugin_id", "display_name", "endpoint",
   non_secret_settings, "status_code", "created_by", "updated_by")
VALUES
  ('demo_resource_channel_disabled', 'zjmfcloud-v1', 'provider-zjmfcloud-v1', '演示 · 魔方云（未启用）', 'https://demo.invalid/zjmfcloud', '{"environment":"demo"}'::jsonb, 'disabled', 1, 1),
  ('demo_resource_channel_degraded', 'dcim', 'provider-dcim', '演示 · DCIM（异常）', 'https://demo.invalid/dcim', '{"environment":"demo"}'::jsonb, 'degraded', 1, 1),
  ('demo_resource_channel_maintenance', 'idcsmart-business-v10', 'provider-idcsmart-business-v10', '演示 · 魔方 V10（维护）', 'https://demo.invalid/idcsmart', '{"environment":"demo"}'::jsonb, 'maintenance', 1, 1),
  ('demo_resource_channel_bt', 'bt-panel', 'provider-bt-panel', '演示 · 宝塔（待配置）', 'https://demo.invalid/bt-panel', '{"environment":"demo"}'::jsonb, 'disabled', 1, 1)
ON CONFLICT ("channel_no") DO NOTHING;

INSERT INTO plugin_idc_resource_pool
  ("pool_no", "channel_no", "display_name", "resource_kind", "status_code",
   "routing_priority", "routing_weight", "selector", "created_by", "updated_by")
VALUES
  ('demo_pool_primary', 'demo_resource_channel_degraded', '演示 · 华东主资源池', 'virtual_server', 'enabled', 10, 100, '{"region":"cn-east-demo","network":"bgp-demo"}'::jsonb, 1, 1),
  ('demo_pool_standby', 'demo_resource_channel_maintenance', '演示 · 华南备用资源池', 'virtual_server', 'maintenance', 20, 80, '{"region":"cn-south-demo"}'::jsonb, 1, 1),
  ('demo_pool_draining', 'demo_resource_channel_degraded', '演示 · 迁移中资源池', 'bare_metal', 'draining', 30, 50, '{"region":"cn-north-demo"}'::jsonb, 1, 1),
  ('demo_pool_disabled', 'demo_resource_channel_disabled', '演示 · 未启用资源池', 'virtual_server', 'disabled', 40, 0, '{}'::jsonb, 1, 1)
ON CONFLICT ("pool_no") DO NOTHING;

INSERT INTO plugin_idc_resource_channel_health_state
  ("channel_no", "sample_count", "failure_count", "unknown_count", "consecutive_failures",
   circuit_open, write_protected_until, window_started_at, "last_summary_code",
   last_observed_at, last_authentication_health_code,
   last_authentication_summary_code, last_authentication_latency_ms,
   last_authentication_observed_at, last_query_health_code,
   last_query_summary_code, last_query_latency_ms, last_query_observed_at,
   last_write_health_code, last_write_summary_code, last_write_latency_ms,
   last_write_observed_at, last_provider_capacity_health_code,
   last_provider_capacity_summary_code, last_provider_capacity_latency_ms,
   last_provider_capacity_observed_at)
VALUES
  ('demo_resource_channel_disabled', 0, 0, 0, 0, false, NULL, CURRENT_TIMESTAMP - INTERVAL '1 hour', 'channel_disabled', CURRENT_TIMESTAMP - INTERVAL '10 minutes', 'unknown', 'not_configured', NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes', 'unknown', 'not_configured', NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes', 'unknown', 'not_configured', NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes', 'unknown', 'not_collected', NULL, CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_resource_channel_degraded', 12, 4, 1, 2, false, NULL, CURRENT_TIMESTAMP - INTERVAL '2 hours', 'provider_timeout', CURRENT_TIMESTAMP - INTERVAL '3 minutes', 'healthy', 'authenticated', 86, CURRENT_TIMESTAMP - INTERVAL '20 minutes', 'degraded', 'provider_timeout', 2300, CURRENT_TIMESTAMP - INTERVAL '3 minutes', 'degraded', 'provider_timeout', 3100, CURRENT_TIMESTAMP - INTERVAL '4 minutes', 'healthy', 'capacity_available', 620, CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_resource_channel_maintenance', 8, 5, 2, 4, true, CURRENT_TIMESTAMP + INTERVAL '20 minutes', CURRENT_TIMESTAMP - INTERVAL '1 hour', 'maintenance_window', CURRENT_TIMESTAMP - INTERVAL '1 minute', 'healthy', 'authenticated', 120, CURRENT_TIMESTAMP - INTERVAL '40 minutes', 'unavailable', 'maintenance', NULL, CURRENT_TIMESTAMP - INTERVAL '1 minute', 'unavailable', 'maintenance', NULL, CURRENT_TIMESTAMP - INTERVAL '1 minute', 'degraded', 'stale_capacity', 1800, CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('demo_resource_channel_bt', 5, 3, 2, 3, true, CURRENT_TIMESTAMP + INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '30 minutes', 'configuration_incomplete', CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'unavailable', 'authentication_failed', 410, CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'unknown', 'query_not_attempted', NULL, CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'unknown', 'write_not_attempted', NULL, CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'unknown', 'capacity_not_supported', NULL, CURRENT_TIMESTAMP - INTERVAL '5 minutes')
ON CONFLICT ("tenant_id", "channel_no") DO NOTHING;

INSERT INTO plugin_idc_resource_health_snapshot
  ("snapshot_no", "channel_no", "health_code", "latency_ms", "consecutive_failures",
   circuit_open, "summary_code", observed_at, "operation_kind")
VALUES
  ('demo_health_auth_healthy', 'demo_resource_channel_degraded', 'healthy', 86, 0, false, 'authenticated', CURRENT_TIMESTAMP - INTERVAL '20 minutes', 'authentication'),
  ('demo_health_query_degraded', 'demo_resource_channel_degraded', 'degraded', 2300, 2, false, 'provider_timeout', CURRENT_TIMESTAMP - INTERVAL '3 minutes', 'query'),
  ('demo_health_write_unavailable', 'demo_resource_channel_maintenance', 'unavailable', NULL, 4, true, 'maintenance', CURRENT_TIMESTAMP - INTERVAL '1 minute', 'write'),
  ('demo_health_capacity_unknown', 'demo_resource_channel_bt', 'unknown', NULL, 3, true, 'capacity_not_supported', CURRENT_TIMESTAMP - INTERVAL '5 minutes', 'provider_capacity')
ON CONFLICT ("snapshot_no") DO NOTHING;

INSERT INTO plugin_idc_resource_provider_capacity_observation
  ("observation_no", "channel_no", "capacity_kind", "status_code",
   "observation_fingerprint", "item_count", observed_at, completed_at, page_size,
   next_cursor, page_count, "trigger_code", "requested_by", "attempt_count",
   "failure_code", created_at)
VALUES
  ('demo_capacity_collecting', 'demo_resource_channel_disabled', 'region', 'collecting', '', 0, CURRENT_TIMESTAMP - INTERVAL '2 minutes', NULL, 100, 'next-page-demo', 1, 'manual', 1, 1, '', CURRENT_TIMESTAMP - INTERVAL '2 minutes'),
  ('demo_capacity_complete', 'demo_resource_channel_degraded', 'plan', 'complete', repeat('1',64), 3, CURRENT_TIMESTAMP - INTERVAL '30 minutes', CURRENT_TIMESTAMP - INTERVAL '29 minutes', 100, '', 1, 'system', 0, 1, '', CURRENT_TIMESTAMP - INTERVAL '31 minutes'),
  ('demo_capacity_failed', 'demo_resource_channel_bt', 'stock', 'failed', '', 0, CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '59 minutes', 100, '', 0, 'manual', 1, 3, 'provider_authentication', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
  ('demo_capacity_superseded', 'demo_resource_channel_maintenance', 'image', 'superseded', repeat('2',64), 2, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '23 hours', 100, '', 1, 'system', 0, 1, '', CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT ("observation_no") DO NOTHING;

INSERT INTO plugin_idc_resource_provider_capacity_observation_item
  ("observation_no", "capacity_kind", "external_ref", projection, observed_at)
VALUES
  ('demo_capacity_complete', 'plan', 'demo-plan-standard', '{"name":"标准型","cpu":2,"memoryGiB":4,"available":true}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_capacity_complete', 'plan', 'demo-plan-compute', '{"name":"计算型","cpu":4,"memoryGiB":8,"available":true}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_capacity_complete', 'plan', 'demo-plan-memory', '{"name":"内存型","cpu":4,"memoryGiB":16,"available":false}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_capacity_superseded', 'image', 'demo-image-ubuntu', '{"name":"Ubuntu 24.04 LTS","arch":"x86_64"}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_capacity_superseded', 'image', 'demo-image-rocky', '{"name":"Rocky Linux 9","arch":"x86_64"}'::jsonb, CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT ("observation_no", "capacity_kind", "external_ref") DO NOTHING;

INSERT INTO plugin_idc_resource_task
  ("task_no", "request_id_digest", "capability_code", "local_resource_no", "channel_no",
   "status_code", "priority", sanitized_parameters, "attempt_count", "max_attempts",
   available_at, accepted_at, finished_at, "failure_code", "failure_summary",
   result_unknown, owner_metadata, "observation_attempt_count", created_at, updated_at)
VALUES
  ('demo_resource_task_pending', repeat('1',64), 'server.provision', 'demo_resource_001', 'demo_resource_channel_disabled', 'pending', 10, '{"region":"cn-east-demo","plan":"standard"}'::jsonb, 0, 8, CURRENT_TIMESTAMP, NULL, NULL, '', '', false, '{"customerNo":"CUS_10000000000000000000000000000001"}'::jsonb, 0, CURRENT_TIMESTAMP - INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_resource_task_running', repeat('2',64), 'server.provision', 'demo_resource_002', 'demo_resource_channel_degraded', 'running', 20, '{"region":"cn-south-demo","plan":"compute"}'::jsonb, 2, 8, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '90 minutes', NULL, '', '', false, '{"customerNo":"CUS_10000000000000000000000000000005"}'::jsonb, 2, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '3 minutes'),
  ('demo_resource_task_retry', repeat('3',64), 'server.query', 'demo_resource_003', 'demo_resource_channel_degraded', 'retry_wait', 30, '{"externalReference":"masked-demo"}'::jsonb, 3, 8, CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '1 day', NULL, 'provider_timeout', '演示：上游查询超时，等待重试', false, '{"customerNo":"CUS_10000000000000000000000000000003"}'::jsonb, 2, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '2 minutes'),
  ('demo_resource_task_succeeded', repeat('4',64), 'server.reboot', 'demo_resource_004', 'demo_resource_channel_degraded', 'succeeded', 40, '{"action":"reboot"}'::jsonb, 1, 8, CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '3 days' + INTERVAL '2 minutes', '', '', false, '{"customerNo":"CUS_10000000000000000000000000000001"}'::jsonb, 1, CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '3 days' + INTERVAL '2 minutes'),
  ('demo_resource_task_failed', repeat('5',64), 'server.provision', 'demo_resource_005', 'demo_resource_channel_bt', 'failed', 50, '{"region":"demo"}'::jsonb, 8, 8, CURRENT_TIMESTAMP - INTERVAL '4 days', NULL, CURRENT_TIMESTAMP - INTERVAL '3 days', 'configuration_incomplete', '演示：接口配置不完整', false, '{"customerNo":"CUS_10000000000000000000000000000006"}'::jsonb, 3, CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '3 days'),
  ('demo_resource_task_unknown', repeat('6',64), 'server.power_on', 'demo_resource_006', 'demo_resource_channel_maintenance', 'result_unknown', 60, '{"action":"power_on"}'::jsonb, 3, 8, CURRENT_TIMESTAMP - INTERVAL '6 hours', CURRENT_TIMESTAMP - INTERVAL '6 hours', NULL, 'result_unknown', '演示：请求已发出但结果暂不确定', true, '{"customerNo":"CUS_10000000000000000000000000000005"}'::jsonb, 3, CURRENT_TIMESTAMP - INTERVAL '6 hours', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_resource_task_takeover', repeat('7',64), 'server.resize', 'demo_resource_007', 'demo_resource_channel_maintenance', 'manual_takeover', 70, '{"targetPlan":"compute"}'::jsonb, 4, 8, CURRENT_TIMESTAMP - INTERVAL '12 hours', CURRENT_TIMESTAMP - INTERVAL '12 hours', CURRENT_TIMESTAMP - INTERVAL '1 hour', 'manual_review_required', '演示：需要人工接管', false, '{"customerNo":"CUS_10000000000000000000000000000001"}'::jsonb, 2, CURRENT_TIMESTAMP - INTERVAL '12 hours', CURRENT_TIMESTAMP - INTERVAL '1 hour')
ON CONFLICT ("task_no") DO NOTHING;
