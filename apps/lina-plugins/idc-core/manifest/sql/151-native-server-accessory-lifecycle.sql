-- Native server accessories reuse the existing commercial documents and object
-- lifecycle. Their subject is a real generic entitlement, never a legacy server.
ALTER TABLE plugin_idc_core_server_accessory_quote
    ADD COLUMN IF NOT EXISTS subject_type_code VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS subject_no VARCHAR(40),
    ADD COLUMN IF NOT EXISTS expected_subject_version BIGINT,
    ADD COLUMN IF NOT EXISTS native_target_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB,
    ALTER COLUMN service_no DROP NOT NULL;

ALTER TABLE plugin_idc_core_server_accessory_entitlement
    ADD COLUMN IF NOT EXISTS subject_type_code VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS subject_no VARCHAR(40),
    ALTER COLUMN service_no DROP NOT NULL;

-- A verified creation failure never bound a real object and releases its slot.
-- It is separate from deletion, preserves paid purchase evidence and never refunds automatically.
ALTER TABLE plugin_idc_core_server_accessory_entitlement
    DROP CONSTRAINT IF EXISTS ck_idc_server_accessory_entitlement_status,
    ADD CONSTRAINT ck_idc_server_accessory_entitlement_status CHECK (
        status_code IN ('pending_binding','active','payment_pending','manual_review','deleted','creation_failed','creation_cancelled','deletion_pending')
    ),
    DROP CONSTRAINT IF EXISTS ck_idc_server_accessory_entitlement_dates,
    ADD CONSTRAINT ck_idc_server_accessory_entitlement_dates CHECK (
        (billing_mode_code='free' AND current_period_start_at IS NULL AND current_period_end_at IS NULL)
        OR (billing_mode_code='paid' AND (
            (status_code IN ('pending_binding','manual_review','creation_failed','creation_cancelled')
                AND current_period_start_at IS NULL AND current_period_end_at IS NULL)
            OR (status_code IN ('active','payment_pending','deleted','deletion_pending')
                AND current_period_start_at IS NOT NULL AND current_period_end_at>current_period_start_at)
        ))
    ),
    DROP CONSTRAINT IF EXISTS ck_idc_accessory_creation_failed,
    ADD CONSTRAINT ck_idc_accessory_creation_failed CHECK (
        status_code NOT IN ('creation_failed','creation_cancelled') OR (object_no IS NULL AND NOT auto_renew_enabled)
    ),
    DROP CONSTRAINT IF EXISTS ck_idc_accessory_deletion_pending,
    ADD CONSTRAINT ck_idc_accessory_deletion_pending CHECK (
        status_code<>'deletion_pending' OR (object_no IS NOT NULL AND NOT auto_renew_enabled AND pending_invoice_no IS NOT NULL)
    );

ALTER TABLE plugin_idc_core_server_accessory_renewal_work_item
    ADD COLUMN IF NOT EXISTS subject_type_code VARCHAR(24) NOT NULL DEFAULT 'service',
    ADD COLUMN IF NOT EXISTS subject_no VARCHAR(40),
    ALTER COLUMN service_no DROP NOT NULL;

-- History is append-only. Disable its guard only inside this atomic statement;
-- any failure restores both the data and trigger state through transaction rollback.
DO $backfill$
BEGIN
    ALTER TABLE plugin_idc_core_server_accessory_quote DISABLE TRIGGER trg_idc_server_accessory_quote_immutable;
    UPDATE plugin_idc_core_server_accessory_quote
       SET subject_no=service_no,expected_subject_version=expected_service_version
     WHERE subject_type_code='service' AND subject_no IS NULL;
    ALTER TABLE plugin_idc_core_server_accessory_quote ENABLE TRIGGER trg_idc_server_accessory_quote_immutable;
    UPDATE plugin_idc_core_server_accessory_entitlement
       SET subject_no=service_no WHERE subject_type_code='service' AND subject_no IS NULL;
    UPDATE plugin_idc_core_server_accessory_renewal_work_item
       SET subject_no=service_no WHERE subject_type_code='service' AND subject_no IS NULL;
