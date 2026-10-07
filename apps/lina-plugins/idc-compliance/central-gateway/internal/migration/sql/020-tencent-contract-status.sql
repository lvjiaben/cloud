-- Preserve the exact safe Tencent contract status beside the provider-neutral
-- route projection so callback and active-query deliveries keep domain meaning.

ALTER TABLE idc_gateway_local_supplier_route
    ADD COLUMN IF NOT EXISTS "provider_status" VARCHAR(32);

ALTER TABLE idc_gateway_local_supplier_route
    DROP CONSTRAINT IF EXISTS ck_gateway_local_route_provider_status;
ALTER TABLE idc_gateway_local_supplier_route
    ADD CONSTRAINT ck_gateway_local_route_provider_status CHECK (
        "provider_status" IS NULL
        OR (
            "provider_code" = 'tencent_essbasic'
            AND "provider_status" IN (
                'INIT', 'PART', 'WILLEXPIRE', 'ALL', 'REJECT', 'DEADLINE',
                'CANCEL', 'INVALID', 'EXCEPTION', 'RELIEVED'
            )
        )
    );

ALTER TABLE idc_gateway_local_callback_evidence
    ADD COLUMN IF NOT EXISTS "provider_status" VARCHAR(32);

ALTER TABLE idc_gateway_local_callback_evidence
    DROP CONSTRAINT IF EXISTS ck_gateway_local_evidence_provider_status;
ALTER TABLE idc_gateway_local_callback_evidence
    ADD CONSTRAINT ck_gateway_local_evidence_provider_status CHECK (
        "provider_status" IS NULL
        OR (
            "provider_code" = 'tencent_essbasic'
            AND "source_kind" IN ('callback', 'active_query')
            AND "provider_status" IN (
                'INIT', 'PART', 'WILLEXPIRE', 'ALL', 'REJECT', 'DEADLINE',
                'CANCEL', 'INVALID', 'EXCEPTION', 'RELIEVED'
            )
        )
    );

CREATE OR REPLACE FUNCTION idc_gateway_guard_local_route_update()
RETURNS TRIGGER AS $$
DECLARE
    state_changed BOOLEAN;
    reference_changed BOOLEAN;
    provider_status_changed BOOLEAN;
BEGIN
    state_changed := OLD."projected_status" IS DISTINCT FROM NEW."projected_status"
        OR OLD."reconciliation_status" IS DISTINCT FROM NEW."reconciliation_status";
    reference_changed := OLD."external_ref_ciphertext" IS DISTINCT FROM NEW."external_ref_ciphertext"
        OR OLD.external_ref_nonce IS DISTINCT FROM NEW.external_ref_nonce
        OR OLD.external_ref_cipher_format_version IS DISTINCT FROM NEW.external_ref_cipher_format_version
        OR OLD.external_ref_encryption_key_version IS DISTINCT FROM NEW.external_ref_encryption_key_version
        OR OLD.external_ref_hmac_key_version IS DISTINCT FROM NEW.external_ref_hmac_key_version
        OR OLD."external_ref_hmac" IS DISTINCT FROM NEW."external_ref_hmac";
    provider_status_changed := OLD."provider_status" IS DISTINCT FROM NEW."provider_status";

    IF OLD."route_no" IS DISTINCT FROM NEW."route_no"
        OR OLD."attempt_no" IS DISTINCT FROM NEW."attempt_no"
        OR OLD."request_no" IS DISTINCT FROM NEW."request_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."provider_code" IS DISTINCT FROM NEW."provider_code"
        OR OLD."operation_scope" IS DISTINCT FROM NEW."operation_scope"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR (OLD."external_ref_hmac" IS NOT NULL AND reference_changed)
        OR NOT (
            (OLD."projected_status" = 'running' AND NEW."projected_status" IN (
                'running', 'succeeded', 'failed', 'result_unknown', 'manual_takeover'
            ))
            OR (OLD."projected_status" = 'result_unknown' AND NEW."projected_status" IN (
                'result_unknown', 'succeeded', 'failed', 'manual_takeover'
            ))
            OR (
                OLD."projected_status" IN ('succeeded', 'failed')
                AND (
                    NEW."projected_status" = OLD."projected_status"
                    OR (
                        OLD."reconciliation_status" = 'reconciliation_required'
                        AND NEW."projected_status" = 'manual_takeover'
                    )
                )
            )
            OR (
                OLD."projected_status" = 'manual_takeover'
                AND NEW."projected_status" = 'manual_takeover'
            )
        )
        OR NOT (
            (OLD."reconciliation_status" = 'none' AND NEW."reconciliation_status" IN (
                'none', 'query_required', 'reconciliation_required', 'manual_takeover'
            ))
            OR (OLD."reconciliation_status" = 'query_required' AND NEW."reconciliation_status" IN (
                'none', 'query_required', 'reconciliation_required', 'manual_takeover'
            ))
            OR (OLD."reconciliation_status" = 'reconciliation_required'
                AND NEW."reconciliation_status" IN (
                    'none', 'reconciliation_required', 'manual_takeover'
                ))
            OR (OLD."reconciliation_status" = 'manual_takeover'
                AND NEW."reconciliation_status" = 'manual_takeover')
        )
        OR (
            (state_changed OR reference_changed OR provider_status_changed)
            AND NEW."version" <> OLD."version" + 1
        )
        OR (
            NOT state_changed AND NOT reference_changed AND NOT provider_status_changed
            AND NEW."version" <> OLD."version"
        )
    THEN
        RAISE EXCEPTION 'local supplier route transition is invalid';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
