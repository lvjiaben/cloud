-- Purpose: Persists business records for idc_gateway_identity_operation.
-- 用途：持久化 idc_gateway_identity_operation 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_identity_operation (
    "operation_no" VARCHAR(40) PRIMARY KEY,
    "instance_no" VARCHAR(40) NOT NULL,
    "operation_kind" VARCHAR(32) NOT NULL,
    "operation_hmac" BYTEA NOT NULL,
    "request_sha256" BYTEA NOT NULL,
    "result_ref" VARCHAR(40) NOT NULL,
    "result_generation" BIGINT,
    "provider_code" VARCHAR(32),
    "actor_kind" VARCHAR(24) NOT NULL,
    "actor_ref" VARCHAR(128) NOT NULL,
    "reason_code" VARCHAR(64),
    "evidence_sha256" BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_gateway_identity_operation_instance FOREIGN KEY ("instance_no")
        REFERENCES idc_gateway_instance ("instance_no"),
    CONSTRAINT uq_gateway_identity_operation_hmac UNIQUE ("operation_hmac"),
    CONSTRAINT ck_gateway_identity_operation_kind CHECK (
        "operation_kind" IN (
            'key_rotation', 'instance_revocation', 'key_revocation',
            'subject_binding', 'subject_revocation'
        )
    ),
    CONSTRAINT ck_gateway_identity_operation_hmac CHECK (octet_length("operation_hmac") = 32),
    CONSTRAINT ck_gateway_identity_operation_request CHECK (octet_length("request_sha256") = 32),
    CONSTRAINT ck_gateway_identity_operation_evidence CHECK (octet_length("evidence_sha256") = 32),
    CONSTRAINT ck_gateway_identity_operation_actor CHECK ("actor_kind" IN ('instance', 'operator', 'system')),
    CONSTRAINT ck_gateway_identity_operation_generation CHECK (
        ("operation_kind" = 'key_rotation' AND "result_generation" IS NOT NULL AND "result_generation" > 1)
        OR ("operation_kind" <> 'key_rotation' AND "result_generation" IS NULL)
    ),
    CONSTRAINT ck_gateway_identity_operation_provider CHECK (
        ("operation_kind" IN ('subject_binding', 'subject_revocation')
            AND "provider_code" IN ('tencent_essbasic', 'nuonuo'))
        OR ("operation_kind" NOT IN ('subject_binding', 'subject_revocation') AND "provider_code" IS NULL)
    ),
    CONSTRAINT ck_gateway_identity_operation_reason CHECK (
        ("operation_kind" = 'key_rotation' AND "actor_kind" = 'instance' AND "reason_code" IS NULL)
        OR ("operation_kind" = 'instance_revocation' AND "actor_kind" IN ('operator', 'system')
            AND "reason_code" IN (
                'credential_compromise', 'merchant_termination',
                'sales_revocation', 'security_incident'
            ))
        OR ("operation_kind" = 'key_revocation' AND "actor_kind" IN ('operator', 'system')
            AND "reason_code" IN ('credential_compromise', 'rotation_abandoned', 'security_incident'))
        OR ("operation_kind" = 'subject_binding' AND "actor_kind" IN ('operator', 'system')
            AND "reason_code" = 'provider_onboarding_approved')
        OR ("operation_kind" = 'subject_revocation' AND "actor_kind" IN ('operator', 'system')
            AND "reason_code" IN (
                'provider_revoked', 'merchant_termination',
                'subject_replaced', 'security_incident'
            ))
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_identity_operation_instance
    ON idc_gateway_identity_operation ("instance_no", "operation_kind", completed_at DESC, "operation_no" DESC);

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_instance_key_pending
    ON idc_gateway_instance_key ("instance_no")
    WHERE "status" = 'pending';

CREATE OR REPLACE FUNCTION idc_gateway_reject_immutable_change()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'gateway immutable identity records cannot be changed';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_identity_operation_immutable ON idc_gateway_identity_operation;
CREATE TRIGGER trg_gateway_identity_operation_immutable
BEFORE UPDATE OR DELETE ON idc_gateway_identity_operation
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_immutable_change();
