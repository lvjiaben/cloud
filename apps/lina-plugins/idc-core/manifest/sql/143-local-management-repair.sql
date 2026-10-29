-- 143: Requeue only an audited, local management binding. Existing business
-- commands, orders, resources and financial facts are not rewritten.
ALTER TABLE plugin_idc_core_product_fulfillment_command
 DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_command_action;
ALTER TABLE plugin_idc_core_product_fulfillment_command
 ADD CONSTRAINT ck_idc_product_fulfillment_command_action CHECK (
 action_code IN ('review_approve','review_reject','takeover','manual_conclude_completed',
 'manual_conclude_unable','recover_preparation','repair_management')
 );
ALTER TABLE plugin_idc_core_product_fulfillment_reason_option
 DROP CONSTRAINT IF EXISTS ck_idc_product_fulfillment_reason_action;
ALTER TABLE plugin_idc_core_product_fulfillment_reason_option
 ADD CONSTRAINT ck_idc_product_fulfillment_reason_action CHECK (
 action_code IN ('review_approve','review_reject','takeover','manual_conclude_completed',
 'manual_conclude_unable','recover_preparation','repair_management')
 );
INSERT INTO plugin_idc_core_product_fulfillment_reason_option
 (tenant_id,action_code,reason_code,label_zh_cn,label_en_us,sort_order,status_code)
 VALUES(0,'repair_management','management_link_repair','重新关联本地管理入口',
 'Retry local management linking',10,'active')
 ON CONFLICT(action_code,reason_code) DO NOTHING;
