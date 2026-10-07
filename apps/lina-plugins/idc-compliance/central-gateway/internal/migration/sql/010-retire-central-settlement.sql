-- Historical migrations 006 and 007 remain in the checksum ledger so an
-- existing installation can upgrade deterministically. This migration removes
-- every vendor-to-customer balance, credit, price, billing-gate, and settlement
-- object from the final schema; no compatibility view or write path remains.

DROP TABLE IF EXISTS idc_gateway_settlement_management_gate_result CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_management_price_result CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_management_account_result CASCADE;
DROP TABLE IF EXISTS idc_gateway_provider_billing_gate CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_management_operation CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_ledger_entry CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_usage_line CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_hold_line CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_hold CASCADE;
DROP TABLE IF EXISTS idc_gateway_settlement_account CASCADE;
DROP TABLE IF EXISTS idc_gateway_price_item CASCADE;
DROP TABLE IF EXISTS idc_gateway_price_version CASCADE;

DROP FUNCTION IF EXISTS idc_gateway_validate_provider_billing_gate_operation() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_guard_provider_billing_gate() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_management_gate_result() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_management_price_result() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_management_account_result() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_settlement_management_result() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_management_ledger() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_reject_settlement_management_mutation() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_settlement_hold_lines() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_price_version_items() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_settlement_ledger_account() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_validate_settlement_account_ledger() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_guard_settlement_account_update() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_guard_settlement_hold_update() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_guard_price_item_mutation() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_guard_price_version_update() CASCADE;
DROP FUNCTION IF EXISTS idc_gateway_reject_settlement_history_mutation() CASCADE;
