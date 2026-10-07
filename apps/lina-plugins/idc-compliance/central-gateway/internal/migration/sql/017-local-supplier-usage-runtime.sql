-- Purpose: Persists business records for idc_gateway_supplier_collection_state.
-- 用途：持久化 idc_gateway_supplier_collection_state 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_collection_state (
    "provider_code" VARCHAR(32) PRIMARY KEY,
    "status_code" VARCHAR(16) NOT NULL,
    next_collect_at TIMESTAMPTZ,
    last_started_at TIMESTAMPTZ,
    last_succeeded_at TIMESTAMPTZ,
    "failure_count" INTEGER NOT NULL DEFAULT 0,
    "last_failure_code" VARCHAR(32),
    "lease_sha256" BYTEA,
    lease_expires_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_gateway_supplier_collection_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_collection_status CHECK (
        "status_code" IN ('disabled', 'ready', 'running', 'retry_wait')
    ),
    CONSTRAINT ck_gateway_supplier_collection_failure CHECK (
        "failure_count" BETWEEN 0 AND 32
        AND (
            "last_failure_code" IS NULL
            OR "last_failure_code" IN (
                'transport_unavailable', 'timeout', 'malformed_response',
                'provider_rejected', 'persistence_unavailable'
            )
        )
    ),
    CONSTRAINT ck_gateway_supplier_collection_lease CHECK (
        ("status_code" = 'running'
            AND "lease_sha256" IS NOT NULL
            AND octet_length("lease_sha256") = 32
            AND lease_expires_at IS NOT NULL
            AND last_started_at IS NOT NULL)
        OR
        ("status_code" <> 'running'
            AND "lease_sha256" IS NULL
            AND lease_expires_at IS NULL)
    ),
    CONSTRAINT ck_gateway_supplier_collection_schedule CHECK (
        ("status_code" = 'disabled' AND next_collect_at IS NULL)
        OR ("status_code" <> 'disabled' AND next_collect_at IS NOT NULL)
    ),
    CONSTRAINT ck_gateway_supplier_collection_time CHECK (
        (last_started_at IS NULL OR last_started_at <= updated_at)
        AND (last_succeeded_at IS NULL OR last_succeeded_at <= updated_at)
    ),
    CONSTRAINT ck_gateway_supplier_collection_version CHECK ("version" > 0)
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_collection_due
    ON idc_gateway_supplier_collection_state (
        next_collect_at, "provider_code"
    ) WHERE "status_code" IN ('ready', 'retry_wait', 'running');

CREATE UNIQUE INDEX IF NOT EXISTS uq_gateway_supplier_budget_alert_once
    ON idc_gateway_supplier_budget_alert (
        "provider_code", "currency_code", "level_code",
        period_started_at, period_ended_at
    );
