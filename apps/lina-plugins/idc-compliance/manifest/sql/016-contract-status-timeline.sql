-- Exact Tencent contract-flow status projection and append-only local history.

ALTER TABLE plugin_idc_compliance_contract_flow
    ADD COLUMN IF NOT EXISTS "provider_status" VARCHAR(32);
ALTER TABLE plugin_idc_compliance_contract_flow
    ADD COLUMN IF NOT EXISTS "gateway_route_version" BIGINT NOT NULL DEFAULT 1;

ALTER TABLE plugin_idc_compliance_contract_flow
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_contract_flow_provider_status;
ALTER TABLE plugin_idc_compliance_contract_flow
    ADD CONSTRAINT ck_plugin_idc_compliance_contract_flow_provider_status
        CHECK (
            "provider_status" IS NULL OR
            (
                "provider_code" = 'tencent_essbasic' AND
                "provider_status" IN (
                    'INIT', 'PART', 'WILLEXPIRE', 'ALL', 'REJECT', 'DEADLINE',
                    'CANCEL', 'INVALID', 'EXCEPTION', 'RELIEVED'
                )
            )
        );
ALTER TABLE plugin_idc_compliance_contract_flow
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_contract_flow_gateway_version;
ALTER TABLE plugin_idc_compliance_contract_flow
    ADD CONSTRAINT ck_plugin_idc_compliance_contract_flow_gateway_version
        CHECK ("gateway_route_version" > 0);

-- Purpose: Persists business records for plugin_idc_compliance_contract_status_event.
-- 用途：持久化 plugin_idc_compliance_contract_status_event 的业务记录。
CREATE TABLE IF NOT EXISTS plugin_idc_compliance_contract_status_event (
    "id" BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    "tenant_id" INT NOT NULL DEFAULT 0,
    "contract_no" VARCHAR(64) NOT NULL,
    "flow_no" VARCHAR(64) NOT NULL,
    "gateway_delivery_no" VARCHAR(64),
    "gateway_route_no" VARCHAR(64) NOT NULL,
    "gateway_route_version" BIGINT NOT NULL,
    "source_kind" VARCHAR(16) NOT NULL,
    "provider_status" VARCHAR(32) NOT NULL,
    "projected_status" VARCHAR(24) NOT NULL,
    "decision_code" VARCHAR(16) NOT NULL,
    "reason_code" VARCHAR(64) NOT NULL,
    "supplier_observed_at" TIMESTAMPTZ NOT NULL,
    "observed_at" TIMESTAMPTZ NOT NULL,
    "created_at" TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_tenant
        CHECK ("tenant_id" = 0),
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_delivery
        CHECK (
            "gateway_delivery_no" IS NULL OR
            "gateway_delivery_no" ~ '^ldv_[0-9a-f]{32}$'
        ),
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_route
        CHECK ("gateway_route_no" ~ '^lwr_[0-9a-f]{32}$'),
    CONSTRAINT ck_idc_compliance_contract_status_evt_route_version
        CHECK ("gateway_route_version" > 1),
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_source
        CHECK ("source_kind" IN ('callback', 'active_query')),
    CONSTRAINT ck_idc_compliance_contract_status_evt_provider_status
        CHECK (
            "provider_status" IN (
                'INIT', 'PART', 'WILLEXPIRE', 'ALL', 'REJECT', 'DEADLINE',
                'CANCEL', 'INVALID', 'EXCEPTION', 'RELIEVED'
            )
        ),
    CONSTRAINT ck_idc_compliance_contract_status_evt_projection
        CHECK (
            "projected_status" IN (
                'signing', 'completed', 'rejected', 'expired', 'revoked',
                'failed', 'terminated'
            )
        ),
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_decision
        CHECK ("decision_code" IN ('applied', 'stale', 'conflict')),
    CONSTRAINT ck_plugin_idc_compliance_contract_status_event_reason
        CHECK ("reason_code" ~ '^[a-z][a-z0-9_]{2,63}$'),
    CONSTRAINT uk_idc_compliance_contract_status_evt_route_version
        UNIQUE ("tenant_id", "gateway_route_no", "gateway_route_version"),
    CONSTRAINT fk_plugin_idc_compliance_contract_status_event_flow
        FOREIGN KEY ("flow_no", "contract_no")
        REFERENCES plugin_idc_compliance_contract_flow ("flow_no", "contract_no")
        ON DELETE RESTRICT
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_contract_status_event_delivery
    ON plugin_idc_compliance_contract_status_event ("tenant_id", "gateway_delivery_no")
    WHERE "gateway_delivery_no" IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_idc_compliance_contract_status_evt_contract
    ON plugin_idc_compliance_contract_status_event (
        "tenant_id", "contract_no", "supplier_observed_at" DESC, "id" DESC
    );
CREATE INDEX IF NOT EXISTS idx_plugin_idc_compliance_contract_status_event_flow
    ON plugin_idc_compliance_contract_status_event (
        "tenant_id", "flow_no", "gateway_route_version" DESC
    );

DROP TRIGGER IF EXISTS trg_idc_compliance_contract_status_evt_append_only
    ON plugin_idc_compliance_contract_status_event;
CREATE TRIGGER trg_idc_compliance_contract_status_evt_append_only
    BEFORE UPDATE OR DELETE ON plugin_idc_compliance_contract_status_event
    FOR EACH ROW EXECUTE FUNCTION plugin_idc_compliance_reject_append_only_mutation();

COMMENT ON TABLE plugin_idc_compliance_contract_status_event IS
    'Append-only exact Tencent contract-flow status timeline owned by this installation';
