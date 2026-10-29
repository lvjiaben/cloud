// This boundary uses the real public catalog/compiler, current paid native
// entitlement and commerce target writer. Only external candidate availability
// and file storage (unused by this file-free schema) are local source fixtures.
package serveroperation

import (
	"context"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/commerce"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
	"strings"
	"testing"
)

type nativeContextRuntime struct{}

func (nativeContextRuntime) CheckPublicPurchaseAvailability(context.Context, string) (bool, error) {
	return true, nil
}
func (nativeContextRuntime) ResolvePublicPurchaseCandidates(context.Context, string, *capmodel.ConfigurationSchemaDeclaration, []configurationcontract.ConfigurationValue) (configurationcontract.ResolvedCandidates, error) {
	return configurationcontract.ResolvedCandidates{}, nil
}

type nativeContextFiles struct {
	purchaseconfigurationfile.Service
}

func (nativeContextFiles) VerifyFields(context.Context, []purchaseconfigurationfile.VerifyInput) ([]purchaseconfigurationfile.VerifiedField, error) {
	return []purchaseconfigurationfile.VerifiedField{}, nil
}

func TestNativeConfigurationPublicContextFeedsQuoteTargetPostgreSQL(t *testing.T) {
	f := newNativeCommercialFixtureWithSeed(t, strings.ReplaceAll(accessoryCheckoutSeedSQL, "BILL_checkout_test", "bill_checkout_test"))
	for _, migration := range []string{"094-generic-configuration-pricing.sql", "110-catalog-product-import.sql", "120-portable-catalog-import.sql", "127-product-commerce-workflow.sql", "134-local-purchase-controls.sql", "142-generic-offer-configuration.sql"} {
		nativeLifecycleMigration(t, f.database, migration)
	}
	for _, statement := range []string{
		`INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy(version_no,strategy_code,granularity_code,policy_digest,created_by) VALUES('ver_checkout_test','manual','per_order_item',repeat('c',64),101)`,
		`INSERT INTO plugin_idc_core_catalog_product_version_configuration(version_no,product_schema_version,product_declaration_snapshot,merchant_value_snapshot,customer_schema_version,customer_declaration_snapshot,declaration_digest,merchant_value_digest,changed_by) VALUES('ver_checkout_test','v1','{}','{}','v1','{"version":"v1","fields":[{"key":"cpu","type":"integer","required":true,"minimum":"1","maximum":"8","label":{"zhCN":"核数","enUS":"CPU cores"}},{"key":"region","type":"single_select","required":true,"label":{"zhCN":"区域","enUS":"Region"},"staticCandidates":[{"value":"region_internal","label":{"zhCN":"区域一","enUS":"Region one"}}]}]}',repeat('a',64),repeat('b',64),101)`,
		`INSERT INTO plugin_idc_core_catalog_price(price_no,version_no,billing_rule_no,currency_code,amount_minor,setup_fee_minor) VALUES('PRC_native_context','ver_checkout_test','bill_checkout_test','CNY',800,200)`,
		`UPDATE plugin_idc_core_catalog_product_version SET status_code='published',published_at=CURRENT_TIMESTAMP WHERE version_no='ver_checkout_test'`,
		`UPDATE plugin_idc_core_catalog_product SET current_version_no='ver_checkout_test' WHERE product_no='prd_checkout_test'`,
	} {
		if _, err := f.database.Exec(f.ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	public, err := catalog.NewPublicCatalogReaderConfigured(f.database, []byte("0123456789abcdef0123456789abcdef"), nativeContextRuntime{}, nativeContextFiles{})
	if err != nil {
		t.Fatal(err)
	}
	if err = catalog.BindPublicBillingLabelResources(public, catalog.PublicBillingLabelResources{"charge.one_time": {ZhCN: "一次性", EnUS: "One-time"}, "period.hour": {ZhCN: "小时", EnUS: "hour"}, "period.day": {ZhCN: "天", EnUS: "day"}, "period.month": {ZhCN: "个月", EnUS: "month"}, "period.year": {ZhCN: "年", EnUS: "year"}}); err != nil {
		t.Fatal(err)
	}
	initial, err := public.GetPurchaseConfiguration(f.ctx, "checkout-test")
	if err != nil {
		t.Fatal(err)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(public)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := freezer.FreezePurchaseConfiguration(f.ctx, catalog.PublicPurchaseConfigurationInput{PublicSlug: "checkout-test", ConfigurationRevision: initial.ConfigurationRevision, Values: initial.Values})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest,order_no,order_item_no,bound_at)
 SELECT 'SCF_native_context',e.customer_no,101,'AQU_checkout_test','aqi_checkout_test',e.product_no,e.version_no,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),repeat('d',64),e.order_no,e.order_item_no,CURRENT_TIMESTAMP FROM plugin_idc_core_service_entitlement e WHERE e.entitlement_no=?`, frozen.ConfigurationRevision, frozen.CustomerDeclarationDigest, frozen.CandidateSnapshotDigest, frozen.CustomerValueDigest, string(frozen.CandidateSnapshotJSON), string(frozen.TypedValueSnapshotJSON), f.target.EntitlementNo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET configuration_snapshot_no='SCF_native_context' WHERE entitlement_no=?`, f.target.EntitlementNo); err != nil {
		t.Fatal(err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(public)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := commerce.New(f.database, resolver, f.native.pricing.principal, f.settlementService(t))
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(f.database, f.native.pricing.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = commerce.BindStructuredConfigurationSnapshots(owner, freezer, files); err != nil {
		t.Fatal(err)
	}
	f.native.targetQuotes = owner
	f.native.prices.catalog = public
	view, err := f.native.ReadConfigurationContext(f.ctx, servercommercialcap.ConfigurationContextRequest{Target: f.target})
	if err != nil || view == nil || !view.Ready || len(view.Fields) != 2 || view.BillingTermToken == "" {
		t.Fatalf("real native form: %+v / %+v", view, err)
	}
	for _, field := range view.Fields {
		if field.FieldToken == "cpu" || field.FieldToken == "region" {
			t.Fatal("raw field key exposed")
		}
		for _, option := range field.Options {
			if option.OptionToken == "region_internal" {
				t.Fatal("raw candidate exposed")
			}
		}
	}
	// Existing real public token plus forged choice must be rejected, not widened.
	invalid := servercommercialcap.ConfigurationContextRequest{Target: f.target, ConfigurationRevision: view.ConfigurationRevision, Values: append([]servercommercialcap.TypedPurchaseValue{}, view.Values...)}
	for i, value := range invalid.Values {
		if value.Type == capmodel.ConfigurationFieldSingleSelect {
			invalid.Values[i].OptionTokens = []string{"forged-option"}
		}
	}
	if _, err = f.native.ReadConfigurationContext(f.ctx, invalid); err == nil {
		t.Fatal("forged choice with valid field accepted")
	}
	// A real scalar violation is returned as an invalid editable form with no term.
	invalid.Values = append([]servercommercialcap.TypedPurchaseValue{}, view.Values...)
	below := "0"
	for i, value := range invalid.Values {
		if value.Type == capmodel.ConfigurationFieldInteger {
			invalid.Values[i].IntegerValue = &below
		}
	}
	invalidView, err := f.native.ReadConfigurationContext(f.ctx, invalid)
	if err != nil || invalidView == nil || invalidView.Ready || invalidView.BillingTermToken != "" || len(invalidView.Issues) == 0 {
		t.Fatalf("invalid form=%+v / %+v", invalidView, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_commerce_order`, 1)
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_business_operation`, 0)
	resolved, err := resolver.ResolvePurchaseConfiguration(f.ctx, catalog.PurchaseConfigurationInput{PublicSlug: "checkout-test", ConfigurationRevision: view.ConfigurationRevision, BillingTermToken: view.BillingTermToken, CurrencyCode: view.CurrencyCode, PurchaseCycles: view.PurchaseCycles, Values: nativeTypedPurchaseValues(view.Values)})
	if err != nil {
		t.Fatalf("context token resolver: %+v", err)
	}
	assembler, err := catalog.PurchaseQuoteAssemblerFromPublicReader(public)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = assembler.AssemblePurchaseQuote(f.ctx, catalog.PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []catalog.PurchaseQuoteLineInput{{ReferenceNo: "context-diagnostic", ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo, BillingRuleNo: resolved.BillingRuleNo, Quantity: 1, PurchaseCycles: 1, PriceSelections: resolved.PriceSelections}}}); err != nil {
		t.Fatalf("published catalog assembly: %+v", err)
	}
	quote, err := f.native.QuoteTarget(f.ctx, servercommercialcap.StructuredTargetInput{Target: f.target, ConfigurationRevision: view.ConfigurationRevision, BillingTermToken: view.BillingTermToken, Values: view.Values})
	if err != nil || quote == nil || quote.StructuredConfigurationNo == "" {
		t.Fatalf("real context -> QuoteTarget: %+v / %+v", quote, err)
	}
	f.assertCount(t, `SELECT count(*) FROM plugin_idc_core_native_server_target_quote`, 1)
	// The same public context supports a real staff actor without impersonation.
	f.native.pricing.principal = trafficTestPrincipal{}
	f.native.pricing.authorization = staffEditAuthorization{edit: true}
	authorizer, err := commerce.NewTargetQuoteAuthorizer(f.native.pricing.bizCtx, f.native.pricing.principal, f.native.pricing.authorization)
	if err != nil {
		t.Fatal(err)
	}
	staffOwner, err := commerce.New(f.database, resolver, f.native.pricing.principal, f.settlementService(t))
	if err != nil {
		t.Fatal(err)
	}
	staffFiles, err := purchaseconfigurationfile.NewOrderFileConsumer(f.database, f.native.pricing.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = commerce.BindStructuredConfigurationSnapshots(staffOwner, freezer, staffFiles); err != nil {
		t.Fatal(err)
	}
	f.native.targetQuotes = staffOwner
	if err = commerce.BindTargetQuoteAuthorizer(staffOwner, authorizer); err != nil {
		t.Fatal(err)
	}
	staffView, err := f.native.ReadConfigurationContext(f.ctx, servercommercialcap.ConfigurationContextRequest{Target: f.target})
	if err != nil {
		t.Fatal(err)
	}
	staffQuote, err := f.native.QuoteTarget(f.ctx, servercommercialcap.StructuredTargetInput{Target: f.target, ConfigurationRevision: staffView.ConfigurationRevision, BillingTermToken: staffView.BillingTermToken, Values: staffView.Values})
	if err != nil || staffQuote == nil {
		t.Fatalf("staff context -> target: %+v / %+v", staffQuote, err)
	}
	count, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_native_server_target_quote b JOIN plugin_idc_core_commerce_quote_snapshot q ON q.quote_no=b.target_quote_no WHERE b.target_quote_no=? AND q.source_code='staff' AND q.created_by=101 AND q.customer_no=? AND b.entitlement_no=?`, staffQuote.QuoteNo, f.target.CustomerNo, f.target.EntitlementNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("staff target actor/owner binding: %v / %+v", count, err)
	}

}