END;
$backfill$;

-- Legacy inserts retain their exact identity without callers knowing the new
-- fields. Native callers must provide their explicit subject and frozen version.
CREATE OR REPLACE FUNCTION plugin_idc_core_normalize_accessory_subject()
RETURNS TRIGGER LANGUAGE plpgsql AS $$ BEGIN
    IF NEW.subject_type_code='service' AND NEW.subject_no IS NULL THEN
        NEW.subject_no := NEW.service_no;
    END IF;
    IF TG_TABLE_NAME='plugin_idc_core_server_accessory_quote' THEN
        IF NEW.subject_type_code='service' AND NEW.expected_subject_version IS NULL THEN
            NEW.expected_subject_version := NEW.expected_service_version;
        END IF;
    END IF;
    RETURN NEW;
END; $$;

DROP TRIGGER IF EXISTS trg_idc_accessory_quote_subject_normalize ON plugin_idc_core_server_accessory_quote;
CREATE TRIGGER trg_idc_accessory_quote_subject_normalize BEFORE INSERT ON plugin_idc_core_server_accessory_quote
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_accessory_subject();
DROP TRIGGER IF EXISTS trg_idc_accessory_ent_subject_normalize ON plugin_idc_core_server_accessory_entitlement;
CREATE TRIGGER trg_idc_accessory_ent_subject_normalize BEFORE INSERT ON plugin_idc_core_server_accessory_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_accessory_subject();
DROP TRIGGER IF EXISTS trg_idc_accessory_work_subject_normalize ON plugin_idc_core_server_accessory_renewal_work_item;
CREATE TRIGGER trg_idc_accessory_work_subject_normalize BEFORE INSERT ON plugin_idc_core_server_accessory_renewal_work_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_normalize_accessory_subject();

ALTER TABLE plugin_idc_core_server_accessory_quote
    DROP CONSTRAINT IF EXISTS ck_idc_accessory_quote_subject,
    ADD CONSTRAINT ck_idc_accessory_quote_subject CHECK (
        subject_no IS NOT NULL AND length(btrim(subject_no)) BETWEEN 1 AND 40
        AND expected_subject_version IS NOT NULL AND expected_subject_version>0
        AND jsonb_typeof(native_target_snapshot)='object'
        AND octet_length(native_target_snapshot::TEXT)<=65536
        AND ((subject_type_code='service' AND service_no IS NOT NULL AND service_no=subject_no
              AND expected_subject_version=expected_service_version AND native_target_snapshot='{}'::JSONB)
          OR (subject_type_code='entitlement' AND service_no IS NULL
              AND native_target_snapshot<>'{}'::JSONB))
    );
ALTER TABLE plugin_idc_core_server_accessory_entitlement
    DROP CONSTRAINT IF EXISTS ck_idc_accessory_ent_subject,
    ADD CONSTRAINT ck_idc_accessory_ent_subject CHECK (
        subject_no IS NOT NULL AND length(btrim(subject_no)) BETWEEN 1 AND 40
        AND ((subject_type_code='service' AND service_no IS NOT NULL AND service_no=subject_no)
          OR (subject_type_code='entitlement' AND service_no IS NULL))
    );
ALTER TABLE plugin_idc_core_server_accessory_renewal_work_item
    DROP CONSTRAINT IF EXISTS ck_idc_accessory_work_subject,
    ADD CONSTRAINT ck_idc_accessory_work_subject CHECK (
        subject_no IS NOT NULL AND length(btrim(subject_no)) BETWEEN 1 AND 40
        AND ((subject_type_code='service' AND service_no IS NOT NULL AND service_no=subject_no)
          OR (subject_type_code='entitlement' AND service_no IS NULL))
    );

