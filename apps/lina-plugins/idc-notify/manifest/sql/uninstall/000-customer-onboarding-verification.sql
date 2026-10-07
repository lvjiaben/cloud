DELETE FROM plugin_idc_notify_template_version
WHERE "tenant_id"=0 AND "event_code"='account.customer_onboarding_verification_code';

DELETE FROM plugin_idc_notify_event_policy
WHERE "tenant_id"=0 AND "event_code"='account.customer_onboarding_verification_code';

DELETE FROM plugin_idc_notify_event_catalog
WHERE "tenant_id"=0 AND "event_code"='account.customer_onboarding_verification_code';
