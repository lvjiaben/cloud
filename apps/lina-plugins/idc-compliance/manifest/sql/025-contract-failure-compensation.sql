-- Installation-local handling policy and bounded work queue for paid required
-- contracts that reached a supplier-authoritative terminal state.

-- Purpose: Persists business records for plugin_idc_compliance_contract_failure_policy.
-- 用途：持久化 plugin_idc_compliance_contract_failure_policy 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_failure_policy (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "handling_days" INT NOT NULL,
    "status_code" VARCHAR(16) NOT NULL DEFAULT 'active',
    "request_digest" CHAR(64) NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_contract_failure_policy_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_contract_failure_policy_value CHECK (
        "policy_version" > 0 AND "handling_days" BETWEEN 0 AND 30
        AND "request_digest" ~ '^[0-9a-f]{64}$'
        AND "status_code" IN ('active', 'superseded')
    ),
    CONSTRAINT uk_idc_compliance_contract_failure_policy_no UNIQUE ("policy_no"),
    CONSTRAINT uk_idc_compliance_contract_failure_policy_version UNIQUE ("policy_version"),
    CONSTRAINT uk_idc_compliance_contract_failure_policy_request UNIQUE ("request_digest")
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_contract_failure_policy_active
    ON plugin_idc_compliance_contract_failure_policy ("tenant_id")
    WHERE "status_code" = 'active';

INSERT INTO plugin_idc_compliance_contract_failure_policy
    ("tenant_id", "policy_no", "policy_version", "handling_days", "status_code", "request_digest")
SELECT 0, 'cfp_default_v1', 1, 3, 'active', repeat('0', 64)
WHERE NOT EXISTS (
    SELECT 1 FROM plugin_idc_compliance_contract_failure_policy
    WHERE "tenant_id" = 0 AND "status_code" = 'active'
);

-- Purpose: Persists business records for plugin_idc_compliance_contract_failure_case.
-- 用途：持久化 plugin_idc_compliance_contract_failure_case 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_failure_case (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "case_no" VARCHAR(40) NOT NULL,
    "contract_no" VARCHAR(64) NOT NULL,
    "order_no" VARCHAR(64) NOT NULL,
    "terminal_event_no" VARCHAR(64) NOT NULL,
    "terminal_status_code" VARCHAR(16) NOT NULL,
    "policy_no" VARCHAR(40) NOT NULL,
    "policy_version" BIGINT NOT NULL,
    "handling_days" INT NOT NULL,
    "handling_deadline" TIMESTAMPTZ NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'pending',
    "trigger_code" VARCHAR(16) NOT NULL DEFAULT '',
    "compensation_no" VARCHAR(40),
    "refund_request_no" VARCHAR(40),
    "attempt_count" INT NOT NULL DEFAULT 0,
    "lease_token" CHAR(64),
    "lease_expires_at" TIMESTAMPTZ,
    "last_reason_code" VARCHAR(128) NOT NULL DEFAULT '',
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_idc_compliance_contract_failure_case_tenant CHECK ("tenant_id" = 0),
    CONSTRAINT ck_idc_compliance_contract_failure_case_terminal CHECK (
        "terminal_status_code" IN ('rejected', 'expired', 'revoked', 'failed')
    ),
    CONSTRAINT ck_idc_compliance_contract_failure_case_policy CHECK (
        "policy_version" > 0 AND "handling_days" BETWEEN 0 AND 30
        AND "handling_deadline" >= "created_at"
    ),
    CONSTRAINT ck_idc_compliance_contract_failure_case_status CHECK (
        "status_code" IN (
            'pending', 'processing', 'signed', 'refund_pending', 'completed', 'manual_review'
        )
        AND "trigger_code" IN ('', 'deadline', 'admin_cancel')
        AND "attempt_count" BETWEEN 0 AND 100
        AND (("status_code" = 'processing' AND "lease_token" IS NOT NULL
                AND "lease_expires_at" IS NOT NULL)
            OR ("status_code" <> 'processing' AND "lease_token" IS NULL
                AND "lease_expires_at" IS NULL))
        AND (("status_code" IN ('signed', 'completed') AND "completed_at" IS NOT NULL)
            OR ("status_code" NOT IN ('signed', 'completed') AND "completed_at" IS NULL))
    ),
    CONSTRAINT uk_idc_compliance_contract_failure_case_no UNIQUE ("case_no"),
    CONSTRAINT uk_idc_compliance_contract_failure_case_contract UNIQUE ("contract_no"),
    CONSTRAINT uk_idc_compliance_contract_failure_case_event UNIQUE ("terminal_event_no"),
    CONSTRAINT fk_idc_compliance_contract_failure_case_contract FOREIGN KEY ("contract_no")
        REFERENCES plugin_idc_compliance_contract ("contract_no") ON DELETE RESTRICT,
    CONSTRAINT fk_idc_compliance_contract_failure_case_policy FOREIGN KEY ("policy_no")
        REFERENCES plugin_idc_compliance_contract_failure_policy ("policy_no") ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_failure_case_due
    ON plugin_idc_compliance_contract_failure_case (
        "tenant_id", "status_code", "handling_deadline", "id"
    ) WHERE "status_code" IN ('pending', 'processing');

COMMENT ON TABLE plugin_idc_compliance_contract_failure_policy IS
    'Versioned installation-local 0-30 natural-day handling policy; default is three days';
COMMENT ON TABLE plugin_idc_compliance_contract_failure_case IS
    'Frozen terminal event and deadline handed to the core-owned compensation command';