-- The shared 088 subject pattern validates a polymorphic FK with a trigger:
-- legacy rows retain their existing service FK; native rows must match core's
-- immutable entitlement identity, customer and currency in the same transaction.
CREATE OR REPLACE FUNCTION plugin_idc_core_validate_accessory_subject()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.subject_type_code='entitlement' THEN
        IF TG_TABLE_NAME='plugin_idc_core_server_accessory_renewal_work_item' THEN
            IF NOT EXISTS (
                SELECT 1 FROM plugin_idc_core_server_accessory_entitlement accessory
                WHERE accessory.tenant_id=NEW.tenant_id AND accessory.entitlement_no=NEW.entitlement_no
                  AND accessory.subject_type_code=NEW.subject_type_code AND accessory.subject_no=NEW.subject_no
                  AND accessory.customer_no=NEW.customer_no
            ) THEN
                RAISE EXCEPTION 'accessory work item subject is unavailable' USING ERRCODE='23503';
            END IF;
        ELSE
            IF NOT EXISTS (
                SELECT 1 FROM plugin_idc_core_service_entitlement owner
                WHERE owner.tenant_id=NEW.tenant_id AND owner.entitlement_no=NEW.subject_no
                  AND owner.customer_no=NEW.customer_no AND owner.currency_code=NEW.currency_code
                  AND owner.entitlement_owner_id='idc-resource'
            ) THEN
                RAISE EXCEPTION 'accessory entitlement subject is unavailable' USING ERRCODE='23503';
            END IF;
            IF TG_TABLE_NAME='plugin_idc_core_server_accessory_entitlement' THEN
                IF TG_OP='INSERT' AND NOT EXISTS (
                    SELECT 1 FROM plugin_idc_core_business_operation operation
                    WHERE operation.tenant_id=NEW.tenant_id AND operation.operation_no=NEW.create_operation_no
                      AND operation.subject_type_code=NEW.subject_type_code AND operation.subject_no=NEW.subject_no
                      AND operation.customer_no=NEW.customer_no AND operation.currency_code=NEW.currency_code
                      AND operation.operation_type_code='addon'
                ) THEN
                    RAISE EXCEPTION 'accessory create operation subject is unavailable' USING ERRCODE='23503';
                END IF;
                IF TG_OP='UPDATE' AND NEW.create_operation_no IS DISTINCT FROM OLD.create_operation_no THEN
                    RAISE EXCEPTION 'accessory create operation is immutable' USING ERRCODE='23514';
                END IF;
            END IF;
        END IF;
    END IF;
    IF TG_OP='UPDATE' AND ROW(NEW.subject_type_code,NEW.subject_no,NEW.service_no)
        IS DISTINCT FROM ROW(OLD.subject_type_code,OLD.subject_no,OLD.service_no) THEN
        RAISE EXCEPTION 'accessory subject is immutable' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END; $$;

DROP TRIGGER IF EXISTS trg_idc_accessory_quote_subject ON plugin_idc_core_server_accessory_quote;
CREATE TRIGGER trg_idc_accessory_quote_subject BEFORE INSERT OR UPDATE ON plugin_idc_core_server_accessory_quote
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_accessory_subject();
DROP TRIGGER IF EXISTS trg_idc_accessory_ent_subject ON plugin_idc_core_server_accessory_entitlement;
CREATE TRIGGER trg_idc_accessory_ent_subject BEFORE INSERT OR UPDATE ON plugin_idc_core_server_accessory_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_accessory_subject();
DROP TRIGGER IF EXISTS trg_idc_accessory_work_subject ON plugin_idc_core_server_accessory_renewal_work_item;
CREATE TRIGGER trg_idc_accessory_work_subject BEFORE INSERT OR UPDATE ON plugin_idc_core_server_accessory_renewal_work_item
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_validate_accessory_subject();

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_accessory_subject_object
    ON plugin_idc_core_server_accessory_entitlement(tenant_id,subject_type_code,subject_no,accessory_kind,object_no)
    WHERE object_no IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_accessory_subject_occupancy
    ON plugin_idc_core_server_accessory_entitlement(tenant_id,subject_type_code,subject_no,accessory_kind,status_code,id);
