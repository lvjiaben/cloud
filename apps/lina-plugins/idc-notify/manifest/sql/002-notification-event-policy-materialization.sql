-- Notify-owned event policy, immutable source-event materialization evidence,
-- and installation defaults for pure business notification intents.

-- Purpose: Persists business records for plugin_idc_notify_event_policy.
-- 用途：持久化 plugin_idc_notify_event_policy 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_event_policy (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "event_code" VARCHAR(96) NOT NULL,
    "recipient_kind" VARCHAR(32) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    "version" BIGINT NOT NULL DEFAULT 1 CHECK ("version" > 0),
    "updated_by" BIGINT CHECK ("updated_by" > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("recipient_kind" IN ('customer_contact','staff_user','verified_destination')),
    CHECK ("recipient_kind" = 'staff_user' OR "channel_code" <> 'wechat_work'),
    PRIMARY KEY ("tenant_id", "event_code", "recipient_kind", "channel_code"),
    FOREIGN KEY ("tenant_id", "event_code")
        REFERENCES plugin_idc_notify_event_catalog("tenant_id", "event_code"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code")
);

-- Purpose: Persists business records for plugin_idc_notify_event_materialization.
-- 用途：持久化 plugin_idc_notify_event_materialization 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_notify_event_materialization (
    "tenant_id" SMALLINT NOT NULL DEFAULT 0 CHECK ("tenant_id" = 0),
    "materialization_no" VARCHAR(80) NOT NULL,
    "source_plugin" VARCHAR(96) NOT NULL,
    "source_event_no" VARCHAR(80) NOT NULL,
    "event_code" VARCHAR(96) NOT NULL,
    "recipient_kind" VARCHAR(32) NOT NULL,
    "recipient_digest" CHAR(64) NOT NULL,
    "channel_code" VARCHAR(32) NOT NULL,
    "policy_version" BIGINT NOT NULL CHECK ("policy_version" > 0),
    "status_code" VARCHAR(24) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "task_no" VARCHAR(80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK ("recipient_kind" IN ('customer_contact','staff_user','verified_destination')),
    CHECK ("status_code" IN ('materialized','skipped','failed')),
    CHECK (length("recipient_digest") = 64),
    CHECK (("status_code" = 'materialized') = ("task_no" IS NOT NULL)),
    PRIMARY KEY ("tenant_id", "materialization_no"),
    UNIQUE ("tenant_id", "source_plugin", "source_event_no", "channel_code"),
    FOREIGN KEY ("tenant_id", "event_code")
        REFERENCES plugin_idc_notify_event_catalog("tenant_id", "event_code"),
    FOREIGN KEY ("tenant_id", "channel_code")
        REFERENCES plugin_idc_notify_channel("tenant_id", "channel_code"),
    FOREIGN KEY ("tenant_id", "task_no")
        REFERENCES plugin_idc_notify_delivery_task("tenant_id", "task_no")
);

CREATE INDEX IF NOT EXISTS plugin_idc_notify_event_materialization_source_idx
    ON plugin_idc_notify_event_materialization ("tenant_id", "source_plugin", "source_event_no");
CREATE INDEX IF NOT EXISTS plugin_idc_notify_event_materialization_status_idx
    ON plugin_idc_notify_event_materialization ("tenant_id", "status_code", created_at, "materialization_no");

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id", "event_code", "topic_code", "object_type", required_variables, optional_variables, security_required)
VALUES
    (0, 'support.ticket_created', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["status","message"]'::jsonb, FALSE),
    (0, 'support.ticket_unassigned', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["status","message"]'::jsonb, FALSE),
    (0, 'support.ticket_sla_near_due', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["deadline_at","message"]'::jsonb, FALSE),
    (0, 'support.ticket_sla_breached', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["deadline_at","message"]'::jsonb, FALSE),
    (0, 'support.ticket_escalated', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["status","message"]'::jsonb, FALSE),
    (0, 'support.ticket_rating_low', 'support', 'support_ticket', '["ticket_no","title","rating"]'::jsonb, '["message"]'::jsonb, FALSE),
    (0, 'support.ticket_rating_neutral', 'support', 'support_ticket', '["ticket_no","title","rating"]'::jsonb, '["message"]'::jsonb, FALSE),
    (0, 'support.ticket_public_updated', 'support', 'support_ticket', '["ticket_no","title"]'::jsonb, '["status","message"]'::jsonb, FALSE),
    (0, 'support.presales_access_link', 'support', 'support_ticket', '["ticket_no","access_url","expires_at"]'::jsonb, '["title"]'::jsonb, FALSE)
ON CONFLICT ("tenant_id", "event_code") DO NOTHING;

WITH event_audience("event_code", "recipient_kind") AS (
    VALUES
        ('account.password_reset', 'customer_contact'),
        ('support.ticket_updated', 'customer_contact'),
        ('support.ticket_created', 'customer_contact'),
        ('support.ticket_unassigned', 'staff_user'),
        ('support.ticket_sla_near_due', 'staff_user'),
        ('support.ticket_sla_breached', 'staff_user'),
        ('support.ticket_escalated', 'staff_user'),
        ('support.ticket_rating_low', 'staff_user'),
        ('support.ticket_rating_neutral', 'staff_user'),
        ('support.ticket_public_updated', 'customer_contact'),
        ('support.presales_access_link', 'verified_destination'),
        ('service.expiry_reminder', 'customer_contact'),
        ('billing.payment_result', 'customer_contact'),
        ('marketing.campaign', 'customer_contact')
), bounded_channels("channel_code") AS (
    VALUES ('in_app'), ('email'), ('sms'), ('wechat_official'), ('wechat_work'), ('signed_webhook')
)
INSERT INTO plugin_idc_notify_event_policy
    ("tenant_id", "event_code", "recipient_kind", "channel_code", enabled)
SELECT 0, audience."event_code", audience."recipient_kind", channel."channel_code",
       CASE
           WHEN audience."event_code" = 'marketing.campaign' THEN FALSE
           WHEN audience."recipient_kind" = 'verified_destination' THEN channel."channel_code" = 'email'
           ELSE channel."channel_code" IN ('in_app','email')
       END
FROM event_audience audience
CROSS JOIN bounded_channels channel
WHERE NOT (audience."recipient_kind" <> 'staff_user' AND channel."channel_code" = 'wechat_work')
ON CONFLICT ("tenant_id", "event_code", "recipient_kind", "channel_code") DO NOTHING;

WITH template_defaults("event_code", "locale_code", "subject_template", "body_template") AS (
    VALUES
        ('account.password_reset', 'zh-CN', '重置账户密码', '请使用此链接重置密码：{{reset_url}}。链接将在 {{expires_minutes}} 分钟后失效。'),
        ('account.password_reset', 'en-US', 'Reset your account password', 'Use this link to reset your password: {{reset_url}}. It expires in {{expires_minutes}} minutes.'),
        ('support.ticket_updated', 'zh-CN', '工单 {{ticket_no}} 已更新', '“{{title}}”有新的进展。'),
        ('support.ticket_updated', 'en-US', 'Ticket {{ticket_no}} was updated', '“{{title}}” has an update.'),
        ('support.ticket_created', 'zh-CN', '工单 {{ticket_no}} 已创建', '我们已收到“{{title}}”。'),
        ('support.ticket_created', 'en-US', 'Ticket {{ticket_no}} was created', 'We received “{{title}}”.'),
        ('support.ticket_unassigned', 'zh-CN', '工单 {{ticket_no}} 等待处理', '“{{title}}”目前尚未分派。'),
        ('support.ticket_unassigned', 'en-US', 'Ticket {{ticket_no}} is unassigned', '“{{title}}” is waiting for assignment.'),
        ('support.ticket_sla_near_due', 'zh-CN', '工单 {{ticket_no}} 即将超时', '请尽快处理“{{title}}”。'),
        ('support.ticket_sla_near_due', 'en-US', 'Ticket {{ticket_no}} is near its SLA deadline', 'Please handle “{{title}}” soon.'),
        ('support.ticket_sla_breached', 'zh-CN', '工单 {{ticket_no}} 已超时', '“{{title}}”已超过服务时限。'),
        ('support.ticket_sla_breached', 'en-US', 'Ticket {{ticket_no}} breached its SLA', '“{{title}}” is past its service deadline.'),
        ('support.ticket_escalated', 'zh-CN', '工单 {{ticket_no}} 已升级', '“{{title}}”已升级处理。'),
        ('support.ticket_escalated', 'en-US', 'Ticket {{ticket_no}} was escalated', '“{{title}}” was escalated.'),
        ('support.ticket_rating_low', 'zh-CN', '工单 {{ticket_no}} 收到低评分', '“{{title}}”收到 {{rating}} 星评价。'),
        ('support.ticket_rating_low', 'en-US', 'Ticket {{ticket_no}} received a low rating', '“{{title}}” received a {{rating}}-star rating.'),
        ('support.ticket_rating_neutral', 'zh-CN', '工单 {{ticket_no}} 收到一般评价', '“{{title}}”收到 {{rating}} 星评价。'),
        ('support.ticket_rating_neutral', 'en-US', 'Ticket {{ticket_no}} received a neutral rating', '“{{title}}” received a {{rating}}-star rating.'),
        ('support.ticket_public_updated', 'zh-CN', '工单 {{ticket_no}} 有新回复', '“{{title}}”有新的公开回复。'),
        ('support.ticket_public_updated', 'en-US', 'Ticket {{ticket_no}} has a new reply', '“{{title}}” has a new public reply.'),
        ('support.presales_access_link', 'zh-CN', '售前咨询 {{ticket_no}} 访问链接', '访问链接：{{access_url}}。有效至 {{expires_at}}。'),
        ('support.presales_access_link', 'en-US', 'Presales inquiry {{ticket_no}} access link', 'Access link: {{access_url}}. It expires at {{expires_at}}.'),
        ('service.expiry_reminder', 'zh-CN', '服务 {{service_no}} 即将到期', '服务将在 {{expires_at}} 到期。'),
        ('service.expiry_reminder', 'en-US', 'Service {{service_no}} is expiring', 'The service expires at {{expires_at}}.'),
        ('billing.payment_result', 'zh-CN', '账单 {{invoice_no}} 支付结果', '当前支付状态：{{status}}。'),
        ('billing.payment_result', 'en-US', 'Payment result for invoice {{invoice_no}}', 'Current payment status: {{status}}.'),
        ('marketing.campaign', 'zh-CN', '{{campaign_name}}', '您收到一条营销活动通知。'),
        ('marketing.campaign', 'en-US', '{{campaign_name}}', 'You received a marketing campaign notification.')
), template_rows AS (
    SELECT defaults."event_code", channels."channel_code", defaults."locale_code",
           defaults."subject_template", defaults."body_template",
           'ntpl_default_' || replace(defaults."event_code", '.', '_') || '_' || channels."channel_code" || '_' ||
             CASE defaults."locale_code" WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS "template_no"
    FROM template_defaults defaults
    CROSS JOIN (VALUES ('in_app'), ('email')) channels("channel_code")
)
INSERT INTO plugin_idc_notify_template_version
    ("tenant_id", "template_no", "version", "event_code", "channel_code", "locale_code",
     "subject_template", "body_template", "provider_template_id", provider_sms_variable_order,
     provider_wechat_data_mapping, "status_code", "content_digest", "created_by")
SELECT 0, "template_no", 1, "event_code", "channel_code", "locale_code",
       "subject_template", "body_template", '', '[]'::jsonb, '{}'::jsonb, 'active',
       encode("sha256"(convert_to("event_code" || "channel_code" || "locale_code" || "subject_template" || "body_template", 'UTF8')), 'hex'),
       1
FROM template_rows
WHERE NOT EXISTS (
    SELECT 1
    FROM plugin_idc_notify_template_version existing
    WHERE existing."tenant_id" = 0
      AND existing."event_code" = template_rows."event_code"
      AND existing."channel_code" = template_rows."channel_code"
      AND existing."locale_code" = template_rows."locale_code"
      AND existing."status_code" = 'active'
)
ON CONFLICT ("tenant_id", "template_no", "version") DO NOTHING;
