-- Managed contract artifact archival, immutable supplier time, and retry state.

ALTER TABLE plugin_idc_compliance_contract_file
    ADD COLUMN IF NOT EXISTS "supplier_observed_at" TIMESTAMPTZ;

ALTER TABLE plugin_idc_compliance_contract_file
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_contract_file_supplier_time;
ALTER TABLE plugin_idc_compliance_contract_file
    ADD CONSTRAINT ck_plugin_idc_compliance_contract_file_supplier_time
        CHECK (
            "file_type_code" NOT IN ('final_pdf', 'sign_report', 'evidence_certificate', 'termination_pdf')
            OR "status_code" <> 'archived'
            OR "supplier_observed_at" IS NOT NULL
        );

CREATE UNIQUE INDEX IF NOT EXISTS uk_idc_compliance_contract_file_archived_flow_type
    ON plugin_idc_compliance_contract_file (
        "tenant_id", "flow_no", "file_type_code"
    )
    WHERE "status_code" = 'archived'
      AND "flow_no" IS NOT NULL
      AND "file_type_code" IN ('final_pdf', 'sign_report', 'termination_pdf');

-- Purpose: Persists business records for plugin_idc_compliance_contract_archive_job.
-- 用途：持久化 plugin_idc_compliance_contract_archive_job 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_archive_job (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "job_no" VARCHAR(64) NOT NULL,
    "contract_no" VARCHAR(64) NOT NULL,
    "flow_no" VARCHAR(64) NOT NULL,
    "gateway_route_no" VARCHAR(64) NOT NULL,
    "artifact_type_code" VARCHAR(32) NOT NULL,
    "status_code" VARCHAR(24) NOT NULL DEFAULT 'queued',
    "attempt_count" INT NOT NULL DEFAULT 0,
    "next_attempt_at" TIMESTAMPTZ,
    "lease_sha256" CHAR(64),
    "lease_expires_at" TIMESTAMPTZ,
    "last_reason_code" VARCHAR(64) NOT NULL DEFAULT 'archive_queued',
    "supplier_observed_at" TIMESTAMPTZ,
    "completed_at" TIMESTAMPTZ,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "version" BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_plugin_idc_compliance_contract_archive_job_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_contract_archive_job_no
        CHECK ("job_no" ~ '^caj_[0-9a-f]{32}$'),
    CONSTRAINT ck_idc_compliance_contract_archive_job_route
        CHECK ("gateway_route_no" ~ '^lwr_[0-9a-f]{32}$'),
    CONSTRAINT ck_idc_compliance_contract_archive_job_artifact
        CHECK ("artifact_type_code" IN (
            'final_pdf', 'sign_report', 'termination_pdf', 'blockchain_metadata'
        )),
    CONSTRAINT ck_idc_compliance_contract_archive_job_status
        CHECK ("status_code" IN (
            'queued', 'processing', 'pending', 'archived', 'skipped', 'result_unknown'
        )),
    CONSTRAINT ck_idc_compliance_contract_archive_job_attempt
        CHECK ("attempt_count" BETWEEN 0 AND 1000 AND "version" > 0),
    CONSTRAINT ck_idc_compliance_contract_archive_job_reason
        CHECK ("last_reason_code" ~ '^[a-z][a-z0-9_]{2,63}$'),
    CONSTRAINT ck_idc_compliance_contract_archive_job_lease
        CHECK (
            ("status_code" = 'processing' AND "lease_sha256" ~ '^[0-9a-f]{64}$'
                AND "lease_expires_at" IS NOT NULL)
            OR
            ("status_code" <> 'processing' AND "lease_sha256" IS NULL
                AND "lease_expires_at" IS NULL)
        ),
    CONSTRAINT ck_idc_compliance_contract_archive_job_completion
        CHECK (
            ("status_code" IN ('archived', 'skipped', 'result_unknown')
                AND "completed_at" IS NOT NULL AND "next_attempt_at" IS NULL)
            OR
            ("status_code" NOT IN ('archived', 'skipped', 'result_unknown')
                AND "completed_at" IS NULL)
        ),
    CONSTRAINT uk_plugin_idc_compliance_contract_archive_job_no
        UNIQUE ("job_no"),
    CONSTRAINT uk_idc_compliance_contract_archive_job_flow_artifact
        UNIQUE ("tenant_id", "flow_no", "artifact_type_code"),
    CONSTRAINT fk_idc_compliance_contract_archive_job_flow
        FOREIGN KEY ("flow_no", "contract_no")
        REFERENCES plugin_idc_compliance_contract_flow ("flow_no", "contract_no")
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_archive_job_claim
    ON plugin_idc_compliance_contract_archive_job (
        "tenant_id", "next_attempt_at", "created_at", "id"
    )
    WHERE "status_code" IN ('queued', 'pending', 'processing');

COMMENT ON TABLE plugin_idc_compliance_contract_archive_job IS
    'Managed bounded jobs for local immutable contract files and short-lived-only evidence metadata';

