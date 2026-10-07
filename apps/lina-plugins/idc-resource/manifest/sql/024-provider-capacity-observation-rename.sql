-- 024: one-way development-data rename from the retired inventory vocabulary.
-- No compatibility table, view, trigger, or duplicate write path is retained.

DO $$
DECLARE
    old_count bigint;
    new_count bigint;
BEGIN
    IF to_regclass(current_schema() || '.plugin_idc_resource_inventory_snapshot') IS NOT NULL AND
       to_regclass(current_schema() || '.plugin_idc_resource_provider_capacity_observation') IS NULL THEN
        ALTER TABLE plugin_idc_resource_inventory_snapshot
            RENAME TO plugin_idc_resource_provider_capacity_observation;
    END IF;
    IF to_regclass(current_schema() || '.plugin_idc_resource_inventory_snapshot_item') IS NOT NULL AND
       to_regclass(current_schema() || '.plugin_idc_resource_provider_capacity_observation_item') IS NULL THEN
        ALTER TABLE plugin_idc_resource_inventory_snapshot_item
            RENAME TO plugin_idc_resource_provider_capacity_observation_item;
    END IF;

    IF to_regclass(current_schema() || '.plugin_idc_resource_inventory_snapshot') IS NOT NULL AND
       to_regclass(current_schema() || '.plugin_idc_resource_provider_capacity_observation') IS NOT NULL THEN
        EXECUTE 'SELECT count(*) FROM plugin_idc_resource_inventory_snapshot' INTO old_count;
        EXECUTE 'SELECT count(*) FROM plugin_idc_resource_provider_capacity_observation' INTO new_count;
        IF old_count > 0 AND new_count > 0 THEN
            RAISE EXCEPTION 'both retired inventory and provider capacity tables contain data; resolve the conflict before migration';
        END IF;
        IF old_count > 0 THEN
            INSERT INTO plugin_idc_resource_provider_capacity_observation
                ("tenant_id","observation_no","channel_no","capacity_kind","status_code",
                 "observation_fingerprint","item_count",observed_at,completed_at,created_at,
                 page_size,next_cursor,page_count,"trigger_code","requested_by","attempt_count",
                 next_attempt_at,"lease_token_digest",lease_expires_at,"failure_code")
            SELECT "tenant_id","snapshot_no","channel_no",inventory_kind,"status_code",
                   "snapshot_fingerprint","item_count",observed_at,completed_at,created_at,
                   page_size,next_cursor,page_count,"trigger_code","requested_by","attempt_count",
                   next_attempt_at,"lease_token_digest",lease_expires_at,"failure_code"
            FROM plugin_idc_resource_inventory_snapshot
            ORDER BY "id";

            INSERT INTO plugin_idc_resource_provider_capacity_observation_item
                ("tenant_id","observation_no","capacity_kind","external_ref",projection,observed_at,created_at)
            SELECT "tenant_id","snapshot_no",inventory_kind,"external_ref",projection,observed_at,created_at
            FROM plugin_idc_resource_inventory_snapshot_item
            ORDER BY "id";
        END IF;
        IF to_regclass(current_schema() || '.plugin_idc_resource_ip_reservation') IS NOT NULL THEN
            ALTER TABLE plugin_idc_resource_ip_reservation
                DROP CONSTRAINT IF EXISTS fk_idc_resource_ip_reservation_snapshot;
            ALTER TABLE plugin_idc_resource_ip_reservation
                DROP CONSTRAINT IF EXISTS fk_idc_resource_ip_reservation_observation;
        END IF;
        DROP TABLE IF EXISTS plugin_idc_resource_inventory_snapshot_item;
        DROP TABLE plugin_idc_resource_inventory_snapshot;
    END IF;
END $$;

DO $$
DECLARE
    target_table text;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        'plugin_idc_resource_provider_capacity_observation',
        'plugin_idc_resource_provider_capacity_observation_item'
    ] LOOP
        IF to_regclass(target_table) IS NOT NULL AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema=current_schema() AND table_name=target_table
              AND column_name='snapshot_no'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I RENAME COLUMN snapshot_no TO observation_no',
                target_table
            );
        END IF;

        IF to_regclass(target_table) IS NOT NULL AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema=current_schema() AND table_name=target_table
              AND column_name='inventory_kind'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I RENAME COLUMN inventory_kind TO capacity_kind',
                target_table
            );
        END IF;
    END LOOP;

    IF to_regclass('plugin_idc_resource_provider_capacity_observation') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_resource_provider_capacity_observation'
          AND column_name='snapshot_fingerprint'
    ) THEN
        ALTER TABLE plugin_idc_resource_provider_capacity_observation
            RENAME COLUMN "snapshot_fingerprint" TO "observation_fingerprint";
    END IF;

    IF to_regclass('plugin_idc_resource_ip_reservation') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_resource_ip_reservation'
          AND column_name='snapshot_no'
    ) THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            RENAME COLUMN "snapshot_no" TO "capacity_observation_no";
    END IF;

    IF to_regclass('plugin_idc_resource_ip_reservation') IS NOT NULL AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_resource_ip_reservation'
          AND column_name='inventory_kind'
    ) THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            RENAME COLUMN inventory_kind TO "capacity_kind";
    END IF;

    IF to_regclass('plugin_idc_resource_ip_reservation') IS NOT NULL THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            DROP CONSTRAINT IF EXISTS fk_idc_resource_ip_reservation_snapshot;
    END IF;
