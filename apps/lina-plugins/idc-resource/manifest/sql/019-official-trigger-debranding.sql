-- Remove legacy branded trigger names only after proving that their complete
-- PostgreSQL trigger contracts are equivalent to the current resource guards.
DO $$
DECLARE
    trigger_contract record;
    target_table oid;
    target_function oid;
    legacy_count integer;
    current_count integer;
    legacy_valid boolean;
    current_valid boolean;
BEGIN
    FOR trigger_contract IN
        SELECT *
        FROM (VALUES
            ('plugin_idc_resource_accessory_snapshot_item', 'trg_plugin_linapro_idc_resource_accessory_item_immutable', 'trg_plugin_idc_resource_accessory_item_immutable', 'plugin_idc_resource_reject_accessory_item_mutation', 27),
            ('plugin_idc_resource_provision_outcome_receipt', 'trg_plugin_linapro_idc_resource_provision_outcome_immutable', 'trg_plugin_idc_resource_provision_outcome_immutable', 'plugin_idc_resource_reject_provision_outcome_mutation', 27),
            ('plugin_idc_resource_task_evidence', 'trg_plugin_linapro_idc_resource_task_evidence_immutable', 'trg_plugin_idc_resource_task_evidence_immutable', 'plugin_idc_resource_reject_task_evidence_mutation', 27),
            ('plugin_idc_resource_task_manual_takeover', 'trg_plugin_linapro_idc_resource_task_takeover_immutable', 'trg_plugin_idc_resource_task_takeover_immutable', 'plugin_idc_resource_reject_task_takeover_mutation', 27)
        ) AS contracts(table_name, legacy_name, current_name, function_name, trigger_type)
    LOOP
        target_table := to_regclass(format('%I.%I', current_schema(), trigger_contract.table_name));
        target_function := to_regprocedure(format('%I.%I()', current_schema(), trigger_contract.function_name));
        IF target_table IS NULL OR target_function IS NULL THEN
            RAISE EXCEPTION 'idc-resource trigger contract prerequisite is missing for %', trigger_contract.current_name;
        END IF;

        SELECT count(*), coalesce(bool_and(
            trigger_table.oid = target_table AND trigger_row.tgfoid = target_function
            AND trigger_row.tgtype = trigger_contract.trigger_type
            AND trigger_row.tgenabled = 'O' AND NOT trigger_row.tgisinternal
            AND trigger_row.tgconstraint = 0 AND NOT trigger_row.tgdeferrable
            AND NOT trigger_row.tginitdeferred AND trigger_row.tgqual IS NULL
            AND trigger_row.tgnargs = 0 AND trigger_row.tgattr::text = ''
        ), false)
        INTO legacy_count, legacy_valid
        FROM pg_trigger AS trigger_row
        JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
        JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
        WHERE trigger_schema.nspname = current_schema()
          AND trigger_row.tgname = trigger_contract.legacy_name;

        SELECT count(*), coalesce(bool_and(
            trigger_table.oid = target_table AND trigger_row.tgfoid = target_function
            AND trigger_row.tgtype = trigger_contract.trigger_type
            AND trigger_row.tgenabled = 'O' AND NOT trigger_row.tgisinternal
            AND trigger_row.tgconstraint = 0 AND NOT trigger_row.tgdeferrable
            AND NOT trigger_row.tginitdeferred AND trigger_row.tgqual IS NULL
            AND trigger_row.tgnargs = 0 AND trigger_row.tgattr::text = ''
        ), false)
        INTO current_count, current_valid
        FROM pg_trigger AS trigger_row
        JOIN pg_class AS trigger_table ON trigger_table.oid = trigger_row.tgrelid
        JOIN pg_namespace AS trigger_schema ON trigger_schema.oid = trigger_table.relnamespace
        WHERE trigger_schema.nspname = current_schema()
          AND trigger_row.tgname = trigger_contract.current_name;

        IF legacy_count > 1 OR (legacy_count = 1 AND NOT legacy_valid) THEN
            RAISE EXCEPTION 'ambiguous legacy idc-resource trigger %', trigger_contract.legacy_name;
        END IF;
        IF current_count > 1 OR (current_count = 1 AND NOT current_valid) THEN
            RAISE EXCEPTION 'ambiguous current idc-resource trigger %', trigger_contract.current_name;
        END IF;
        IF legacy_count = 0 AND current_count = 0 THEN
            RAISE EXCEPTION 'idc-resource trigger contract is missing for %', trigger_contract.current_name;
        ELSIF legacy_count = 1 AND current_count = 0 THEN
            EXECUTE format(
                'ALTER TRIGGER %I ON %I.%I RENAME TO %I',
                trigger_contract.legacy_name,
                current_schema(),
                trigger_contract.table_name,
                trigger_contract.current_name
            );
        ELSIF legacy_count = 1 AND current_count = 1 THEN
            EXECUTE format(
                'DROP TRIGGER %I ON %I.%I',
                trigger_contract.legacy_name,
                current_schema(),
                trigger_contract.table_name
            );
        END IF;
    END LOOP;
END
$$;
