-- Local acceptance data only. These channels point to .invalid and stay non-enabled.

INSERT INTO plugin_idc_payment_channel
  ("channel_no", "provider_code", "display_name", "endpoint_origin", "merchant_reference",
   "secret_ref", supported_modes, supported_currencies, capability_snapshot,
   "status_code", "created_by", "updated_by")
VALUES
  ('demo_pay_alipay', 'alipay', '演示 · 支付宝（未启用）', 'https://demo.invalid/alipay', 'demo-merchant-alipay', 'demo-secret-alipay', '["web","qr"]'::jsonb, '["CNY"]'::jsonb, '{"refund":true,"query":true}'::jsonb, 'disabled', 1, 1),
  ('demo_pay_wechat', 'wechatpay', '演示 · 微信支付（验证中）', 'https://demo.invalid/wechat', 'demo-merchant-wechat', 'demo-secret-wechat', '["jsapi","native"]'::jsonb, '["CNY"]'::jsonb, '{"refund":true,"query":true}'::jsonb, 'verifying', 1, 1),
  ('demo_pay_epay', 'epay', '演示 · 易支付（异常）', 'https://demo.invalid/epay', 'demo-merchant-epay', 'demo-secret-epay', '["redirect"]'::jsonb, '["CNY"]'::jsonb, '{"refund":false,"query":true}'::jsonb, 'degraded', 1, 1),
  ('demo_pay_unionpay', 'unionpay', '演示 · 银联支付（维护）', 'https://demo.invalid/unionpay', 'demo-merchant-unionpay', 'demo-secret-unionpay', '["web"]'::jsonb, '["CNY"]'::jsonb, '{"refund":true,"query":true}'::jsonb, 'maintenance', 1, 1)
ON CONFLICT ("channel_no") DO NOTHING;

INSERT INTO plugin_idc_payment_attempt
  ("attempt_no", "channel_no", "invoice_no", "order_no", "purpose_code",
   "purpose_reference", "customer_ref_digest", "currency_code", "amount_minor",
   "balance_lock_reference", "balance_locked_minor", "mode_code", "request_digest",
   "external_trade_reference", "status_code", expires_at, submitted_at,
   confirmed_at, closed_at, result_unknown, "funding_result_digest",
   created_at, updated_at)
