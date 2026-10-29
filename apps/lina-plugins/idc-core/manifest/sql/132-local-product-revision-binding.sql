-- Purpose: Preserve the current published binding in a same-product local editing
-- revision without admitting sales; normal binding and publication remain guarded.
-- 用途：允许原商品修改草稿保存完整历史绑定，不因接口停用阻断本地编辑。

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_fulfillment_binding()
RETURNS TRIGGER AS $$
DECLARE
    target_version plugin_idc_core_catalog_product_version%ROWTYPE;
    local_revision BOOLEAN := FALSE;
    target_interface plugin_idc_core_product_automation_interface_instance%ROWTYPE;
BEGIN
    SELECT *
      INTO target_version
      FROM plugin_idc_core_catalog_product_version
     WHERE "tenant_id" = NEW."tenant_id"
       AND "version_no" = NEW."version_no"
     FOR UPDATE;

    IF target_version.version_no IS NULL THEN
        RAISE EXCEPTION 'catalog product version is unavailable for fulfillment binding';
    END IF;
    IF target_version.status_code <> 'draft' THEN
        RAISE EXCEPTION 'only a draft product version can receive a fulfillment binding';
    END IF;

    SELECT *
      INTO target_interface
      FROM plugin_idc_core_product_automation_interface_instance
     WHERE "tenant_id" = NEW."tenant_id"
       AND "interface_instance_no" = NEW."interface_instance_no"
     FOR UPDATE;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'product automation interface instance is unavailable';
    END IF;
    IF target_interface."extension_point_code" <> 'product-fulfillment' THEN
        RAISE EXCEPTION 'product automation interface uses an incompatible extension point';
    END IF;
    -- A same-product editing revision can preserve the exact current published
    -- binding while its interface is stopped, renamed or its package unavailable.
    -- This is local draft storage only; publication/execution admission is separate.
    IF TG_OP = 'INSERT' AND target_version.change_summary = 'merchant_revision' THEN
        SELECT EXISTS (
            SELECT 1
              FROM plugin_idc_core_catalog_product product
              JOIN plugin_idc_core_catalog_product_version source
                ON source.version_no = product.current_version_no
               AND source.product_no = product.product_no
               AND source.tenant_id = product.tenant_id
               AND source.status_code = 'published'
              JOIN plugin_idc_core_catalog_product_version_fulfillment_binding historical
                ON historical.version_no = source.version_no
               AND historical.tenant_id = source.tenant_id
             WHERE product.tenant_id = NEW.tenant_id
               AND product.product_no = target_version.product_no
               AND target_version.version_number > source.version_number
               AND historical.interface_instance_no = NEW.interface_instance_no
               AND historical.interface_display_name_snapshot = NEW.interface_display_name_snapshot
               AND historical.plugin_id = NEW.plugin_id
               AND historical.extension_point_code = NEW.extension_point_code
               AND historical.plugin_release_version = NEW.plugin_release_version
               AND historical.contract_version = NEW.contract_version
               AND historical.authorized_methods = NEW.authorized_methods
               AND historical.release_digest = NEW.release_digest
               AND historical.declaration_digest = NEW.declaration_digest
        ) INTO local_revision;
    END IF;
    IF NOT local_revision THEN
        IF target_interface."new_business_state" <> 'accepting' THEN
            RAISE EXCEPTION 'product automation interface is not accepting new business';
        END IF;
        IF NEW."plugin_id" IS DISTINCT FROM target_interface."plugin_id"
           OR NEW."interface_display_name_snapshot" IS DISTINCT FROM target_interface."display_name"
           OR NEW."extension_point_code" IS DISTINCT FROM target_interface."extension_point_code"
           OR NEW."plugin_release_version" IS DISTINCT FROM target_interface."plugin_release_version"
           OR NEW."contract_version" IS DISTINCT FROM target_interface."contract_version"
           OR NEW."authorized_methods" IS DISTINCT FROM target_interface."authorized_methods"
           OR NEW."release_digest" IS DISTINCT FROM target_interface."release_digest"
           OR NEW."declaration_digest" IS DISTINCT FROM target_interface."declaration_digest" THEN
            RAISE EXCEPTION 'fulfillment binding snapshot does not match the selected interface instance';
        END IF;
    END IF;

    -- The first reference is an irreversible database fact. Keeping this in
    -- the binding trigger prevents direct SQL writers from bypassing the
    -- deletion guard after a later draft unbind.
    IF NOT target_interface."ever_referenced" THEN
        UPDATE plugin_idc_core_product_automation_interface_instance
           SET "ever_referenced" = TRUE,
               "version" = target_interface."version" + 1,
               "changed_by" = NEW."bound_by",
               "changed_at" = CURRENT_TIMESTAMP
         WHERE "tenant_id" = NEW."tenant_id"
           AND "interface_instance_no" = NEW."interface_instance_no"
           AND "version" = target_interface."version"
           AND "ever_referenced" = FALSE;
        IF NOT FOUND THEN
            RAISE EXCEPTION 'product automation interface reference history update conflicted';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

