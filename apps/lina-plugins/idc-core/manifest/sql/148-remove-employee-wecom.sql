-- Remove the retired channel from new writes without erasing historical consent evidence.
DELETE FROM sys_dict_data
WHERE tenant_id = 0 AND dict_type = 'idc_notification_channel' AND value = 'wechat_work';

-- Remove obsolete current settings; the immutable consent events remain available as evidence.
DELETE FROM plugin_idc_core_customer_contact_preference WHERE channel_code = 'wechat_work';
DELETE FROM plugin_idc_core_marketing_channel_consent WHERE channel_code = 'wechat_work';

ALTER TABLE plugin_idc_core_customer_contact_preference
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_core_contact_pref_channel;
ALTER TABLE plugin_idc_core_customer_contact_preference
    ADD CONSTRAINT ck_plugin_idc_core_contact_pref_channel
    CHECK (channel_code IN ('in_app', 'email', 'sms', 'wechat_official')) NOT VALID;

ALTER TABLE plugin_idc_core_marketing_channel_consent_event
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_consent_event_channel;
ALTER TABLE plugin_idc_core_marketing_channel_consent_event
    ADD CONSTRAINT ck_idc_marketing_consent_event_channel
    CHECK (channel_code IN ('in_app', 'email', 'sms', 'wechat_official')) NOT VALID;

ALTER TABLE plugin_idc_core_marketing_channel_consent
    DROP CONSTRAINT IF EXISTS ck_idc_marketing_consent_channel;
ALTER TABLE plugin_idc_core_marketing_channel_consent
    ADD CONSTRAINT ck_idc_marketing_consent_channel
    CHECK (channel_code IN ('in_app', 'email', 'sms', 'wechat_official')) NOT VALID;
