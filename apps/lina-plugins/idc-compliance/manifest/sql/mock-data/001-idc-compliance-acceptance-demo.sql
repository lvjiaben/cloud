-- Local acceptance data only. NuoNuo remains unconfigured.

INSERT INTO plugin_idc_compliance_provider_configuration
  ("configuration_no", "provider_code", "status_code", "scene_id",
   "credential_generation", "last_validation_code")
VALUES
  ('compliance-provider-aliyun-face', 'aliyun_face', 'draft', '', 0, '')
ON CONFLICT ("configuration_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_supplier_configuration
  ("configuration_no", "provider_code", credential_status_code,
   "credential_generation", "capability_evidence_code", adapter_enabled,
   subject_status_code, last_operation_kind_code, last_operation_result_code)
VALUES
  ('local-supplier-nuonuo_invoice', 'nuonuo_invoice', 'unconfigured', 0, 'external_confirmation_required', false, 'unbound', 'none', 'none')
ON CONFLICT ("configuration_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_enterprise_encryption_key
  ("key_no", "secret_ref", "generation", "status_code")
VALUES
  ('enterprise_material_key_v9001', 'demo://local-acceptance-only', 9001, 'active')
ON CONFLICT ("key_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_verification_case
  ("case_no", "customer_no", "principal_no", "subject_type_code", "provider_code",
   "status_code", "consent_no", "masked_name", "masked_identity_no",
   "identity_lookup_digest", "decision_reason_code", verified_at, rejected_at,
   expires_at, request_idempotency_key_digest, "request_fingerprint",
   "encryption_key_no", applicant_role_code, created_at, updated_at)
VALUES
  ('demo_verify_personal_draft', 'CUS_10000000000000000000000000000002', 'demo_principal_002', 'personal', 'aliyun_face', 'draft', 'demo_consent_001', '陈*凡', '310***********1002', repeat('1',64), '', NULL, NULL, NULL, NULL, NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
  ('demo_verify_personal_pending', 'CUS_10000000000000000000000000000001', 'demo_principal_001', 'personal', 'aliyun_face', 'pending', 'demo_consent_002', '林*云', '310***********0123', repeat('2',64), '', NULL, NULL, NULL, NULL, NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '20 minutes', CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
  ('demo_verify_personal_review', 'CUS_10000000000000000000000000000003', 'demo_principal_003', 'personal', 'aliyun_face', 'review_required', 'demo_consent_003', '周*墨', '440***********3021', repeat('3',64), 'image_quality_low', NULL, NULL, NULL, NULL, NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_verify_personal_verified', 'CUS_10000000000000000000000000000001', 'demo_principal_001', 'personal', 'aliyun_face', 'verified', 'demo_consent_004', '林*云', '310***********0123', repeat('4',64), '', CURRENT_TIMESTAMP - INTERVAL '100 days', NULL, CURRENT_TIMESTAMP + INTERVAL '9 years', NULL, NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '100 days', CURRENT_TIMESTAMP - INTERVAL '100 days'),
  ('demo_verify_personal_rejected', 'CUS_10000000000000000000000000000003', 'demo_principal_003', 'personal', 'aliyun_face', 'rejected', 'demo_consent_005', '周*墨', '440***********3021', repeat('5',64), 'identity_mismatch', NULL, CURRENT_TIMESTAMP - INTERVAL '5 days', NULL, NULL, NULL, NULL, '', CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
  ('demo_verify_enterprise_pending', 'CUS_10000000000000000000000000000006', 'demo_principal_006', 'enterprise', 'manual_enterprise', 'pending', 'demo_consent_006', '远航数据服务有限公司', '91320101********02', NULL, '', NULL, NULL, NULL, repeat('6',64), repeat('7',64), 'enterprise_material_key_v9001', 'legal_representative', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '3 days'),
  ('demo_verify_enterprise_review', 'CUS_10000000000000000000000000000005', 'demo_principal_005', 'enterprise', 'manual_enterprise', 'review_required', 'demo_consent_007', '云杉科技有限公司', '91310101********01', NULL, 'material_review', NULL, NULL, NULL, repeat('8',64), repeat('9',64), 'enterprise_material_key_v9001', 'authorized_agent', CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '2 days')
ON CONFLICT ("case_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_contract_template
  ("template_no", origin_code, created_at)
VALUES
  ('ctpl_20000000000000000000000000000001', 'local', CURRENT_TIMESTAMP - INTERVAL '30 days')
ON CONFLICT ("template_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_contract_template_version
  ("template_no", "version_no", "name", "scope_code", "signing_order_code",
   "language_code", "status_code", preview_confirmed, created_at, updated_at)
VALUES
  ('ctpl_20000000000000000000000000000001', 1, '演示服务采购合同', 'all_customers', 'customer_first', 'zh-CN', 'draft', false, CURRENT_TIMESTAMP - INTERVAL '30 days', CURRENT_TIMESTAMP - INTERVAL '5 days')
ON CONFLICT ("template_no", "version_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_contract
  ("contract_no", "customer_no", "principal_no", "order_no", "contract_type_code",
   required_for_fulfillment, "template_no", "template_version",
   "variable_snapshot_ref", "variable_snapshot_sha256", "signer_snapshot_sha256",
   "currency_code", "amount_minor", "status_code", "subject_type_code",
   "signing_order_code", snapshot_key_ref, order_snapshot_sha256,
   created_at, updated_at, signing_expires_at)
VALUES
  ('CTR_20000000000000000000000000000001', 'CUS_10000000000000000000000000000002', 'demo_principal_002', 'ORD_20000000000000000000000000000001', 'service_purchase', false, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/draft/variables', repeat('1',64), repeat('2',64), 'CNY', 12900, 'draft', 'individual', 'customer_first', 'demo://contract/draft/key', repeat('3',64), CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '6 days'),
  ('CTR_20000000000000000000000000000002', 'CUS_10000000000000000000000000000006', 'demo_principal_006', 'ORD_20000000000000000000000000000004', 'service_purchase', true, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/pending/variables', repeat('4',64), repeat('5',64), 'CNY', 85000, 'pending_launch', 'enterprise', 'parallel', 'demo://contract/pending/key', repeat('6',64), CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '10 minutes', CURRENT_TIMESTAMP + INTERVAL '5 days'),
  ('CTR_20000000000000000000000000000003', 'CUS_10000000000000000000000000000005', 'demo_principal_005', 'ORD_20000000000000000000000000000007', 'service_purchase', true, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/signing/variables', repeat('7',64), repeat('8',64), 'CNY', 150000, 'signing', 'enterprise', 'merchant_first', 'demo://contract/signing/key', repeat('9',64), CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '4 days'),
  ('CTR_20000000000000000000000000000004', 'CUS_10000000000000000000000000000003', 'demo_principal_003', 'ORD_20000000000000000000000000000003', 'change', false, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/rejected/variables', repeat('a',64), repeat('b',64), 'CNY', 29900, 'rejected', 'individual', 'customer_first', 'demo://contract/rejected/key', repeat('c',64), CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP + INTERVAL '1 day'),
  ('CTR_20000000000000000000000000000005', 'CUS_10000000000000000000000000000004', 'demo_principal_004', 'ORD_20000000000000000000000000000006', 'renewal', false, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/expired/variables', repeat('d',64), repeat('e',64), 'CNY', 9900, 'expired', 'individual', 'customer_first', 'demo://contract/expired/key', repeat('f',64), CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '3 days'),
  ('CTR_20000000000000000000000000000006', 'CUS_10000000000000000000000000000005', 'demo_principal_005', 'ORD_20000000000000000000000000000002', 'other', false, 'ctpl_20000000000000000000000000000001', 1, 'demo://contract/failed/variables', repeat('0',64), repeat('1',64), 'CNY', 46000, 'failed', 'enterprise', 'parallel', 'demo://contract/failed/key', repeat('2',64), CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP + INTERVAL '3 days')
ON CONFLICT ("contract_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_tax_invoice
  ("tax_invoice_no", "request_sha256", "customer_no", "principal_no",
   "seller_snapshot_ref", "seller_snapshot_sha256", "buyer_snapshot_ref",
   "buyer_snapshot_sha256", "invoice_type_code", "status_code",
   "tax_currency_code", "gross_amount_minor", "net_amount_minor", "tax_amount_minor",
   "billing_currency_code", "masked_invoice_number", "decision_reason_code",
   decision_internal_note, created_at, updated_at)
VALUES
  ('demo_tax_draft', repeat('1',64), 'CUS_10000000000000000000000000000002', 'demo_principal_002', 'demo://seller/1', repeat('2',64), 'demo://buyer/1', repeat('3',64), 'pending_confirmation', 'draft', 'CNY', 10600, 10000, 600, 'CNY', '', '', '', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_tax_review', repeat('4',64), 'CUS_10000000000000000000000000000006', 'demo_principal_006', 'demo://seller/2', repeat('5',64), 'demo://buyer/2', repeat('6',64), 'vat_electronic_normal', 'pending_review', 'CNY', 53000, 50000, 3000, 'CNY', '', '', '', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
  ('demo_tax_approved', repeat('7',64), 'CUS_10000000000000000000000000000005', 'demo_principal_005', 'demo://seller/3', repeat('8',64), 'demo://buyer/3', repeat('9',64), 'vat_electronic_special', 'approved_pending_submission', 'CNY', 106000, 100000, 6000, 'CNY', '', '', '', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_tax_queued', repeat('a',64), 'CUS_10000000000000000000000000000001', 'demo_principal_001', 'demo://seller/4', repeat('b',64), 'demo://buyer/4', repeat('c',64), 'digital_electronic_normal', 'queued', 'CNY', 21200, 20000, 1200, 'CNY', '', '', '', CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('demo_tax_rejected', repeat('d',64), 'CUS_10000000000000000000000000000003', 'demo_principal_003', 'demo://seller/5', repeat('e',64), 'demo://buyer/5', repeat('f',64), 'vat_electronic_normal', 'rejected', 'CNY', 31800, 30000, 1800, 'CNY', '', 'buyer_or_tax_information_incorrect', '演示：购方税务信息需要修改', CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '4 days'),
  ('demo_tax_failed', repeat('0',64), 'CUS_10000000000000000000000000000005', 'demo_principal_005', 'demo://seller/6', repeat('1',64), 'demo://buyer/6', repeat('2',64), 'digital_electronic_special', 'failed', 'CNY', 84800, 80000, 4800, 'CNY', '', '', '演示：供应商未配置，未执行真实开票', CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
  ('demo_tax_unknown', repeat('3',64), 'CUS_10000000000000000000000000000006', 'demo_principal_006', 'demo://seller/7', repeat('4',64), 'demo://buyer/7', repeat('5',64), 'digital_electronic_normal', 'result_unknown', 'CNY', 63600, 60000, 3600, 'CNY', '', '', '演示：结果暂不确定', CURRENT_TIMESTAMP - INTERVAL '7 days', CURRENT_TIMESTAMP - INTERVAL '6 days'),
  ('demo_tax_cancelled', repeat('6',64), 'CUS_10000000000000000000000000000004', 'demo_principal_004', 'demo://seller/8', repeat('7',64), 'demo://buyer/8', repeat('8',64), 'pending_confirmation', 'canceled', 'CNY', 5300, 5000, 300, 'CNY', '', 'customer_requested_cancellation', '演示：客户取消申请', CURRENT_TIMESTAMP - INTERVAL '8 days', CURRENT_TIMESTAMP - INTERVAL '7 days')
ON CONFLICT ("tax_invoice_no") DO NOTHING;

INSERT INTO plugin_idc_compliance_red_letter_task
  ("task_no", "tax_invoice_no", "request_sha256", "refund_operation_no",
   "reason_code", "status_code", "gross_amount_minor", "net_amount_minor",
   "tax_amount_minor", customer_confirmation_required, customer_confirmed_at,
   available_at, "failure_code", "customer_no", "currency_code",
   allocation_resolved, manual_reason_code, created_at, updated_at)
VALUES
  ('demo_red_pending', 'demo_tax_review', repeat('1',64), 'demo_refund_operation_001', 'service_refund', 'pending_review', 53000, 50000, 3000, false, NULL, CURRENT_TIMESTAMP, '', 'CUS_10000000000000000000000000000006', 'CNY', true, '', CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '1 hour'),
  ('demo_red_confirm', 'demo_tax_approved', repeat('2',64), 'demo_refund_operation_002', 'customer_request', 'awaiting_confirmation', 106000, 100000, 6000, true, NULL, CURRENT_TIMESTAMP, '', 'CUS_10000000000000000000000000000005', 'CNY', true, '', CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('demo_red_manual', 'demo_tax_queued', repeat('3',64), 'demo_refund_operation_003', 'allocation_review', 'manual_required', 21200, 0, 0, false, NULL, CURRENT_TIMESTAMP, '', 'CUS_10000000000000000000000000000001', 'CNY', false, 'allocation_selection_required', CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP - INTERVAL '3 hours'),
  ('demo_red_unknown', 'demo_tax_unknown', repeat('4',64), 'demo_refund_operation_004', 'provider_unknown', 'result_unknown', 63600, 60000, 3600, false, NULL, CURRENT_TIMESTAMP, 'provider_result_unknown', 'CUS_10000000000000000000000000000006', 'CNY', true, '', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_red_failed', 'demo_tax_failed', repeat('5',64), 'demo_refund_operation_005', 'provider_unavailable', 'failed', 84800, 80000, 4800, false, NULL, CURRENT_TIMESTAMP, 'provider_unconfigured', 'CUS_10000000000000000000000000000005', 'CNY', true, '', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT ("task_no") DO NOTHING;
