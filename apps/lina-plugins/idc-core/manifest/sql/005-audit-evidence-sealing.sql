-- Audit rows are inserted unsealed only long enough to obtain their generated
-- identity. Both deferred events inspect the final row state at commit, so an
-- ignored application error can never leave a readable unsigned audit row.

CREATE OR REPLACE FUNCTION plugin_idc_core_require_sealed_audit_event()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    "target_id" BIGINT;
    stored_context JSONB;
BEGIN
    IF TG_OP = 'DELETE' THEN
        "target_id" := OLD."id";
    ELSE
        "target_id" := NEW."id";
    END IF;

    SELECT event."context_snapshot"
      INTO stored_context
      FROM plugin_idc_core_business_audit_event AS event
     WHERE event."id" = "target_id";

    IF stored_context IS NULL
       OR stored_context = '{}'::JSONB
       OR COALESCE(stored_context #>> '{schema_version}', '') <> '1'
       OR COALESCE(stored_context #>> '{integrity,version}', '') <> '1'
       OR COALESCE(stored_context #>> '{integrity,key_id}', '') !~ '^[a-z0-9][a-z0-9._:-]{0,63}$'
       OR COALESCE(stored_context #>> '{integrity,replay_key_id}', '') !~ '^[a-z0-9][a-z0-9._:-]{0,63}$'
       OR COALESCE(stored_context #>> '{integrity,raw_replay_hmac}', '') !~ '^[0-9a-f]{64}$'
       OR COALESCE(stored_context #>> '{integrity,row_hmac}', '') !~ '^[0-9a-f]{64}$' THEN
        RAISE EXCEPTION 'audit event % cannot be committed without a valid integrity seal', "target_id"
            USING ERRCODE = '23514';
    END IF;

    RETURN NULL;
END;
$$;

-- PostgreSQL kept the old overlong trigger name when the associated
-- constraint received its final unbranded name. Repair only that exact
-- catalog identity in place so the constraint OID, deferral semantics, and
-- historical evidence remain untouched.
DO $audit_trigger_name$
DECLARE
    current_constraint_oid OID;
    legacy_trigger_oid OID;
    current_trigger_oid OID;
BEGIN
    SELECT constraint_row.oid
      INTO current_constraint_oid
      FROM pg_constraint AS constraint_row
     WHERE constraint_row.conrelid = 'plugin_idc_core_business_audit_event'::regclass
       AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
       AND constraint_row.contype = 't';

    IF current_constraint_oid IS NOT NULL THEN
        SELECT trigger_row.oid
          INTO legacy_trigger_oid
          FROM pg_trigger AS trigger_row
         WHERE trigger_row.tgconstraint = current_constraint_oid
           AND trigger_row.tgrelid = 'plugin_idc_core_business_audit_event'::regclass
           AND trigger_row.tgname = 'trg_plugin_linapro_idc_core_require_sealed_audit_event'
           AND NOT trigger_row.tgisinternal
           AND trigger_row.tgfoid = 'plugin_idc_core_require_sealed_audit_event()'::regprocedure;
        SELECT trigger_row.oid
          INTO current_trigger_oid
          FROM pg_trigger AS trigger_row
         WHERE trigger_row.tgconstraint = current_constraint_oid
           AND trigger_row.tgrelid = 'plugin_idc_core_business_audit_event'::regclass
           AND trigger_row.tgname = 'trg_plugin_idc_core_require_sealed_audit_event'
           AND NOT trigger_row.tgisinternal
           AND trigger_row.tgfoid = 'plugin_idc_core_require_sealed_audit_event()'::regprocedure;

        IF legacy_trigger_oid IS NOT NULL AND current_trigger_oid IS NOT NULL THEN
            RAISE EXCEPTION 'audit evidence constraint trigger naming is ambiguous'
                USING ERRCODE = '23514';
        ELSIF legacy_trigger_oid IS NOT NULL THEN
            ALTER TRIGGER trg_plugin_linapro_idc_core_require_sealed_audit_event
                ON plugin_idc_core_business_audit_event
                RENAME TO trg_plugin_idc_core_require_sealed_audit_event;
        ELSIF current_trigger_oid IS NULL THEN
            RAISE EXCEPTION 'audit evidence constraint trigger catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    END IF;
END;
$audit_trigger_name$;

DO $constraint_trigger$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM pg_constraint
         WHERE conrelid = 'plugin_idc_core_business_audit_event'::regclass
           AND conname = 'trg_plugin_idc_core_require_sealed_audit_event'
           AND contype = 't'
    ) THEN
        IF NOT EXISTS (
            SELECT 1
              FROM pg_constraint AS constraint_row
              JOIN pg_trigger AS trigger_row
                ON trigger_row.tgconstraint = constraint_row.oid
             WHERE constraint_row.conrelid = 'plugin_idc_core_business_audit_event'::regclass
               AND constraint_row.conname = 'trg_plugin_idc_core_require_sealed_audit_event'
               AND constraint_row.contype = 't'
               AND trigger_row.tgrelid = constraint_row.conrelid
               AND NOT trigger_row.tgisinternal
               AND trigger_row.tgname = 'trg_plugin_idc_core_require_sealed_audit_event'
               AND trigger_row.tgfoid = 'plugin_idc_core_require_sealed_audit_event()'::regprocedure
        ) THEN
            RAISE EXCEPTION 'constraint trigger trg_plugin_idc_core_require_sealed_audit_event catalog contract is invalid'
                USING ERRCODE = '23514';
        END IF;
    ELSE
        CREATE CONSTRAINT TRIGGER trg_plugin_idc_core_require_sealed_audit_event
        AFTER INSERT OR UPDATE OR DELETE
        ON plugin_idc_core_business_audit_event
        DEFERRABLE INITIALLY DEFERRED
        FOR EACH ROW
        EXECUTE FUNCTION plugin_idc_core_require_sealed_audit_event();
    END IF;
END;
$constraint_trigger$;

COMMENT ON COLUMN plugin_idc_core_business_number."reservation_scope_digest"
    IS 'Versioned keyed HMAC of the normalized caller or command scope';
COMMENT ON COLUMN plugin_idc_core_business_number."idempotency_key_digest"
    IS 'Versioned keyed HMAC of the number reservation idempotency key';
