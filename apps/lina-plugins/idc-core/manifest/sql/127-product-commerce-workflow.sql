-- Freeze the independent quantity pricing algorithm on each immutable dimension.
-- Existing candidate dimensions keep fixed pricing and their historical digests.
ALTER TABLE plugin_idc_core_catalog_configuration_price_dimension
    ADD COLUMN IF NOT EXISTS pricing_mode_code VARCHAR(16) NOT NULL DEFAULT 'fixed';
ALTER TABLE plugin_idc_core_catalog_configuration_price_dimension
    DROP CONSTRAINT IF EXISTS ck_idc_catalog_config_price_mode;
ALTER TABLE plugin_idc_core_catalog_configuration_price_dimension
    ADD CONSTRAINT ck_idc_catalog_config_price_mode CHECK (
        pricing_mode_code IN ('fixed','per_unit','band_total')
    );
ALTER TABLE plugin_idc_core_commerce_configuration_price_selection
    DROP CONSTRAINT IF EXISTS ck_idc_com_config_price_selection_quantity;
ALTER TABLE plugin_idc_core_commerce_configuration_price_selection
    ADD CONSTRAINT ck_idc_com_config_price_selection_quantity CHECK (quantity BETWEEN 1 AND 2147483647);
-- At most 128 selected price dimensions, each represented by a signed SQL INT.
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    DROP CONSTRAINT IF EXISTS ck_idc_com_config_counts;
ALTER TABLE plugin_idc_core_commerce_configuration_snapshot
    ADD CONSTRAINT ck_idc_com_config_counts CHECK (
        selected_option_count BETWEEN 0 AND 5000
        AND total_selected_quantity BETWEEN 0 AND 274877906816
        AND ((selected_option_count = 0 AND total_selected_quantity = 0)
             OR (selected_option_count > 0 AND total_selected_quantity > 0))
    );

-- Trial time starts at the first successful activation, not when a pending
-- entitlement is materialized. The paid order item is the immutable source;
-- supplier-owned peer service dates and later resumes keep their own dates.
CREATE OR REPLACE FUNCTION plugin_idc_core_initialize_trial_period()
RETURNS TRIGGER AS $$
DECLARE
    frozen_unit VARCHAR(16);
    frozen_count BIGINT;
    activated_at TIMESTAMPTZ;
    local_start TIMESTAMP;
BEGIN
    IF NEW.charge_type_code <> 'trial' OR NEW.status_code <> 'active'
       OR NEW.upstream_service_no <> '' THEN
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
       AND item.order_item_no=NEW.order_item_no AND item.charge_type_code='trial';
    IF NOT FOUND OR frozen_count < 1 OR frozen_count > 2147483647
       OR frozen_unit NOT IN ('hour','day','month','year') THEN
        RAISE EXCEPTION 'trial activation has no valid frozen billing period' USING ERRCODE='23514';
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

-- Install the lifecycle hook only when the entitlement dependency is present.
DO $trial_activation$ BEGIN
    IF to_regclass('plugin_idc_core_service_entitlement') IS NOT NULL THEN
        DROP TRIGGER IF EXISTS trg_idc_initialize_trial_period ON plugin_idc_core_service_entitlement;
        CREATE TRIGGER trg_idc_initialize_trial_period
        BEFORE INSERT OR UPDATE OF status_code ON plugin_idc_core_service_entitlement
        FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_initialize_trial_period();
    END IF;
END $trial_activation$;

-- Stop every unpaid renewal entry point, including direct invoice balance
-- payments. Allocations and ledger writes share the invoice transaction, so
-- rejection rolls them back. Completed payment replays stay readable.
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_renewal_sale_payment()
RETURNS TRIGGER AS $$
DECLARE
    sale_status TEXT;
