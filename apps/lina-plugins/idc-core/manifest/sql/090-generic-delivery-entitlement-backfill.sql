-- 090: Preserve unambiguous development data created before generic delivery
-- and entitlement records became authoritative. This migration never invents
-- a plugin contract: legacy order services without an exact frozen generic
-- fulfillment intent stop the upgrade and must be exported before rebuilding
-- the development database.

DO $guard$
DECLARE
    unmapped_count BIGINT;
BEGIN
    SELECT count(*) INTO unmapped_count
    FROM plugin_idc_core_service_instance legacy_service
    WHERE legacy_service."tenant_id" = 0
      AND legacy_service."source_code" = 'order'
      AND legacy_service."entitlement_status_code" NOT IN ('pending_activation', 'cancelled')
      AND NOT EXISTS (
          SELECT 1
          FROM plugin_idc_core_commerce_fulfillment_unit legacy_unit
          JOIN plugin_idc_core_product_fulfillment_intent intent
            ON intent."tenant_id" = legacy_unit."tenant_id"
           AND intent."order_item_no" = legacy_unit."order_item_no"
           AND intent."unit_sequence" = legacy_unit."unit_sequence"
          WHERE legacy_unit."tenant_id" = legacy_service."tenant_id"
            AND legacy_unit."fulfillment_no" = legacy_service."source_fulfillment_no"
            AND intent."order_no" = legacy_service."order_no"
            AND intent."customer_no" = legacy_service."customer_no"
            AND intent."product_no" = legacy_service."product_no"
            AND intent."version_no" = legacy_service."version_no"
            AND intent."status_code" = 'completed'
            AND intent."interface_instance_no" <> ''
            AND intent."plugin_id" <> ''
            AND intent."plugin_release_version" <> ''
            AND intent."contract_version" <> ''
            AND intent."declaration_digest" ~ '^[0-9a-f]{64}$'
      );
    IF unmapped_count > 0 THEN
        RAISE EXCEPTION USING
            ERRCODE = '23514',
            MESSAGE = format(
                'cannot safely migrate %s legacy order service(s): export required development data, then rebuild the development database',
                unmapped_count
            );
    END IF;
END;
$guard$;

-- A legacy server service proves that this exact completed unit needs ongoing
-- management. It does not prove that renewal is supported, so renewal remains
-- unavailable until a frozen plugin declaration says otherwise.
UPDATE plugin_idc_core_product_fulfillment_intent intent
SET "ongoing_management" = TRUE,
    "renewal_mode_code" = 'unavailable',
    "renewal_method" = '',
    "entitlement_owner_id" = 'idc-resource',
    "version" = intent."version" + 1,
    "updated_at" = GREATEST(intent."updated_at", CURRENT_TIMESTAMP)
FROM plugin_idc_core_commerce_fulfillment_unit legacy_unit
JOIN plugin_idc_core_service_instance legacy_service
  ON legacy_service."tenant_id" = legacy_unit."tenant_id"
 AND legacy_service."source_fulfillment_no" = legacy_unit."fulfillment_no"
WHERE intent."tenant_id" = legacy_unit."tenant_id"
  AND intent."order_item_no" = legacy_unit."order_item_no"
  AND intent."unit_sequence" = legacy_unit."unit_sequence"
  AND intent."order_no" = legacy_service."order_no"
  AND intent."customer_no" = legacy_service."customer_no"
  AND intent."product_no" = legacy_service."product_no"
  AND intent."version_no" = legacy_service."version_no"
  AND intent."status_code" = 'completed'
  AND legacy_service."source_code" = 'order'
  AND legacy_service."entitlement_status_code" NOT IN ('pending_activation', 'cancelled')
  AND NOT intent."ongoing_management";

DO $guard$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM plugin_idc_core_product_fulfillment_intent left_intent
        JOIN plugin_idc_core_product_fulfillment_intent right_intent
          ON right_intent."tenant_id" = left_intent."tenant_id"
         AND right_intent."order_item_no" = left_intent."order_item_no"
         AND right_intent."intent_no" > left_intent."intent_no"
        WHERE ROW(
            left_intent."order_no",left_intent."customer_no",left_intent."product_no",left_intent."version_no",
            left_intent."quantity",left_intent."strategy_code",left_intent."policy_digest",
            left_intent."configuration_snapshot_no",left_intent."interface_instance_no",left_intent."plugin_id",
            left_intent."plugin_release_version",left_intent."contract_version",left_intent."declaration_digest",
            left_intent."ongoing_management",left_intent."renewal_mode_code",left_intent."renewal_method",
            left_intent."entitlement_owner_id"
        ) IS DISTINCT FROM ROW(
            right_intent."order_no",right_intent."customer_no",right_intent."product_no",right_intent."version_no",
            right_intent."quantity",right_intent."strategy_code",right_intent."policy_digest",
            right_intent."configuration_snapshot_no",right_intent."interface_instance_no",right_intent."plugin_id",
            right_intent."plugin_release_version",right_intent."contract_version",right_intent."declaration_digest",
            right_intent."ongoing_management",right_intent."renewal_mode_code",right_intent."renewal_method",
            right_intent."entitlement_owner_id"
        )
    ) THEN
        RAISE EXCEPTION 'cannot safely backfill delivery: one order item has inconsistent frozen fulfillment evidence'
            USING ERRCODE = '23514';
    END IF;