CREATE INDEX IF NOT EXISTS idx_idc_accessory_quote_subject
    ON plugin_idc_core_server_accessory_quote(tenant_id,subject_type_code,subject_no,issued_at DESC,id DESC);

-- Current-owner keyset pages never scan legacy service history or unbound objects.
CREATE INDEX IF NOT EXISTS idx_idc_native_commercial_history
    ON plugin_idc_core_business_operation(tenant_id,subject_no,customer_no,id DESC)
    WHERE subject_type_code='entitlement' AND service_no IS NULL
      AND resource_owner_code='idc-resource' AND operation_type_code IN ('addon','resize','ip_change');
CREATE INDEX IF NOT EXISTS idx_idc_native_accessory_billing
    ON plugin_idc_core_server_accessory_entitlement(tenant_id,subject_no,customer_no,id DESC)
    WHERE subject_type_code='entitlement' AND object_no IS NOT NULL;

-- Native recovery reads one rotating batch and one original financial owner,
-- without scanning historical operations or all refunds owned by a customer.
CREATE INDEX IF NOT EXISTS idx_idc_native_ready_recovery
    ON plugin_idc_core_business_operation(tenant_id,updated_at,operation_no)
    WHERE subject_type_code='entitlement' AND service_no IS NULL
      AND resource_owner_code='idc-resource' AND operation_type_code IN ('addon','resize','ip_change')
      AND status_code IN ('ready','manual_review') AND provider_task_no IS NULL AND execution_started_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_idc_refund_order_evidence
    ON plugin_idc_core_refund_request(tenant_id,order_no,invoice_no,customer_no,currency_code,refund_no)
    WHERE status_code<>'rejected';
CREATE INDEX IF NOT EXISTS idx_idc_refund_posted_command
    ON plugin_idc_core_billing_refund_allocation(tenant_id,refund_source_reference,invoice_no)
    WHERE direction_code='apply';
CREATE INDEX IF NOT EXISTS idx_idc_refund_posted_reversal
    ON plugin_idc_core_billing_refund_allocation(tenant_id,original_refund_allocation_no)
    WHERE direction_code='reverse';

COMMENT ON COLUMN plugin_idc_core_server_accessory_quote.native_target_snapshot
    IS 'Source-validated frozen native resource route, directory and local server evidence; never accepted from HTTP';

