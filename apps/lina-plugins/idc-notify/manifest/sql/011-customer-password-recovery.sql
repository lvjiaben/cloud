-- Plugin-owned cleanup state makes a completed password reset distinguishable
-- from a temporary session-revocation failure. No host identity storage is read.
ALTER TABLE plugin_idc_notify_login_contact_guard
    ADD COLUMN IF NOT EXISTS recovery_no VARCHAR(48) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS cleanup_pending BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS recovery_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS recovery_revision BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cleanup_token_digest VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_notify_contact_verification
    DROP CONSTRAINT IF EXISTS ck_idc_notify_contact_verification_purpose;
ALTER TABLE plugin_idc_notify_contact_verification
    ADD CONSTRAINT ck_idc_notify_contact_verification_purpose CHECK
    (purpose_code IN ('support.presales','customer.onboarding','account.contact_old','account.contact_new','account.recovery'));
INSERT INTO plugin_idc_notify_event_catalog
    (tenant_id,event_code,topic_code,object_type,required_variables,optional_variables,security_required,enabled,version)
VALUES (0,'account.password_recovery_code','security','account','["code","expires_minutes"]'::jsonb,'[]'::jsonb,TRUE,TRUE,1)
ON CONFLICT (tenant_id,event_code) DO NOTHING;
WITH defaults(locale_code,subject_template,body_template) AS (
 VALUES ('zh-CN','找回账户密码','您正在找回账户密码，验证码为 {{code}}，{{expires_minutes}} 分钟内有效。请勿将验证码告知他人。'),
        ('en-US','Reset your account password','Your password reset code is {{code}}. It expires in {{expires_minutes}} minutes. Do not share this code.')
), channels(channel_code) AS (VALUES ('email'),('sms')), rows AS (
 SELECT defaults.*,channels.channel_code,
 'ntpl_recovery_' || channels.channel_code || '_' || CASE locale_code WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS template_no
 FROM defaults CROSS JOIN channels
)
INSERT INTO plugin_idc_notify_template_version
 (tenant_id,template_no,version,event_code,channel_code,locale_code,subject_template,body_template,provider_template_id,provider_sms_variable_order,provider_wechat_data_mapping,status_code,content_digest,created_by,provider_code,provider_sms_data_mapping)
SELECT 0,template_no,1,'account.password_recovery_code',channel_code,locale_code,subject_template,body_template,'','[]'::jsonb,'{}'::jsonb,'active',
 encode(sha256(convert_to('account.password_recovery_code'||channel_code||locale_code||subject_template||body_template,'UTF8')),'hex'),1,CASE channel_code WHEN 'sms' THEN 'smsbao_http_v1' ELSE '' END,'{}'::jsonb
FROM rows ON CONFLICT (tenant_id,template_no,version) DO NOTHING;

-- Capture exact pre-reset session IDs before committing the password write.
-- Indexed denial remains effective even when host revocation temporarily fails;
-- later retries cannot match a newly issued session with the same timestamp.
CREATE TABLE IF NOT EXISTS plugin_idc_notify_revoked_login_session (
 tenant_id INTEGER NOT NULL DEFAULT 0 CHECK (tenant_id = 0),
 user_id INTEGER NOT NULL CHECK (user_id > 0),
 session_digest VARCHAR(64) NOT NULL CHECK (session_digest ~ '^[0-9a-f]{64}$'),
 session_id VARCHAR(256) NOT NULL,
 recovery_no VARCHAR(48) NOT NULL,
 cleanup_done BOOLEAN NOT NULL DEFAULT FALSE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 PRIMARY KEY(tenant_id,session_digest),
 UNIQUE(tenant_id,user_id,session_id)
);
CREATE INDEX IF NOT EXISTS idx_idc_notify_revoked_login_pending
 ON plugin_idc_notify_revoked_login_session(tenant_id,user_id,recovery_no,session_digest)
 WHERE cleanup_done=FALSE;
