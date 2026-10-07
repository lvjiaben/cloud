ALTER TABLE idc_gateway_subject_binding
    DROP CONSTRAINT IF EXISTS ck_gateway_subject_cipher;

ALTER TABLE idc_gateway_subject_binding
    ADD CONSTRAINT ck_gateway_subject_cipher CHECK (
        octet_length("subject_ciphertext") BETWEEN 32 AND 256
        AND "cipher_version" = 1
    );

CREATE OR REPLACE FUNCTION idc_gateway_guard_subject_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."binding_no" IS DISTINCT FROM NEW."binding_no"
        OR OLD."instance_no" IS DISTINCT FROM NEW."instance_no"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."subject_ciphertext" IS DISTINCT FROM NEW."subject_ciphertext"
        OR OLD."subject_lookup_hmac" IS DISTINCT FROM NEW."subject_lookup_hmac"
        OR OLD."cipher_version" IS DISTINCT FROM NEW."cipher_version"
        OR OLD.bound_at IS DISTINCT FROM NEW.bound_at
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD."status" <> 'active'
        OR NEW."status" <> 'revoked'
    THEN
        RAISE EXCEPTION 'gateway subject binding identity and ciphertext are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_subject_guard_update ON idc_gateway_subject_binding;
CREATE TRIGGER trg_gateway_subject_guard_update
BEFORE UPDATE ON idc_gateway_subject_binding
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_subject_update();
