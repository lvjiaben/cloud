// This test keeps real published configuration, frozen cart values and native
// source prices separate. No zero-price fallback or supplier mutation is used.
package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type nativeBillingRuntimeStub struct {
	factor int64
	calls  int
}

func (*nativeBillingRuntimeStub) ResolvePublicPurchaseCandidates(context.Context, string, *capmodel.ConfigurationSchemaDeclaration, []configurationcontract.ConfigurationValue) (configurationcontract.ResolvedCandidates, error) {
	return configurationcontract.ResolvedCandidates{}, nil
}
func (s *nativeBillingRuntimeStub) ReadImportedPurchaseBillingTerms(_ context.Context, inputs []ImportedBillingInput) ([]*channelcommercecap.BillingTerms, error) {
	s.calls++
	result := make([]*channelcommercecap.BillingTerms, len(inputs))
	for i, input := range inputs {
		quantity := int64(0)
		for _, value := range input.Values {
			if value.IntegerValue != nil {
				quantity = *value.IntegerValue
			}
			if value.IntegerTextValue != nil {
				quantity, _ = strconv.ParseInt(*value.IntegerTextValue, 10, 64)
			}
		}
		amount := quantity * s.factor
		result[i] = &channelcommercecap.BillingTerms{ConfigurationRevision: "source-config", CurrencyCode: input.CurrencyCode, Terms: []channelcommercecap.BillingTerm{{BillingTermToken: "monthly", Status: "available", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, SubtotalAmount: strconv.FormatInt(amount, 10), SetupAmount: "50", TaxAmount: "0", TotalAmount: strconv.FormatInt(amount+50, 10)}}}
	}
	return result, nil
}
func TestNativeRemotePublicAndFrozenCartPricesPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	db := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := db.Exec(ctx, publicNumberedFixtureSQL+importExportFixtureSQL+`CREATE TABLE plugin_idc_core_commerce_configuration_snapshot(tenant_id INT,configuration_snapshot_no TEXT,version_no TEXT,typed_value_snapshot JSONB,customer_value_digest TEXT);`); err != nil {
		t.Fatal(err)
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cfg_8", Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "1", Maximum: "100", Label: &capmodel.LocalizedText{ZhCN: "数量", EnUS: "Quantity"}}}}
	snapshot := channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportRemoteEvaluationProtocolVersion, PricingMode: channelcommercecap.ImportPricingRemoteEvaluation, Product: channelcommercecap.CatalogProduct{PublicSlug: "41"}, ConfigurationRevision: "source-config", Schema: schema, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "cfg_8", FieldToken: "config:8"}}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, PricePhase: "both", QuantityMode: "fixed", MinimumPurchaseCycles: 1}}, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_quantity_unit", OngoingService: true, RenewalMode: "unavailable", MaximumPurchaseCycles: 1}}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	read := resourceinterfaceprojectioncap.CatalogReadEvidence{SaleImportProtocol: productfulfillmentcap.SaleImportProtocolNativeV10CommonV1, InterfaceInstanceNo: "pfi_" + strings.Repeat("1", 32), OwnerID: "idc-resource", ResourceChannelRef: "CHN_native", InterfaceConfigurationRevision: "rci_" + strings.Repeat("2", 32), PluginID: "native-v10", PluginReleaseVersion: "v1", ContractVersion: "v1", AuthorizedMethods: []string{"provision", "query"}, ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64)}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	readJSON, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_contract VALUES(0,'v1',CAST(? AS JSONB),?,CAST(? AS JSONB))`, string(raw), hex.EncodeToString(digest[:]), string(readJSON)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_import_term(tenant_id,version_no,billing_rule_no,currency_code,upstream_term_reference,markup_percent) VALUES(0,'v1','b-v1','CNY','monthly','10'); DELETE FROM plugin_idc_core_catalog_price WHERE version_no='v1';`); err != nil {
		t.Fatal(err)
	}
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_version_configuration(tenant_id,version_no,version,customer_declaration_snapshot,declaration_digest) VALUES(0,'v1',1,CAST(? AS JSONB),'native')`, string(schemaJSON)); err != nil {
		t.Fatal(err)
	}
	runtime := &nativeBillingRuntimeStub{factor: 1000}
	reader, err := newPublicCatalogReader(db, []byte("0123456789abcdef0123456789abcdef"), nil, runtime, nil)
	if err != nil {
		t.Fatal(err)
	}
	service := reader.(*publicCatalogReaderImpl)
	service.billingLabels = PublicBillingLabelResources{"period.month": {ZhCN: "月", EnUS: "month"}}
	listed, listErr := service.GetBySlug(ctx, "alpha-one")
	if listErr != nil || listed.Availability != "available" || len(listed.Currencies) != 1 || listed.Currencies[0] != "CNY" {
		t.Fatalf("remote product omitted from catalog: %+v %v", listed, listErr)
	}
	page, listErr := service.Search(ctx, PublicCatalogSearchInput{})
	if listErr != nil || len(page.AvailableCurrencies) != 1 || page.AvailableCurrencies[0] != "CNY" {
		t.Fatalf("remote currencies omitted: %+v %v", page, listErr)
	}
	view, err := service.GetPurchaseConfiguration(ctx, "alpha-one")
	if err != nil {
		t.Fatal(err)
	}
	text := "2"
	input := PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: PublicPurchaseConfigurationInput{PublicSlug: "alpha-one", ConfigurationRevision: view.ConfigurationRevision, Values: []PublicPurchaseConfigurationValue{{FieldToken: view.Fields[0].FieldToken, Type: capmodel.ConfigurationFieldInteger, IntegerValue: &text}}}, CurrencyCode: "CNY"}
	quote, err := service.QuotePurchaseConfiguration(ctx, input)
	if err != nil || len(quote.Terms) != 1 || quote.Terms[0].TotalAmount != "2255" {
		t.Fatalf("live account quote %+v err=%v", quote, err)
	}
	runtime.factor = 2000
	changed, err := service.QuotePurchaseConfiguration(ctx, input)
	if err != nil || changed.Terms[0].TotalAmount != "4455" {
		t.Fatalf("source price change ignored %+v err=%v", changed, err)
	}
	runtime.factor = 1000
	snapshotNos := []string{"CFG_one", "CFG_two"}
	values := []int64{2, 4}
	for i, number := range values {
		compiled, err := configurationcontract.CompileResolved(schema, []configurationcontract.ConfigurationValue{{FieldKey: "cfg_8", IntegerValue: &number}}, configurationcontract.SurfaceCustomer, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_commerce_configuration_snapshot VALUES(0,?,'v1',CAST(? AS JSONB),?)`, snapshotNos[i], string(compiled.ValuesJSON), frozenConfigurationDigest(compiled.ValuesJSON)); err != nil {
			t.Fatal(err)
		}
	}
	source, err := loadPublicQuoteSource(ctx, db, "v1", "CNY")
	if err != nil {
		t.Fatal(err)
	}
	for index := range source.BillingRules {
		source.BillingRules[index].VersionNo = "v1"
	}
	cart := PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []PurchaseQuoteLineInput{{VersionNo: "v1", BillingRuleNo: "b-v1", ConfigurationSnapshotNo: snapshotNos[0], Quantity: 3, PurchaseCycles: 1}, {VersionNo: "v1", BillingRuleNo: "b-v1", ConfigurationSnapshotNo: snapshotNos[1], Quantity: 1, PurchaseCycles: 1}}}
	priced, rows, err := service.nativePurchaseQuoteRows(ctx, cart, []string{"v1"}, source.BillingRules, source.Prices)
	if err != nil || len(rows) != 0 {
		t.Fatalf("native cart rows %v %v", rows, err)
	}
	first, err := nativePurchaseQuoteCharge(priced.Lines[0])
	if err != nil || first.ChargeAmountMinor != 6600 || first.SetupFeeMinor != 165 {
		t.Fatalf("per-unit price %+v %v", first, err)
	}
	second, err := nativePurchaseQuoteCharge(priced.Lines[1])
	if err != nil || second.ChargeAmountMinor != 4400 {
		t.Fatalf("second selection price %+v %v", second, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price(tenant_id,version_no,billing_rule_no,currency_code,status_code,amount_minor,setup_fee_minor,minimum_quantity) VALUES(0,'v1','b-v1','CNY','inactive',4000,7,0);UPDATE plugin_idc_core_catalog_import_term SET whole_item_rule='{"kind":"fixed","fixedAmountMinor":4000,"discountBasisPoints":null}' WHERE version_no='v1';`); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QuotePurchaseConfiguration(ctx, input); err == nil {
		t.Fatal("inactive native fixed price remained sellable")
	}
	policy, err := ReadImportedNativePricing(ctx, db, "v1")
	if err != nil || policy.Mode != "manual" || policy.ManualPrices[0].SetupFee != "0.07" {
		t.Fatalf("inactive native policy cannot be restored: %+v %v", policy, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_catalog_price SET status_code='active' WHERE version_no='v1'`); err != nil {
		t.Fatal(err)
	}
	restored, err := service.QuotePurchaseConfiguration(ctx, input)
	if err != nil || restored.Terms[0].TotalAmount != "4007" {
		t.Fatalf("manual setup override lost: %+v %v", restored, err)
	}
	calls := runtime.calls
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_commerce_configuration_snapshot SET customer_value_digest=repeat('0',64) WHERE configuration_snapshot_no=?`, snapshotNos[0]); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.nativePurchaseQuoteRows(ctx, cart, []string{"v1"}, source.BillingRules, nil); err == nil || runtime.calls != calls {
		t.Fatal("tampered frozen values reached supplier")
	}
}
