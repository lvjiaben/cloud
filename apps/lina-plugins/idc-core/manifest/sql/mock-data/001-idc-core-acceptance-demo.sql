-- Local acceptance data only. Stable demo identifiers keep this script idempotent.

INSERT INTO plugin_idc_core_customer_subject
  ("customer_no", "customer_type_code", "status_code", "display_name", search_name,
   status_reason_code, status_reason, status_changed_by, status_changed_at,
   "created_by", "updated_by", created_at, updated_at)
VALUES
  ('CUS_10000000000000000000000000000001', 'individual', 'active', '林晓云', '林晓云 13800138001', '', '', 0, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '120 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('CUS_10000000000000000000000000000002', 'individual', 'pending_activation', '陈一凡', '陈一凡 13800138002', '', '', 0, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '15 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('CUS_10000000000000000000000000000003', 'individual', 'restricted', '周子墨', '周子墨 13800138003', 'risk_review', '演示：账户处于人工复核中', 1, CURRENT_TIMESTAMP - INTERVAL '3 days', 1, 1, CURRENT_TIMESTAMP - INTERVAL '90 days', CURRENT_TIMESTAMP - INTERVAL '3 days'),
  ('CUS_10000000000000000000000000000004', 'individual', 'disabled', '吴思远', '吴思远 13800138004', 'customer_request', '演示：客户主动停用', 1, CURRENT_TIMESTAMP - INTERVAL '10 days', 1, 1, CURRENT_TIMESTAMP - INTERVAL '180 days', CURRENT_TIMESTAMP - INTERVAL '10 days'),
  ('CUS_10000000000000000000000000000005', 'enterprise', 'active', '云杉科技有限公司', '云杉科技 企业客户', '', '', 0, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '220 days', CURRENT_TIMESTAMP - INTERVAL '4 hours'),
  ('CUS_10000000000000000000000000000006', 'enterprise', 'active', '远航数据服务有限公司', '远航数据 企业客户', '', '', 0, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '28 days', CURRENT_TIMESTAMP - INTERVAL '5 days')
ON CONFLICT ("customer_no") DO NOTHING;

INSERT INTO plugin_idc_core_customer_individual_profile
  ("customer_no", "legal_name", "legal_name_masked", "identity_identifier_masked",
   "identity_lookup_digest", "identity_case_no", "identity_status_code",
   identity_verified_at, identity_expires_at)
VALUES
  ('CUS_10000000000000000000000000000001', '林晓云', '林*云', '310***********0123', repeat('1', 64), 'demo_kyc_verified', 'verified', CURRENT_TIMESTAMP - INTERVAL '100 days', CURRENT_TIMESTAMP + INTERVAL '9 years'),
  ('CUS_10000000000000000000000000000002', '', '', '', '', '', 'pending', NULL, NULL),
  ('CUS_10000000000000000000000000000003', '周子墨', '周*墨', '440***********3021', repeat('2', 64), 'demo_kyc_rejected', 'rejected', NULL, NULL),
  ('CUS_10000000000000000000000000000004', '', '', '', '', '', 'unverified', NULL, NULL)
ON CONFLICT ("customer_no") DO NOTHING;

INSERT INTO plugin_idc_core_customer_enterprise_profile
  ("customer_no", "legal_name", "registration_identifier", "registration_identifier_masked",
   "registration_lookup_digest", "industry_code", "verification_case_no",
   "verification_status_code", verified_at, expires_at)
VALUES
  ('CUS_10000000000000000000000000000005', '云杉科技有限公司', '91310101DEMO000001', '91310101********01', repeat('3', 64), 'internet_service', 'demo_ent_verified', 'verified', CURRENT_TIMESTAMP - INTERVAL '180 days', CURRENT_TIMESTAMP + INTERVAL '2 years'),
  ('CUS_10000000000000000000000000000006', '远航数据服务有限公司', '91320101DEMO000002', '91320101********02', repeat('4', 64), 'data_service', 'demo_ent_pending', 'pending', NULL, NULL)
ON CONFLICT ("customer_no") DO NOTHING;

INSERT INTO plugin_idc_core_customer_contact
  ("contact_no", "customer_no", "display_name", "email", email_lookup_digest,
   email_verified_at, phone_country_code, "phone", phone_lookup_digest,
   phone_verified_at, "status_code", "created_by", "updated_by")
VALUES
  ('demo_contact_001', 'CUS_10000000000000000000000000000001', '林晓云', 'lin.demo@example.com', repeat('a', 64), CURRENT_TIMESTAMP - INTERVAL '100 days', '+86', '13800138001', repeat('b', 64), CURRENT_TIMESTAMP - INTERVAL '100 days', 'active', 1, 1),
  ('demo_contact_002', 'CUS_10000000000000000000000000000002', '陈一凡', '', '', NULL, '+86', '13800138002', repeat('c', 64), NULL, 'active', 1, 1),
  ('demo_contact_003', 'CUS_10000000000000000000000000000003', '周子墨', 'zhou.demo@example.com', repeat('d', 64), NULL, '+86', '13800138003', repeat('e', 64), CURRENT_TIMESTAMP - INTERVAL '80 days', 'active', 1, 1),
  ('demo_contact_004', 'CUS_10000000000000000000000000000004', '吴思远', 'wu.demo@example.com', repeat('f', 64), NULL, '+86', '13800138004', repeat('9', 64), NULL, 'archived', 1, 1),
  ('demo_contact_005', 'CUS_10000000000000000000000000000005', '赵经理', 'ops@yunsan-demo.example.com', repeat('8', 64), CURRENT_TIMESTAMP - INTERVAL '180 days', '+86', '13800138005', repeat('7', 64), CURRENT_TIMESTAMP - INTERVAL '180 days', 'active', 1, 1),
  ('demo_contact_006', 'CUS_10000000000000000000000000000006', '孙经理', 'service@yuanhang-demo.example.com', repeat('6', 64), NULL, '+86', '13800138006', repeat('5', 64), NULL, 'active', 1, 1)
ON CONFLICT ("contact_no") DO NOTHING;

INSERT INTO plugin_idc_core_catalog_category
  ("category_no", "name_zh_cn", "name_en_us", "sort_order", "status_code", "created_by", "updated_by")
VALUES
  ('ctg_00000000000000000000000000000001', '云服务器', 'Cloud Servers', 10, 'active', 1, 1),
  ('ctg_00000000000000000000000000000002', '安全与证书', 'Security and Certificates', 20, 'active', 1, 1),
  ('ctg_00000000000000000000000000000003', '已停用产品', 'Inactive Products', 90, 'inactive', 1, 1)
ON CONFLICT ("category_no") DO NOTHING;

INSERT INTO plugin_idc_core_catalog_product
  ("product_no", "category_no", "status_code", "public_slug", "name_zh_cn", "name_en_us",
   "summary_zh_cn", "summary_en_us", "sort_order", "created_by", "updated_by")
VALUES
  ('prd_00000000000000000000000000000011', 'ctg_00000000000000000000000000000001', 'active', 'acceptance-cloud-server', '弹性云服务器', 'Elastic Cloud Server', '演示产品：按月云服务器', 'Demo monthly cloud server', 10, 1, 1),
  ('prd_00000000000000000000000000000012', 'ctg_00000000000000000000000000000002', 'draft', 'acceptance-ssl-certificate', 'SSL 证书', 'SSL Certificate', '演示产品：待发布证书', 'Demo draft certificate', 20, 1, 1),
  ('prd_00000000000000000000000000000013', 'ctg_00000000000000000000000000000001', 'suspended', 'acceptance-bare-metal', '裸金属服务器', 'Bare Metal Server', '演示产品：暂停售卖', 'Demo suspended bare metal', 30, 1, 1),
  ('prd_00000000000000000000000000000014', 'ctg_00000000000000000000000000000003', 'retired', 'acceptance-retired-host', '经典主机', 'Classic Hosting', '演示产品：已退市', 'Demo retired hosting', 90, 1, 1)
ON CONFLICT ("product_no") DO NOTHING;

INSERT INTO plugin_idc_core_catalog_product_version
  ("version_no", "product_no", "version_number", "status_code", "source_code",
   "source_fingerprint", "change_summary", published_at, retired_at, "created_by")
VALUES
  ('ver_00000000000000000000000000000021', 'prd_00000000000000000000000000000011', 1, 'published', 'manual', repeat('1', 64), '演示正式版本', CURRENT_TIMESTAMP - INTERVAL '100 days', NULL, 1),
  ('ver_00000000000000000000000000000022', 'prd_00000000000000000000000000000012', 1, 'draft', 'manual', repeat('2', 64), '演示草稿版本', NULL, NULL, 1),
  ('ver_00000000000000000000000000000023', 'prd_00000000000000000000000000000013', 1, 'superseded', 'manual', repeat('3', 64), '演示已替代版本', CURRENT_TIMESTAMP - INTERVAL '200 days', NULL, 1),
  ('ver_00000000000000000000000000000024', 'prd_00000000000000000000000000000014', 1, 'retired', 'manual', repeat('4', 64), '演示退市版本', CURRENT_TIMESTAMP - INTERVAL '400 days', CURRENT_TIMESTAMP - INTERVAL '30 days', 1)
ON CONFLICT ("version_no") DO NOTHING;

-- Every product version owns one simple-inventory row.  Keep the acceptance
-- data aligned with the production create path so opening an older seeded
-- product never disappears behind the management detail INNER JOIN.
INSERT INTO plugin_idc_core_catalog_product_version_inventory
  ("version_no", "tracking_enabled", "sellable_quantity", "version")
VALUES
  ('ver_00000000000000000000000000000021', TRUE, 100, 1),
  ('ver_00000000000000000000000000000022', FALSE, 0, 1),
  ('ver_00000000000000000000000000000023', FALSE, 0, 1),
  ('ver_00000000000000000000000000000024', FALSE, 0, 1)
ON CONFLICT ("version_no") DO NOTHING;

UPDATE plugin_idc_core_catalog_product
SET "current_version_no" = CASE "product_no"
  WHEN 'prd_00000000000000000000000000000011' THEN 'ver_00000000000000000000000000000021'
  WHEN 'prd_00000000000000000000000000000012' THEN 'ver_00000000000000000000000000000022'
  WHEN 'prd_00000000000000000000000000000013' THEN 'ver_00000000000000000000000000000023'
  WHEN 'prd_00000000000000000000000000000014' THEN 'ver_00000000000000000000000000000024'
END
WHERE "product_no" IN ('prd_00000000000000000000000000000011', 'prd_00000000000000000000000000000012', 'prd_00000000000000000000000000000013', 'prd_00000000000000000000000000000014')
  AND "current_version_no" IS NULL;

INSERT INTO plugin_idc_core_catalog_billing_rule
  ("billing_rule_no", "version_no", "rule_code", "charge_type_code", "period_unit_code",
   "period_count", "price_phase_code", "quantity_mode_code", "status_code")
VALUES
  ('bil_10000000000000000000000000000021', 'ver_00000000000000000000000000000021', 'monthly', 'recurring', 'month', 1, 'both', 'fixed', 'active')
ON CONFLICT ("billing_rule_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_configuration_snapshot
  ("configuration_snapshot_no", "product_no", "version_no", "source_revision_digest",
   "selection_digest", "selected_option_count", "total_selected_quantity", "content_digest")
VALUES
  ('CFG_10000000000000000000000000000021', 'prd_00000000000000000000000000000011', 'ver_00000000000000000000000000000021', repeat('1', 64), repeat('2', 64), 0, 0, repeat('3', 64))
ON CONFLICT ("configuration_snapshot_no") DO NOTHING;

INSERT INTO plugin_idc_core_support_ticket
  ("ticket_no", "ticket_type", "visibility_code", "customer_no", "anonymous_destination_digest",
   "anonymous_destination_kind", "subject", "base_stage", "priority_code", follow_up_required,
   "sla_risk_code", resolved_at, closed_at, "closed_reason_code", "closed_by_type",
   "assignee_user_id", "created_by", "updated_by", created_at, updated_at)
VALUES
  ('demo_ticket_new', 'customer', 'customer', 'CUS_10000000000000000000000000000001', NULL, NULL, '云服务器无法远程连接', 'new', 'urgent', true, 'at_risk', NULL, NULL, NULL, NULL, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '20 minutes', CURRENT_TIMESTAMP - INTERVAL '20 minutes'),
  ('demo_ticket_progress', 'customer', 'customer', 'CUS_10000000000000000000000000000005', NULL, NULL, '申请增加出口带宽', 'in_progress', 'high', false, 'none', NULL, NULL, NULL, NULL, 1, 1, 1, CURRENT_TIMESTAMP - INTERVAL '8 hours', CURRENT_TIMESTAMP - INTERVAL '30 minutes'),
  ('demo_ticket_wait_customer', 'customer', 'customer', 'CUS_10000000000000000000000000000002', NULL, NULL, '账单金额需要确认', 'waiting_customer', 'normal', true, 'none', NULL, NULL, NULL, NULL, 1, 1, 1, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '10 hours'),
  ('demo_ticket_wait_upstream', 'customer', 'customer', 'CUS_10000000000000000000000000000006', NULL, NULL, '机房线路波动排查', 'waiting_upstream', 'high', false, 'breached', NULL, NULL, NULL, NULL, 1, 1, 1, CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_ticket_resolved', 'customer', 'customer', 'CUS_10000000000000000000000000000001', NULL, NULL, '重置控制台登录密码', 'resolved', 'low', false, 'none', CURRENT_TIMESTAMP - INTERVAL '1 day', NULL, NULL, NULL, 1, 1, 1, CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_ticket_closed', 'internal', 'internal', NULL, NULL, NULL, '演示：检查夜间巡检结果', 'closed', 'normal', false, 'none', CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '4 days', 'completed', 'staff', 1, 1, 1, CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '4 days'),
  ('demo_ticket_presales', 'presales', 'anonymous', NULL, repeat('8', 64), 'email', '咨询香港节点交付周期', 'new', 'normal', false, 'none', NULL, NULL, NULL, NULL, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP - INTERVAL '1 hour')
ON CONFLICT ("ticket_no") DO NOTHING;

INSERT INTO plugin_idc_core_content_item
  ("content_no", "content_type", "visibility_code", "title", "summary", "body", "public_path",
   "status_code", published_at, offline_at, "created_by", "updated_by", created_at, updated_at)
VALUES
  ('demo_content_news', 'news', 'public', '华东新节点正式开放', '演示新闻摘要', '这是用于本地验收的新闻内容。', '/news/demo-east-node', 'published', CURRENT_TIMESTAMP - INTERVAL '5 days', NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
  ('demo_content_announcement', 'announcement', 'all_logged_in', '控制台维护通知', '演示公告摘要', '本公告仅用于本地界面验收。', '/announcements/demo-maintenance', 'draft', NULL, NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_content_knowledge', 'knowledge', 'all_logged_in', '如何重置服务器密码', '演示知识库摘要', '步骤一：进入服务器详情。步骤二：选择重置密码。', '/knowledge/demo-reset-password', 'published', CURRENT_TIMESTAMP - INTERVAL '20 days', NULL, 1, 1, CURRENT_TIMESTAMP - INTERVAL '22 days', CURRENT_TIMESTAMP - INTERVAL '20 days'),
  ('demo_content_offline', 'download', 'internal', '旧版接入说明', '演示下线内容', '该内容已下线，仅用于验收状态样式。', '/downloads/demo-offline-guide', 'offline', CURRENT_TIMESTAMP - INTERVAL '100 days', CURRENT_TIMESTAMP - INTERVAL '10 days', 1, 1, CURRENT_TIMESTAMP - INTERVAL '110 days', CURRENT_TIMESTAMP - INTERVAL '10 days')
ON CONFLICT ("content_no") DO NOTHING;

INSERT INTO plugin_idc_core_marketing_campaign
  ("campaign_no", "campaign_code", "name_zh_cn", "name_en_us", "description_zh_cn",
   "description_en_us", "campaign_type_code", starts_at, ends_at, "priority",
   "stack_policy_code", "stack_group_code", "global_quota_limit", "global_reserved_count",
   "global_consumed_count", "per_customer_quota_limit", "status_code", "created_by", "updated_by")
VALUES
  ('demo_campaign_draft', 'demo.draft', '新客首单优惠', 'New Customer Discount', '演示草稿活动', 'Demo draft campaign', 'discount', CURRENT_TIMESTAMP + INTERVAL '3 days', CURRENT_TIMESTAMP + INTERVAL '33 days', 100, 'exclusive', 'newcomer', 1000, 0, 0, 1, 'draft', 1, 1),
  ('demo_campaign_scheduled', 'demo.scheduled', '国庆预热券', 'Holiday Coupon', '演示待开始活动', 'Demo scheduled campaign', 'coupon', CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '10 days', 200, 'same_group_exclusive', 'holiday', 500, 15, 0, 2, 'scheduled', 1, 1),
  ('demo_campaign_active', 'demo.active', '云服务器续费返现', 'Renewal Cashback', '演示进行中活动', 'Demo active campaign', 'cashback', CURRENT_TIMESTAMP - INTERVAL '7 days', CURRENT_TIMESTAMP + INTERVAL '23 days', 300, 'stackable', 'renewal', 2000, 80, 320, 3, 'active', 1, 1),
  ('demo_campaign_paused', 'demo.paused', '充值赠送积分', 'Recharge Bonus Points', '演示暂停活动', 'Demo paused campaign', 'points', CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP + INTERVAL '10 days', 400, 'exclusive', 'recharge', 800, 30, 120, 1, 'paused', 1, 1),
  ('demo_campaign_ended', 'demo.ended', '暑期特惠', 'Summer Sale', '演示已结束活动', 'Demo ended campaign', 'voucher', CURRENT_TIMESTAMP - INTERVAL '60 days', CURRENT_TIMESTAMP - INTERVAL '30 days', 500, 'exclusive', 'seasonal', 1200, 0, 1100, 1, 'ended', 1, 1)
ON CONFLICT ("campaign_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_quote_snapshot
  ("quote_no", "customer_no", "customer_subject_version", "source_code", "currency_code",
   "item_count", "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "input_digest", "pricing_rule_digest", "eligibility_digest",
   "inventory_digest", "content_digest", issued_at, expires_at, "created_by")
VALUES
  ('QTE_20000000000000000000000000000001', 'CUS_10000000000000000000000000000001', 1, 'customer', 'CNY', 1, 12900, 0, 12900, 0, 12900, repeat('1',64), repeat('2',64), repeat('3',64), repeat('4',64), repeat('5',64), CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP + INTERVAL '22 hours', 1),
  ('QTE_20000000000000000000000000000002', 'CUS_10000000000000000000000000000005', 1, 'customer', 'CNY', 2, 46800, 800, 46000, 0, 46000, repeat('6',64), repeat('7',64), repeat('8',64), repeat('9',64), repeat('a',64), CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '1 day', 1),
  ('QTE_20000000000000000000000000000003', 'CUS_10000000000000000000000000000003', 1, 'customer', 'CNY', 1, 29900, 0, 29900, 0, 29900, repeat('b',64), repeat('c',64), repeat('d',64), repeat('e',64), repeat('f',64), CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP + INTERVAL '10 days', 1),
  ('QTE_20000000000000000000000000000004', 'CUS_10000000000000000000000000000006', 1, 'customer', 'CNY', 3, 88000, 3000, 85000, 0, 85000, repeat('0',64), repeat('1',64), repeat('2',64), repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP - INTERVAL '8 days', CURRENT_TIMESTAMP + INTERVAL '5 days', 1),
  ('QTE_20000000000000000000000000000005', 'CUS_10000000000000000000000000000001', 1, 'customer', 'CNY', 1, 59900, 0, 59900, 0, 59900, repeat('5',64), repeat('6',64), repeat('7',64), repeat('8',64), repeat('9',64), CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP + INTERVAL '1 day', 1),
  ('QTE_20000000000000000000000000000006', 'CUS_10000000000000000000000000000002', 1, 'customer', 'CNY', 1, 9900, 0, 9900, 0, 9900, repeat('a',64), repeat('b',64), repeat('c',64), repeat('d',64), repeat('e',64), CURRENT_TIMESTAMP - INTERVAL '12 days', CURRENT_TIMESTAMP + INTERVAL '1 day', 1),
  ('QTE_20000000000000000000000000000007', 'CUS_10000000000000000000000000000005', 1, 'staff', 'CNY', 2, 159000, 9000, 150000, 0, 150000, repeat('f',64), repeat('0',64), repeat('1',64), repeat('2',64), repeat('3',64), CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP + INTERVAL '2 days', 1),
  ('QTE_20000000000000000000000000000008', 'CUS_10000000000000000000000000000006', 1, 'staff', 'CNY', 1, 36000, 1000, 35000, 0, 35000, repeat('4',64), repeat('5',64), repeat('6',64), repeat('7',64), repeat('8',64), CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '6 days', 1)
ON CONFLICT ("quote_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_order
  ("order_no", "customer_no", "quote_no", "currency_code", "source_code", "status_code",
   "risk_decision_code", "item_count", "subtotal_minor", "discount_minor", "net_amount_minor",
   "tax_amount_minor", "total_amount_minor", "checkout_idempotency_digest",
   "checkout_request_digest", payment_due_at, paid_at, completed_at, cancelled_at,
   "created_by", created_at, updated_at)
VALUES
  ('ORD_20000000000000000000000000000001', 'CUS_10000000000000000000000000000001', 'QTE_20000000000000000000000000000001', 'CNY', 'portal', 'pending_payment', 'approved', 1, 12900, 0, 12900, 0, 12900, repeat('1',64), repeat('2',64), CURRENT_TIMESTAMP + INTERVAL '22 hours', NULL, NULL, NULL, 1, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('ORD_20000000000000000000000000000002', 'CUS_10000000000000000000000000000005', 'QTE_20000000000000000000000000000002', 'CNY', 'portal', 'payment_processing', 'approved', 2, 46800, 800, 46000, 0, 46000, repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP + INTERVAL '1 day', NULL, NULL, NULL, 1, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
  ('ORD_20000000000000000000000000000003', 'CUS_10000000000000000000000000000003', 'QTE_20000000000000000000000000000003', 'CNY', 'portal', 'paid', 'approved', 1, 29900, 0, 29900, 0, 29900, repeat('5',64), repeat('6',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '4 days', NULL, NULL, 1, CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '4 days'),
  ('ORD_20000000000000000000000000000004', 'CUS_10000000000000000000000000000006', 'QTE_20000000000000000000000000000004', 'CNY', 'portal', 'fulfilling', 'approved', 3, 88000, 3000, 85000, 0, 85000, repeat('7',64), repeat('8',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '7 days', NULL, NULL, 1, CURRENT_TIMESTAMP - INTERVAL '8 days', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('ORD_20000000000000000000000000000005', 'CUS_10000000000000000000000000000001', 'QTE_20000000000000000000000000000005', 'CNY', 'portal', 'fulfilled', 'approved', 1, 59900, 0, 59900, 0, 59900, repeat('9',64), repeat('a',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '19 days', CURRENT_TIMESTAMP - INTERVAL '18 days', NULL, 1, CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP - INTERVAL '18 days'),
  ('ORD_20000000000000000000000000000006', 'CUS_10000000000000000000000000000002', 'QTE_20000000000000000000000000000006', 'CNY', 'portal', 'cancelled', 'approved', 1, 9900, 0, 9900, 0, 9900, repeat('b',64), repeat('c',64), CURRENT_TIMESTAMP + INTERVAL '1 day', NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '11 days', 1, CURRENT_TIMESTAMP - INTERVAL '12 days', CURRENT_TIMESTAMP - INTERVAL '11 days'),
  ('ORD_20000000000000000000000000000007', 'CUS_10000000000000000000000000000005', 'QTE_20000000000000000000000000000007', 'CNY', 'portal', 'manual_review', 'manual_review', 2, 159000, 9000, 150000, 0, 150000, repeat('d',64), repeat('e',64), CURRENT_TIMESTAMP + INTERVAL '2 days', NULL, NULL, NULL, 1, CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP - INTERVAL '3 hours')
ON CONFLICT ("order_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_assisted_quote_request
  ("request_no", "customer_no", "currency_code", "status_code", "item_count",
   "manual_discount_minor", "payment_term_code", "payment_due_seconds", "reason",
   "internal_note", "idempotency_digest", "request_digest", expires_at, "requested_by",
   "decision_code", "decision_by", decision_at, "decision_reason",
   "decision_request_digest", "decision_evidence_digest", "quote_no", created_at, updated_at)
VALUES
  ('demo_assisted_pending', 'CUS_10000000000000000000000000000001', 'CNY', 'pending_approval', 1, 0, 'standard_prepay', 3600, '客户需要由销售协助确认云服务器配置', '演示：等待主管审批', repeat('1',64), repeat('2',64), CURRENT_TIMESTAMP + INTERVAL '2 days', 1, NULL, NULL, NULL, NULL, NULL, NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('demo_assisted_rejected', 'CUS_10000000000000000000000000000003', 'CNY', 'rejected', 1, 2000, 'standard_prepay', 3600, '客户申请超出活动规则的额外折扣', '演示：折扣超出授权范围', repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP + INTERVAL '1 day', 1, 'rejected', 2, CURRENT_TIMESTAMP - INTERVAL '1 day', '折扣超出当前审批范围', repeat('5',64), repeat('6',64), NULL, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_assisted_approved', 'CUS_10000000000000000000000000000006', 'CNY', 'approved', 1, 1000, 'extended_prepay', 172800, '企业客户需要延长付款期限并锁定当前报价', '演示：已审批，等待客户下单', repeat('7',64), repeat('8',64), CURRENT_TIMESTAMP + INTERVAL '6 days', 1, 'approved', 2, CURRENT_TIMESTAMP - INTERVAL '20 hours', '客户资质与折扣范围均已确认', repeat('9',64), repeat('a',64), 'QTE_20000000000000000000000000000008', CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '20 hours')
ON CONFLICT ("request_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_assisted_quote_request_item
  ("request_item_no", "request_no", "currency_code", "line_number", "product_no", "version_no",
   "billing_rule_no", "configuration_snapshot_no", "product_name_zh_cn",
   "product_name_en_us", "purchase_cycles", "content_digest")
VALUES
  ('demo_assisted_item_pending', 'demo_assisted_pending', 'CNY', 1, 'prd_00000000000000000000000000000011', 'ver_00000000000000000000000000000021', 'bil_10000000000000000000000000000021', 'CFG_10000000000000000000000000000021', '弹性云服务器', 'Elastic Cloud Server', 1, repeat('b',64)),
  ('demo_assisted_item_rejected', 'demo_assisted_rejected', 'CNY', 1, 'prd_00000000000000000000000000000011', 'ver_00000000000000000000000000000021', 'bil_10000000000000000000000000000021', 'CFG_10000000000000000000000000000021', '弹性云服务器', 'Elastic Cloud Server', 12, repeat('c',64)),
  ('demo_assisted_item_approved', 'demo_assisted_approved', 'CNY', 1, 'prd_00000000000000000000000000000011', 'ver_00000000000000000000000000000021', 'bil_10000000000000000000000000000021', 'CFG_10000000000000000000000000000021', '弹性云服务器', 'Elastic Cloud Server', 3, repeat('d',64))
ON CONFLICT ("request_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_assisted_quote_item_quantity
  ("request_item_no", "quantity", "content_digest")
VALUES
  ('demo_assisted_item_pending', 1, repeat('e',64)),
  ('demo_assisted_item_rejected', 2, repeat('f',64)),
  ('demo_assisted_item_approved', 3, repeat('0',64))
ON CONFLICT ("request_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_invoice
  ("invoice_no", "customer_no", "source_code", "currency_code", "status_code", "item_count",
   "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "paid_amount_minor", waived_amount_minor, "credit_amount_minor",
   "refundable_amount_minor", "refunded_amount_minor", "source_snapshot_digest",
   "content_digest", payment_due_at, issued_at, settled_at, cancelled_at,
   created_at, updated_at)
VALUES
  ('demo_invoice_draft', 'CUS_10000000000000000000000000000002', 'manual', 'CNY', 'draft', 1, 10000, 0, 10000, 0, 10000, 0, 0, 0, 0, 0, repeat('1',64), repeat('2',64), CURRENT_TIMESTAMP + INTERVAL '7 days', NULL, NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
  ('demo_invoice_pending', 'CUS_10000000000000000000000000000001', 'manual', 'CNY', 'pending_payment', 1, 12900, 0, 12900, 0, 12900, 0, 0, 0, 0, 0, repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP + INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 hours', NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('demo_invoice_overdue', 'CUS_10000000000000000000000000000003', 'manual', 'CNY', 'overdue', 1, 29900, 0, 29900, 0, 29900, 0, 0, 0, 0, 0, repeat('9',64), repeat('a',64), CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '12 days', NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '13 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
  ('demo_invoice_cancelled', 'CUS_10000000000000000000000000000004', 'manual', 'CNY', 'cancelled', 1, 9900, 0, 9900, 0, 9900, 0, 0, 0, 0, 0, repeat('b',64), repeat('c',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '3 days', NULL, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_invoice_bad_debt', 'CUS_10000000000000000000000000000006', 'manual', 'CNY', 'bad_debt', 1, 46000, 0, 46000, 0, 46000, 0, 0, 0, 0, 0, repeat('d',64), repeat('e',64), CURRENT_TIMESTAMP - INTERVAL '20 days', CURRENT_TIMESTAMP - INTERVAL '27 days', NULL, NULL, CURRENT_TIMESTAMP - INTERVAL '28 days', CURRENT_TIMESTAMP - INTERVAL '2 days')
ON CONFLICT ("invoice_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_invoice_item
  ("invoice_item_no", "invoice_no", "customer_no", "currency_code", "line_number",
   "item_type_code", "source_reference", "description_zh_cn", "description_en_us",
   "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "refundable_cap_minor", "content_digest")
VALUES
  ('demo_inv_item_draft', 'demo_invoice_draft', 'CUS_10000000000000000000000000000002', 'CNY', 1, 'manual', 'demo-draft-service', '演示草稿服务费', 'Demo draft service', 10000, 0, 10000, 0, 10000, 10000, repeat('1',64)),
  ('demo_inv_item_pending', 'demo_invoice_pending', 'CUS_10000000000000000000000000000001', 'CNY', 1, 'manual', 'demo-pending-service', '演示待付款服务费', 'Demo pending service', 12900, 0, 12900, 0, 12900, 12900, repeat('2',64)),
  ('demo_inv_item_overdue', 'demo_invoice_overdue', 'CUS_10000000000000000000000000000003', 'CNY', 1, 'manual', 'demo-overdue-service', '演示逾期服务费', 'Demo overdue service', 29900, 0, 29900, 0, 29900, 29900, repeat('6',64)),
  ('demo_inv_item_cancelled', 'demo_invoice_cancelled', 'CUS_10000000000000000000000000000004', 'CNY', 1, 'manual', 'demo-cancelled-service', '演示已取消服务费', 'Demo cancelled service', 9900, 0, 9900, 0, 9900, 9900, repeat('7',64)),
  ('demo_inv_item_bad_debt', 'demo_invoice_bad_debt', 'CUS_10000000000000000000000000000006', 'CNY', 1, 'manual', 'demo-bad-debt-service', '演示坏账服务费', 'Demo bad-debt service', 46000, 0, 46000, 0, 46000, 46000, repeat('8',64))
ON CONFLICT ("invoice_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_account
  ("account_no", "owner_type_code", "owner_reference", "customer_no", "account_type_code",
   "normal_balance_code", "currency_code", "status_code")
VALUES
  ('demo_acc_verified', 'customer', 'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000001', 'customer_balance', 'credit', 'CNY', 'active'),
  ('demo_acc_enterprise', 'customer', 'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000005', 'customer_balance', 'credit', 'CNY', 'active'),
  ('demo_acc_restricted', 'customer', 'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000003', 'customer_balance', 'credit', 'CNY', 'frozen'),
  ('demo_acc_cash', 'merchant', 'merchant', NULL, 'merchant_cash', 'debit', 'CNY', 'active')
ON CONFLICT ("account_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_transaction
  ("transaction_no", "transaction_type_code", "currency_code", "source_type_code",
   "source_reference", "source_snapshot_digest", "entry_count", "debit_total_minor",
   "credit_total_minor", "content_digest", "posted_by", posted_at, created_at)
VALUES
  ('demo_tx_recharge', 'recharge', 'CNY', 'demo_recharge', 'demo-recharge-001', repeat('1',64), 2, 50000, 50000, repeat('2',64), 1, CURRENT_TIMESTAMP - INTERVAL '6 days', CURRENT_TIMESTAMP - INTERVAL '6 days'),
  ('demo_tx_payment', 'payment', 'CNY', 'demo_payment', 'demo-payment-001', repeat('3',64), 2, 85000, 85000, repeat('4',64), 1, CURRENT_TIMESTAMP - INTERVAL '8 days', CURRENT_TIMESTAMP - INTERVAL '8 days'),
  ('demo_tx_refund', 'refund', 'CNY', 'demo_refund', 'demo-refund-001', repeat('5',64), 2, 20000, 20000, repeat('6',64), 1, CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_tx_gift', 'gift', 'CNY', 'demo_marketing', 'demo-gift-001', repeat('7',64), 2, 3000, 3000, repeat('8',64), 1, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day')
ON CONFLICT ("transaction_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_entry
  ("entry_no", "transaction_no", "currency_code", "line_number", "account_no",
   "direction_code", "entry_role_code", "amount_minor", "invoice_no", "reason_code", "content_digest")
VALUES
  ('demo_entry_recharge_1', 'demo_tx_recharge', 'CNY', 1, 'demo_acc_cash', 'debit', 'cash', 50000, NULL, 'demo_recharge', repeat('1',64)),
  ('demo_entry_recharge_2', 'demo_tx_recharge', 'CNY', 2, 'demo_acc_verified', 'credit', 'balance', 50000, NULL, 'demo_recharge', repeat('2',64)),
  ('demo_entry_payment_1', 'demo_tx_payment', 'CNY', 1, 'demo_acc_cash', 'debit', 'cash', 85000, NULL, 'demo_payment', repeat('3',64)),
  ('demo_entry_payment_2', 'demo_tx_payment', 'CNY', 2, 'demo_acc_enterprise', 'credit', 'balance', 85000, NULL, 'demo_payment', repeat('4',64)),
  ('demo_entry_refund_1', 'demo_tx_refund', 'CNY', 1, 'demo_acc_cash', 'debit', 'refund', 20000, NULL, 'demo_refund', repeat('5',64)),
  ('demo_entry_refund_2', 'demo_tx_refund', 'CNY', 2, 'demo_acc_verified', 'credit', 'balance', 20000, NULL, 'demo_refund', repeat('6',64)),
  ('demo_entry_gift_1', 'demo_tx_gift', 'CNY', 1, 'demo_acc_cash', 'debit', 'expense', 3000, NULL, 'demo_gift', repeat('7',64)),
  ('demo_entry_gift_2', 'demo_tx_gift', 'CNY', 2, 'demo_acc_verified', 'credit', 'gift', 3000, NULL, 'demo_gift', repeat('8',64))
ON CONFLICT ("entry_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_balance_snapshot
  ("snapshot_no", "account_no", "currency_code", "normal_balance_code", "transaction_no",
   "sequence_no", "previous_snapshot_no", "previous_sequence_no", "opening_balance_minor",
   "debit_amount_minor", "credit_amount_minor", "closing_balance_minor", "content_digest")
VALUES
  ('demo_snap_cash_1', 'demo_acc_cash', 'CNY', 'debit', 'demo_tx_recharge', 1, NULL, NULL, 0, 50000, 0, 50000, repeat('1',64)),
  ('demo_snap_verified_1', 'demo_acc_verified', 'CNY', 'credit', 'demo_tx_recharge', 1, NULL, NULL, 0, 0, 50000, 50000, repeat('2',64)),
  ('demo_snap_cash_2', 'demo_acc_cash', 'CNY', 'debit', 'demo_tx_payment', 2, 'demo_snap_cash_1', 1, 50000, 85000, 0, 135000, repeat('3',64)),
  ('demo_snap_enterprise_1', 'demo_acc_enterprise', 'CNY', 'credit', 'demo_tx_payment', 1, NULL, NULL, 0, 0, 85000, 85000, repeat('4',64)),
  ('demo_snap_cash_3', 'demo_acc_cash', 'CNY', 'debit', 'demo_tx_refund', 3, 'demo_snap_cash_2', 2, 135000, 20000, 0, 155000, repeat('5',64)),
  ('demo_snap_verified_2', 'demo_acc_verified', 'CNY', 'credit', 'demo_tx_refund', 2, 'demo_snap_verified_1', 1, 50000, 0, 20000, 70000, repeat('6',64)),
  ('demo_snap_cash_4', 'demo_acc_cash', 'CNY', 'debit', 'demo_tx_gift', 4, 'demo_snap_cash_3', 3, 155000, 3000, 0, 158000, repeat('7',64)),
  ('demo_snap_verified_3', 'demo_acc_verified', 'CNY', 'credit', 'demo_tx_gift', 3, 'demo_snap_verified_2', 2, 70000, 0, 3000, 73000, repeat('8',64))
ON CONFLICT ("snapshot_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_adjustment_request
  ("adjustment_no", "customer_no", "invoice_no", "currency_code", "adjustment_type_code",
   "amount_minor", "reason_code", "evidence_digest", "idempotency_digest", "request_digest",
   approval_required, "status_code", "requested_by", created_at, updated_at)
VALUES
  ('demo_adjust_pending', 'CUS_10000000000000000000000000000001', 'demo_invoice_pending', 'CNY', 'invoice_waiver', 1200, 'service_compensation', repeat('1',64), repeat('2',64), repeat('3',64), true, 'pending_review', 1, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
  ('demo_adjust_approved', 'CUS_10000000000000000000000000000006', 'demo_invoice_bad_debt', 'CNY', 'invoice_waiver', 2000, 'contract_discount', repeat('4',64), repeat('5',64), repeat('6',64), true, 'approved', 1, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '10 hours'),
  ('demo_adjust_rejected', 'CUS_10000000000000000000000000000003', 'demo_invoice_overdue', 'CNY', 'writeoff', 5000, 'evidence_insufficient', repeat('7',64), repeat('8',64), repeat('9',64), true, 'rejected', 1, CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
  ('demo_adjust_cancelled', 'CUS_10000000000000000000000000000004', 'demo_invoice_cancelled', 'CNY', 'invoice_waiver', 900, 'customer_cancelled', repeat('a',64), repeat('b',64), repeat('c',64), true, 'cancelled', 1, CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '4 days')
ON CONFLICT ("adjustment_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_adjustment_approval
  ("approval_no", "adjustment_no", "requester_id_snapshot", "reviewer_id",
   "decision_code", "comment_digest", "evidence_digest", decided_at)
VALUES
  ('demo_approval_approved', 'demo_adjust_approved', 1, 2, 'approved', repeat('1',64), repeat('2',64), CURRENT_TIMESTAMP - INTERVAL '10 hours'),
  ('demo_approval_rejected', 'demo_adjust_rejected', 1, 2, 'rejected', repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP - INTERVAL '2 days')
ON CONFLICT ("approval_no") DO NOTHING;

INSERT INTO plugin_idc_core_sales_agent_level
  ("level_no", "level_code", "name_zh_cn", "name_en_us", "sort_order", "status_code")
VALUES
  ('demo_agent_level_standard', 'standard', '标准代理', 'Standard Agent', 10, 'active'),
  ('demo_agent_level_gold', 'gold', '金牌代理', 'Gold Agent', 20, 'active')
ON CONFLICT ("level_no") DO NOTHING;

INSERT INTO plugin_idc_core_sales_agent_application
  ("application_no", "customer_no", "requested_level_no", "status_code", "application_digest",
   submitted_at, "reviewed_by", "review_reason_code", reviewed_at)
VALUES
  ('demo_agent_app_pending', 'CUS_10000000000000000000000000000001', 'demo_agent_level_standard', 'submitted', repeat('1',64), CURRENT_TIMESTAMP - INTERVAL '1 day', NULL, NULL, NULL),
  ('demo_agent_app_approved', 'CUS_10000000000000000000000000000005', 'demo_agent_level_gold', 'approved', repeat('2',64), CURRENT_TIMESTAMP - INTERVAL '30 days', 1, 'qualified', CURRENT_TIMESTAMP - INTERVAL '29 days'),
  ('demo_agent_app_rejected', 'CUS_10000000000000000000000000000003', 'demo_agent_level_standard', 'rejected', repeat('3',64), CURRENT_TIMESTAMP - INTERVAL '7 days', 1, 'risk_review_failed', CURRENT_TIMESTAMP - INTERVAL '6 days')
ON CONFLICT ("application_no") DO NOTHING;

INSERT INTO plugin_idc_core_sales_commission_account
  ("account_no", "beneficiary_type_code", "beneficiary_reference", "currency_code",
   credited_amount_minor, pending_amount_minor, available_amount_minor,
   "frozen_amount_minor", "paid_amount_minor", "reversed_amount_minor", "status_code")
VALUES
  ('demo_commission_active', 'agent', 'demo_agent_001', 'CNY', 120000, 15000, 45000, 10000, 45000, 5000, 'active'),
  ('demo_commission_frozen', 'affiliate', 'demo_affiliate_001', 'CNY', 68000, 8000, 20000, 30000, 10000, 0, 'frozen'),
  ('demo_commission_closed', 'sales', 'demo_sales_001', 'CNY', 36000, 0, 0, 0, 34000, 2000, 'closed')
ON CONFLICT ("account_no") DO NOTHING;

-- Attach a compact acceptance dataset to the first real local customer, when one exists.
INSERT INTO plugin_idc_core_commerce_quote_snapshot
  ("quote_no", "customer_no", "customer_subject_version", "source_code", "currency_code",
   "item_count", "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "input_digest", "pricing_rule_digest", "eligibility_digest",
   "inventory_digest", "content_digest", issued_at, expires_at, "created_by")
SELECT values_row."quote_no", customer."customer_no", customer."version", 'customer', 'CNY',
       1, values_row."amount_minor", 0, values_row."amount_minor", 0,
       values_row."amount_minor", values_row."input_digest", values_row.pricing_digest,
       values_row."eligibility_digest", values_row."inventory_digest",
       values_row."content_digest", values_row.issued_at, values_row.expires_at, 1
FROM (
  SELECT "customer_no", "version"
  FROM plugin_idc_core_customer_subject
  WHERE "customer_no" NOT IN (
    'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
    'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
    'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
  )
  ORDER BY "id"
  LIMIT 1
) customer
CROSS JOIN (
  VALUES
    ('QTE_10000000000000000000000000000001'::varchar, 12900::bigint, repeat('1',64)::bpchar, repeat('2',64)::bpchar, repeat('3',64)::bpchar, repeat('4',64)::bpchar, repeat('5',64)::bpchar, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP + INTERVAL '22 hours'),
    ('QTE_10000000000000000000000000000002', 46800, repeat('6',64), repeat('7',64), repeat('8',64), repeat('9',64), repeat('a',64), CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP + INTERVAL '1 day'),
    ('QTE_10000000000000000000000000000003', 29900, repeat('b',64), repeat('c',64), repeat('d',64), repeat('e',64), repeat('f',64), CURRENT_TIMESTAMP - INTERVAL '40 days', CURRENT_TIMESTAMP + INTERVAL '1 day')
) values_row("quote_no", "amount_minor", "input_digest", pricing_digest,
             "eligibility_digest", "inventory_digest", "content_digest", issued_at, expires_at)
ON CONFLICT ("quote_no") DO NOTHING;

INSERT INTO plugin_idc_core_commerce_order
  ("order_no", "customer_no", "quote_no", "currency_code", "source_code", "status_code",
   "risk_decision_code", "item_count", "subtotal_minor", "discount_minor",
   "net_amount_minor", "tax_amount_minor", "total_amount_minor",
   "checkout_idempotency_digest", "checkout_request_digest", payment_due_at,
   paid_at, completed_at, "created_by", created_at, updated_at)
SELECT values_row."order_no", customer."customer_no", values_row."quote_no", 'CNY',
       'portal', values_row."status_code", 'approved', 1, values_row."amount_minor",
       0, values_row."amount_minor", 0, values_row."amount_minor",
       values_row."idempotency_digest", values_row."request_digest",
       values_row.payment_due_at, values_row.paid_at, values_row.completed_at,
       1, values_row.created_at, values_row.updated_at
FROM (
  SELECT "customer_no"
  FROM plugin_idc_core_customer_subject
  WHERE "customer_no" NOT IN (
    'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
    'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
    'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
  )
  ORDER BY "id"
  LIMIT 1
) customer
CROSS JOIN (
  VALUES
    ('ORD_10000000000000000000000000000001'::varchar, 'QTE_10000000000000000000000000000001'::varchar, 'pending_payment'::varchar, 12900::bigint, repeat('a',64)::bpchar, repeat('b',64)::bpchar, CURRENT_TIMESTAMP + INTERVAL '22 hours', NULL::timestamptz, NULL::timestamptz, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
    ('ORD_10000000000000000000000000000002', 'QTE_10000000000000000000000000000002', 'fulfilling', 46800, repeat('c',64), repeat('d',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '9 days', NULL, CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
    ('ORD_10000000000000000000000000000003', 'QTE_10000000000000000000000000000003', 'fulfilled', 29900, repeat('e',64), repeat('f',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '39 days', CURRENT_TIMESTAMP - INTERVAL '38 days', CURRENT_TIMESTAMP - INTERVAL '40 days', CURRENT_TIMESTAMP - INTERVAL '38 days')
) values_row("order_no", "quote_no", "status_code", "amount_minor",
             "idempotency_digest", "request_digest", payment_due_at, paid_at,
             completed_at, created_at, updated_at)
ON CONFLICT ("order_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_invoice
  ("invoice_no", "customer_no", "source_code", "currency_code", "status_code", "item_count",
   "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "source_snapshot_digest", "content_digest", payment_due_at,
   issued_at, cancelled_at, created_at, updated_at)
SELECT values_row."invoice_no", customer."customer_no", 'manual', 'CNY',
       values_row."status_code", 1, values_row."amount_minor", 0,
       values_row."amount_minor", 0, values_row."amount_minor",
       values_row."source_digest", values_row."content_digest",
       values_row.payment_due_at, values_row.issued_at,
       values_row.cancelled_at, values_row.created_at, values_row.updated_at
FROM (
  SELECT "customer_no"
  FROM plugin_idc_core_customer_subject
  WHERE "customer_no" NOT IN (
    'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
    'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
    'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
  )
  ORDER BY "id"
  LIMIT 1
) customer
CROSS JOIN (
  VALUES
    ('INV_10000000000000000000000000000001'::varchar, 'draft'::varchar, 9900::bigint, repeat('1',64)::bpchar, repeat('2',64)::bpchar, CURRENT_TIMESTAMP + INTERVAL '7 days', NULL::timestamptz, NULL::timestamptz, CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '1 day'),
    ('INV_10000000000000000000000000000002', 'pending_payment', 12900, repeat('3',64), repeat('4',64), CURRENT_TIMESTAMP + INTERVAL '2 days', CURRENT_TIMESTAMP - INTERVAL '2 hours', NULL, CURRENT_TIMESTAMP - INTERVAL '3 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
    ('INV_10000000000000000000000000000003', 'overdue', 29900, repeat('5',64), repeat('6',64), CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '12 days', NULL, CURRENT_TIMESTAMP - INTERVAL '13 days', CURRENT_TIMESTAMP - INTERVAL '5 days'),
    ('INV_10000000000000000000000000000004', 'cancelled', 5900, repeat('7',64), repeat('8',64), CURRENT_TIMESTAMP + INTERVAL '1 day', CURRENT_TIMESTAMP - INTERVAL '4 days', CURRENT_TIMESTAMP - INTERVAL '3 days', CURRENT_TIMESTAMP - INTERVAL '5 days', CURRENT_TIMESTAMP - INTERVAL '3 days')
) values_row("invoice_no", "status_code", "amount_minor", "source_digest", "content_digest",
             payment_due_at, issued_at, cancelled_at, created_at, updated_at)
ON CONFLICT ("invoice_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_invoice_item
  ("invoice_item_no", "invoice_no", "customer_no", "currency_code", "line_number",
   "item_type_code", "source_reference", "description_zh_cn", "description_en_us",
   "subtotal_minor", "discount_minor", "net_amount_minor", "tax_amount_minor",
   "total_amount_minor", "refundable_cap_minor", "content_digest")
SELECT 'IVI_' || md5(invoice."invoice_no"), invoice."invoice_no",
       invoice."customer_no", invoice."currency_code", 1, 'manual',
       invoice."invoice_no",
       '本地验收演示服务费', 'Local acceptance demo service',
       invoice."subtotal_minor", invoice."discount_minor", invoice."net_amount_minor",
       invoice."tax_amount_minor", invoice."total_amount_minor",
       invoice."total_amount_minor", repeat('9',64)
FROM plugin_idc_core_billing_invoice invoice
WHERE invoice."invoice_no" IN (
  'INV_10000000000000000000000000000001', 'INV_10000000000000000000000000000002',
  'INV_10000000000000000000000000000003', 'INV_10000000000000000000000000000004'
)
ON CONFLICT ("invoice_item_no") DO NOTHING;

INSERT INTO plugin_idc_core_catalog_resource_pool_binding
  ("binding_no", "version_no", "pool_no", region_code, line_code, "priority",
   weight, new_order_enabled, "status_code")
VALUES
  ('rpb_10000000000000000000000000000021', 'ver_00000000000000000000000000000021', 'demo_pool_primary', 'cn-east-demo', 'bgp-demo', 10, 100, true, 'active')
ON CONFLICT ("binding_no") DO NOTHING;

INSERT INTO plugin_idc_core_service_instance
  ("service_no", "customer_no", "source_code", "product_no", "version_no",
   "billing_rule_no", "configuration_snapshot_no", "product_type_code",
   "currency_code", "binding_no", "channel_no", "pool_no", "entitlement_status_code",
   "runtime_status_code", "provider_task_status_code", "renewal_mode_code",
   "period_unit_code", "period_count", entitlement_started_at,
   entitlement_expires_at, customer_alias, customer_remark,
   ownership_started_at, created_at, updated_at)
SELECT values_row."service_no", customer."customer_no", 'manual',
       'prd_00000000000000000000000000000011', 'ver_00000000000000000000000000000021', 'bil_10000000000000000000000000000021',
       'CFG_10000000000000000000000000000021', 'cloud_server', 'CNY',
       'rpb_10000000000000000000000000000021', 'demo_resource_channel_degraded', 'demo_pool_primary', values_row.entitlement_status,
       values_row.runtime_status, values_row.task_status, 'unresolved',
       'month', 1, values_row.started_at, values_row.expires_at,
       values_row.alias, values_row."remark", values_row.ownership_started_at,
       values_row.created_at, values_row.updated_at
FROM (
  SELECT "customer_no"
  FROM plugin_idc_core_customer_subject
  WHERE "customer_no" NOT IN (
    'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
    'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
    'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
  )
  ORDER BY "id"
  LIMIT 1
) customer
CROSS JOIN (
  VALUES
    ('SVC_10000000000000000000000000000001'::varchar, 'pending_activation'::varchar, 'pending_provision'::varchar, 'pending'::varchar, NULL::timestamptz, NULL::timestamptz, '待交付服务器'::varchar, '演示：等待资源交付'::varchar, CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours', CURRENT_TIMESTAMP - INTERVAL '2 hours'),
    ('SVC_10000000000000000000000000000002', 'active', 'running', 'succeeded', CURRENT_TIMESTAMP - INTERVAL '90 days', CURRENT_TIMESTAMP + INTERVAL '275 days', '生产应用-A', '演示：运行中的云服务器', CURRENT_TIMESTAMP - INTERVAL '90 days', CURRENT_TIMESTAMP - INTERVAL '90 days', CURRENT_TIMESTAMP - INTERVAL '10 minutes'),
    ('SVC_10000000000000000000000000000003', 'active', 'stopped', 'succeeded', CURRENT_TIMESTAMP - INTERVAL '60 days', CURRENT_TIMESTAMP + INTERVAL '30 days', '测试环境-B', '演示：已关机但服务有效', CURRENT_TIMESTAMP - INTERVAL '60 days', CURRENT_TIMESTAMP - INTERVAL '60 days', CURRENT_TIMESTAMP - INTERVAL '1 day'),
    ('SVC_10000000000000000000000000000004', 'grace', 'suspended', 'manual_review', CURRENT_TIMESTAMP - INTERVAL '120 days', CURRENT_TIMESTAMP + INTERVAL '10 days', '财务复核-C', '演示：服务处于宽限期并已暂停', CURRENT_TIMESTAMP - INTERVAL '120 days', CURRENT_TIMESTAMP - INTERVAL '120 days', CURRENT_TIMESTAMP - INTERVAL '2 days'),
    ('SVC_10000000000000000000000000000005', 'expired', 'stopped', 'none', CURRENT_TIMESTAMP - INTERVAL '400 days', CURRENT_TIMESTAMP - INTERVAL '35 days', '历史服务器-D', '演示：服务已到期', CURRENT_TIMESTAMP - INTERVAL '400 days', CURRENT_TIMESTAMP - INTERVAL '400 days', CURRENT_TIMESTAMP - INTERVAL '35 days')
) values_row("service_no", entitlement_status, runtime_status, task_status,
             started_at, expires_at, alias, "remark", ownership_started_at,
             created_at, updated_at)
ON CONFLICT ("service_no") DO NOTHING;

-- Repair acceptance rows created by earlier revisions that did not yet carry
-- the resource capability route. This is deliberately limited to the fixed
-- demo identifiers and never rewrites real customer services.
UPDATE plugin_idc_core_service_instance
SET "binding_no" = 'rpb_10000000000000000000000000000021',
    "channel_no" = 'demo_resource_channel_degraded',
    "pool_no" = 'demo_pool_primary',
    updated_at = CURRENT_TIMESTAMP
WHERE "service_no" IN (
  'SVC_10000000000000000000000000000001',
  'SVC_10000000000000000000000000000002',
  'SVC_10000000000000000000000000000003',
  'SVC_10000000000000000000000000000004',
  'SVC_10000000000000000000000000000005'
)
AND (
  COALESCE("binding_no", '') = ''
  OR COALESCE("channel_no", '') = ''
  OR COALESCE("pool_no", '') = ''
);

INSERT INTO plugin_idc_core_billing_ledger_account
  ("account_no", "owner_type_code", "owner_reference", "customer_no",
   "account_type_code", "normal_balance_code", "currency_code", "status_code")
SELECT 'bac_' || substr(encode("sha256"(
         convert_to('customer', 'UTF8') || decode('00', 'hex') ||
         convert_to("customer_no", 'UTF8') || decode('00', 'hex') ||
         convert_to('customer_balance', 'UTF8') || decode('00', 'hex') ||
         convert_to('CNY', 'UTF8')
       ), 'hex'), 1, 32),
       'customer', "customer_no", "customer_no",
       'customer_balance', 'credit', 'CNY', 'active'
FROM plugin_idc_core_customer_subject
WHERE "customer_no" NOT IN (
  'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
  'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
  'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
)
ORDER BY "id"
LIMIT 1
ON CONFLICT DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_account
  ("account_no", "owner_type_code", "owner_reference", "customer_no",
   "account_type_code", "normal_balance_code", "currency_code", "status_code")
SELECT 'bac_' || substr(encode("sha256"(
         convert_to('merchant', 'UTF8') || decode('00', 'hex') ||
         convert_to('demo_current_merchant', 'UTF8') || decode('00', 'hex') ||
         convert_to('merchant_cash', 'UTF8') || decode('00', 'hex') ||
         convert_to('CNY', 'UTF8')
       ), 'hex'), 1, 32),
       'merchant', 'demo_current_merchant', NULL,
       'merchant_cash', 'debit', 'CNY', 'active'
WHERE EXISTS (
  SELECT 1 FROM plugin_idc_core_billing_ledger_account
  WHERE "owner_type_code" = 'customer'
    AND "account_type_code" = 'customer_balance'
    AND "currency_code" = 'CNY'
    AND "owner_reference" = (
      SELECT "customer_no" FROM plugin_idc_core_customer_subject
      WHERE "customer_no" NOT IN (
        'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
        'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
        'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
      ) ORDER BY "id" LIMIT 1
    )
)
ON CONFLICT DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_transaction
  ("transaction_no", "transaction_type_code", "currency_code", "source_type_code",
   "source_reference", "source_snapshot_digest", "entry_count",
   "debit_total_minor", "credit_total_minor", "content_digest", "posted_by",
   posted_at, created_at)
SELECT 'demo_tx_current_recharge_v2', 'recharge', 'CNY', 'demo_recharge',
       'demo-current-recharge-v2', repeat('a',64), 2, 28800, 28800,
       repeat('b',64), 1, CURRENT_TIMESTAMP - INTERVAL '3 days',
       CURRENT_TIMESTAMP - INTERVAL '3 days'
WHERE EXISTS (
  SELECT 1 FROM plugin_idc_core_billing_ledger_account
  WHERE "owner_type_code" = 'customer'
    AND "account_type_code" = 'customer_balance'
    AND "currency_code" = 'CNY'
    AND "owner_reference" = (
      SELECT "customer_no" FROM plugin_idc_core_customer_subject
      WHERE "customer_no" NOT IN (
        'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
        'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
        'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
      ) ORDER BY "id" LIMIT 1
    )
)
ON CONFLICT ("transaction_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_ledger_entry
  ("entry_no", "transaction_no", "currency_code", "line_number", "account_no",
   "direction_code", "entry_role_code", "amount_minor", "reason_code", "content_digest")
SELECT values_row."entry_no", 'demo_tx_current_recharge_v2', 'CNY',
       values_row."line_number",
       CASE
         WHEN values_row."account_no" = 'current_customer'
           THEN (SELECT "account_no"
                 FROM plugin_idc_core_billing_ledger_account
                 WHERE "owner_type_code" = 'customer'
                   AND "account_type_code" = 'customer_balance'
                   AND "currency_code" = 'CNY'
                   AND "owner_reference" = (
                     SELECT "customer_no" FROM plugin_idc_core_customer_subject
                     WHERE "customer_no" NOT IN (
                       'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
                       'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
                       'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
                     ) ORDER BY "id" LIMIT 1
                   )
                 LIMIT 1)
         WHEN values_row."account_no" = 'current_cash'
           THEN (SELECT "account_no"
                 FROM plugin_idc_core_billing_ledger_account
                 WHERE "owner_type_code" = 'merchant'
                   AND "owner_reference" = 'demo_current_merchant'
                   AND "account_type_code" = 'merchant_cash'
                   AND "currency_code" = 'CNY'
                 LIMIT 1)
       END,
       values_row."direction_code",
       values_row."entry_role_code", 28800, 'demo_recharge', values_row."content_digest"
FROM (
  VALUES
    ('demo_entry_current_cash_v2'::varchar, 1::int, 'current_cash'::varchar, 'debit'::varchar, 'cash'::varchar, repeat('c',64)::bpchar),
    ('demo_entry_current_customer_v2'::varchar, 2::int, 'current_customer'::varchar, 'credit'::varchar, 'balance'::varchar, repeat('d',64)::bpchar)
) values_row("entry_no", "line_number", "account_no", "direction_code", "entry_role_code", "content_digest")
WHERE EXISTS (
  SELECT 1 FROM plugin_idc_core_billing_ledger_transaction
  WHERE "transaction_no" = 'demo_tx_current_recharge_v2'
)
ON CONFLICT ("entry_no") DO NOTHING;

INSERT INTO plugin_idc_core_billing_balance_snapshot
  ("snapshot_no", "account_no", "currency_code", "normal_balance_code",
   "transaction_no", "sequence_no", "previous_snapshot_no", "previous_sequence_no",
   "opening_balance_minor", "debit_amount_minor", "credit_amount_minor",
   "closing_balance_minor", "content_digest")
SELECT values_row."snapshot_no",
       CASE
         WHEN values_row."account_no" = 'current_customer'
           THEN (SELECT "account_no"
                 FROM plugin_idc_core_billing_ledger_account
                 WHERE "owner_type_code" = 'customer'
                   AND "account_type_code" = 'customer_balance'
                   AND "currency_code" = 'CNY'
                   AND "owner_reference" = (
                     SELECT "customer_no" FROM plugin_idc_core_customer_subject
                     WHERE "customer_no" NOT IN (
                       'CUS_10000000000000000000000000000001', 'CUS_10000000000000000000000000000002',
                       'CUS_10000000000000000000000000000003', 'CUS_10000000000000000000000000000004',
                       'CUS_10000000000000000000000000000005', 'CUS_10000000000000000000000000000006'
                     ) ORDER BY "id" LIMIT 1
                   )
                 LIMIT 1)
         WHEN values_row."account_no" = 'current_cash'
           THEN (SELECT "account_no"
                 FROM plugin_idc_core_billing_ledger_account
                 WHERE "owner_type_code" = 'merchant'
                   AND "owner_reference" = 'demo_current_merchant'
                   AND "account_type_code" = 'merchant_cash'
                   AND "currency_code" = 'CNY'
                 LIMIT 1)
       END,
       'CNY',
       values_row."normal_balance_code", 'demo_tx_current_recharge_v2', 1,
       NULL, NULL, 0, values_row."debit_amount_minor",
       values_row."credit_amount_minor", 28800, values_row."content_digest"
FROM (
  VALUES
    ('demo_snap_current_cash_v2'::varchar, 'current_cash'::varchar, 'debit'::varchar, 28800::bigint, 0::bigint, repeat('e',64)::bpchar),
    ('demo_snap_current_customer_v2'::varchar, 'current_customer'::varchar, 'credit'::varchar, 0::bigint, 28800::bigint, repeat('f',64)::bpchar)
) values_row("snapshot_no", "account_no", "normal_balance_code",
             "debit_amount_minor", "credit_amount_minor", "content_digest")
WHERE EXISTS (
  SELECT 1 FROM plugin_idc_core_billing_ledger_transaction
  WHERE "transaction_no" = 'demo_tx_current_recharge_v2'
)
ON CONFLICT ("snapshot_no") DO NOTHING;
