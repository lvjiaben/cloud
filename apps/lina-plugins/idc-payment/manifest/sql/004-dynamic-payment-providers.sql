-- 004: Extend the payment-channel provider boundary for the managed UnionPay plugin.
-- Existing channel rows remain unchanged; only the provider check constraint is replaced.

ALTER TABLE plugin_idc_payment_channel
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_payment_channel_provider;

ALTER TABLE plugin_idc_payment_channel
    ADD CONSTRAINT ck_plugin_idc_payment_channel_provider
    CHECK ("provider_code" IN ('alipay', 'wechatpay', 'epay', 'unionpay'));
