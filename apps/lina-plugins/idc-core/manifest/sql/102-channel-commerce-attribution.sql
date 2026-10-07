-- Channel orders keep one core commerce order while separating the payer from
-- the service owner. Ordinary portal and staff orders need no attribution row.
-- Purpose: Stores payer and service-owner attribution for channel commerce orders.
-- 用途：保存渠道商业订单的付款方和服务归属方关系。
CREATE TABLE IF NOT EXISTS plugin_idc_core_channel_order_attribution (
    "tenant_id" INT NOT NULL DEFAULT 0,
    "order_no" VARCHAR(40) PRIMARY KEY,
    "channel_id" VARCHAR(80) NOT NULL,
    "external_customer_id" VARCHAR(160) NOT NULL DEFAULT '',
    "ownership_mode" VARCHAR(32) NOT NULL,
    "payer_customer_no" VARCHAR(40) NOT NULL,
    "service_owner_customer_no" VARCHAR(40) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_channel_order_attr_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_channel_order_attr_mode CHECK (
        "ownership_mode" IN ('channel_owned', 'delegated_customer')
        AND (("ownership_mode" = 'channel_owned'
              AND "payer_customer_no" = "service_owner_customer_no"
              AND "external_customer_id" = '')
             OR ("ownership_mode" = 'delegated_customer'
                 AND "payer_customer_no" <> "service_owner_customer_no"
                 AND btrim("external_customer_id") <> ''))
    ),
    CONSTRAINT fk_idc_channel_order_attr_order FOREIGN KEY ("order_no")
        REFERENCES plugin_idc_core_commerce_order("order_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_order_attr_payer FOREIGN KEY ("payer_customer_no")
        REFERENCES plugin_idc_core_customer_subject("customer_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_channel_order_attr_owner FOREIGN KEY ("service_owner_customer_no")
        REFERENCES plugin_idc_core_customer_subject("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_channel_order_attr_channel
    ON plugin_idc_core_channel_order_attribution("tenant_id", "channel_id", "created_at" DESC, "order_no");
CREATE INDEX IF NOT EXISTS idx_idc_channel_order_attr_owner
    ON plugin_idc_core_channel_order_attribution("tenant_id", "service_owner_customer_no", "created_at" DESC, "order_no");

ALTER TABLE plugin_idc_core_product_fulfillment_intent
    ADD COLUMN IF NOT EXISTS "payer_customer_no" VARCHAR(40);
UPDATE plugin_idc_core_product_fulfillment_intent
SET "payer_customer_no" = "customer_no"
WHERE "payer_customer_no" IS NULL;
ALTER TABLE plugin_idc_core_product_fulfillment_intent
    ALTER COLUMN "payer_customer_no" SET NOT NULL;
CREATE OR REPLACE FUNCTION plugin_idc_core_default_fulfillment_payer()
RETURNS TRIGGER AS $function$
BEGIN
    IF NEW."payer_customer_no" IS NULL OR btrim(NEW."payer_customer_no") = '' THEN
        NEW."payer_customer_no" := NEW."customer_no";
    END IF;
    RETURN NEW;
END
$function$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_idc_product_fulfillment_default_payer
    ON plugin_idc_core_product_fulfillment_intent;
CREATE TRIGGER trg_idc_product_fulfillment_default_payer
BEFORE INSERT ON plugin_idc_core_product_fulfillment_intent
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_default_fulfillment_payer();
ALTER TABLE plugin_idc_core_product_fulfillment_intent
    DROP CONSTRAINT IF EXISTS fk_idc_product_fulfillment_settlement_owner;
DO $constraint$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'plugin_idc_core_product_fulfillment_intent'::regclass
          AND conname = 'fk_idc_product_fulfillment_settlement_payer'
    ) THEN
        ALTER TABLE plugin_idc_core_product_fulfillment_intent
            ADD CONSTRAINT fk_idc_product_fulfillment_settlement_payer FOREIGN KEY (
                "settlement_no", "order_no", "invoice_no", "payer_customer_no", "currency_code"
            ) REFERENCES plugin_idc_core_financial_settlement(
                "settlement_no", "order_no", "invoice_no", "customer_no", "currency_code"
            ) ON DELETE RESTRICT;
    END IF;
END
$constraint$;

CREATE INDEX IF NOT EXISTS idx_idc_product_fulfillment_payer
    ON plugin_idc_core_product_fulfillment_intent("tenant_id", "payer_customer_no", "intent_no");
