-- Serialize plugin-protected profile writes without accessing host identity tables.
CREATE TABLE IF NOT EXISTS plugin_idc_notify_login_contact_guard (
    tenant_id INTEGER NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
    user_id INTEGER NOT NULL CHECK (user_id > 0),
    revision BIGINT NOT NULL DEFAULT 0 CHECK (revision >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, user_id)
);

-- Account cooldown and active-proof invalidation use the account digest, not
-- the destination-based index used by public onboarding and presales flows.
CREATE INDEX IF NOT EXISTS idx_idc_notify_contact_account_retry
    ON plugin_idc_notify_contact_verification
    (tenant_id,ticket_digest,purpose_code,retry_after_at DESC);
CREATE INDEX IF NOT EXISTS idx_idc_notify_contact_account_active
    ON plugin_idc_notify_contact_verification
    (tenant_id,ticket_digest,purpose_code,browser_digest,status_code)
    WHERE status_code IN ('prepared','sent','result_unknown','verified');

ALTER TABLE plugin_idc_notify_contact_verification
    DROP CONSTRAINT IF EXISTS ck_idc_notify_contact_verification_purpose;
ALTER TABLE plugin_idc_notify_contact_verification
    ADD CONSTRAINT ck_idc_notify_contact_verification_purpose
    CHECK (purpose_code IN ('support.presales','customer.onboarding','account.contact_old','account.contact_new'));

INSERT INTO plugin_idc_notify_event_catalog
    (tenant_id,event_code,topic_code,object_type,required_variables,optional_variables,security_required,enabled,version)
VALUES (0,'account.login_contact_verification_code','security','account','["code","expires_minutes"]'::jsonb,'[]'::jsonb,TRUE,TRUE,1)
ON CONFLICT (tenant_id,event_code) DO NOTHING;

WITH defaults(locale_code,subject_template,body_template) AS (
 VALUES ('zh-CN','账户联系方式验证码','您正在更换账户联系方式，验证码为 {{code}}，{{expires_minutes}} 分钟内有效。请勿将验证码告知他人。'),
        ('en-US','Account contact verification','Your account contact verification code is {{code}}. It expires in {{expires_minutes}} minutes. Do not share this code.')
), channels(channel_code) AS (VALUES ('email'),('sms')),
rows AS (
 SELECT defaults.*,channels.channel_code,
 'ntpl_contact_change_' || channels.channel_code || '_' || CASE locale_code WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS template_no
 FROM defaults CROSS JOIN channels
)
INSERT INTO plugin_idc_notify_template_version
 (tenant_id,template_no,version,event_code,channel_code,locale_code,subject_template,body_template,provider_template_id,provider_sms_variable_order,provider_wechat_data_mapping,status_code,content_digest,created_by,provider_code,provider_sms_data_mapping)
SELECT 0,template_no,1,'account.login_contact_verification_code',channel_code,locale_code,subject_template,body_template,'','[]'::jsonb,'{}'::jsonb,'active',
 encode(sha256(convert_to('account.login_contact_verification_code'||channel_code||locale_code||subject_template||body_template,'UTF8')),'hex'),1,CASE channel_code WHEN 'sms' THEN 'smsbao_http_v1' ELSE '' END,'{}'::jsonb
FROM rows
ON CONFLICT (tenant_id,template_no,version) DO NOTHING;
