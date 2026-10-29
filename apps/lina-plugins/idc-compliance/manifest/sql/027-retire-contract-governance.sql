-- Stop retired electronic-contract supplier capabilities without destroying historical evidence.
UPDATE plugin_idc_compliance_supplier_configuration
SET adapter_enabled = FALSE
WHERE provider_code = 'tencent_essbasic' AND adapter_enabled;
