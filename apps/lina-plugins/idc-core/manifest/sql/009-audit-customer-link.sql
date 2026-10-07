-- 009: Immutable customer lookup links for one bounded batch audit event.

-- Purpose: Immutable customer lookup links covered by one sealed bounded batch audit event.
-- 用途：保存业务审计客户关联相关数据。
CREATE TABLE IF NOT EXISTS plugin_idc_core_business_audit_customer_link (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "audit_no" VARCHAR(40) NOT NULL,
    "customer_no" VARCHAR(40) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_core_audit_customer_link_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT uk_plugin_idc_core_audit_customer_link UNIQUE ("audit_no", "customer_no"),
    CONSTRAINT fk_plugin_idc_core_audit_customer_link_audit FOREIGN KEY ("audit_no")
        REFERENCES plugin_idc_core_business_audit_event ("audit_no") ON DELETE RESTRICT,
    CONSTRAINT fk_plugin_idc_core_audit_customer_link_customer FOREIGN KEY ("customer_no")
        REFERENCES plugin_idc_core_customer_subject ("customer_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_plugin_idc_core_audit_customer_link_search
    ON plugin_idc_core_business_audit_customer_link (
        "tenant_id", "customer_no", "audit_no"
    );

COMMENT ON TABLE plugin_idc_core_business_audit_customer_link IS
    'Immutable customer lookup links covered by one sealed bounded batch audit event';

CREATE OR REPLACE FUNCTION plugin_idc_core_validate_audit_customer_link_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM inserted_audit_customer_links AS link
          LEFT JOIN plugin_idc_core_business_audit_event AS event
            ON event."tenant_id" = link."tenant_id"
           AND event."audit_no" = link."audit_no"
         WHERE event."id" IS NULL
            OR event."context_snapshot" <> '{}'::JSONB
    ) THEN
        RAISE EXCEPTION 'audit customer links can only be attached before event sealing';
    END IF;
    RETURN NULL;
END;
$$;

CREATE OR REPLACE FUNCTION plugin_idc_core_reject_audit_customer_link_mutation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    RAISE EXCEPTION 'audit customer links are append-only';
END;
$$;

DO $legacy_audit_customer_trigger_names$
DECLARE
    trigger_contract RECORD;
    legacy_trigger pg_trigger%ROWTYPE;
    current_trigger pg_trigger%ROWTYPE;
BEGIN
    FOR trigger_contract IN
        SELECT * FROM (VALUES
            ('trg_plugin_linapro_idc_core_validate_audit_customer_link_insert',
             'trg_plugin_idc_core_validate_audit_customer_link_insert',
             'plugin_idc_core_validate_audit_customer_link_insert()'::REGPROCEDURE),
            ('trg_plugin_linapro_idc_core_reject_audit_customer_link_update',
             'trg_plugin_idc_core_reject_audit_customer_link_update',
             'plugin_idc_core_reject_audit_customer_link_mutation()'::REGPROCEDURE),
            ('trg_plugin_linapro_idc_core_reject_audit_customer_link_delete',
             'trg_plugin_idc_core_reject_audit_customer_link_delete',
             'plugin_idc_core_reject_audit_customer_link_mutation()'::REGPROCEDURE)
        ) AS contracts(legacy_name, current_name, function_oid)
    LOOP
        SELECT * INTO legacy_trigger
          FROM pg_trigger
         WHERE tgrelid = 'plugin_idc_core_business_audit_customer_link'::REGCLASS
           AND tgname = trigger_contract.legacy_name;
        IF FOUND THEN
            IF legacy_trigger.tgisinternal
               OR legacy_trigger.tgconstraint <> 0
               OR legacy_trigger.tgfoid <> trigger_contract.function_oid THEN
                RAISE EXCEPTION 'legacy audit customer trigger catalog contract is invalid'
                    USING ERRCODE = '23514';
            END IF;

            SELECT * INTO current_trigger
              FROM pg_trigger
             WHERE tgrelid = 'plugin_idc_core_business_audit_customer_link'::REGCLASS
               AND tgname = trigger_contract.current_name;
            IF FOUND AND (
                current_trigger.tgisinternal
                OR current_trigger.tgconstraint <> 0
                OR current_trigger.tgfoid <> trigger_contract.function_oid
                OR ROW(current_trigger.tgtype, current_trigger.tgnargs,
                       current_trigger.tgargs, current_trigger.tgattr::TEXT,
                       current_trigger.tgqual::TEXT, current_trigger.tgoldtable,
                       current_trigger.tgnewtable)
                   IS DISTINCT FROM
                   ROW(legacy_trigger.tgtype, legacy_trigger.tgnargs,
                       legacy_trigger.tgargs, legacy_trigger.tgattr::TEXT,
                       legacy_trigger.tgqual::TEXT, legacy_trigger.tgoldtable,
                       legacy_trigger.tgnewtable)
            ) THEN
                RAISE EXCEPTION 'legacy and current audit customer triggers are not equivalent'
                    USING ERRCODE = '23514';
            END IF;

            EXECUTE format(
                'DROP TRIGGER %I ON plugin_idc_core_business_audit_customer_link',
                trigger_contract.legacy_name
            );
        END IF;
    END LOOP;
END;
$legacy_audit_customer_trigger_names$;

DROP TRIGGER IF EXISTS trg_plugin_idc_core_validate_audit_customer_link_insert
    ON plugin_idc_core_business_audit_customer_link;
CREATE TRIGGER trg_plugin_idc_core_validate_audit_customer_link_insert
AFTER INSERT ON plugin_idc_core_business_audit_customer_link
REFERENCING NEW TABLE AS inserted_audit_customer_links
FOR EACH STATEMENT
EXECUTE FUNCTION plugin_idc_core_validate_audit_customer_link_insert();

DROP TRIGGER IF EXISTS trg_plugin_idc_core_reject_audit_customer_link_update
    ON plugin_idc_core_business_audit_customer_link;
CREATE TRIGGER trg_plugin_idc_core_reject_audit_customer_link_update
BEFORE UPDATE ON plugin_idc_core_business_audit_customer_link
FOR EACH STATEMENT
EXECUTE FUNCTION plugin_idc_core_reject_audit_customer_link_mutation();

DROP TRIGGER IF EXISTS trg_plugin_idc_core_reject_audit_customer_link_delete
    ON plugin_idc_core_business_audit_customer_link;
CREATE TRIGGER trg_plugin_idc_core_reject_audit_customer_link_delete
BEFORE DELETE ON plugin_idc_core_business_audit_customer_link
FOR EACH STATEMENT
EXECUTE FUNCTION plugin_idc_core_reject_audit_customer_link_mutation();