-- Ordinary resource writes have no commercial operation. Keep their admitted
-- identity and verified outcome after the source transaction commits, so refund
-- and ownership changes cannot cross an executing or uncertain provider write.
CREATE TABLE IF NOT EXISTS plugin_idc_core_native_server_write_fence (
    tenant_id INT NOT NULL DEFAULT 0 CHECK (tenant_id=0),
    task_no VARCHAR(64) PRIMARY KEY,
    operation_no VARCHAR(40) NOT NULL UNIQUE,
    entitlement_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_service_entitlement(entitlement_no) ON DELETE RESTRICT,
    customer_no VARCHAR(40) NOT NULL REFERENCES plugin_idc_core_customer_subject(customer_no) ON DELETE RESTRICT,
    expected_entitlement_version BIGINT NOT NULL CHECK (expected_entitlement_version>0),
    action_code VARCHAR(40) NOT NULL CHECK (action_code IN (
        'start','stop','reboot','reset_password','reinstall','enter_rescue','exit_rescue','console',
        'security_group_attach','security_group_detach','nat_create','nat_delete',
        'virtual_media_mount','virtual_media_unmount','snapshot_delete','snapshot_restore','backup_delete','backup_restore')),
    request_digest CHAR(64) NOT NULL CHECK (request_digest ~ '^[0-9a-f]{64}$'),
    status_code VARCHAR(24) NOT NULL DEFAULT 'executing' CHECK (status_code IN ('executing','result_unknown','succeeded','failed')),
    evidence_digest CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ,
    CONSTRAINT ck_idc_native_write_identity CHECK (length(btrim(task_no))>0 AND operation_no LIKE 'nso\_%' ESCAPE '\'),
    CONSTRAINT ck_idc_native_write_receipt CHECK (
        (status_code='executing' AND evidence_digest IS NULL AND completed_at IS NULL)
        OR (status_code='result_unknown' AND evidence_digest ~ '^[0-9a-f]{64}$' AND evidence_digest IS NOT NULL AND completed_at IS NULL)
        OR (status_code IN ('succeeded','failed') AND evidence_digest ~ '^[0-9a-f]{64}$' AND evidence_digest IS NOT NULL AND completed_at IS NOT NULL))
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_native_write_active_parent
    ON plugin_idc_core_native_server_write_fence(tenant_id,entitlement_no)
    WHERE status_code IN ('executing','result_unknown');
CREATE INDEX IF NOT EXISTS idx_idc_native_write_reconcile
    ON plugin_idc_core_native_server_write_fence(tenant_id,updated_at,task_no)
    WHERE status_code IN ('executing','result_unknown');

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_native_write_fence()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN
        RAISE EXCEPTION 'native write history is retained' USING ERRCODE='23514';
    END IF;
    IF TG_OP='INSERT' THEN
        IF NEW.status_code<>'executing' OR NOT EXISTS (
            SELECT 1 FROM plugin_idc_core_service_entitlement owner
            WHERE owner.tenant_id=NEW.tenant_id AND owner.entitlement_no=NEW.entitlement_no
              AND owner.customer_no=NEW.customer_no AND owner.entitlement_owner_id='idc-resource'
        ) THEN
            RAISE EXCEPTION 'native write subject is unavailable' USING ERRCODE='23503';
        END IF;
    ELSE
        IF ROW(NEW.tenant_id,NEW.task_no,NEW.operation_no,NEW.entitlement_no,NEW.customer_no,
               NEW.expected_entitlement_version,NEW.action_code,NEW.request_digest,NEW.created_at)
           IS DISTINCT FROM ROW(OLD.tenant_id,OLD.task_no,OLD.operation_no,OLD.entitlement_no,OLD.customer_no,
               OLD.expected_entitlement_version,OLD.action_code,OLD.request_digest,OLD.created_at)
           OR (OLD.status_code IN ('succeeded','failed') AND ROW(NEW.status_code,NEW.evidence_digest,NEW.completed_at)
               IS DISTINCT FROM ROW(OLD.status_code,OLD.evidence_digest,OLD.completed_at))
           OR (OLD.status_code='result_unknown' AND NEW.status_code='executing') THEN
            RAISE EXCEPTION 'native write identity or receipt is immutable' USING ERRCODE='23514';
        END IF;
    END IF;
    RETURN NEW;
END; $$;
DROP TRIGGER IF EXISTS trg_idc_native_write_fence ON plugin_idc_core_native_server_write_fence;
CREATE TRIGGER trg_idc_native_write_fence BEFORE INSERT OR UPDATE OR DELETE ON plugin_idc_core_native_server_write_fence
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_native_write_fence();

CREATE OR REPLACE FUNCTION plugin_idc_core_guard_native_write_owner()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.tenant_id,NEW.customer_no,NEW.entitlement_owner_id) IS DISTINCT FROM ROW(OLD.tenant_id,OLD.customer_no,OLD.entitlement_owner_id)
       AND EXISTS (SELECT 1 FROM plugin_idc_core_native_server_write_fence fence
           WHERE fence.tenant_id=OLD.tenant_id AND fence.entitlement_no=OLD.entitlement_no
             AND fence.status_code IN ('executing','result_unknown')) THEN
        RAISE EXCEPTION 'native write owner is busy' USING ERRCODE='23514';
    END IF;
    RETURN NEW;
END; $$;
DROP TRIGGER IF EXISTS trg_idc_native_write_owner ON plugin_idc_core_service_entitlement;
CREATE TRIGGER trg_idc_native_write_owner BEFORE UPDATE ON plugin_idc_core_service_entitlement
FOR EACH ROW EXECUTE FUNCTION plugin_idc_core_guard_native_write_owner();
