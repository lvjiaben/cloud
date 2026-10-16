-- 121: Freeze exact same-system delivery/service mappings before activating
-- entitlements. Existing non-peer fulfillment keeps one entitlement per intent.
ALTER TABLE plugin_idc_core_product_fulfillment_task
 ADD COLUMN IF NOT EXISTS upstream_deliveries JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE plugin_idc_core_service_entitlement
 ADD COLUMN IF NOT EXISTS upstream_order_reference VARCHAR(128) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS upstream_delivery_no VARCHAR(40) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS upstream_service_no VARCHAR(40) NOT NULL DEFAULT '';
ALTER TABLE plugin_idc_core_service_entitlement DROP CONSTRAINT IF EXISTS uk_idc_service_entitlement_intent;
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_entitlement_upstream_service
 ON plugin_idc_core_service_entitlement(source_intent_no,upstream_service_no);
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_peer_service_mapping()
RETURNS TRIGGER AS $$
BEGIN
 IF TG_OP='UPDATE' AND (OLD.upstream_order_reference IS DISTINCT FROM NEW.upstream_order_reference OR
 OLD.upstream_delivery_no IS DISTINCT FROM NEW.upstream_delivery_no OR OLD.upstream_service_no IS DISTINCT FROM NEW.upstream_service_no) THEN
 RAISE EXCEPTION 'upstream service mapping is immutable' USING ERRCODE='23514';
 END IF;
 IF NEW.upstream_service_no<>'' AND (NEW.plugin_id<>'idc-peer' OR NEW.upstream_order_reference='' OR NEW.upstream_delivery_no='') THEN
 RAISE EXCEPTION 'upstream service mapping is incomplete' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_plugin_idc_core_peer_mapping ON plugin_idc_core_service_entitlement;
CREATE TRIGGER trg_plugin_idc_core_peer_mapping BEFORE INSERT OR UPDATE ON plugin_idc_core_service_entitlement
 FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_peer_service_mapping();

CREATE OR REPLACE FUNCTION plugin_idc_core_reconcile_product_delivery()
RETURNS TRIGGER AS $$
DECLARE
    target_order_item_no VARCHAR(40);
    target_delivery_no VARCHAR(40);
    target_status VARCHAR(32);
    intent_count INT;
    terminal_count INT;
    completed_count INT;
    incomplete_count INT;
    canceled_count INT;
BEGIN
    target_order_item_no := COALESCE(NEW."order_item_no", OLD."order_item_no");
    SELECT "delivery_no" INTO target_delivery_no
    FROM plugin_idc_core_product_delivery_record
    WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no
    FOR UPDATE;
    IF target_delivery_no IS NULL THEN
        RETURN NULL;
    END IF;

    SELECT count(*),
           count(*) FILTER (WHERE "status_code" IN ('completed','incomplete','canceled')),
           count(*) FILTER (WHERE "status_code"='completed'),
           count(*) FILTER (WHERE "status_code"='incomplete'),
           count(*) FILTER (WHERE "status_code"='canceled')
      INTO intent_count,terminal_count,completed_count,incomplete_count,canceled_count
    FROM plugin_idc_core_product_fulfillment_intent
    WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no;

    IF intent_count > 0 AND completed_count = intent_count THEN
        target_status := 'completed';
    ELSIF intent_count > 0 AND canceled_count = intent_count THEN
        target_status := 'canceled';
    ELSIF intent_count > 0 AND terminal_count = intent_count THEN
        target_status := 'incomplete';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='manual_pending') THEN
        target_status := 'manual_pending';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='processing') THEN
        target_status := 'processing';
    ELSIF EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_intent WHERE "tenant_id"=0 AND "order_item_no"=target_order_item_no AND "status_code"='review_pending') THEN
        target_status := 'review_pending';
    ELSE
        target_status := 'queued';
    END IF;

    UPDATE plugin_idc_core_product_delivery_record
    SET "status_code"=target_status,
        "completed_at"=CASE WHEN target_status IN ('completed','incomplete','canceled') THEN COALESCE("completed_at",CURRENT_TIMESTAMP) ELSE NULL END,
        "version"=CASE WHEN "status_code" IS DISTINCT FROM target_status THEN "version"+1 ELSE "version" END,
        "updated_at"=CASE WHEN "status_code" IS DISTINCT FROM target_status THEN CURRENT_TIMESTAMP ELSE "updated_at" END
    WHERE "delivery_no"=target_delivery_no;

    INSERT INTO plugin_idc_core_service_entitlement(
            "tenant_id","entitlement_no","delivery_no","source_intent_no","customer_no","order_no","order_item_no",
            "product_no","version_no","unit_sequence","configuration_snapshot_no","interface_instance_no",
            "plugin_id","plugin_release_version","contract_version","declaration_digest","status_code","started_at","version"
        )
        SELECT 0,'ent_'||substr(md5('entitlement:'||intent."intent_no"),1,32),target_delivery_no,intent."intent_no",
               intent."customer_no",intent."order_no",intent."order_item_no",intent."product_no",intent."version_no",
               intent."unit_sequence",intent."configuration_snapshot_no",intent."interface_instance_no",
               intent."plugin_id",intent."plugin_release_version",intent."contract_version",intent."declaration_digest",
               CASE intent."status_code" WHEN 'completed' THEN 'active' WHEN 'canceled' THEN 'cancelled' ELSE 'pending_activation' END,
               CURRENT_TIMESTAMP,1
        FROM plugin_idc_core_product_fulfillment_intent intent
        WHERE intent."tenant_id"=0 AND intent."order_item_no"=target_order_item_no
          AND intent."plugin_id" <> 'idc-peer'
          AND intent."status_code" IN ('processing','completed','canceled') AND intent."ongoing_management"
        ON CONFLICT("entitlement_no") DO UPDATE
        SET "status_code"=CASE EXCLUDED."status_code"
                WHEN 'active' THEN 'active'
                WHEN 'cancelled' THEN 'cancelled'
                ELSE plugin_idc_core_service_entitlement."status_code"
            END,
            "version"=CASE WHEN plugin_idc_core_service_entitlement."status_code" IS DISTINCT FROM EXCLUDED."status_code"
                           AND EXCLUDED."status_code" IN ('active','cancelled')
                           THEN plugin_idc_core_service_entitlement."version"+1 ELSE plugin_idc_core_service_entitlement."version" END,
            "updated_at"=CASE WHEN plugin_idc_core_service_entitlement."status_code" IS DISTINCT FROM EXCLUDED."status_code"
                              AND EXCLUDED."status_code" IN ('active','cancelled')
                              THEN CURRENT_TIMESTAMP ELSE plugin_idc_core_service_entitlement."updated_at" END;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
