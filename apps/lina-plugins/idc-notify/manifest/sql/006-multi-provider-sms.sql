ALTER TABLE plugin_idc_notify_template_version
    ADD COLUMN IF NOT EXISTS "provider_code" VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE plugin_idc_notify_template_version
    ADD COLUMN IF NOT EXISTS provider_sms_data_mapping JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE plugin_idc_notify_template_version
SET "provider_code" = 'tencent_cloud_sms_v20210111'
WHERE "channel_code" = 'sms'
  AND "provider_code" = ''
  AND "provider_template_id" <> '';

ALTER TABLE plugin_idc_notify_template_version
    DROP CONSTRAINT IF EXISTS plugin_idc_notify_template_version_provider_sms_data_mapping_check;

ALTER TABLE plugin_idc_notify_template_version
    ADD CONSTRAINT plugin_idc_notify_template_version_provider_sms_data_mapping_check
    CHECK (jsonb_typeof(provider_sms_data_mapping) = 'object');

ALTER TABLE plugin_idc_notify_template_version
    DROP CONSTRAINT IF EXISTS plugin_idc_notify_template_version_sms_provider_check;

ALTER TABLE plugin_idc_notify_template_version
    ADD CONSTRAINT plugin_idc_notify_template_version_sms_provider_check
    CHECK (
        "channel_code" <> 'sms' OR "provider_code" IN (
            'aliyun_sms_v20170525',
            'smsbao_http_v1',
            'tencent_cloud_sms_v20210111'
        )
    );
