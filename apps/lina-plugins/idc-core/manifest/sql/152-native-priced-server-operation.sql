-- 152: Native entitlement IP/resize quotes share the immutable priced quote
-- facts without fabricating channel attribution or changing addon policies.
CREATE TABLE IF NOT EXISTS plugin_idc_core_native_server_target_quote (
 tenant_id INT NOT NULL DEFAULT 0,
 id BIGSERIAL UNIQUE,
 target_quote_no VARCHAR(40) PRIMARY KEY,
 quote_item_no VARCHAR(40) NOT NULL UNIQUE,
 structured_configuration_no VARCHAR(40) NOT NULL UNIQUE,
 entitlement_no VARCHAR(40) NOT NULL,
 customer_no VARCHAR(40) NOT NULL,
 expected_entitlement_version BIGINT NOT NULL,
 customer_subject_version BIGINT NOT NULL,
 currency_code CHAR(3) NOT NULL,
 native_target_snapshot JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 CONSTRAINT ck_idc_native_target_quote_identity CHECK (COALESCE(tenant_id=0
  AND expected_entitlement_version>0 AND customer_subject_version>0
  AND jsonb_typeof(native_target_snapshot)='object'
  AND native_target_snapshot->>'EntitlementNo'=entitlement_no
  AND native_target_snapshot->>'CustomerNo'=customer_no
  AND (native_target_snapshot->>'EntitlementVersion')::BIGINT=expected_entitlement_version
  AND octet_length(native_target_snapshot::TEXT)<=65536,FALSE)),
 CONSTRAINT fk_idc_native_target_quote_quote FOREIGN KEY(target_quote_no)
  REFERENCES plugin_idc_core_commerce_quote_snapshot(quote_no) ON DELETE RESTRICT,
 CONSTRAINT fk_idc_native_target_quote_item FOREIGN KEY(quote_item_no)
  REFERENCES plugin_idc_core_commerce_quote_item_snapshot(quote_item_no) ON DELETE RESTRICT,
 CONSTRAINT fk_idc_native_target_quote_structured FOREIGN KEY(structured_configuration_no)
  REFERENCES plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no) ON DELETE RESTRICT,
 CONSTRAINT fk_idc_native_target_quote_entitlement FOREIGN KEY(entitlement_no)
  REFERENCES plugin_idc_core_service_entitlement(entitlement_no) ON DELETE RESTRICT,
 CONSTRAINT fk_idc_native_target_quote_customer FOREIGN KEY(customer_no)
  REFERENCES plugin_idc_core_customer_subject(customer_no) ON DELETE RESTRICT
);

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_native_target_quote()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP<>'INSERT' THEN
  RAISE EXCEPTION 'native server target quote bindings are immutable';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM plugin_idc_core_commerce_quote_snapshot q
  JOIN plugin_idc_core_commerce_quote_item_snapshot i ON i.tenant_id=q.tenant_id AND i.quote_no=q.quote_no
  JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc ON sc.tenant_id=i.tenant_id AND sc.quote_item_no=i.quote_item_no
  JOIN plugin_idc_core_service_entitlement e ON e.tenant_id=q.tenant_id AND e.entitlement_no=NEW.entitlement_no
  JOIN plugin_idc_core_customer_subject c ON c.tenant_id=e.tenant_id AND c.customer_no=e.customer_no
  WHERE q.tenant_id=NEW.tenant_id AND q.quote_no=NEW.target_quote_no AND q.purpose_code='change' AND q.source_code IN ('customer','staff') AND q.created_by>0
   AND q.customer_no=NEW.customer_no AND q.customer_subject_version=NEW.customer_subject_version AND q.item_count=1
   AND i.quote_item_no=NEW.quote_item_no AND i.product_no=e.product_no AND i.version_no=e.version_no
   AND sc.snapshot_no=NEW.structured_configuration_no AND sc.customer_no=e.customer_no AND sc.order_no IS NULL
   AND e.customer_no=NEW.customer_no AND e.version=NEW.expected_entitlement_version AND e.entitlement_owner_id='idc-resource'
   AND c.version=NEW.customer_subject_version AND q.currency_code=NEW.currency_code AND e.currency_code=NEW.currency_code) THEN
  RAISE EXCEPTION 'native server target quote identity is invalid';
 END IF;
 RETURN NEW;
END; $$;
DROP TRIGGER IF EXISTS trg_idc_native_target_quote ON plugin_idc_core_native_server_target_quote;
CREATE TRIGGER trg_idc_native_target_quote BEFORE INSERT OR UPDATE OR DELETE
 ON plugin_idc_core_native_server_target_quote FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_native_target_quote();
