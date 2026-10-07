-- 022: remove the retired capacity-to-catalog publication direction.
--
-- Provider capacity observations remain resource-owner operational data.
-- They never create, update, or synchronize generic product catalog records.

DO $$
DECLARE
    target_table regclass;
    trigger_row record;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        to_regclass('plugin_idc_resource_catalog_draft_publication_event'),
        to_regclass('plugin_idc_resource_catalog_draft_publication')
    ]
    LOOP
        IF target_table IS NULL THEN
            CONTINUE;
        END IF;
        FOR trigger_row IN
            SELECT trigger_name
            FROM information_schema.triggers
            WHERE event_object_schema = current_schema()
              AND event_object_table = target_table::text
        LOOP
            EXECUTE format('DROP TRIGGER IF EXISTS %I ON %s', trigger_row.trigger_name, target_table);
        END LOOP;
    END LOOP;
END $$;

DROP TABLE IF EXISTS plugin_idc_resource_catalog_draft_publication_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_resource_catalog_draft_publication CASCADE;

DO $$
DECLARE
    constraint_row record;
BEGIN
    FOR constraint_row IN
        SELECT constraint_value.conrelid::regclass AS table_name,
               constraint_value.conname AS constraint_name
        FROM pg_constraint AS constraint_value
        JOIN pg_class AS relation ON relation.oid = constraint_value.conrelid
        JOIN pg_namespace AS namespace ON namespace.oid = relation.relnamespace
        WHERE namespace.nspname = current_schema()
          AND constraint_value.conname = 'uk_plugin_idc_resource_inventory_pub_source'
    LOOP
        EXECUTE format(
            'ALTER TABLE %s DROP CONSTRAINT IF EXISTS %I',
            constraint_row.table_name,
            constraint_row.constraint_name
        );
    END LOOP;
END $$;

DO $$
DECLARE
    function_row record;
BEGIN
    FOR function_row IN
        SELECT namespace.nspname AS schema_name,
               procedure.proname AS function_name,
               pg_get_function_identity_arguments(procedure.oid) AS identity_arguments
        FROM pg_proc AS procedure
        JOIN pg_namespace AS namespace ON namespace.oid = procedure.pronamespace
        WHERE namespace.nspname = current_schema()
          AND procedure.proname LIKE 'plugin_idc_resource%catalog%publication%'
    LOOP
        EXECUTE format(
            'DROP FUNCTION IF EXISTS %I.%I(%s) CASCADE',
            function_row.schema_name,
            function_row.function_name,
            function_row.identity_arguments
        );
    END LOOP;
END $$;

DO $$
BEGIN
    IF to_regclass('sys_dict_data') IS NOT NULL THEN
        DELETE FROM sys_dict_data
        WHERE "tenant_id" = 0
          AND "dict_type" = 'idc_resource_catalog_publication_status';
    END IF;
    IF to_regclass('sys_dict_type') IS NOT NULL THEN
        DELETE FROM sys_dict_type
        WHERE "tenant_id" = 0
          AND "type" = 'idc_resource_catalog_publication_status';
    END IF;
END $$;