END;
$guard$;

WITH ranked_intent AS (
    SELECT intent.*,
           row_number() OVER (PARTITION BY intent."tenant_id",intent."order_item_no" ORDER BY intent."unit_sequence",intent."intent_no") AS rank_no,
           count(*) OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS intent_count,
           count(*) FILTER (WHERE intent."status_code" = 'completed') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS completed_count,
           count(*) FILTER (WHERE intent."status_code" = 'incomplete') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS incomplete_count,
           count(*) FILTER (WHERE intent."status_code" = 'canceled') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS canceled_count,
           bool_or(intent."status_code" = 'manual_pending') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS has_manual_pending,
           bool_or(intent."status_code" = 'processing') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS has_processing,
           bool_or(intent."status_code" = 'review_pending') OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS has_review_pending,
           min(intent."financially_settled_at") OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS delivery_started_at,
           max(intent."updated_at") OVER (PARTITION BY intent."tenant_id",intent."order_item_no") AS delivery_completed_at
    FROM plugin_idc_core_product_fulfillment_intent intent
    WHERE intent."tenant_id" = 0
), delivery_source AS (
    SELECT ranked_intent.*,
           CASE
             WHEN completed_count = intent_count THEN 'completed'
             WHEN canceled_count = intent_count THEN 'canceled'
             WHEN completed_count + incomplete_count + canceled_count = intent_count THEN 'incomplete'
             WHEN has_manual_pending THEN 'manual_pending'
             WHEN has_processing THEN 'processing'
             WHEN has_review_pending THEN 'review_pending'
             ELSE 'queued'
           END AS "delivery_status"
    FROM ranked_intent
    WHERE rank_no = 1
)
INSERT INTO plugin_idc_core_product_delivery_record(
    "tenant_id","delivery_no","order_no","order_item_no","customer_no","product_no","version_no","quantity",
    "strategy_code","policy_digest","configuration_snapshot_no","interface_instance_no","plugin_id",
    "plugin_release_version","contract_version","declaration_digest","ongoing_management","status_code",
    "renewal_mode_code","renewal_method","entitlement_owner_id","started_at","completed_at","version","content_digest",
    "created_at","updated_at"
)
SELECT source."tenant_id",
       'del_' || substr(md5('delivery:' || source."order_item_no"),1,32),
       source."order_no",source."order_item_no",source."customer_no",source."product_no",source."version_no",source."quantity",
       source."strategy_code",source."policy_digest",source."configuration_snapshot_no",source."interface_instance_no",
       source."plugin_id",source."plugin_release_version",source."contract_version",source."declaration_digest",
       source."ongoing_management",source."delivery_status",source."renewal_mode_code",source."renewal_method",
       source."entitlement_owner_id",source.delivery_started_at,
       CASE WHEN source."delivery_status" IN ('completed','incomplete','canceled') THEN source.delivery_completed_at END,
       1,
       md5('delivery:' || source."order_item_no" || ':' || source."policy_digest") ||
       md5('delivery-evidence:' || source."order_item_no" || ':' || source."policy_digest"),
       source.delivery_started_at,source.delivery_completed_at
FROM delivery_source source
ON CONFLICT("order_item_no") DO NOTHING;

-- Insert the historical entitlements directly. The normal reconciliation
-- trigger only sees future intent status changes and therefore cannot perform
-- this upgrade backfill by itself.
INSERT INTO plugin_idc_core_service_entitlement(
    "tenant_id","entitlement_no","delivery_no","source_intent_no","customer_no","order_no","order_item_no",
    "product_no","version_no","unit_sequence","configuration_snapshot_no","interface_instance_no",
    "plugin_id","plugin_release_version","contract_version","declaration_digest","status_code","started_at","expires_at",
    "renewal_mode_code","renewal_method","entitlement_owner_id","currency_code","billing_rule_no",
    "charge_type_code","period_unit_code","period_count","version","created_at","updated_at"
)
SELECT intent."tenant_id",
       'ent_' || substr(md5('entitlement:' || intent."intent_no"),1,32),
       delivery."delivery_no",intent."intent_no",intent."customer_no",intent."order_no",intent."order_item_no",
       intent."product_no",intent."version_no",intent."unit_sequence",intent."configuration_snapshot_no",
       intent."interface_instance_no",intent."plugin_id",intent."plugin_release_version",intent."contract_version",
       intent."declaration_digest",
       CASE legacy_service."entitlement_status_code"
         WHEN 'expired' THEN 'expired'
         WHEN 'terminating' THEN 'terminated'
         WHEN 'terminated' THEN 'terminated'
         ELSE 'active'
       END,
       COALESCE(legacy_service."entitlement_started_at",intent."updated_at",CURRENT_TIMESTAMP),
       legacy_service."entitlement_expires_at",
       intent."renewal_mode_code",intent."renewal_method",intent."entitlement_owner_id",
       item."currency_code",item."billing_rule_no",item."charge_type_code",item."period_unit_code",item."period_count",
       1,
       COALESCE(legacy_service."created_at",intent."updated_at",CURRENT_TIMESTAMP),
       COALESCE(legacy_service."updated_at",intent."updated_at",CURRENT_TIMESTAMP)
