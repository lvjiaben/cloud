// This file exercises the real export queries and dynamic candidate tokens in
// an isolated PostgreSQL schema, including stale and foreign-token rejection.

package catalog

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func TestImportSnapshotAndCandidatesPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, publicNumberedFixtureSQL+importExportFixtureSQL); err != nil {
		t.Fatal(err)
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "region", Type: capmodel.ConfigurationFieldAddress,
		Label:             &capmodel.LocalizedText{ZhCN: "地区", EnUS: "Region"},
		DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.region"},
	}}}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_configuration VALUES(0,'v1',1,?::jsonb,'digest-one')`, string(raw)); err != nil {
		t.Fatal(err)
	}
	resolver := &publicDynamicCandidateResolverStub{}
	reader, err := newPublicCatalogReader(db, []byte("0123456789abcdef0123456789abcdef"), nil, resolver, nil)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := reader.ReadImportSnapshot(ctx, "alpha-one")
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.Prices) != 1 || exported.Prices[0].Base[0].AmountMinor != 100 || exported.Prices[0].TermReference == "" || exported.ConfigurationRevision == "" {
		t.Fatalf("incomplete export: %+v", exported)
	}
	if exported.Schema.Fields[0].Key == "region" || exported.Schema.Fields[0].DynamicCandidates.Method != importedCandidateMethod {
		t.Fatal("private schema leaked")
	}
	initial, err := reader.ReadImportCandidates(ctx, channelcommercecap.ImportCandidatesInput{PublicSlug: "alpha-one", ConfigurationRevision: exported.ConfigurationRevision})
	if err != nil {
		t.Fatal(err)
	}
	if len(initial.Fields) != 1 || len(initial.Fields[0].Options) != 2 || initial.Fields[0].FieldKey != exported.Schema.Fields[0].Key || initial.Fields[0].Options[1].ParentValue != initial.Fields[0].Options[0].Value {
		t.Fatalf("candidate hierarchy/mapping: %+v", initial)
	}
	input := channelcommercecap.ImportCandidatesInput{PublicSlug: "alpha-one", ConfigurationRevision: exported.ConfigurationRevision, Values: []channelcommercecap.PurchaseValue{{FieldToken: initial.Fields[0].FieldToken, Type: capmodel.ConfigurationFieldAddress, AddressOptionTokens: []string{initial.Fields[0].Options[0].OptionToken, initial.Fields[0].Options[1].OptionToken}}}}
	selected, err := reader.ReadImportCandidates(ctx, input)
	if err != nil || selected.Fields[0].Options[1].Value != initial.Fields[0].Options[1].Value || len(resolver.values[len(resolver.values)-1]) != 1 || resolver.values[len(resolver.values)-1][0].AddressOptionTokens[1] != "cn-zj" {
		t.Fatalf("authoritative dynamic selection: %+v %v", selected, err)
	}
	input.PublicSlug = "alpha-two"
	if _, err = reader.ReadImportCandidates(ctx, input); err == nil {
		t.Fatal("foreign product revision accepted")
	}
	input.PublicSlug = "alpha-one"
	input.Values[0].FieldToken += "tamper"
	if _, err = reader.ReadImportCandidates(ctx, input); err == nil {
		t.Fatal("tampered field accepted")
	}
	input.Values = nil
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET version=2 WHERE version_no='v1'`); err != nil {
		t.Fatal(err)
	}
	if _, err = reader.ReadImportCandidates(ctx, input); err == nil {
		t.Fatal("stale configuration accepted")
	}
	if _, err = reader.ReadImportSnapshot(ctx, "alpha-draft"); err == nil {
		t.Fatal("unpublished product exported")
	}
}

const importExportFixtureSQL = `
CREATE TABLE plugin_idc_core_catalog_product_version_configuration(tenant_id INT,version_no TEXT,version BIGINT,customer_declaration_snapshot JSONB,declaration_digest TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id INT,version_no TEXT);
CREATE TABLE plugin_idc_core_catalog_product_version_fulfillment_policy(tenant_id INT,version_no TEXT,granularity_code TEXT,ongoing_management BOOL,renewal_mode_code TEXT);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy VALUES(0,'v1','per_order_item',false,'unavailable');
ALTER TABLE plugin_idc_core_catalog_billing_rule ADD COLUMN id BIGSERIAL,ADD COLUMN charge_type_code TEXT DEFAULT 'recurring',ADD COLUMN period_unit_code TEXT DEFAULT 'month',ADD COLUMN period_count INT DEFAULT 1,ADD COLUMN price_phase_code TEXT DEFAULT 'new_purchase',ADD COLUMN quantity_mode_code TEXT DEFAULT 'fixed',ADD COLUMN trial_quantity INT,ADD COLUMN minimum_purchase_cycles INT DEFAULT 1,ADD COLUMN maximum_renewal_cycles INT,ADD COLUMN meter_unit_code TEXT,ADD COLUMN tier_mode_code TEXT,ADD COLUMN usage_collection_unit_code TEXT,ADD COLUMN usage_collection_count INT,ADD COLUMN usage_rounding_mode_code TEXT,ADD COLUMN usage_rounding_quantity BIGINT,ADD COLUMN usage_minimum_quantity BIGINT,ADD COLUMN usage_maximum_quantity BIGINT,ADD COLUMN usage_missing_policy_code TEXT;
ALTER TABLE plugin_idc_core_catalog_price ADD COLUMN id BIGSERIAL,ADD COLUMN configuration_price_dimension_no TEXT,ADD COLUMN amount_minor BIGINT DEFAULT 100,ADD COLUMN setup_fee_minor BIGINT DEFAULT 0,ADD COLUMN renewal_amount_minor BIGINT,ADD COLUMN minimum_quantity BIGINT DEFAULT 0,ADD COLUMN maximum_quantity BIGINT;
CREATE TABLE plugin_idc_core_catalog_tax_policy(id BIGSERIAL,tenant_id INT,version_no TEXT,currency_code TEXT,price_includes_tax BOOL,tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT);
INSERT INTO plugin_idc_core_catalog_tax_policy VALUES(DEFAULT,0,'v1','CNY',false,'exempt',NULL,'service','line','half_up',1);
CREATE TABLE plugin_idc_core_catalog_configuration_price_dimension(id BIGSERIAL,tenant_id INT,version_no TEXT,dimension_no TEXT,field_key TEXT,candidate_value TEXT,pricing_mode_code TEXT DEFAULT 'fixed',status_code TEXT);
CREATE TABLE plugin_idc_core_catalog_configuration_group(id BIGSERIAL,tenant_id INT,version_no TEXT,group_no TEXT,group_code TEXT,sort_order INT);
CREATE TABLE plugin_idc_core_catalog_configuration_option(id BIGSERIAL,tenant_id INT,version_no TEXT,group_no TEXT,option_no TEXT,normalized_value JSONB,status_code TEXT,sort_order INT);
CREATE TABLE plugin_idc_core_catalog_import_term(tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,whole_item_rule JSONB);
`
