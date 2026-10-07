DROP TABLE IF EXISTS plugin_idc_notify_security_code_verification;
DROP TABLE IF EXISTS plugin_idc_notify_security_code_purpose;
DELETE FROM plugin_idc_notify_template_version
WHERE "tenant_id" = 0 AND "event_code" = 'compliance.contract_signer_phone_verification';
DELETE FROM plugin_idc_notify_event_catalog
WHERE "tenant_id" = 0 AND "event_code" = 'compliance.contract_signer_phone_verification';
