// This test follows a paid hidden default through two real export and quote
// projections. Opaque identities change while the selected fee remains exact.
package catalog

import (
	"context"
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"os"
	"testing"
)

func TestImportHiddenDefaultReexportPricePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, publicNumberedFixtureSQL+importExportFixtureSQL); err != nil {
		t.Fatal(err)
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "plan", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: "档位", EnUS: "Plan"}, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "free", Label: &capmodel.LocalizedText{ZhCN: "基础", EnUS: "Basic"}}, {Value: "paid", Label: &capmodel.LocalizedText{ZhCN: "付费", EnUS: "Paid"}}}}}}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_configuration(tenant_id,version_no,version,customer_declaration_snapshot,customer_presentation_snapshot,declaration_digest) VALUES(0,'v1',1,CAST(? AS JSONB),' {"plan":{"customerVisible":false,"defaultValue":"paid"}}','digest')`, string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension(tenant_id,version_no,dimension_no,field_key,candidate_value,pricing_mode_code,status_code) VALUES(0,'v1','cpd_11111111111111111111111111111111','plan','paid','fixed','active'); INSERT INTO plugin_idc_core_catalog_price(tenant_id,version_no,billing_rule_no,currency_code,status_code,configuration_price_dimension_no,amount_minor,setup_fee_minor,minimum_quantity) VALUES(0,'v1','b-v1','CNY','active','cpd_11111111111111111111111111111111',200,10,0);`); err != nil {
		t.Fatal(err)
	}
	reader, err := newPublicCatalogReader(db, []byte("0123456789abcdef0123456789abcdef"), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader.(*publicCatalogReaderImpl).billingLabels = PublicBillingLabelResources{"period.month": {ZhCN: "月", EnUS: "month"}}
	first, err := reader.ReadImportSnapshot(ctx, "alpha-one")
	if err != nil {
		t.Fatal(err)
	}
	if first.ProtocolVersion != channelcommercecap.ImportPurchaseCycleLimitProtocolVersion || first.Mappings[0].CustomerVisible == nil || *first.Mappings[0].CustomerVisible {
		t.Fatalf("hidden policy not exported: %+v", first)
	}
	verifyHiddenExportQuote(t, ctx, reader, first)
	presentations := map[string]configurationcontract.Presentation{}
	for _, m := range first.Mappings {
		presentations[m.FieldKey] = configurationcontract.Presentation{CustomerVisible: m.CustomerVisible, DefaultValue: m.DefaultValue}
	}
	policy, err := json.Marshal(presentations)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = json.Marshal(first.Schema)
	if err != nil {
		t.Fatal(err)
	}
	var defaultValue string
	if err = json.Unmarshal(first.Mappings[0].DefaultValue, &defaultValue); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_configuration SET version=2,customer_declaration_snapshot=CAST(? AS JSONB),customer_presentation_snapshot=CAST(? AS JSONB),declaration_digest='second-digest' WHERE version_no='v1'`, string(raw), string(policy)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_configuration_price_dimension SET field_key=?,candidate_value=? WHERE dimension_no='cpd_11111111111111111111111111111111'`, first.Mappings[0].FieldKey, defaultValue); err != nil {
		t.Fatal(err)
	}
	second, err := reader.ReadImportSnapshot(ctx, "alpha-one")
	if err != nil {
		t.Fatal(err)
	}
	if string(second.Mappings[0].DefaultValue) == string(first.Mappings[0].DefaultValue) {
		t.Fatal("second resale reused source option identity")
	}
	verifyHiddenExportQuote(t, ctx, reader, second)
}
func verifyHiddenExportQuote(t *testing.T, ctx context.Context, reader PublicCatalogReader, snapshot *channelcommercecap.ImportSnapshot) {
	t.Helper()
	if len(snapshot.Prices) != 1 || len(snapshot.Prices[0].Options) != 1 || snapshot.Prices[0].Options[0].Bands[0].AmountMinor != 200 {
		t.Fatalf("paid option price lost: %+v", snapshot.Prices)
	}
	var selected string
	if err := json.Unmarshal(snapshot.Mappings[0].DefaultValue, &selected); err != nil {
		t.Fatal(err)
	}
	if snapshot.Prices[0].Options[0].Value != selected {
		t.Fatal("hidden default and price identity diverged")
	}
	quote, err := reader.(PublicStructuredConfigurationReader).QuotePurchaseConfiguration(ctx, PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: PublicPurchaseConfigurationInput{PublicSlug: "alpha-one", ConfigurationRevision: snapshot.ConfigurationRevision}, CurrencyCode: "CNY"})
	if err != nil || len(quote.Terms) != 1 || quote.Terms[0].TotalAmount != "310" {
		t.Fatalf("hidden selected price mismatch: %+v %v", quote, err)
	}
}
