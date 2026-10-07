-- Add the frozen, customer-safe product labels required by business-owner
-- projections. The durable event remains the only data path; owners do not
-- read the product core's tables.
ALTER TABLE plugin_idc_core_entitlement_created_outbox
    ADD COLUMN IF NOT EXISTS "product_name_zh_cn" varchar(256),
    ADD COLUMN IF NOT EXISTS "product_name_en_us" varchar(256);

UPDATE plugin_idc_core_entitlement_created_outbox event
SET "product_name_zh_cn"=intent."product_name_zh_cn",
    "product_name_en_us"=intent."product_name_en_us",
    "updated_at"=CURRENT_TIMESTAMP
FROM plugin_idc_core_product_fulfillment_intent intent
WHERE intent."tenant_id"=event."tenant_id"
  AND intent."intent_no"=event."intent_no"
  AND (event."product_name_zh_cn" IS NULL OR event."product_name_en_us" IS NULL);

DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM plugin_idc_core_entitlement_created_outbox
        WHERE "product_name_zh_cn" IS NULL OR "product_name_en_us" IS NULL
           OR btrim("product_name_zh_cn")='' OR btrim("product_name_en_us")=''
    ) THEN
        RAISE EXCEPTION 'entitlement-created product labels cannot be reconstructed; export the development evidence and rebuild the development database';
    END IF;
END $$;

ALTER TABLE plugin_idc_core_entitlement_created_outbox
    ALTER COLUMN "product_name_zh_cn" SET NOT NULL,
    ALTER COLUMN "product_name_en_us" SET NOT NULL;

CREATE OR REPLACE FUNCTION plugin_idc_core_enqueue_entitlement_created()
RETURNS TRIGGER AS $$ BEGIN
    INSERT INTO plugin_idc_core_entitlement_created_outbox(
        "tenant_id","event_no","entitlement_no","intent_no","customer_no","product_no","version_no",
        "product_name_zh_cn","product_name_en_us",
        "configuration_snapshot_no","interface_instance_no","plugin_id","plugin_release_version",
        "contract_version","declaration_digest","owner_id")
    SELECT NEW."tenant_id",'ece_'||substr(md5('entitlement-created:'||NEW."entitlement_no"),1,32),
        NEW."entitlement_no",NEW."source_intent_no",NEW."customer_no",NEW."product_no",NEW."version_no",
        intent."product_name_zh_cn",intent."product_name_en_us",
        NEW."configuration_snapshot_no",NEW."interface_instance_no",NEW."plugin_id",
        NEW."plugin_release_version",NEW."contract_version",NEW."declaration_digest",NEW."entitlement_owner_id"
    FROM plugin_idc_core_product_fulfillment_intent intent
    WHERE intent."tenant_id"=NEW."tenant_id" AND intent."intent_no"=NEW."source_intent_no"
    ON CONFLICT("entitlement_no") DO NOTHING;
    RETURN NEW;
END; $$ LANGUAGE plpgsql;
