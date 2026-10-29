-- Start local recurring service periods only when initial delivery succeeds.
-- Existing services and upstream-owned periods are never rewritten on migration.
CREATE OR REPLACE FUNCTION plugin_idc_core_initialize_recurring_period()
RETURNS TRIGGER AS $$
DECLARE
    frozen_unit VARCHAR(16);
    frozen_count BIGINT;
    activated_at TIMESTAMPTZ;
    local_start TIMESTAMP;
BEGIN
    IF NEW.charge_type_code <> 'recurring' OR NEW.status_code <> 'active'
       OR NEW.upstream_service_no <> '' OR NEW.renewal_mode_code <> 'local_entitlement' THEN
        RETURN NEW;
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF OLD.status_code <> 'pending_activation' THEN
            RETURN NEW;
        END IF;
    END IF;
    SELECT item.period_unit_code,item.period_count::BIGINT*item.purchase_cycles::BIGINT
      INTO frozen_unit,frozen_count
      FROM plugin_idc_core_commerce_order_item item
     WHERE item.tenant_id=NEW.tenant_id AND item.order_no=NEW.order_no
       AND item.order_item_no=NEW.order_item_no AND item.charge_type_code='recurring';
    IF NOT FOUND OR frozen_count < 1 OR frozen_count > 2147483647
       OR frozen_unit NOT IN ('hour','day','month','year') THEN
        RAISE EXCEPTION 'recurring activation has no valid frozen billing period' USING ERRCODE='23514';
    END IF;
    activated_at := clock_timestamp();
    local_start := activated_at AT TIME ZONE 'UTC';
    NEW.started_at := activated_at;
    NEW.expires_at := CASE frozen_unit
        WHEN 'hour' THEN (local_start + make_interval(hours => frozen_count::INT)) AT TIME ZONE 'UTC'
        WHEN 'day' THEN (local_start + make_interval(days => frozen_count::INT)) AT TIME ZONE 'UTC'
        WHEN 'month' THEN (local_start + make_interval(months => frozen_count::INT)) AT TIME ZONE 'UTC'
        WHEN 'year' THEN (local_start + make_interval(years => frozen_count::INT)) AT TIME ZONE 'UTC'
    END;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_initialize_recurring_period ON plugin_idc_core_service_entitlement;
CREATE TRIGGER trg_idc_initialize_recurring_period
BEFORE INSERT OR UPDATE OF status_code ON plugin_idc_core_service_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_initialize_recurring_period();