COMMENT ON TABLE plugin_idc_core_native_server_target_quote IS
 'Immutable real owned change quote and typed configuration bound to one native server revision';

ALTER TABLE plugin_idc_core_channel_service_quote
 ADD COLUMN IF NOT EXISTS source_code VARCHAR(16) NOT NULL DEFAULT 'channel',
 ADD COLUMN IF NOT EXISTS native_target_snapshot JSONB,
 ADD COLUMN IF NOT EXISTS prepared_price JSONB,
 ADD COLUMN IF NOT EXISTS native_target_quote_no VARCHAR(40) REFERENCES plugin_idc_core_native_server_target_quote(target_quote_no) ON DELETE RESTRICT,
 ALTER COLUMN channel_id DROP NOT NULL;

ALTER TABLE plugin_idc_core_channel_service_quote
 DROP CONSTRAINT IF EXISTS ck_idc_native_priced_quote_source,
 ADD CONSTRAINT ck_idc_native_priced_quote_source CHECK (COALESCE((
  (source_code='channel' AND channel_id IS NOT NULL AND length(btrim(channel_id))>0
   AND native_target_snapshot IS NULL AND prepared_price IS NULL AND native_target_quote_no IS NULL)
  OR
  (source_code='native' AND channel_id IS NULL AND payer_customer_no=service_owner_customer_no
   AND payer_customer_subject_version=customer_subject_version
   AND native_target_snapshot IS NOT NULL AND prepared_price IS NOT NULL
   AND jsonb_typeof(native_target_snapshot)='object'
   AND native_target_snapshot->>'EntitlementNo'=service_no
   AND native_target_snapshot->>'CustomerNo'=service_owner_customer_no
   AND (native_target_snapshot->>'EntitlementVersion')::BIGINT=expected_service_version
   AND native_target_snapshot->>'ChannelNo'=resource_channel_no
   AND native_target_snapshot->>'PoolNo'=resource_pool_no
   AND length(native_target_snapshot->>'LocalResourceNo') BETWEEN 1 AND 128
   AND jsonb_typeof(prepared_price)='object'
   AND prepared_price->>'Action'=action_code
   AND prepared_price->'Context'->>'EntitlementNo'=service_no
   AND prepared_price->'Context'->>'CustomerNo'=service_owner_customer_no
   AND prepared_price->'Context'->>'LocalResourceNo'=native_target_snapshot->>'LocalResourceNo'
   AND prepared_price->'Context'->>'ChannelNo'=resource_channel_no
   AND prepared_price->'Context'->>'PoolNo'=resource_pool_no
   AND (prepared_price->>'TotalAmountMinor')::BIGINT=amount_minor
   AND ((action_code='ip_change' AND native_target_quote_no IS NULL)
     OR (action_code='resize' AND native_target_quote_no IS NOT NULL
      AND prepared_price->>'TargetQuoteNo'=native_target_quote_no
      AND length(prepared_price->>'TargetStructuredConfigurationNo') BETWEEN 1 AND 40))
   AND octet_length(native_target_snapshot::TEXT)<=65536
   AND octet_length(prepared_price::TEXT)<=1048576)
 ),FALSE));
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_native_priced_quote_request
 ON plugin_idc_core_channel_service_quote(tenant_id,service_owner_customer_no,idempotency_digest)
 WHERE source_code='native';
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_native_priced_target_quote
 ON plugin_idc_core_channel_service_quote(native_target_quote_no)
 WHERE native_target_quote_no IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_native_priced_quote_entitlement
 ON plugin_idc_core_channel_service_quote(tenant_id,service_no,expires_at)
 WHERE source_code='native';
COMMENT ON COLUMN plugin_idc_core_channel_service_quote.source_code IS
 'Actual admission owner: channel retains real attribution; native has no channel identity';
COMMENT ON COLUMN plugin_idc_core_channel_service_quote.prepared_price IS
 'Immutable core price, configuration, merchant policy and local IP reservation facts for native admission';

