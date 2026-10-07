-- 074: Establish generic product-automation interface instances and immutable
-- published-version bindings. Plugin-owned configuration schemas and values are
-- intentionally deferred to FB-52 0.10; fulfillment tasks are deferred to 0.11.

CREATE OR REPLACE FUNCTION plugin_idc_core_is_normalized_method_set(methods TEXT[])
RETURNS BOOLEAN AS $$
    SELECT methods IS NOT NULL
       AND NOT EXISTS (
            SELECT 1
              FROM unnest(methods) AS method_name
             WHERE method_name !~ '^[a-z][a-z0-9]*([._-][a-z0-9]+)*$'
       )
       AND methods = ARRAY(
            SELECT DISTINCT btrim(method_name)
              FROM unnest(methods) AS method_name
             WHERE btrim(method_name) <> ''
             ORDER BY 1
       );
$$ LANGUAGE sql IMMUTABLE;

-- Purpose: Generic product-fulfillment interface with immutable identity/evidence and constrained accepting/stopped new-business state; an empty normalized method set means no methods are currently authorized; configuration declarations start in FB-52 0.10.
-- 用途：保存商品自动化接口实例相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_product_automation_interface_instance (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "display_name" VARCHAR(128) NOT NULL,
    "plugin_id" VARCHAR(64) NOT NULL,
    "extension_point_code" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "contract_version" VARCHAR(32) NOT NULL,
    "authorized_methods" TEXT[] NOT NULL,
    "release_digest" CHAR(64) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "new_business_state" VARCHAR(16) NOT NULL DEFAULT 'accepting',
    "ever_referenced" BOOLEAN NOT NULL DEFAULT FALSE,
    "version" BIGINT NOT NULL DEFAULT 1,
    "created_by" INT NOT NULL DEFAULT 0,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "changed_by" INT NOT NULL,
    "changed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_product_automation_interface_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_product_automation_interface_creator CHECK ("created_by" > 0),
    CONSTRAINT ck_idc_product_automation_interface_change_actor CHECK ("changed_by" > 0),
    CONSTRAINT ck_idc_product_automation_interface_state CHECK (
        "new_business_state" IN ('accepting', 'stopped') AND "version" > 0
    ),
    CONSTRAINT ck_idc_product_automation_interface_no CHECK (
        "interface_instance_no" ~ '^pfi_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_idc_product_automation_display_name CHECK (
        "display_name" = btrim("display_name")
        AND length("display_name") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_product_automation_plugin_id CHECK (
        "plugin_id" ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
        AND length("plugin_id") BETWEEN 2 AND 64
    ),
    CONSTRAINT ck_idc_product_automation_extension_point CHECK (
        "extension_point_code" = 'product-fulfillment'
    ),
    CONSTRAINT ck_idc_product_automation_versions CHECK (
        btrim("plugin_release_version") <> ''
        AND btrim("contract_version") <> ''
        AND plugin_idc_core_is_normalized_method_set("authorized_methods")
        AND "release_digest" ~ '^[0-9a-f]{64}$'
        AND "declaration_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_product_automation_interface_no UNIQUE ("interface_instance_no"),
    CONSTRAINT uk_idc_product_automation_interface_tenant_no UNIQUE (
        "tenant_id", "interface_instance_no"
    )
);

-- Older revisions of 074 installed a trigger that rejected every update. Drop
-- it before backfilling the newly introduced lifecycle columns.
DROP TRIGGER IF EXISTS trg_idc_guard_automation_interface_update
    ON plugin_idc_core_product_automation_interface_instance;

ALTER TABLE plugin_idc_core_product_automation_interface_instance
    ADD COLUMN IF NOT EXISTS "new_business_state" VARCHAR(16) NOT NULL DEFAULT 'accepting',
    ADD COLUMN IF NOT EXISTS "ever_referenced" BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS "version" BIGINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS "changed_by" INT,
    ADD COLUMN IF NOT EXISTS "changed_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;
UPDATE plugin_idc_core_product_automation_interface_instance
SET "changed_by" = "created_by"
WHERE "changed_by" IS NULL;
ALTER TABLE plugin_idc_core_product_automation_interface_instance
    ALTER COLUMN "changed_by" SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_product_automation_interface_change_actor'
          AND conrelid = 'plugin_idc_core_product_automation_interface_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_product_automation_interface_instance
            ADD CONSTRAINT ck_idc_product_automation_interface_change_actor CHECK ("changed_by" > 0);
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_idc_product_automation_interface_state'
          AND conrelid = 'plugin_idc_core_product_automation_interface_instance'::regclass
    ) THEN
        ALTER TABLE plugin_idc_core_product_automation_interface_instance
            ADD CONSTRAINT ck_idc_product_automation_interface_state CHECK (
                "new_business_state" IN ('accepting', 'stopped') AND "version" > 0
            );
    END IF;
END $$;

DROP INDEX IF EXISTS idx_idc_product_automation_interface_candidates;
CREATE INDEX idx_idc_product_automation_interface_candidates
    ON plugin_idc_core_product_automation_interface_instance (
        "tenant_id", "new_business_state", "extension_point_code", "plugin_id", "id"
    );

CREATE INDEX IF NOT EXISTS idx_idc_product_automation_interface_management
    ON plugin_idc_core_product_automation_interface_instance (
        "tenant_id", "new_business_state", "display_name" text_pattern_ops, "id"
    );

-- Purpose: At-most-one automation interface binding per product version with frozen plugin release and business-extension declaration evidence.
-- 用途：保存目录商品版本履约绑定相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_version_fulfillment_binding (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "version_no" VARCHAR(40) NOT NULL,
    "interface_instance_no" VARCHAR(40) NOT NULL,
    "interface_display_name_snapshot" VARCHAR(128) NOT NULL,
    "plugin_id" VARCHAR(64) NOT NULL,
    "extension_point_code" VARCHAR(64) NOT NULL,
    "plugin_release_version" VARCHAR(32) NOT NULL,
    "contract_version" VARCHAR(32) NOT NULL,
    "authorized_methods" TEXT[] NOT NULL,
    "release_digest" CHAR(64) NOT NULL,
    "declaration_digest" CHAR(64) NOT NULL,
    "bound_by" INT NOT NULL DEFAULT 0,
    "bound_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_catalog_fulfillment_binding_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_catalog_fulfillment_binding_actor CHECK ("bound_by" > 0),
    CONSTRAINT ck_idc_catalog_fulfillment_binding_extension CHECK (
        "extension_point_code" = 'product-fulfillment'
    ),
    CONSTRAINT ck_idc_catalog_fulfillment_binding_display_name CHECK (
        "interface_display_name_snapshot" = btrim("interface_display_name_snapshot")
        AND length("interface_display_name_snapshot") BETWEEN 1 AND 128
    ),
    CONSTRAINT ck_idc_catalog_fulfillment_binding_methods CHECK (
        plugin_idc_core_is_normalized_method_set("authorized_methods")
        AND "release_digest" ~ '^[0-9a-f]{64}$'
        AND "declaration_digest" ~ '^[0-9a-f]{64}$'
    ),
    CONSTRAINT uk_idc_catalog_fulfillment_binding_version UNIQUE ("version_no"),
    CONSTRAINT fk_idc_catalog_fulfillment_binding_version FOREIGN KEY ("version_no")
        REFERENCES plugin_idc_core_catalog_product_version ("version_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_catalog_fulfillment_binding_interface FOREIGN KEY ("interface_instance_no")
        REFERENCES plugin_idc_core_product_automation_interface_instance ("interface_instance_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_catalog_fulfillment_binding_interface
    ON plugin_idc_core_catalog_product_version_fulfillment_binding (
        "tenant_id", "interface_instance_no", "version_no"
    );

-- Preserve the permanent "ever referenced" fact when 074 is reapplied over
-- development databases that already contain bindings.
UPDATE plugin_idc_core_product_automation_interface_instance AS interface
SET "ever_referenced" = TRUE,
    "version" = interface."version" + 1,
    "changed_at" = CURRENT_TIMESTAMP
WHERE interface."ever_referenced" = FALSE
  AND EXISTS (
      SELECT 1
      FROM plugin_idc_core_catalog_product_version_fulfillment_binding AS binding
      WHERE binding."tenant_id" = interface."tenant_id"
        AND binding."interface_instance_no" = interface."interface_instance_no"
  );

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_fulfillment_binding()
RETURNS TRIGGER AS $$
DECLARE
    target_version_status VARCHAR(16);
    target_interface plugin_idc_core_product_automation_interface_instance%ROWTYPE;
BEGIN
    SELECT "status_code"
      INTO target_version_status
      FROM plugin_idc_core_catalog_product_version
     WHERE "tenant_id" = NEW."tenant_id"
       AND "version_no" = NEW."version_no"
     FOR UPDATE;

    IF target_version_status IS NULL THEN
        RAISE EXCEPTION 'catalog product version is unavailable for fulfillment binding';
    END IF;
    IF target_version_status <> 'draft' THEN
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

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_automation_interface_update()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        IF OLD."ever_referenced" THEN
            RAISE EXCEPTION 'referenced product automation interface instances cannot be deleted';
        END IF;
        RETURN OLD;
    END IF;

    -- Database time is authoritative for every permitted revision.
    NEW."changed_at" := CURRENT_TIMESTAMP;
    IF ROW(
        NEW."id", NEW."tenant_id", NEW."interface_instance_no",
        NEW."plugin_id", NEW."extension_point_code", NEW."plugin_release_version",
        NEW."contract_version", NEW."authorized_methods", NEW."release_digest",
        NEW."declaration_digest", NEW."created_by", NEW."created_at"
    ) IS DISTINCT FROM ROW(
        OLD."id", OLD."tenant_id", OLD."interface_instance_no",
        OLD."plugin_id", OLD."extension_point_code", OLD."plugin_release_version",
        OLD."contract_version", OLD."authorized_methods", OLD."release_digest",
        OLD."declaration_digest", OLD."created_by", OLD."created_at"
    ) THEN
        RAISE EXCEPTION 'product automation interface identity and evidence are immutable';
    END IF;
    IF NEW."version" <> OLD."version" + 1
       OR NEW."changed_by" <= 0
       OR NEW."changed_at" < OLD."changed_at" THEN
        RAISE EXCEPTION 'product automation interface revision is invalid';
    END IF;
    IF (CASE WHEN NEW."display_name" IS DISTINCT FROM OLD."display_name" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state" THEN 1 ELSE 0 END)
       + (CASE WHEN NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced" THEN 1 ELSE 0 END) <> 1 THEN
        RAISE EXCEPTION 'product automation interface update must change exactly one managed fact';
    END IF;
    IF NEW."new_business_state" IS DISTINCT FROM OLD."new_business_state"
       AND (OLD."new_business_state", NEW."new_business_state") NOT IN (
            ('accepting', 'stopped'), ('stopped', 'accepting')
       ) THEN
        RAISE EXCEPTION 'product automation interface state transition is invalid';
    END IF;
    IF NEW."ever_referenced" IS DISTINCT FROM OLD."ever_referenced"
       AND (OLD."ever_referenced" OR NOT NEW."ever_referenced") THEN
        RAISE EXCEPTION 'product automation interface reference history is irreversible';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_automation_interface_update
    ON plugin_idc_core_product_automation_interface_instance;
CREATE TRIGGER trg_idc_guard_automation_interface_update
BEFORE UPDATE OR DELETE
ON plugin_idc_core_product_automation_interface_instance
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_automation_interface_update();

DROP TRIGGER IF EXISTS trg_idc_validate_fulfillment_binding
    ON plugin_idc_core_catalog_product_version_fulfillment_binding;
CREATE TRIGGER trg_idc_validate_fulfillment_binding
BEFORE INSERT OR UPDATE
ON plugin_idc_core_catalog_product_version_fulfillment_binding
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_fulfillment_binding();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_fulfillment_binding_history()
RETURNS TRIGGER AS $$
DECLARE
    target_version_status VARCHAR(16);
BEGIN
    IF TG_OP = 'UPDATE'
       AND (NEW."id" IS DISTINCT FROM OLD."id"
            OR NEW."tenant_id" IS DISTINCT FROM OLD."tenant_id"
            OR NEW."version_no" IS DISTINCT FROM OLD."version_no") THEN
        RAISE EXCEPTION 'fulfillment binding ownership is immutable';
    END IF;

    SELECT "status_code"
      INTO target_version_status
      FROM plugin_idc_core_catalog_product_version
     WHERE "tenant_id" = OLD."tenant_id"
       AND "version_no" = OLD."version_no"
     FOR UPDATE;

    IF target_version_status IS DISTINCT FROM 'draft' THEN
        RAISE EXCEPTION 'published product version fulfillment binding is immutable';
    END IF;
    RETURN CASE WHEN TG_OP = 'DELETE' THEN OLD ELSE NEW END;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_guard_fulfillment_binding_history
    ON plugin_idc_core_catalog_product_version_fulfillment_binding;
CREATE TRIGGER trg_idc_guard_fulfillment_binding_history
BEFORE UPDATE OR DELETE
ON plugin_idc_core_catalog_product_version_fulfillment_binding
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_fulfillment_binding_history();

COMMENT ON TABLE plugin_idc_core_product_automation_interface_instance IS
    'Generic product-fulfillment interface with immutable identity/evidence and constrained accepting/stopped new-business state; an empty normalized method set means no methods are currently authorized; configuration declarations start in FB-52 0.10';
COMMENT ON TABLE plugin_idc_core_catalog_product_version_fulfillment_binding IS
    'At-most-one automation interface binding per product version with frozen plugin release and business-extension declaration evidence';

-- Earlier development databases removed the former brand segment after
-- PostgreSQL had already shortened generated object names. Restore the names
-- a fresh idc-core database receives without replacing object OIDs or identity
-- dependencies. Every present cleanup target with an unexpected catalog shape
-- fails closed; absent tables are left to their owning migration.
DO $generated_object_names$
DECLARE
    mapping RECORD;
    table_oid REGCLASS;
    old_object_oid REGCLASS;
    current_object_oid REGCLASS;
    constraint_oid OID;
    constraint_index_oid OID;
    identity_attnum SMALLINT;
    dependency_count BIGINT;
BEGIN
    FOR mapping IN
        SELECT *
          FROM (VALUES
            ('plugin_idc_core_billing_invoice_transform_destination',
                'plugin_idc_core_billing_invoice_transform_destinat_pkey',
                'plugin_idc_core_billing_invoice_transform_destination_pkey'),
            ('plugin_idc_core_commerce_assisted_quote_request_item',
                'plugin_idc_core_commerce_assisted_quote_request_it_pkey',
                'plugin_idc_core_commerce_assisted_quote_request_item_pkey'),
            ('plugin_idc_core_commerce_configuration_snapshot_item',
                'plugin_idc_core_commerce_configuration_snapshot_it_pkey',
                'plugin_idc_core_commerce_configuration_snapshot_item_pkey'),
            ('plugin_idc_core_marketing_campaign_newcomer_request',
                'plugin_idc_core_marketing_campaign_newcomer_reques_pkey',
                'plugin_idc_core_marketing_campaign_newcomer_request_pkey'),
            ('plugin_idc_core_server_accessory_price_policy_version',
                'plugin_idc_core_server_accessory_price_policy_vers_pkey',
                'plugin_idc_core_server_accessory_price_policy_version_pkey'),
            ('plugin_idc_core_support_ticket_custom_field_value_option',
                'plugin_idc_core_support_ticket_custom_field_value__pkey',
                'plugin_idc_core_support_ticket_custom_field_value_option_pkey')
          ) AS primary_key_names(table_name, old_name, current_name)
    LOOP
        table_oid := to_regclass(format('%I.%I', current_schema(), mapping.table_name));
        IF table_oid IS NULL THEN
            CONTINUE;
        END IF;
        old_object_oid := to_regclass(format('%I.%I', current_schema(), mapping.old_name));
        current_object_oid := to_regclass(format('%I.%I', current_schema(), mapping.current_name));
        IF old_object_oid IS NOT NULL AND current_object_oid IS NOT NULL THEN
            RAISE EXCEPTION 'generated-name cleanup found both old and current primary-key indexes for %',
                mapping.table_name;
        END IF;

        SELECT constraint_row.oid, constraint_row.conindid
          INTO constraint_oid, constraint_index_oid
          FROM pg_constraint AS constraint_row
         WHERE constraint_row.conrelid = table_oid
           AND constraint_row.conname = mapping.old_name
           AND constraint_row.contype = 'p';
        IF constraint_oid IS NOT NULL THEN
            IF old_object_oid IS NULL OR old_object_oid <> constraint_index_oid
               OR EXISTS (
                SELECT 1
                  FROM pg_constraint AS constraint_row
                 WHERE constraint_row.conrelid = table_oid
                   AND constraint_row.conname = mapping.current_name
            ) OR current_object_oid IS NOT NULL THEN
                RAISE EXCEPTION 'generated-name cleanup found both old and current primary keys for %',
                    mapping.table_name;
            END IF;
            IF NOT EXISTS (
                SELECT 1
                  FROM pg_attribute AS attribute_row
                 WHERE attribute_row.attrelid = table_oid
                   AND attribute_row.attname = 'id'
                   AND NOT attribute_row.attisdropped
                   AND ARRAY[attribute_row.attnum]::SMALLINT[] = (
                        SELECT constraint_row.conkey
                          FROM pg_constraint AS constraint_row
                         WHERE constraint_row.oid = constraint_oid
                   )
            ) OR NOT EXISTS (
                SELECT 1
                  FROM pg_class AS index_row
                  JOIN pg_namespace AS namespace_row
                    ON namespace_row.oid = index_row.relnamespace
                 WHERE index_row.oid = constraint_index_oid
                   AND index_row.relkind = 'i'
                   AND index_row.relname = mapping.old_name
                   AND namespace_row.nspname = current_schema()
            ) THEN
                RAISE EXCEPTION 'legacy primary key catalog contract is invalid for %',
                    mapping.table_name;
            END IF;
            EXECUTE format('ALTER TABLE %I.%I RENAME CONSTRAINT %I TO %I',
                current_schema(), mapping.table_name, mapping.old_name, mapping.current_name);
        ELSE
            SELECT constraint_row.oid, constraint_row.conindid
              INTO constraint_oid, constraint_index_oid
              FROM pg_constraint AS constraint_row
             WHERE constraint_row.conrelid = table_oid
               AND constraint_row.conname = mapping.current_name
               AND constraint_row.contype = 'p';
            IF constraint_oid IS NULL OR current_object_oid IS NULL
               OR current_object_oid <> constraint_index_oid OR old_object_oid IS NOT NULL
               OR NOT EXISTS (
                SELECT 1
                  FROM pg_attribute AS attribute_row
                 WHERE attribute_row.attrelid = table_oid
                   AND attribute_row.attname = 'id'
                   AND NOT attribute_row.attisdropped
                   AND ARRAY[attribute_row.attnum]::SMALLINT[] = (
                        SELECT constraint_row.conkey
                          FROM pg_constraint AS constraint_row
                         WHERE constraint_row.oid = constraint_oid
                   )
            ) OR NOT EXISTS (
                SELECT 1
                  FROM pg_class AS index_row
                  JOIN pg_namespace AS namespace_row
                    ON namespace_row.oid = index_row.relnamespace
                 WHERE index_row.oid = constraint_index_oid
                   AND index_row.relkind = 'i'
                   AND index_row.relname = mapping.current_name
                   AND namespace_row.nspname = current_schema()
            ) THEN
                RAISE EXCEPTION 'current primary key catalog contract is invalid for %',
                    mapping.table_name;
            END IF;
        END IF;
    END LOOP;

    FOR mapping IN
        SELECT *
          FROM (VALUES
            ('plugin_idc_core_billing_invoice_transform_destination', 'id',
                'plugin_idc_core_billing_invoice_transform_destin_id_seq',
                'plugin_idc_core_billing_invoice_transform_destination_id_seq'),
            ('plugin_idc_core_billing_invoice_waiver_allocation', 'id',
                'plugin_idc_core_billing_invoice_waiver_allocatio_id_seq',
                'plugin_idc_core_billing_invoice_waiver_allocation_id_seq'),
            ('plugin_idc_core_commerce_assisted_quote_request_item', 'id',
                'plugin_idc_core_commerce_assisted_quote_request__id_seq',
                'plugin_idc_core_commerce_assisted_quote_request_item_id_seq'),
            ('plugin_idc_core_commerce_configuration_snapshot_item', 'id',
                'plugin_idc_core_commerce_configuration_snapshot__id_seq',
                'plugin_idc_core_commerce_configuration_snapshot_item_id_seq'),
            ('plugin_idc_core_commerce_quote_adjustment_snapshot', 'id',
                'plugin_idc_core_commerce_quote_adjustment_snapsh_id_seq',
                'plugin_idc_core_commerce_quote_adjustment_snapshot_id_seq'),
            ('plugin_idc_core_customer_custom_field_value_option', 'id',
                'plugin_idc_core_customer_custom_field_value_opti_id_seq',
                'plugin_idc_core_customer_custom_field_value_option_id_seq'),
            ('plugin_idc_core_marketing_campaign_newcomer_request', 'id',
                'plugin_idc_core_marketing_campaign_newcomer_requ_id_seq',
                'plugin_idc_core_marketing_campaign_newcomer_request_id_seq'),
            ('plugin_idc_core_server_accessory_entitlement_event', 'id',
                'plugin_idc_core_server_accessory_entitlement_eve_id_seq',
                'plugin_idc_core_server_accessory_entitlement_event_id_seq'),
            ('plugin_idc_core_server_accessory_price_policy_version', 'id',
                'plugin_idc_core_server_accessory_price_policy_ve_id_seq',
                'plugin_idc_core_server_accessory_price_policy_version_id_seq'),
            ('plugin_idc_core_server_accessory_renewal_work_item', 'id',
                'plugin_idc_core_server_accessory_renewal_work_it_id_seq',
                'plugin_idc_core_server_accessory_renewal_work_item_id_seq')
          ) AS identity_names(table_name, column_name, old_name, current_name)
    LOOP
        table_oid := to_regclass(format('%I.%I', current_schema(), mapping.table_name));
        IF table_oid IS NULL THEN
            CONTINUE;
        END IF;
        SELECT attribute_row.attnum
          INTO identity_attnum
          FROM pg_attribute AS attribute_row
         WHERE attribute_row.attrelid = table_oid
           AND attribute_row.attname = mapping.column_name
           AND attribute_row.attidentity <> ''
           AND NOT attribute_row.attisdropped;
        IF identity_attnum IS NULL THEN
            RAISE EXCEPTION 'identity cleanup column %.% is invalid',
                mapping.table_name, mapping.column_name;
        END IF;

        old_object_oid := to_regclass(format('%I.%I', current_schema(), mapping.old_name));
        current_object_oid := to_regclass(format('%I.%I', current_schema(), mapping.current_name));
        IF old_object_oid IS NOT NULL AND current_object_oid IS NOT NULL THEN
            RAISE EXCEPTION 'identity cleanup found both old and current sequences for %',
                mapping.table_name;
        END IF;
        IF old_object_oid IS NULL AND current_object_oid IS NULL THEN
            RAISE EXCEPTION 'identity cleanup sequence is missing for %', mapping.table_name;
        END IF;

        SELECT count(*)
          INTO dependency_count
          FROM pg_class AS sequence_row
          JOIN pg_namespace AS namespace_row
            ON namespace_row.oid = sequence_row.relnamespace
          JOIN pg_depend AS dependency_row
            ON dependency_row.objid = sequence_row.oid
         WHERE sequence_row.oid = COALESCE(old_object_oid, current_object_oid)
           AND sequence_row.relkind = 'S'
           AND namespace_row.nspname = current_schema()
           AND dependency_row.refobjid = table_oid
           AND dependency_row.refobjsubid = identity_attnum
           AND dependency_row.deptype = 'i';
        IF dependency_count <> 1 THEN
            RAISE EXCEPTION 'identity sequence dependency is invalid for %', mapping.table_name;
        END IF;

        IF old_object_oid IS NOT NULL THEN
            EXECUTE format('ALTER SEQUENCE %I.%I RENAME TO %I',
                current_schema(), mapping.old_name, mapping.current_name);
        END IF;
    END LOOP;
END;
$generated_object_names$;
