-- Safe source-side link to the installation-local gateway route. The Tencent
-- flow identifier stays sealed inside the local gateway and never enters this table.

ALTER TABLE plugin_idc_compliance_contract_flow
    ADD COLUMN IF NOT EXISTS "gateway_route_no" VARCHAR(64);

ALTER TABLE plugin_idc_compliance_contract_flow
    DROP CONSTRAINT IF EXISTS ck_plugin_idc_compliance_contract_flow_gateway_route;
ALTER TABLE plugin_idc_compliance_contract_flow
    ADD CONSTRAINT ck_plugin_idc_compliance_contract_flow_gateway_route
        CHECK (
            "gateway_route_no" IS NULL OR
            "gateway_route_no" ~ '^lwr_[0-9a-f]{32}$'
        );

CREATE UNIQUE INDEX IF NOT EXISTS uk_plugin_idc_compliance_contract_flow_gateway_route
    ON plugin_idc_compliance_contract_flow ("tenant_id", "gateway_route_no")
    WHERE "gateway_route_no" IS NOT NULL;

COMMENT ON COLUMN plugin_idc_compliance_contract_flow."gateway_route_no" IS
    'Opaque installation-local supplier-write route; never a Tencent flow identifier';