FROM plugin_idc_core_product_fulfillment_intent intent
JOIN plugin_idc_core_product_delivery_record delivery
  ON delivery."tenant_id" = intent."tenant_id"
 AND delivery."order_item_no" = intent."order_item_no"
JOIN plugin_idc_core_commerce_order_item item
  ON item."tenant_id" = intent."tenant_id"
 AND item."order_item_no" = intent."order_item_no"
LEFT JOIN plugin_idc_core_commerce_fulfillment_unit legacy_unit
  ON legacy_unit."tenant_id" = intent."tenant_id"
 AND legacy_unit."order_item_no" = intent."order_item_no"
 AND legacy_unit."unit_sequence" = intent."unit_sequence"
LEFT JOIN plugin_idc_core_service_instance legacy_service
  ON legacy_service."tenant_id" = legacy_unit."tenant_id"
 AND legacy_service."source_fulfillment_no" = legacy_unit."fulfillment_no"
 AND legacy_service."source_code" = 'order'
WHERE intent."tenant_id" = 0
  AND intent."status_code" = 'completed'
  AND intent."ongoing_management"
ON CONFLICT("source_intent_no") DO NOTHING;

-- Preserve the old service dates/status where an entitlement was migrated.
UPDATE plugin_idc_core_service_entitlement entitlement
SET "status_code" = CASE legacy_service."entitlement_status_code"
        WHEN 'expired' THEN 'expired'
        WHEN 'terminating' THEN 'terminated'
        WHEN 'terminated' THEN 'terminated'
        ELSE 'active'
    END,
    "started_at" = COALESCE(legacy_service."entitlement_started_at", entitlement."started_at"),
    "expires_at" = legacy_service."entitlement_expires_at",
    "version" = entitlement."version" + 1,
    "updated_at" = GREATEST(entitlement."updated_at", legacy_service."updated_at")
FROM plugin_idc_core_product_fulfillment_intent intent
JOIN plugin_idc_core_commerce_fulfillment_unit legacy_unit
  ON legacy_unit."tenant_id" = intent."tenant_id"
 AND legacy_unit."order_item_no" = intent."order_item_no"
 AND legacy_unit."unit_sequence" = intent."unit_sequence"
JOIN plugin_idc_core_service_instance legacy_service
  ON legacy_service."tenant_id" = legacy_unit."tenant_id"
 AND legacy_service."source_fulfillment_no" = legacy_unit."fulfillment_no"
WHERE entitlement."tenant_id" = intent."tenant_id"
  AND entitlement."source_intent_no" = intent."intent_no"
  AND legacy_service."source_code" = 'order'
  AND legacy_service."entitlement_status_code" NOT IN ('pending_activation', 'cancelled')
  AND ROW(entitlement."status_code",entitlement."started_at",entitlement."expires_at") IS DISTINCT FROM ROW(
      CASE legacy_service."entitlement_status_code"
        WHEN 'expired' THEN 'expired'
        WHEN 'terminating' THEN 'terminated'
        WHEN 'terminated' THEN 'terminated'
        ELSE 'active'
      END,
      COALESCE(legacy_service."entitlement_started_at",entitlement."started_at"),
      legacy_service."entitlement_expires_at"
  );

-- The trigger covers newly inserted entitlements. This insert covers valid
-- entitlements that existed before the outbox table was introduced.
INSERT INTO plugin_idc_core_entitlement_created_outbox(
    "tenant_id","event_no","entitlement_no","intent_no","customer_no","product_no","version_no",
    "product_name_zh_cn","product_name_en_us",
    "configuration_snapshot_no","interface_instance_no","plugin_id","plugin_release_version",
    "contract_version","declaration_digest","owner_id"
)
SELECT entitlement."tenant_id",
       'ece_' || substr(md5('entitlement-created:' || entitlement."entitlement_no"),1,32),
       entitlement."entitlement_no",entitlement."source_intent_no",entitlement."customer_no",
       entitlement."product_no",entitlement."version_no",intent."product_name_zh_cn",intent."product_name_en_us",
       entitlement."configuration_snapshot_no",
       entitlement."interface_instance_no",entitlement."plugin_id",entitlement."plugin_release_version",
       entitlement."contract_version",entitlement."declaration_digest",entitlement."entitlement_owner_id"
FROM plugin_idc_core_service_entitlement entitlement
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent."tenant_id"=entitlement."tenant_id"
 AND intent."intent_no"=entitlement."source_intent_no"
WHERE entitlement."tenant_id" = 0
  AND entitlement."entitlement_owner_id" <> ''
ON CONFLICT("entitlement_no") DO NOTHING;