-- Initial native resource task admission may fill the frozen route once. All
-- commercial identities and every later task/route/evidence assignment stay fixed.
CREATE OR REPLACE FUNCTION plugin_idc_core_guard_business_operation_intent()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    frozen_target JSONB;
    frozen_digest TEXT;
    native_quote BOOLEAN := FALSE;
    initial_binding BOOLEAN := FALSE;
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'business operations cannot be deleted';
    END IF;
    IF OLD.subject_type_code='entitlement' AND OLD.resource_owner_code='idc-resource' THEN
        IF OLD.operation_type_code='addon' THEN
            SELECT quote.native_target_snapshot,quote.content_digest
              INTO frozen_target,frozen_digest
            FROM plugin_idc_core_server_accessory_quote quote
            WHERE quote.tenant_id=OLD.tenant_id AND quote.quote_no=OLD.quote_no
              AND quote.subject_type_code='entitlement' AND quote.subject_no=OLD.subject_no
              AND quote.customer_no=OLD.customer_no AND quote.currency_code=OLD.currency_code
              AND quote.expected_subject_version=OLD.expected_subject_version;
        ELSIF OLD.operation_type_code IN ('resize','ip_change') THEN
            SELECT quote.native_target_snapshot,quote.content_digest
              INTO frozen_target,frozen_digest
            FROM plugin_idc_core_channel_service_quote quote
            WHERE quote.tenant_id=OLD.tenant_id AND quote.quote_no=OLD.quote_no
              AND quote.source_code='native' AND quote.service_no=OLD.subject_no
              AND quote.service_owner_customer_no=OLD.customer_no
              AND quote.currency_code=OLD.currency_code
              AND quote.expected_service_version=OLD.expected_subject_version
              AND quote.action_code=OLD.operation_type_code;
        END IF;
        native_quote := frozen_target IS NOT NULL;
        initial_binding := COALESCE(native_quote
          AND OLD.status_code='ready' AND NEW.status_code='executing'
          AND OLD.execution_mode_code='unresolved' AND NEW.execution_mode_code='provider_confirmed'
          AND OLD.provider_task_no IS NULL AND NEW.provider_task_no IS NOT NULL
          AND length(btrim(NEW.provider_task_no)) BETWEEN 1 AND 64
          AND OLD.channel_no IS NULL AND OLD.capability_evidence_no IS NULL
          AND OLD.capability_evidence_digest IS NULL
          AND frozen_target->>'EntitlementNo'=OLD.subject_no
          AND frozen_target->>'CustomerNo'=OLD.customer_no
          AND (frozen_target->>'EntitlementVersion')::BIGINT=OLD.expected_subject_version
          AND NEW.channel_no=frozen_target->>'ChannelNo'
          AND NEW.pool_no=frozen_target->>'PoolNo'
          AND NEW.capability_evidence_no=OLD.quote_no
          AND NEW.capability_evidence_digest=frozen_digest
          AND NEW.version=OLD.version+1,FALSE);
    END IF;
    IF ROW(OLD.tenant_id,OLD.operation_no,OLD.operation_type_code,OLD.customer_no,
           OLD.payer_customer_no,OLD.service_no,OLD.subject_type_code,OLD.subject_no,OLD.currency_code,
           OLD.price_snapshot_no,OLD.quote_no,OLD.order_no,OLD.invoice_no,
           OLD.financial_settlement_required,OLD.resource_owner_code,OLD.pool_no,
           OLD.idempotency_digest,OLD.request_digest,OLD.content_digest,
           OLD.expected_service_version,OLD.expected_subject_version,OLD.created_at)
       IS DISTINCT FROM
       ROW(NEW.tenant_id,NEW.operation_no,NEW.operation_type_code,NEW.customer_no,
           NEW.payer_customer_no,NEW.service_no,NEW.subject_type_code,NEW.subject_no,NEW.currency_code,
           NEW.price_snapshot_no,NEW.quote_no,NEW.order_no,NEW.invoice_no,
           NEW.financial_settlement_required,NEW.resource_owner_code,NEW.pool_no,
           NEW.idempotency_digest,NEW.request_digest,NEW.content_digest,
           NEW.expected_service_version,NEW.expected_subject_version,NEW.created_at) THEN
        RAISE EXCEPTION 'business operation intent is immutable';
    END IF;
    IF NOT initial_binding AND ROW(OLD.execution_mode_code,OLD.channel_no,
            OLD.capability_evidence_no,OLD.capability_evidence_digest)
        IS DISTINCT FROM ROW(NEW.execution_mode_code,NEW.channel_no,
            NEW.capability_evidence_no,NEW.capability_evidence_digest) THEN
        RAISE EXCEPTION 'business operation execution binding is immutable';
    END IF;
    IF native_quote AND NOT initial_binding AND OLD.provider_task_no IS DISTINCT FROM NEW.provider_task_no THEN
        RAISE EXCEPTION 'native business operation task is immutable';
    END IF;
    RETURN NEW;
END; $$;
