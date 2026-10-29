-- Customer notes belong to the core entitlement aggregate. They have their
-- own revision and never change paid terms or the lifecycle version.
ALTER TABLE plugin_idc_core_service_entitlement
 ADD COLUMN IF NOT EXISTS customer_remark VARCHAR(1000) NOT NULL DEFAULT '',
 ADD COLUMN IF NOT EXISTS customer_remark_revision BIGINT NOT NULL DEFAULT 1 CHECK(customer_remark_revision>0);
COMMENT ON COLUMN plugin_idc_core_service_entitlement.customer_remark IS 'Current customer non-sensitive service note';
COMMENT ON COLUMN plugin_idc_core_service_entitlement.customer_remark_revision IS 'Independent optimistic revision for the customer service note';