BEGIN
    IF NEW.source_code <> 'renewal' OR
       (NEW.paid_amount_minor <= OLD.paid_amount_minor AND
        NEW.credit_amount_minor <= OLD.credit_amount_minor) THEN
        RETURN NEW;
    END IF;
    SELECT product.status_code INTO sale_status
    FROM plugin_idc_core_business_operation operation
    JOIN plugin_idc_core_service_entitlement entitlement
      ON entitlement.tenant_id=operation.tenant_id
     AND entitlement.entitlement_no=operation.subject_no
     AND entitlement.customer_no=operation.customer_no
    JOIN plugin_idc_core_catalog_product product
      ON product.tenant_id=entitlement.tenant_id
     AND product.product_no=entitlement.product_no
    WHERE operation.tenant_id=NEW.tenant_id AND operation.invoice_no=NEW.invoice_no
      AND operation.operation_type_code='renewal'
      AND operation.subject_type_code='entitlement'
      AND operation.payer_customer_no=NEW.customer_no
      AND product.deleted_at IS NULL
    FOR SHARE OF product;
    IF sale_status IS DISTINCT FROM 'active' THEN
        RAISE EXCEPTION 'renewal product is no longer on sale' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_idc_renewal_sale_payment ON plugin_idc_core_billing_invoice;
CREATE TRIGGER trg_idc_renewal_sale_payment
BEFORE UPDATE OF paid_amount_minor,credit_amount_minor ON plugin_idc_core_billing_invoice
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_renewal_sale_payment();

-- Mutable offer associations are replaced as a set. Paid target evidence lives
-- independently so removing an offer never rewrites an existing order.
CREATE TABLE IF NOT EXISTS plugin_idc_core_catalog_product_addon (
    tenant_id BIGINT NOT NULL DEFAULT 0,
    product_no VARCHAR(128) NOT NULL,
    addon_product_no VARCHAR(128) NOT NULL,
    PRIMARY KEY(tenant_id,product_no,addon_product_no),
    CHECK(product_no <> addon_product_no)
);
CREATE TABLE IF NOT EXISTS plugin_idc_core_commerce_addon_target (
    tenant_id BIGINT NOT NULL DEFAULT 0,
    quote_no VARCHAR(40) NOT NULL,
    order_no VARCHAR(40),
    customer_no VARCHAR(128) NOT NULL,
    entitlement_no VARCHAR(128) NOT NULL,
    product_no VARCHAR(128) NOT NULL,
    PRIMARY KEY(tenant_id,quote_no),
    UNIQUE(tenant_id,order_no)
);
CREATE INDEX IF NOT EXISTS idx_idc_core_addon_target_entitlement
ON plugin_idc_core_commerce_addon_target(tenant_id,customer_no,entitlement_no);

-- Payment must not buy an add-on for a terminated, expired or foreign service.
-- It runs in the same transaction as allocation and is harmless to paid replays.
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_addon_payment()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.paid_amount_minor <= OLD.paid_amount_minor AND
       NEW.credit_amount_minor <= OLD.credit_amount_minor THEN RETURN NEW; END IF;
    IF EXISTS(SELECT 1 FROM plugin_idc_core_commerce_addon_target target
       JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=target.tenant_id AND orders.order_no=target.order_no
       WHERE orders.order_no=NEW.order_no AND target.tenant_id=NEW.tenant_id)
       AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_addon_target target
         JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=target.tenant_id AND orders.order_no=target.order_no
         JOIN plugin_idc_core_service_entitlement service ON service.tenant_id=target.tenant_id AND service.entitlement_no=target.entitlement_no AND service.customer_no=target.customer_no
         WHERE orders.order_no=NEW.order_no AND target.tenant_id=NEW.tenant_id
           AND service.status_code='active' AND (service.expires_at IS NULL OR service.expires_at>clock_timestamp())) THEN
       RAISE EXCEPTION 'add-on target service is no longer available' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS trg_idc_addon_payment ON plugin_idc_core_billing_invoice;
CREATE TRIGGER trg_idc_addon_payment BEFORE UPDATE OF paid_amount_minor,credit_amount_minor
ON plugin_idc_core_billing_invoice FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_addon_payment();