END $$;

DO $$
BEGIN
    IF to_regclass(current_schema() || '.plugin_idc_resource_ip_reservation') IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid='plugin_idc_resource_ip_reservation'::regclass
          AND conname='fk_idc_resource_ip_reservation_observation'
    ) THEN
        ALTER TABLE plugin_idc_resource_ip_reservation
            ADD CONSTRAINT fk_idc_resource_ip_reservation_observation
            FOREIGN KEY ("capacity_observation_no","capacity_kind")
            REFERENCES plugin_idc_resource_provider_capacity_observation ("observation_no","capacity_kind")
            ON DELETE RESTRICT;
    END IF;
END $$;

DO $$
DECLARE
    target_table text;
BEGIN
    FOREACH target_table IN ARRAY ARRAY[
        'plugin_idc_resource_provider_capacity_observation',
        'plugin_idc_resource_provider_capacity_observation_item'
    ] LOOP
        IF to_regclass(target_table) IS NOT NULL AND EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_schema=current_schema() AND table_name=target_table
              AND column_name='inventory_kind'
        ) THEN
            EXECUTE format(
                'ALTER TABLE %I RENAME COLUMN inventory_kind TO capacity_kind',
                target_table
            );
        END IF;
    END LOOP;
END $$;

DO $$
DECLARE
    column_row record;
    target_column text;
BEGIN
    FOR column_row IN
        SELECT column_name
        FROM information_schema.columns
        WHERE table_schema=current_schema()
          AND table_name='plugin_idc_resource_channel_health_state'
          AND column_name LIKE 'last_inventory_%'
    LOOP
        target_column := replace(
            column_row.column_name,
            'last_inventory_',
            'last_provider_capacity_'
        );
        IF EXISTS (
            SELECT 1 FROM information_schema.columns
             WHERE table_schema = current_schema()
               AND table_name = 'plugin_idc_resource_channel_health_state'
               AND column_name = target_column
        ) THEN
            EXECUTE format(
                'UPDATE plugin_idc_resource_channel_health_state SET %I = %I WHERE %I IS NOT NULL',
                target_column, column_row.column_name, column_row.column_name
            );
            EXECUTE format(
                'ALTER TABLE plugin_idc_resource_channel_health_state DROP COLUMN %I CASCADE',
                column_row.column_name
            );
        ELSE
            EXECUTE format(
                'ALTER TABLE plugin_idc_resource_channel_health_state RENAME COLUMN %I TO %I',
                column_row.column_name, target_column
            );
        END IF;
    END LOOP;
END $$;

UPDATE plugin_idc_resource_health_snapshot
SET "operation_kind"='provider_capacity'
WHERE "operation_kind"='inventory';

DO $$
DECLARE
    constraint_row record;
    renamed text;
BEGIN
    FOR constraint_row IN
        SELECT relation.relname AS table_name, constraint_value.conname
        FROM pg_constraint constraint_value
        JOIN pg_class relation ON relation.oid=constraint_value.conrelid
        JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
        WHERE namespace.nspname=current_schema()
          AND constraint_value.conname LIKE '%inventory%'
    LOOP
        renamed := replace(
            replace(constraint_row.conname, 'inventory_snapshot', 'provider_capacity_observation'),
            'inventory', 'provider_capacity'
        );
        EXECUTE format(
            'ALTER TABLE %I RENAME CONSTRAINT %I TO %I',
            constraint_row.table_name, constraint_row.conname, renamed
        );
    END LOOP;
END $$;

DO $$
DECLARE
    index_row record;
    renamed text;
BEGIN
    FOR index_row IN
        SELECT relation.relname
        FROM pg_class relation
        JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
        WHERE namespace.nspname=current_schema() AND relation.relkind='i'
          AND relation.relname LIKE '%inventory%'
    LOOP
        renamed := replace(
            replace(index_row.relname, 'inventory_snapshot', 'provider_capacity_observation'),
            'inventory', 'provider_capacity'
        );
        IF to_regclass(renamed) IS NULL THEN
            EXECUTE format('ALTER INDEX %I RENAME TO %I', index_row.relname, renamed);
        END IF;
    END LOOP;
END $$;
