-- 095: Retire the invalid generic batch-renewal storage. Renewal price,
-- eligibility, term, and expiry belong to the plugin that owns the frozen
-- long-lived entitlement contract, not to the generic financial core.
-- 095：退役错误的通用批量续费存储。续费价格、资格、周期和到期时间归属于
-- 冻结长期权益契约的业务插件，不属于通用财务核心。

DO $generic_batch_renewal_retirement$
DECLARE
    has_rows BOOLEAN;
BEGIN
    IF to_regclass('plugin_idc_core_financial_batch_renewal_item') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM plugin_idc_core_financial_batch_renewal_item LIMIT 1)'
            INTO has_rows;
        IF has_rows THEN
            RAISE EXCEPTION 'generic batch-renewal evidence exists; export the development evidence and rebuild the development database before retiring the invalid model';
        END IF;
    END IF;
    IF to_regclass('plugin_idc_core_financial_batch_renewal') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM plugin_idc_core_financial_batch_renewal LIMIT 1)'
            INTO has_rows;
        IF has_rows THEN
            RAISE EXCEPTION 'generic batch-renewal evidence exists; export the development evidence and rebuild the development database before retiring the invalid model';
        END IF;
    END IF;
END;
$generic_batch_renewal_retirement$;

DROP TABLE IF EXISTS plugin_idc_core_financial_batch_renewal_item;
DROP TABLE IF EXISTS plugin_idc_core_financial_batch_renewal;
