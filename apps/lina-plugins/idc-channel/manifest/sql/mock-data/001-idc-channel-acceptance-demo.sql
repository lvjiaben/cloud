-- Local acceptance data only. Tokens and callbacks are inert demo records.

INSERT INTO plugin_idc_channel_account
  ("channel_id", "customer_no", "display_name", "status_code", new_sales_enabled,
   "approved_by", approved_at, paused_at, "created_by", "updated_by", created_at, updated_at)
VALUES
  ('demo_channel_pending', 'CUS_10000000000000000000000000000002', '演示渠道 · 待审核', 'pending_approval', false, 0, NULL, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_channel_approved', 'CUS_10000000000000000000000000000005', '演示渠道 · 正常销售', 'approved', true, 1, CURRENT_TIMESTAMP - INTERVAL '60 days', NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '61 days', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
  ('demo_channel_paused', 'CUS_10000000000000000000000000000006', '演示渠道 · 已暂停', 'paused', false, 1, CURRENT_TIMESTAMP - INTERVAL '30 days', CURRENT_TIMESTAMP - INTERVAL '2 days', 1, 1, CURRENT_TIMESTAMP - INTERVAL '31 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_channel_closed', 'CUS_10000000000000000000000000000004', '演示渠道 · 已关闭', 'closed', false, 1, CURRENT_TIMESTAMP - INTERVAL '100 days', NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '101 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
ON CONFLICT ("customer_no") DO NOTHING;

INSERT INTO plugin_idc_channel_external_customer
  ("channel_id", "external_customer_id", "customer_no", "invitation_no",
   "status_code", "identity_status_code", created_at, updated_at)
VALUES
  ('demo_channel_approved', 'demo_ext_customer_active', 'CUS_10000000000000000000000000000001', 'demo_invitation_001', 'active', 'verified', CURRENT_TIMESTAMP - INTERVAL '30 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_channel_approved', 'demo_ext_customer_pending', 'CUS_10000000000000000000000000000003', 'demo_invitation_002', 'pending_activation', 'pending', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_channel_paused', 'demo_ext_customer_suspended', 'CUS_10000000000000000000000000000006', 'demo_invitation_003', 'suspended', 'rejected', CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
  ('demo_channel_closed', 'demo_ext_customer_closed', 'CUS_10000000000000000000000000000004', 'demo_invitation_004', 'closed', 'revoked', CURRENT_TIMESTAMP - INTERVAL '90 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
ON CONFLICT ("channel_id", "external_customer_id") DO NOTHING;

INSERT INTO plugin_idc_channel_publication
  ("channel_id", "product_version_id", "currency", "price", "stock", "status",
   "public_slug", "configuration_revision", billing_term_token,
   "purchase_cycles", "quantity", purchase_values, price_minor, created_at)
VALUES
  ('demo_channel_approved', 90001, 'CNY', 129.00, 80, 'published', 'demo-cloud-standard', 'demo-rev-1', 'month-1', 1, 1, '[{"region":"cn-east-demo","plan":"standard"}]'::jsonb, 12900, CURRENT_TIMESTAMP - INTERVAL '20 days'),
  ('demo_channel_approved', 90002, 'CNY', 399.00, 12, 'published', 'demo-cloud-compute', 'demo-rev-2', 'month-1', 1, 1, '[{"region":"cn-south-demo","plan":"compute"}]'::jsonb, 39900, CURRENT_TIMESTAMP - INTERVAL '10 days'),
  ('demo_channel_paused', 90003, 'CNY', 1999.00, 0, 'paused', 'demo-bare-metal', 'demo-rev-3', 'month-1', 1, 1, '[{"region":"cn-north-demo"}]'::jsonb, 199900, CURRENT_TIMESTAMP - INTERVAL '30 days'),
  ('demo_channel_closed', 90004, 'CNY', 59.00, 0, 'offline', 'demo-classic-host', 'demo-rev-4', 'month-1', 1, 1, '[]'::jsonb, 5900, CURRENT_TIMESTAMP - INTERVAL '100 days')
ON CONFLICT ("channel_id", "product_version_id", "currency") DO NOTHING;

UPDATE plugin_idc_channel_order
SET core_order_no = 'demo_channel_order_pending'
WHERE "channel_id" = 'demo_channel_approved'
  AND "external_customer_id" = 'demo_ext_customer_pending'
  AND core_order_no IS NULL
  AND "status" = 'pending_identity';

INSERT INTO plugin_idc_channel_order
  ("channel_id", "external_customer_id", "ownership_mode", "product_version_id",
   "currency", "amount", "status", financially_settled, core_order_no,
   core_invoice_no, "payer_customer_no", "service_owner_customer_no",
   "total_amount_minor", identity_ready, created_at, updated_at)
SELECT values_row.*
FROM (
  VALUES
    ('demo_channel_approved'::varchar, 'demo_ext_customer_pending'::varchar, 'customer_owned'::varchar, 90001::bigint, 'CNY'::varchar, 129.00::numeric, 'pending_identity'::varchar, false, 'demo_channel_order_pending'::varchar, NULL::varchar, 'CUS_10000000000000000000000000000003'::varchar, 'CUS_10000000000000000000000000000003'::varchar, 12900::bigint, false, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
    ('demo_channel_approved', 'demo_ext_customer_active', 'channel_owned', 90001, 'CNY', 129.00, 'pending_payment', false, 'ORD_20000000000000000000000000000001', 'demo_invoice_pending', 'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000001', 12900, true, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
    ('demo_channel_approved', 'demo_ext_customer_active', 'channel_owned', 90002, 'CNY', 399.00, 'fulfilling', true, 'ORD_20000000000000000000000000000004', 'demo_invoice_bad_debt', 'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000001', 39900, true, CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
    ('demo_channel_paused', 'demo_ext_customer_suspended', 'customer_owned', 90003, 'CNY', 1999.00, 'manual_review', false, 'ORD_20000000000000000000000000000007', NULL, 'CUS_10000000000000000000000000000006', 'CUS_10000000000000000000000000000006', 199900, true, CURRENT_TIMESTAMP - INTERVAL '7 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
    ('demo_channel_closed', 'demo_ext_customer_closed', 'customer_owned', 90004, 'CNY', 59.00, 'cancelled', false, 'ORD_20000000000000000000000000000006', 'demo_invoice_cancelled', 'CUS_10000000000000000000000000000004', 'CUS_10000000000000000000000000000004', 5900, true, CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
) values_row("channel_id", "external_customer_id", "ownership_mode", "product_version_id",
             "currency", "amount", "status", financially_settled, core_order_no,
             core_invoice_no, "payer_customer_no", "service_owner_customer_no",
             "total_amount_minor", identity_ready, created_at, updated_at)
WHERE NOT EXISTS (
  SELECT 1
  FROM plugin_idc_channel_order existing
  WHERE existing."channel_id" = values_row."channel_id"
    AND existing.core_order_no = values_row.core_order_no
);

INSERT INTO plugin_idc_channel_token
  ("channel_id", "token_hash", scopes, source_cidrs, "rate_limit",
   expires_at, revoked_at, "created_by", created_at)
VALUES
  ('demo_channel_approved', repeat('1',64), '["catalog:read","order:write"]'::jsonb, '["192.0.2.0/24"]'::jsonb, 120, CURRENT_TIMESTAMP + INTERVAL '30 days', NULL, 1, CURRENT_TIMESTAMP - INTERVAL '30 days'),
  ('demo_channel_paused', repeat('2',64), '["catalog:read"]'::jsonb, '[]'::jsonb, 60, CURRENT_TIMESTAMP + INTERVAL '7 days', NULL, 1, CURRENT_TIMESTAMP - INTERVAL '20 days'),
  ('demo_channel_closed', repeat('3',64), '["catalog:read"]'::jsonb, '[]'::jsonb, 30, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '10 days', 1, CURRENT_TIMESTAMP - INTERVAL '100 days')
ON CONFLICT ("channel_id", "token_hash") DO NOTHING;

INSERT INTO plugin_idc_channel_request
  ("channel_id", "request_id", "operation", "request_hash", response,
   "status", "token_id", "result_code", completed_at, created_at)
SELECT values_row."channel_id", values_row."request_id", values_row."operation",
       values_row."request_hash", values_row.response, values_row."status",
       token."id", values_row."result_code", values_row.completed_at,
       values_row.created_at
FROM (
  VALUES
    ('demo_channel_approved'::varchar, 'demo_request_accepted'::varchar, 'catalog.list'::varchar, repeat('4',64)::varchar, '{"count":4}'::jsonb, 'accepted'::varchar, 'accepted'::varchar, NULL::timestamptz, CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
    ('demo_channel_approved', 'demo_request_completed', 'order.create', repeat('5',64), '{"orderId":"demo-channel-order"}'::jsonb, 'completed', 'succeeded', CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '61 minutes'),
    ('demo_channel_paused', 'demo_request_rejected', 'order.create', repeat('6',64), '{"error":"channel_paused"}'::jsonb, 'rejected', 'channel_paused', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
    ('demo_channel_closed', 'demo_request_failed', 'catalog.list', repeat('7',64), '{"error":"channel_closed"}'::jsonb, 'failed', 'channel_closed', CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
) AS values_row("channel_id", "request_id", "operation", "request_hash", response, "status", "result_code", completed_at, created_at)
LEFT JOIN plugin_idc_channel_token token
  ON token."channel_id" = values_row."channel_id" AND token.revoked_at IS NULL
ON CONFLICT ("channel_id", "request_id") DO NOTHING;

INSERT INTO plugin_idc_channel_webhook
  ("channel_id", "event_id", "event_version", "event_type", "subject_ref",
   "sequence_no", payload, "payload_text", "payload_hash", "signature",
   "status", "attempts", next_attempt_at, "lease_token", lease_expires_at,
   delivered_at, "last_error", updated_at, created_at)
VALUES
  ('demo_channel_approved', 'demo_webhook_pending', 'v1', 'order.updated', 'ORD_20000000000000000000000000000001', 1, '{"status":"pending_payment"}'::jsonb, '{"status":"pending_payment"}', repeat('8',64), 'demo-signature', 'pending', 0, CURRENT_TIMESTAMP, '', NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_channel_approved', 'demo_webhook_delivered', 'v1', 'order.updated', 'ORD_20000000000000000000000000000004', 2, '{"status":"fulfilling"}'::jsonb, '{"status":"fulfilling"}', repeat('9',64), 'demo-signature', 'delivered', 1, NULL, '', NULL, CURRENT_TIMESTAMP - INTERVAL '20 minutes', '', CURRENT_TIMESTAMP - INTERVAL '20 minutes', CURRENT_TIMESTAMP - INTERVAL '21 minutes'),
  ('demo_channel_paused', 'demo_webhook_retry', 'v1', 'channel.updated', 'demo_channel_paused', 3, '{"status":"paused"}'::jsonb, '{"status":"paused"}', repeat('a',64), 'demo-signature', 'retry_wait', 3, CURRENT_TIMESTAMP + INTERVAL '10 minutes', '', NULL, NULL, 'demo endpoint unavailable', CURRENT_TIMESTAMP - INTERVAL '1 minute', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_channel_closed', 'demo_webhook_failed', 'v1', 'channel.updated', 'demo_channel_closed', 4, '{"status":"closed"}'::jsonb, '{"status":"closed"}', repeat('b',64), 'demo-signature', 'failed', 8, NULL, '', NULL, NULL, 'channel closed', CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
ON CONFLICT ("channel_id", "event_id") DO NOTHING;
