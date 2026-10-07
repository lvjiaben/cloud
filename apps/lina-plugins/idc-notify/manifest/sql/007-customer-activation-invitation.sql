-- A pending customer may receive exactly one security-class activation link.
-- Email has a built-in body. SMS stays unavailable until the merchant creates
-- an active provider-specific template with its own provider template ID.

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id", "event_code", "topic_code", "object_type", required_variables,
     optional_variables, security_required, enabled, "version")
VALUES
    (0, 'customer.activation_invitation', 'security', 'account',
     '["activation_url","customer_display_name","expires_at"]'::jsonb,
     '[]'::jsonb, TRUE, TRUE, 1)
ON CONFLICT ("tenant_id", "event_code") DO UPDATE SET
    "topic_code" = EXCLUDED."topic_code",
    "object_type" = EXCLUDED."object_type",
    required_variables = EXCLUDED.required_variables,
    optional_variables = EXCLUDED.optional_variables,
    security_required = EXCLUDED.security_required,
    enabled = EXCLUDED.enabled;

WITH channels("channel_code", enabled) AS (
    VALUES ('email', TRUE), ('sms', TRUE)
)
INSERT INTO plugin_idc_notify_event_policy
    ("tenant_id", "event_code", "recipient_kind", "channel_code", enabled)
SELECT 0, 'customer.activation_invitation', 'customer_contact',
       "channel_code", enabled
FROM channels
ON CONFLICT ("tenant_id", "event_code", "recipient_kind", "channel_code") DO NOTHING;

WITH defaults("locale_code", "subject_template", "body_template") AS (
    VALUES
      ('zh-CN', '激活您的客户账号',
       '{{customer_display_name}}，请在 {{expires_at}} 前打开以下一次性链接完成登录账号绑定：{{activation_url}}'),
      ('en-US', 'Activate your customer account',
       '{{customer_display_name}}, use this single-use link before {{expires_at}} to bind your login: {{activation_url}}')
), rows AS (
    SELECT "locale_code", "subject_template", "body_template",
           'ntpl_customer_activation_invitation_email_' ||
             CASE "locale_code" WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS "template_no"
    FROM defaults
)
INSERT INTO plugin_idc_notify_template_version
    ("tenant_id", "template_no", "version", "event_code", "channel_code", "locale_code",
     "subject_template", "body_template", "provider_template_id",
     provider_sms_variable_order, provider_wechat_data_mapping, "status_code",
     "content_digest", "created_by")
SELECT 0, "template_no", 1, 'customer.activation_invitation', 'email', "locale_code",
       "subject_template", "body_template", '', '[]'::jsonb, '{}'::jsonb, 'active',
       encode("sha256"(convert_to('customer.activation_invitation' || 'email' || "locale_code" || "subject_template" || "body_template", 'UTF8')), 'hex'),
       1
FROM rows
WHERE NOT EXISTS (
    SELECT 1
    FROM plugin_idc_notify_template_version existing
    WHERE existing."tenant_id" = 0
      AND existing."event_code" = 'customer.activation_invitation'
      AND existing."channel_code" = 'email'
      AND existing."locale_code" = rows."locale_code"
      AND existing."status_code" = 'active'
)
ON CONFLICT ("tenant_id", "template_no", "version") DO NOTHING;