-- Modern paid upgrades preserve their quote and every payment/provider retry.
-- Financial intent/history is retained permanently rather than soft-deleted.
CREATE TABLE IF NOT EXISTS plugin_idc_core_entitlement_upgrade (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    upgrade_no VARCHAR(40) NOT NULL UNIQUE,
    customer_no VARCHAR(40) NOT NULL,
    entitlement_no VARCHAR(40) NOT NULL,
    product_no VARCHAR(40) NOT NULL,
    source_version BIGINT NOT NULL CHECK (source_version>0),
    source_configuration_no VARCHAR(40) NOT NULL,
    target_configuration_no VARCHAR(40),
    target_price_snapshot_no VARCHAR(40),
    currency_code CHAR(3) NOT NULL,
    source_period_net_minor BIGINT NOT NULL CHECK (source_period_net_minor>=0),
    target_period_net_minor BIGINT NOT NULL CHECK (target_period_net_minor>=source_period_net_minor),
    charge_net_minor BIGINT NOT NULL CHECK (charge_net_minor>=0),
    charge_tax_minor BIGINT NOT NULL CHECK (charge_tax_minor>=0),
    charge_total_minor BIGINT NOT NULL CHECK (charge_total_minor=charge_net_minor+charge_tax_minor),
    period_started_at TIMESTAMPTZ NOT NULL,
    period_expires_at TIMESTAMPTZ NOT NULL CHECK (period_expires_at>period_started_at),
    quote_expires_at TIMESTAMPTZ NOT NULL,
    frozen_target JSONB NOT NULL CHECK (jsonb_typeof(frozen_target)='object'),
    frozen_price JSONB NOT NULL CHECK (jsonb_typeof(frozen_price)='object'),
    provider_parameters JSONB NOT NULL CHECK (jsonb_typeof(provider_parameters)='object'),
    channel_no VARCHAR(64) NOT NULL,
    pool_no VARCHAR(64) NOT NULL,
    invoice_no VARCHAR(40) UNIQUE,
    provider_task_no VARCHAR(64),
    status_code VARCHAR(24) NOT NULL CHECK (status_code IN ('quoted','pending_payment','ready','executing','retry_wait','succeeded','manual_review','cancelled')),
    idempotency_digest CHAR(64) NOT NULL CHECK (idempotency_digest ~ '^[0-9a-f]{64}$'),
    request_digest CHAR(64) NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    content_digest CHAR(64) NOT NULL CHECK (content_digest ~ '^[0-9a-f]{64}$'),
    last_error_code VARCHAR(128) NOT NULL DEFAULT '',
    attempt_count INT NOT NULL DEFAULT 0 CHECK (attempt_count>=0),
    available_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    version BIGINT NOT NULL DEFAULT 1 CHECK (version>0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (customer_no,idempotency_digest)
);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_entitlement_upgrade_customer
 ON plugin_idc_core_entitlement_upgrade(tenant_id,customer_no,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_entitlement_upgrade_due
 ON plugin_idc_core_entitlement_upgrade(tenant_id,status_code,available_at,id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_core_entitlement_upgrade_active
 ON plugin_idc_core_entitlement_upgrade(tenant_id,entitlement_no)
 WHERE status_code IN ('pending_payment','ready','executing','retry_wait','manual_review');

-- Upgrade invoice writes recheck the locked service even through the generic financial payment entry.
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_upgrade_payment() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE u plugin_idc_core_entitlement_upgrade%ROWTYPE;
BEGIN
 IF NEW.paid_amount_minor>OLD.paid_amount_minor OR (NEW.status_code='paid' AND OLD.status_code<>'paid') THEN
  SELECT * INTO u FROM plugin_idc_core_entitlement_upgrade WHERE tenant_id=0 AND invoice_no=NEW.invoice_no FOR UPDATE;
  IF FOUND AND NOT EXISTS (
   SELECT 1 FROM plugin_idc_core_service_entitlement e
   JOIN plugin_idc_core_catalog_product p ON p.tenant_id=e.tenant_id AND p.product_no=e.product_no
   JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
   WHERE e.tenant_id=0 AND e.entitlement_no=u.entitlement_no AND e.customer_no=u.customer_no AND e.version=u.source_version
   AND e.status_code='active' AND c.status_code='active' AND e.expires_at=u.period_expires_at AND e.expires_at>CURRENT_TIMESTAMP
   AND p.status_code='active' AND p.deleted_at IS NULL AND p.current_version_no=e.version_no
   AND u.status_code='pending_payment' AND u.quote_expires_at>CURRENT_TIMESTAMP
   FOR UPDATE OF e,p,c
  ) THEN RAISE EXCEPTION 'upgrade service or quote changed'; END IF;
 END IF;
 RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS plugin_idc_core_guard_upgrade_payment ON plugin_idc_core_billing_invoice;
CREATE TRIGGER plugin_idc_core_guard_upgrade_payment BEFORE UPDATE ON plugin_idc_core_billing_invoice
 FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_upgrade_payment();
