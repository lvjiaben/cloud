-- Preserve the original paid snapshots and pin a separate operational selection
-- on each immutable administrator recovery command. No business data is rewritten.
ALTER TABLE plugin_idc_core_product_fulfillment_command
 ADD COLUMN IF NOT EXISTS recovery_configuration JSONB;
COMMENT ON COLUMN plugin_idc_core_product_fulfillment_command.recovery_configuration IS
 'Operational upstream correction for this recovery only; original paid specifications and amounts remain immutable';
