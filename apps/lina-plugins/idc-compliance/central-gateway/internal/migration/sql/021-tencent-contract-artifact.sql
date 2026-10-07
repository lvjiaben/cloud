-- Installation-local state for one automatically requested Tencent signing report per completed flow.

-- Purpose: Persists business records for idc_gateway_tencent_contract_report_job.
-- 用途：持久化 idc_gateway_tencent_contract_report_job 的业务记录。
CREATE TABLE IF NOT EXISTS idc_gateway_tencent_contract_report_job (
    "job_no" VARCHAR(40) PRIMARY KEY,
    "installation_id" VARCHAR(70) NOT NULL,
    "source_route_no" VARCHAR(40) NOT NULL,
    "report_route_no" VARCHAR(40) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL,
    "report_cipher_format_version" SMALLINT,
    report_encryption_key_version INTEGER,
    "report_nonce" BYTEA,
    "report_ciphertext" BYTEA,
    next_query_at TIMESTAMPTZ,
    "last_reason_code" VARCHAR(64) NOT NULL,
    supplier_observed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_gateway_tencent_report_source_route FOREIGN KEY (
        "source_route_no"
    ) REFERENCES idc_gateway_local_supplier_route ("route_no"),
    CONSTRAINT fk_gateway_tencent_report_write_route FOREIGN KEY (
        "report_route_no"
    ) REFERENCES idc_gateway_local_supplier_route ("route_no"),
    CONSTRAINT uk_gateway_tencent_report_source UNIQUE (
        "installation_id", "source_route_no"
    ),
    CONSTRAINT uk_gateway_tencent_report_write_route UNIQUE ("report_route_no"),
    CONSTRAINT ck_gateway_tencent_report_job_no CHECK (
        "job_no" ~ '^lrj_[0-9a-f]{32}$'
    ),
    CONSTRAINT ck_gateway_tencent_report_installation CHECK (
        "installation_id" ~ '^linst_[0-9a-f]{64}$'
    ),
    CONSTRAINT ck_gateway_tencent_report_routes CHECK (
        "source_route_no" ~ '^lwr_[0-9a-f]{32}$'
        AND "report_route_no" ~ '^lwr_[0-9a-f]{32}$'
        AND "source_route_no" <> "report_route_no"
    ),
    CONSTRAINT ck_gateway_tencent_report_status CHECK (
        "status_code" IN ('executing', 'ready', 'failed', 'result_unknown')
    ),
    CONSTRAINT ck_gateway_tencent_report_reason CHECK (
        "last_reason_code" ~ '^[a-z][a-z0-9_]{2,63}$'
        AND "version" > 0
    ),
    CONSTRAINT ck_gateway_tencent_report_reference CHECK (
        (
            "report_cipher_format_version" IS NULL
            AND report_encryption_key_version IS NULL
            AND "report_nonce" IS NULL
            AND "report_ciphertext" IS NULL
            AND "status_code" IN ('failed', 'result_unknown')
        )
        OR
        (
            "report_cipher_format_version" > 0
            AND report_encryption_key_version > 0
            AND octet_length("report_nonce") = 12
            AND octet_length("report_ciphertext") > 16
            AND "status_code" IN ('executing', 'ready', 'failed')
        )
    ),
    CONSTRAINT ck_gateway_tencent_report_schedule CHECK (
        ("status_code" = 'executing' AND next_query_at IS NOT NULL)
        OR ("status_code" <> 'executing' AND next_query_at IS NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_gateway_tencent_report_due
    ON idc_gateway_tencent_contract_report_job (
        "installation_id", next_query_at, "job_no"
    ) WHERE "status_code" = 'executing';

CREATE OR REPLACE FUNCTION idc_gateway_guard_tencent_report_job_update()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD."job_no" IS DISTINCT FROM NEW."job_no"
        OR OLD."installation_id" IS DISTINCT FROM NEW."installation_id"
        OR OLD."source_route_no" IS DISTINCT FROM NEW."source_route_no"
        OR OLD."report_route_no" IS DISTINCT FROM NEW."report_route_no"
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR (
            OLD."report_ciphertext" IS NOT NULL
            AND (
                OLD."report_cipher_format_version" IS DISTINCT FROM NEW."report_cipher_format_version"
                OR OLD.report_encryption_key_version IS DISTINCT FROM NEW.report_encryption_key_version
                OR OLD."report_nonce" IS DISTINCT FROM NEW."report_nonce"
                OR OLD."report_ciphertext" IS DISTINCT FROM NEW."report_ciphertext"
            )
        )
    THEN
        RAISE EXCEPTION 'Tencent report job ownership and sealed report reference are immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_gateway_tencent_report_job_guard
    ON idc_gateway_tencent_contract_report_job;
CREATE TRIGGER trg_gateway_tencent_report_job_guard
BEFORE UPDATE ON idc_gateway_tencent_contract_report_job
FOR EACH ROW EXECUTE FUNCTION idc_gateway_guard_tencent_report_job_update();

DROP TRIGGER IF EXISTS trg_gateway_tencent_report_job_no_delete
    ON idc_gateway_tencent_contract_report_job;
CREATE TRIGGER trg_gateway_tencent_report_job_no_delete
BEFORE DELETE ON idc_gateway_tencent_contract_report_job
FOR EACH ROW EXECUTE FUNCTION idc_gateway_reject_local_immutable_mutation();
