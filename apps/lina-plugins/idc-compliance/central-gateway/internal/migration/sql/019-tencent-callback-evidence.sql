-- This migration keeps the original 256 KiB callback-body bound while allowing
-- one locally encrypted frame to retain both exact signed bytes and verified plaintext.

ALTER TABLE idc_gateway_local_callback_evidence
    DROP CONSTRAINT ck_gateway_local_evidence_cipher;

ALTER TABLE idc_gateway_local_callback_evidence
    ADD CONSTRAINT ck_gateway_local_evidence_cipher CHECK (
        octet_length("payload_ciphertext") BETWEEN 17 AND 561168
        AND octet_length("payload_nonce") = 12
        AND "cipher_format_version" = 1
        AND encryption_key_version > 0
    );
