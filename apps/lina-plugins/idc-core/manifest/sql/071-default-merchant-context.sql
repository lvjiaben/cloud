-- 071: Seed the required single-merchant runtime context.
--
-- Legal identity stays empty until the installation owner supplies verified
-- enterprise details. This row only makes the required local context
-- available so dependent pages can fail closed with a useful readiness state.

INSERT INTO plugin_idc_core_merchant_profile (
    "tenant_id",
    "merchant_no",
    "display_name",
    "legal_name",
    "country_code",
    "default_locale",
    "default_currency",
    "timezone",
    "enabled"
) VALUES (
    0,
    'merchant-default',
    'LinaPro.AI',
    '',
    'CN',
    'zh-CN',
    'CNY',
    'Asia/Shanghai',
    TRUE
)
ON CONFLICT ("tenant_id") DO NOTHING;
