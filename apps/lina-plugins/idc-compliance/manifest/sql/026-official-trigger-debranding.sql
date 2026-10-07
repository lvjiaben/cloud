-- Remove legacy branded trigger names after proving that every trigger still
-- has the exact table, timing, event, row scope, and function contract owned
-- by the current idc-compliance migration chain.
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
            ('plugin_idc_compliance_contract_file', 'trg_plugin_linapro_idc_compliance_contract_file_immutable', 'trg_plugin_idc_compliance_contract_file_immutable', 'plugin_idc_compliance_reject_archived_file_mutation', 27),
            ('plugin_idc_compliance_contract', 'trg_plugin_linapro_idc_compliance_contract_snapshot_guard', 'trg_plugin_idc_compliance_contract_snapshot_guard', 'plugin_idc_compliance_guard_contract_snapshot', 19),
            ('plugin_idc_compliance_contract_template_content', 'trg_plugin_linapro_idc_compliance_ctplc_append_only', 'trg_plugin_idc_compliance_ctplc_append_only', 'plugin_idc_compliance_reject_template_history_mutation', 27),
            ('plugin_idc_compliance_contract_template_field', 'trg_plugin_linapro_idc_compliance_ctplf_append_only', 'trg_plugin_idc_compliance_ctplf_append_only', 'plugin_idc_compliance_reject_template_history_mutation', 27),
            ('plugin_idc_compliance_contract_template_field_set', 'trg_plugin_linapro_idc_compliance_ctplfs_append_only', 'trg_plugin_idc_compliance_ctplfs_append_only', 'plugin_idc_compliance_reject_template_history_mutation', 27),
            ('plugin_idc_compliance_ctpl_provider_component', 'trg_plugin_linapro_idc_compliance_ctplpc_append_only', 'trg_plugin_idc_compliance_ctplpc_append_only', 'plugin_idc_compliance_reject_provider_mapping_mutation', 27),
            ('plugin_idc_compliance_ctpl_provider_field_mapping', 'trg_plugin_linapro_idc_compliance_ctplpfm_append_only', 'trg_plugin_idc_compliance_ctplpfm_append_only', 'plugin_idc_compliance_reject_provider_mapping_mutation', 27),
            ('plugin_idc_compliance_ctpl_provider_mapping_set', 'trg_plugin_linapro_idc_compliance_ctplpms_append_only', 'trg_plugin_idc_compliance_ctplpms_append_only', 'plugin_idc_compliance_reject_provider_mapping_mutation', 27),
            ('plugin_idc_compliance_ctpl_provider_recipient', 'trg_plugin_linapro_idc_compliance_ctplpr_append_only', 'trg_plugin_idc_compliance_ctplpr_append_only', 'plugin_idc_compliance_reject_provider_mapping_mutation', 27),
            ('plugin_idc_compliance_ctpl_provider_recipient_mapping', 'trg_plugin_linapro_idc_compliance_ctplprm_append_only', 'trg_plugin_idc_compliance_ctplprm_append_only', 'plugin_idc_compliance_reject_provider_mapping_mutation', 27),
            ('plugin_idc_compliance_contract_template_version', 'trg_plugin_linapro_idc_compliance_ctplv_guard', 'trg_plugin_idc_compliance_ctplv_guard', 'plugin_idc_compliance_guard_template_version', 19),
            ('plugin_idc_compliance_external_evidence', 'trg_plugin_linapro_idc_compliance_external_evidence_append_only', 'trg_plugin_idc_compliance_external_evidence_append_only', 'plugin_idc_compliance_reject_append_only_mutation', 27),
            ('plugin_idc_compliance_red_letter_allocation', 'trg_plugin_linapro_idc_compliance_red_alloc_append_only', 'trg_plugin_idc_compliance_red_alloc_append_only', 'plugin_idc_compliance_reject_append_only_mutation', 27),
            ('plugin_idc_compliance_red_letter_action', 'trg_plugin_linapro_idc_compliance_red_letter_action_append', 'trg_plugin_idc_compliance_red_letter_action_append', 'plugin_idc_compliance_reject_append_only_mutation', 27),
            ('plugin_idc_compliance_red_letter_candidate', 'trg_plugin_linapro_idc_compliance_red_letter_candidate_append', 'trg_plugin_idc_compliance_red_letter_candidate_append', 'plugin_idc_compliance_reject_append_only_mutation', 27),
            ('plugin_idc_compliance_tax_invoice_allocation', 'trg_plugin_linapro_idc_compliance_tax_alloc_immutable', 'trg_plugin_idc_compliance_tax_alloc_immutable', 'plugin_idc_compliance_lock_tax_invoice_allocation', 27),
            ('plugin_idc_compliance_tax_invoice_action', 'trg_plugin_linapro_idc_compliance_tax_inv_action_append', 'trg_plugin_idc_compliance_tax_inv_action_append', 'plugin_idc_compliance_reject_append_only_mutation', 27),
            ('plugin_idc_compliance_tax_invoice_file', 'trg_plugin_linapro_idc_compliance_tax_invoice_file_immutable', 'trg_plugin_idc_compliance_tax_invoice_file_immutable', 'plugin_idc_compliance_reject_archived_file_mutation', 27),
            ('plugin_idc_compliance_tax_invoice_item', 'trg_plugin_linapro_idc_compliance_tax_invoice_item_immutable', 'trg_plugin_idc_compliance_tax_invoice_item_immutable', 'plugin_idc_compliance_lock_tax_invoice_item', 27),
            ('plugin_idc_compliance_tax_invoice', 'trg_plugin_linapro_idc_compliance_tax_invoice_request_immutable', 'trg_plugin_idc_compliance_tax_invoice_request_immutable', 'plugin_idc_compliance_lock_tax_invoice_request', 27)
        ) AS contracts(table_name, legacy_name, current_name, function_name, trigger_type)
    LOOP
        target_table := to_regclass(format('%I.%I', current_schema(), trigger_contract.table_name));
        target_function := to_regprocedure(format('%I.%I()', current_schema(), trigger_contract.function_name));
        IF target_table IS NULL OR target_function IS NULL THEN
            RAISE EXCEPTION 'idc-compliance trigger contract prerequisite is missing for %', trigger_contract.current_name;
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
            RAISE EXCEPTION 'ambiguous legacy idc-compliance trigger %', trigger_contract.legacy_name;
        END IF;
        IF current_count > 1 OR (current_count = 1 AND NOT current_valid) THEN
            RAISE EXCEPTION 'ambiguous current idc-compliance trigger %', trigger_contract.current_name;
        END IF;
        IF legacy_count = 0 AND current_count = 0 THEN
            RAISE EXCEPTION 'idc-compliance trigger contract is missing for %', trigger_contract.current_name;
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
