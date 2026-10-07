-- Purpose: Persists business records for idc_gateway_supplier_plan_snapshot.
-- 用途：持久化 idc_gateway_supplier_plan_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_plan_snapshot (
    "plan_snapshot_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "plan_ref_hmac" BYTEA NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "currency_code" CHAR(3),
    valid_from TIMESTAMPTZ,
    valid_until TIMESTAMPTZ,
    "evidence_no" VARCHAR(40) NOT NULL REFERENCES idc_gateway_supplier_evidence ("evidence_no"),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_plan_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_plan_ref CHECK (octet_length("plan_ref_hmac") = 32),
    CONSTRAINT ck_gateway_supplier_plan_status CHECK (
        "status_code" IN ('active', 'suspended', 'expired', 'unknown')
    ),
    CONSTRAINT ck_gateway_supplier_plan_currency CHECK (
        "currency_code" IS NULL OR "currency_code" ~ '^[A-Z]{3}$'
    ),
    CONSTRAINT ck_gateway_supplier_plan_window CHECK (
        valid_until IS NULL OR (valid_from IS NOT NULL AND valid_until > valid_from)
    ),
    CONSTRAINT uq_gateway_supplier_plan_observation UNIQUE (
        "provider_code", "plan_ref_hmac", observed_at
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_plan_current
    ON idc_gateway_supplier_plan_snapshot ("provider_code", observed_at DESC, "plan_snapshot_no");

-- Purpose: Persists business records for idc_gateway_supplier_usage_snapshot.
-- 用途：持久化 idc_gateway_supplier_usage_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_usage_snapshot (
    "usage_snapshot_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "metric_code" VARCHAR(48) NOT NULL,
    period_started_at TIMESTAMPTZ NOT NULL,
    period_ended_at TIMESTAMPTZ NOT NULL,
    "quantity_numerator" BIGINT NOT NULL,
    "quantity_denominator" BIGINT NOT NULL DEFAULT 1,
    "supplier_cost_minor" BIGINT,
    "currency_code" CHAR(3),
    "evidence_no" VARCHAR(40) NOT NULL REFERENCES idc_gateway_supplier_evidence ("evidence_no"),
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_usage_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_usage_metric CHECK (
        "metric_code" ~ '^[a-z][a-z0-9_.-]{1,47}$'
    ),
    CONSTRAINT ck_gateway_supplier_usage_period CHECK (period_ended_at > period_started_at),
    CONSTRAINT ck_gateway_supplier_usage_quantity CHECK (
        "quantity_numerator" >= 0 AND "quantity_denominator" > 0
    ),
    CONSTRAINT ck_gateway_supplier_usage_cost CHECK (
        "supplier_cost_minor" IS NULL OR "supplier_cost_minor" >= 0
    ),
    CONSTRAINT ck_gateway_supplier_usage_currency CHECK (
        ("supplier_cost_minor" IS NULL AND "currency_code" IS NULL) OR
        ("supplier_cost_minor" IS NOT NULL AND "currency_code" ~ '^[A-Z]{3}$')
    ),
    CONSTRAINT uq_gateway_supplier_usage_observation UNIQUE (
        "provider_code", "metric_code", period_started_at, period_ended_at, observed_at
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_usage_period
    ON idc_gateway_supplier_usage_snapshot (
        "provider_code", period_started_at DESC, period_ended_at DESC, "usage_snapshot_no"
    );

-- Purpose: Persists business records for idc_gateway_supplier_budget_policy.
-- 用途：持久化 idc_gateway_supplier_budget_policy 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_budget_policy (
    "provider_code" VARCHAR(32) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "warning_minor" BIGINT NOT NULL,
    "critical_minor" BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    "updated_by_employee_no" VARCHAR(64) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY ("provider_code", "currency_code"),
    CONSTRAINT ck_gateway_supplier_budget_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_budget_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_gateway_supplier_budget_amount CHECK (
        "warning_minor" >= 0 AND "critical_minor" >= "warning_minor"
    ),
    CONSTRAINT ck_gateway_supplier_budget_employee CHECK (
        "updated_by_employee_no" ~ '^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$'
    ),
    CONSTRAINT ck_gateway_supplier_budget_version CHECK ("version" > 0)
);

-- Purpose: Persists business records for idc_gateway_supplier_budget_alert.
-- 用途：持久化 idc_gateway_supplier_budget_alert 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_budget_alert (
    "alert_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "level_code" VARCHAR(16) NOT NULL,
    "observed_minor" BIGINT NOT NULL,
    "threshold_minor" BIGINT NOT NULL,
    period_started_at TIMESTAMPTZ NOT NULL,
    period_ended_at TIMESTAMPTZ NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_alert_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_alert_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_gateway_supplier_alert_level CHECK ("level_code" IN ('warning', 'critical')),
    CONSTRAINT ck_gateway_supplier_alert_amount CHECK (
        "observed_minor" >= 0 AND "threshold_minor" >= 0
    ),
    CONSTRAINT ck_gateway_supplier_alert_period CHECK (period_ended_at > period_started_at),
    CONSTRAINT ck_gateway_supplier_alert_digest CHECK (octet_length("evidence_set_sha256") = 32)
);

CREATE INDEX IF NOT EXISTS idx_gateway_supplier_alert_period
    ON idc_gateway_supplier_budget_alert (
        "provider_code", period_started_at DESC, "level_code", "alert_no"
    );

-- Purpose: Persists business records for idc_gateway_supplier_reconciliation.
-- 用途：持久化 idc_gateway_supplier_reconciliation 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_reconciliation (
    "reconciliation_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    period_started_at TIMESTAMPTZ NOT NULL,
    period_ended_at TIMESTAMPTZ NOT NULL,
    "currency_code" CHAR(3) NOT NULL,
    "supplier_cost_minor" BIGINT NOT NULL,
    "local_observed_cost_minor" BIGINT NOT NULL,
    "difference_minor" BIGINT NOT NULL,
    "status_code" VARCHAR(32) NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_recon_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_recon_period CHECK (period_ended_at > period_started_at),
    CONSTRAINT ck_gateway_supplier_recon_currency CHECK ("currency_code" ~ '^[A-Z]{3}$'),
    CONSTRAINT ck_gateway_supplier_recon_amount CHECK (
        "supplier_cost_minor" >= 0 AND "local_observed_cost_minor" >= 0 AND
        "difference_minor" = "supplier_cost_minor" - "local_observed_cost_minor"
    ),
    CONSTRAINT ck_gateway_supplier_recon_status CHECK (
        "status_code" IN ('exact', 'within_tolerance', 'mismatch', 'insufficient_evidence')
    ),
    CONSTRAINT ck_gateway_supplier_recon_digest CHECK (octet_length("evidence_set_sha256") = 32),
    CONSTRAINT uq_gateway_supplier_recon_period UNIQUE (
        "provider_code", period_started_at, period_ended_at, "currency_code", observed_at
    )
);

-- Purpose: Persists business records for idc_gateway_supplier_sla_snapshot.
-- 用途：持久化 idc_gateway_supplier_sla_snapshot 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_supplier_sla_snapshot (
    "sla_snapshot_no" VARCHAR(40) PRIMARY KEY,
    "provider_code" VARCHAR(32) NOT NULL,
    period_started_at TIMESTAMPTZ NOT NULL,
    period_ended_at TIMESTAMPTZ NOT NULL,
    "request_count" BIGINT NOT NULL,
    "success_count" BIGINT NOT NULL,
    availability_ppm INTEGER NOT NULL,
    "latency_p95_millis" BIGINT NOT NULL,
    "latency_p99_millis" BIGINT NOT NULL,
    "evidence_set_sha256" BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_gateway_supplier_sla_provider CHECK (
        "provider_code" IN ('tencent_essbasic', 'nuonuo_invoice')
    ),
    CONSTRAINT ck_gateway_supplier_sla_period CHECK (period_ended_at > period_started_at),
    CONSTRAINT ck_gateway_supplier_sla_count CHECK (
        "request_count" >= 0 AND "success_count" >= 0 AND "success_count" <= "request_count"
    ),
    CONSTRAINT ck_gateway_supplier_sla_availability CHECK (
        availability_ppm BETWEEN 0 AND 1000000
    ),
    CONSTRAINT ck_gateway_supplier_sla_latency CHECK (
        "latency_p95_millis" >= 0 AND "latency_p99_millis" >= "latency_p95_millis"
    ),
    CONSTRAINT ck_gateway_supplier_sla_digest CHECK (octet_length("evidence_set_sha256") = 32),
    CONSTRAINT uq_gateway_supplier_sla_period UNIQUE (
        "provider_code", period_started_at, period_ended_at
    )
);

CREATE TRIGGER trg_gateway_supplier_plan_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_plan_snapshot
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_supplier_usage_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_usage_snapshot
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_supplier_alert_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_budget_alert
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_supplier_recon_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_reconciliation
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
CREATE TRIGGER trg_gateway_supplier_sla_immutable
    BEFORE UPDATE OR DELETE ON idc_gateway_supplier_sla_snapshot
    FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
