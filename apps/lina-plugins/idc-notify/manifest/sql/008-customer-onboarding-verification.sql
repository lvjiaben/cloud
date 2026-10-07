DO $$
DECLARE item RECORD;
BEGIN
    FOR item IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'plugin_idc_notify_contact_verification'::regclass
          AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%purpose_code%'
    LOOP
        EXECUTE format('ALTER TABLE plugin_idc_notify_contact_verification DROP CONSTRAINT %I', item.conname);
    END LOOP;
END $$;

ALTER TABLE plugin_idc_notify_contact_verification
    ADD CONSTRAINT ck_idc_notify_contact_verification_purpose
    CHECK ("purpose_code" IN ('support.presales', 'customer.onboarding'));

INSERT INTO plugin_idc_notify_event_catalog
    ("tenant_id","event_code","topic_code","object_type",required_variables,optional_variables,security_required,enabled,"version")
VALUES
    (0,'account.customer_onboarding_verification_code','security','account','["code","expires_minutes"]'::jsonb,'[]'::jsonb,TRUE,TRUE,1)
ON CONFLICT ("tenant_id","event_code") DO UPDATE SET
    "topic_code"=EXCLUDED."topic_code",
    "object_type"=EXCLUDED."object_type",
    required_variables=EXCLUDED.required_variables,
    optional_variables=EXCLUDED.optional_variables,
    security_required=EXCLUDED.security_required,
    enabled=EXCLUDED.enabled;

WITH defaults("locale_code","subject_template","body_template") AS (
    VALUES
      ('zh-CN','客户注册验证码','您的验证码是 {{code}}，{{expires_minutes}} 分钟内有效。请勿转发给他人。'),
      ('en-US','Customer registration verification code','Your verification code is {{code}}. It expires in {{expires_minutes}} minutes. Do not share it.')
), rows AS (
    SELECT "locale_code","subject_template","body_template",
           'ntpl_customer_onboarding_email_' || CASE "locale_code" WHEN 'zh-CN' THEN 'zhcn' ELSE 'enus' END AS "template_no"
    FROM defaults
)
INSERT INTO plugin_idc_notify_template_version
    ("tenant_id","template_no","version","event_code","channel_code","locale_code","subject_template","body_template","provider_template_id",provider_sms_variable_order,provider_wechat_data_mapping,"status_code","content_digest","created_by")
SELECT 0,"template_no",1,'account.customer_onboarding_verification_code','email',"locale_code","subject_template","body_template",'','[]'::jsonb,'{}'::jsonb,'active',
       encode("sha256"(convert_to('account.customer_onboarding_verification_code' || 'email' || "locale_code" || "subject_template" || "body_template",'UTF8')),'hex'),1
FROM rows
WHERE NOT EXISTS (
    SELECT 1 FROM plugin_idc_notify_template_version existing
    WHERE existing."tenant_id"=0
      AND existing."event_code"='account.customer_onboarding_verification_code'
      AND existing."channel_code"='email'
      AND existing."locale_code"=rows."locale_code"
      AND existing."status_code"='active'
)
ON CONFLICT ("tenant_id","template_no","version") DO NOTHING;