VALUES
  ('demo_pay_attempt_created', 'demo_pay_alipay', '', '', 'balance_recharge', 'demo_recharge_001', repeat('1',64), 'CNY', 10000, '', 0, 'web', repeat('2',64), '', 'created', CURRENT_TIMESTAMP + INTERVAL '20 minutes', NULL, NULL, NULL, false, repeat('3',64), CURRENT_TIMESTAMP - INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_pay_attempt_confirming', 'demo_pay_wechat', '', '', 'balance_recharge', 'demo_recharge_002', repeat('4',64), 'CNY', 50000, '', 0, 'native', repeat('5',64), 'demo-trade-masked-002', 'confirming', CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '8 minutes', NULL, NULL, true, repeat('6',64), CURRENT_TIMESTAMP - INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '1 minute'),
  ('demo_pay_attempt_failed', 'demo_pay_epay', '', '', 'balance_recharge', 'demo_recharge_003', repeat('7',64), 'CNY', 8800, '', 0, 'redirect', repeat('8',64), 'demo-trade-masked-003', 'failed', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '2 days', NULL, CURRENT_TIMESTAMP - INTERVAL '1 day', false, repeat('9',64), CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_pay_attempt_expired', 'demo_pay_unionpay', '', '', 'balance_recharge', 'demo_recharge_004', repeat('a',64), 'CNY', 32000, '', 0, 'web', repeat('b',64), '', 'expired', CURRENT_TIMESTAMP - INTERVAL '3 days', NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '3 days', false, repeat('c',64), CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '3 days'),
  ('demo_pay_attempt_review', 'demo_pay_wechat', '', '', 'balance_recharge', 'demo_recharge_005', repeat('d',64), 'CNY', 16800, '', 0, 'jsapi', repeat('e',64), 'demo-trade-masked-005', 'manual_review', CURRENT_TIMESTAMP + INTERVAL '5 minutes', CURRENT_TIMESTAMP - INTERVAL '20 minutes', NULL, NULL, true, repeat('f',64), CURRENT_TIMESTAMP - INTERVAL '30 minutes', CURRENT_TIMESTAMP - INTERVAL '2 minutes')
ON CONFLICT ("attempt_no") DO NOTHING;

INSERT INTO plugin_idc_payment_reconciliation_batch
  ("batch_no", "channel_no", business_date, "source_digest", "status_code",
   "total_count", "difference_count", daily_scope_key, merchant_timezone,
   "source_type_code", "capability_code", payment_count, refund_count,
   result_unknown, "failure_code", created_at, updated_at)
VALUES
  ('demo_recon_pending', 'demo_pay_alipay', CURRENT_DATE, repeat('1',64), 'pending', 0, 0, 'demo-alipay-today', 'Asia/Shanghai', 'provider_statement', 'manual_upload', 0, 0, false, '', CURRENT_TIMESTAMP - INTERVAL '10 minutes', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_recon_processing', 'demo_pay_wechat', CURRENT_DATE - 1, repeat('2',64), 'processing', 24, 0, 'demo-wechat-yesterday', 'Asia/Shanghai', 'provider_statement', 'automatic_pull', 22, 2, false, '', CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '5 minutes'),
  ('demo_recon_matched', 'demo_pay_alipay', CURRENT_DATE - 2, repeat('3',64), 'matched', 36, 0, 'demo-alipay-two-days', 'Asia/Shanghai', 'provider_statement', 'manual_upload', 32, 4, false, '', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days' + INTERVAL '5 minutes'),
  ('demo_recon_differences', 'demo_pay_epay', CURRENT_DATE - 3, repeat('4',64), 'differences', 18, 3, 'demo-epay-three-days', 'Asia/Shanghai', 'provider_statement', 'automatic_pull', 16, 2, false, '', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_recon_failed', 'demo_pay_unionpay', CURRENT_DATE - 4, repeat('5',64), 'failed', 0, 0, 'demo-unionpay-four-days', 'Asia/Shanghai', 'provider_statement', 'unavailable', 0, 0, false, 'channel_maintenance', CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '4 days')
ON CONFLICT ("batch_no") DO NOTHING;

INSERT INTO plugin_idc_payment_reconciliation_line
  ("line_no", "batch_no", "attempt_no", "external_reference", "difference_code",
   "local_amount_minor", "channel_amount_minor", "currency_code", "resolution_code",
   "evidence_digest", "record_type_code", "customer_ref_digest",
   local_status_code, channel_status_code, created_at, resolved_at)
VALUES
  ('demo_recon_line_amount', 'demo_recon_differences', 'demo_pay_attempt_failed', 'demo-ext-amount', 'amount_mismatch', 8800, 8000, 'CNY', 'pending', repeat('1',64), 'payment', repeat('2',64), 'failed', 'succeeded', CURRENT_TIMESTAMP - INTERVAL '3 days', NULL),
  ('demo_recon_line_status', 'demo_recon_differences', 'demo_pay_attempt_review', 'demo-ext-status', 'status_mismatch', 16800, 16800, 'CNY', 'accepted', repeat('3',64), 'payment', repeat('4',64), 'manual_review', 'processing', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_recon_line_missing', 'demo_recon_differences', '', 'demo-ext-missing', 'local_missing', 0, 12800, 'CNY', 'supplemented', repeat('5',64), 'payment', repeat('6',64), '', 'succeeded', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT ("line_no") DO NOTHING;

INSERT INTO plugin_idc_payment_exception_fund
  ("exception_no", "attempt_no", "channel_no", "currency_code", "amount_minor",
   "reason_code", "status_code", "evidence_digest", created_at)
VALUES
  ('demo_exception_pending', 'demo_pay_attempt_confirming', 'demo_pay_wechat', 'CNY', 50000, 'unmatched_success', 'pending', repeat('1',64), CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_exception_disputed', 'demo_pay_attempt_failed', 'demo_pay_epay', 'CNY', 8800, 'chargeback', 'disputed', repeat('2',64), CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_exception_closed', 'demo_pay_attempt_expired', 'demo_pay_unionpay', 'CNY', 32000, 'settlement_reversal', 'closed', repeat('3',64), CURRENT_TIMESTAMP - INTERVAL '4 days')
ON CONFLICT ("exception_no") DO NOTHING;

INSERT INTO plugin_idc_payment_exception_fund_action
  ("action_no", "exception_no", "action_code", "status_code", "requested_by",
   "completed_by", "request_digest", "proof_digest", "failure_code",
   requested_at, completed_at, updated_at)
VALUES
  ('demo_exception_action_pending', 'demo_exception_pending', 'allocate', 'manual_pending', 1, 0, repeat('4',64), repeat('5',64), '', CURRENT_TIMESTAMP - INTERVAL '20 minutes', NULL, CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
  ('demo_exception_action_failed', 'demo_exception_disputed', 'original_refund', 'failed', 1, 1, repeat('6',64), repeat('7',64), 'channel_unavailable', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '23 hours', CURRENT_TIMESTAMP - INTERVAL '23 hours'),
  ('demo_exception_action_complete', 'demo_exception_closed', 'original_refund', 'completed', 1, 1, repeat('8',64), repeat('9',64), '', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days')
ON CONFLICT ("action_no") DO NOTHING;
