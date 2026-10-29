-- Seed accessory renewal intents without changing existing administrator choices.
-- Monetary variables are frozen ISO currency major-unit strings; period_unit is
-- day/month/year and period_count is a decimal integer supplied by the owner.
INSERT INTO plugin_idc_notify_event_catalog
    (tenant_id,event_code,topic_code,object_type,required_variables,optional_variables,security_required)
VALUES
    (0,'billing.accessory_renewal_due','billing','billing','["invoice_no","amount","currency","tax_amount","period_unit","period_count"]'::jsonb,'[]'::jsonb,FALSE),
    (0,'billing.accessory_renewal_failed','billing','billing','["invoice_no"]'::jsonb,'[]'::jsonb,FALSE),
    (0,'billing.accessory_renewal_attention','billing','billing','["pending_count","invoice_no"]'::jsonb,'["locale_code"]'::jsonb,FALSE)
ON CONFLICT (tenant_id,event_code) DO NOTHING;

WITH audiences(event_code,recipient_kind) AS (
    VALUES ('billing.accessory_renewal_due','customer_contact'),
           ('billing.accessory_renewal_failed','customer_contact'),
           ('billing.accessory_renewal_attention','staff_user')
), channels(channel_code) AS (
    VALUES ('in_app'),('email'),('sms'),('wechat_official'),('signed_webhook')
)
INSERT INTO plugin_idc_notify_event_policy
    (tenant_id,event_code,recipient_kind,channel_code,enabled)
SELECT 0,event_code,recipient_kind,channel_code,
       channel_code='in_app' OR (recipient_kind='customer_contact' AND channel_code='email')
FROM audiences CROSS JOIN channels
ON CONFLICT (tenant_id,event_code,recipient_kind,channel_code) DO NOTHING;

WITH defaults(event_code,locale_code,subject_template,body_template) AS (
    VALUES
      ('billing.accessory_renewal_due','zh-CN','附属资源续费提醒：账单 {{invoice_no}}',
       '快照或备份续费账单 {{invoice_no}} 待付款，本次含税总金额为 {{amount}} {{currency}}，其中税额为 {{tax_amount}} {{currency}}。完整续费周期：{{period_count}}{{period_unit}}。本通知不是扣款成功凭证，请查看账单并及时付款。'),
      ('billing.accessory_renewal_due','en-US','Accessory renewal reminder: invoice {{invoice_no}}',
       'Snapshot or backup renewal invoice {{invoice_no}} awaits payment. The tax-inclusive total is {{amount}} {{currency}}, including tax of {{tax_amount}} {{currency}}. Full renewal period: {{period_count}} {{period_unit}}. This notice does not confirm a successful charge. Please review and pay the invoice.'),
      ('billing.accessory_renewal_failed','zh-CN','附属资源续费未付款：账单 {{invoice_no}}',
       '快照或备份续费账单 {{invoice_no}} 尚未付款，对象和数据仍保留，恢复等使用操作已暂停，您仍可查看或删除对象。请充值后再次余额付款，或直接支付账单；付款成功后自动恢复使用，并从付款成功当天重新计算完整周期，不追收暂停期间费用。'),
      ('billing.accessory_renewal_failed','en-US','Accessory renewal unpaid: invoice {{invoice_no}}',
       'Snapshot or backup renewal invoice {{invoice_no}} remains unpaid. The object and its data are retained; restore and other usage operations are paused. You can still view or delete the object. Top up and retry balance payment, or pay the invoice directly. Successful payment restores usage and starts a full new period on the payment date, without charging for the paused period.'),
      ('billing.accessory_renewal_attention','zh-CN','附属资源续费待处理',
       '有 {{pending_count}} 项附属资源续费待处理，最近账单号为 {{invoice_no}}。请在后台续费待办中处理。'),
      ('billing.accessory_renewal_attention','en-US','Accessory renewals need attention',
       '{{pending_count}} accessory renewals need attention. The latest invoice is {{invoice_no}}. Please handle them in the administration renewal worklist.')
), rows AS (
    SELECT defaults.*,channels.channel_code,
           'ntpl_accessory_' || replace(defaults.event_code,'billing.accessory_','') || '_' || channels.channel_code || '_' ||
           CASE locale_code WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS template_no
    FROM defaults CROSS JOIN (VALUES ('in_app'),('email')) channels(channel_code)
    WHERE defaults.event_code <> 'billing.accessory_renewal_attention' OR channels.channel_code='in_app'
)
INSERT INTO plugin_idc_notify_template_version
    (tenant_id,template_no,version,event_code,channel_code,locale_code,subject_template,body_template,
     provider_template_id,provider_sms_variable_order,provider_wechat_data_mapping,status_code,content_digest,created_by)
SELECT 0,template_no,1,event_code,channel_code,locale_code,subject_template,body_template,
       '', '[]'::jsonb,'{}'::jsonb,'active',
       encode(sha256(convert_to(event_code||channel_code||locale_code||subject_template||body_template,'UTF8')),'hex'),1
FROM rows
WHERE NOT EXISTS (
    SELECT 1 FROM plugin_idc_notify_template_version existing
    WHERE existing.tenant_id=0 AND existing.event_code=rows.event_code
      AND existing.channel_code=rows.channel_code AND existing.locale_code=rows.locale_code
)
ON CONFLICT (tenant_id,template_no,version) DO NOTHING;
