DROP TABLE IF EXISTS plugin_idc_payment_reconciliation_supplement;
DROP TABLE IF EXISTS plugin_idc_payment_reconciliation_channel_health;
DROP TABLE IF EXISTS plugin_idc_payment_reconciliation_source;

DROP INDEX IF EXISTS idx_plugin_idc_payment_reconciliation_batch_list;
DROP INDEX IF EXISTS uk_plugin_idc_payment_reconciliation_daily;

ALTER TABLE IF EXISTS plugin_idc_payment_reconciliation_line
    DROP COLUMN IF EXISTS "version",
    DROP COLUMN IF EXISTS channel_status_code,
    DROP COLUMN IF EXISTS local_status_code,
    DROP COLUMN IF EXISTS "customer_ref_digest",
    DROP COLUMN IF EXISTS "record_type_code";

ALTER TABLE IF EXISTS plugin_idc_payment_reconciliation_batch
    DROP COLUMN IF EXISTS "version",
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS "failure_code",
    DROP COLUMN IF EXISTS result_unknown,
    DROP COLUMN IF EXISTS refund_count,
    DROP COLUMN IF EXISTS payment_count,
    DROP COLUMN IF EXISTS "capability_code",
    DROP COLUMN IF EXISTS "source_type_code",
    DROP COLUMN IF EXISTS merchant_timezone,
    DROP COLUMN IF EXISTS daily_scope_key;
