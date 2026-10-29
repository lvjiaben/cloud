// This file verifies the public-product owner read with isolated PostgreSQL
// mappings. Customer access never borrows the direct-staff catalog authorizer.
package interfacecandidate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type importedBillingExtensions struct {
	pluginhost.BusinessExtensionRegistrar
	product, resource *capmodel.BusinessExtensionProviderEvidence
	detail            resourceinterfaceprojectioncap.CatalogDetail
	terms             channelcommercecap.BillingTerms
	invoked           []string
	afterRead         func()
}

func (e *importedBillingExtensions) GetProviderExact(_ context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if input.Point == capmodel.BusinessExtensionProductFulfillment {
		return e.product, nil
	}
	return e.resource, nil
}
func (e *importedBillingExtensions) WithConfidentialOutboundContext(ctx context.Context, _ pluginhost.ConfidentialOutboundContextInput, action pluginhost.ConfidentialOutboundAction) error {
	return action(ctx, &capmodel.BusinessExtensionRuntimeContext{ContextID: "native-billing"})
}
func (e *importedBillingExtensions) InvokeProviderMethod(_ context.Context, input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	e.invoked = append(e.invoked, input.Method)
	var value any = e.detail
	if input.Method == resourcecap.DynamicProviderMethodCatalogBillingTerms {
		value = e.terms
	}
	raw, err := json.Marshal(value)
	if e.afterRead != nil {
		e.afterRead()
	}
	return &capmodel.BusinessExtensionMethodResult{Payload: raw}, err
}
func TestImportedBillingRequiresFrozenExactBindingPostgreSQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("LINAPRO_IDC_RESOURCE_TEST_DSN"))
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_RESOURCE_TEST_DSN")
	}
	ctx := t.Context()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var random [8]byte
	if _, err = rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	schema := "idc_imported_billing_" + hex.EncodeToString(random[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, catalogReadFixtureSQL); err != nil {
		t.Fatal(err)
	}
	const interfaceNo = "pfi_11111111111111111111111111111111"
	const revision = "rci_11111111111111111111111111111111"
	product := nativeCatalogProductEvidence(t)
	a, b := product.ReleaseDigest, product.DeclarationDigest
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_channel (tenant_id,channel_no,provider_id,provider_plugin_id,provider_runtime_type,provider_contract_version,provider_release_version,provider_release_digest,provider_declaration_digest,endpoint,non_secret_settings,status_code,deleted_at) VALUES(0,'CHN_native','native-provider',?,'dynamic','v1','v1.0.0',?,?,'https://native.example','{}','enabled',NULL)`, product.PluginID, a, b); err != nil {
		t.Fatal(err)
	}
	methods := []string{resourcecap.DynamicProviderMethodReadCatalogProduct, resourcecap.DynamicProviderMethodCatalogBillingTerms}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_channel_binding VALUES(0,?,'CHN_native',?,'v1.0.0','v1',ARRAY['catalog.products.detail','catalog.billing-terms'],?,?,?,'v1.0.0','v1',ARRAY['provision','query','configuration.candidates'],?,?,'accepting',?)`, interfaceNo, product.PluginID, a, b, product.PluginID, a, b, revision); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_resource_interface_configuration_revision VALUES(0,?,?,'CHN_native','https://native.example','{}','{}')`, revision, interfaceNo); err != nil {
		t.Fatal(err)
	}
	snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "41"}, ConfigurationRevision: strings.Repeat("a", 64), Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{}}, Mappings: []channelcommercecap.ImportFieldMapping{}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{MaximumPurchaseCycles: 1, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable}}
	extensions := &importedBillingExtensions{product: product, resource: &capmodel.BusinessExtensionProviderEvidence{PluginID: product.PluginID, Point: resourcecap.DynamicProviderPoint, ReleaseVersion: product.ReleaseVersion, ContractVersion: product.ContractVersion, ReleaseDigest: a, DeclarationDigest: b, AuthorizedMethods: methods}, detail: resourceinterfaceprojectioncap.CatalogDetail{Product: resourceinterfaceprojectioncap.CatalogProduct{Reference: "41", Name: "Native product"}, ImportSnapshot: snapshot, BasePrices: []resourceinterfaceprojectioncap.CatalogPrice{}, Options: []resourceinterfaceprojectioncap.CatalogOption{}, ProductConfiguration: []resourceinterfaceprojectioncap.CatalogConfigurationValue{}}, terms: channelcommercecap.BillingTerms{ConfigurationRevision: snapshot.ConfigurationRevision, CurrencyCode: "CNY", Terms: []channelcommercecap.BillingTerm{{BillingTermToken: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, SetupAmount: "0", SubtotalAmount: "850", TaxAmount: "0", TotalAmount: "850"}}}}
	reader := NewLazy()
	if err = reader.Bind(database, extensions, func(context.Context, channelconfig.Configuration, string, []resourcecap.SecretBinding) (pluginhost.ConfidentialOutboundHandler, func(), error) {
		return func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			return nil, nil
		}, func() {}, nil
	}); err != nil {
		t.Fatal(err)
	}
	deniedStaff := &catalogAuthorizerStub{deny: true}
	source := &catalogSource{reader: reader, authorizer: deniedStaff}
	current, err := loadBinding(ctx, database, interfaceNo)
	if err != nil {
		t.Fatal(err)
	}
	evidence := *current.readEvidence(interfaceNo)
	evidence.SaleImportProtocol = productfulfillmentcap.SaleImportProtocolNativeFinanceV1
	request := resourceinterfaceprojectioncap.ImportedBillingRequest{Evidence: evidence, Input: channelcommercecap.ConfigurationInput{PublicSlug: "41", ConfigurationRevision: snapshot.ConfigurationRevision, CurrencyCode: "CNY"}}
	var candidateRuntime resourceinterfaceprojectioncap.CandidateSource = reader
	billingRuntime, ok := candidateRuntime.(resourceinterfaceprojectioncap.ImportedBillingSource)
	if !ok {
		t.Fatal("candidate runtime does not expose owner billing bridge")
	}
	result, err := billingRuntime.ReadImportedBillingTerms(ctx, request)
	if err != nil || result.Terms[0].TotalAmount != "850" || deniedStaff.calls != 0 || len(extensions.invoked) != 2 {
		t.Fatalf("native owner billing failed result=%+v calls=%v err=%v", result, extensions.invoked, err)
	}
	request.Evidence.ResourceChannelRef = "CHN_foreign"
	if _, err = source.ReadImportedBillingTerms(ctx, request); err == nil || len(extensions.invoked) != 2 {
		t.Fatal("foreign frozen binding reached supplier")
	}
	request.Evidence = evidence
	extensions.product.ReleaseDigest = strings.Repeat("c", 64)
	if _, err = source.ReadImportedBillingTerms(ctx, request); err == nil || len(extensions.invoked) != 2 {
		t.Fatal("changed exact provider reached supplier")
	}
	extensions.product.ReleaseDigest = a
	request.Input.ConfigurationRevision = strings.Repeat("b", 64)
	if _, err = source.ReadImportedBillingTerms(ctx, request); err == nil || len(extensions.invoked) != 3 {
		t.Fatal("source configuration change reached quote")
	}
	request.Input.ConfigurationRevision = snapshot.ConfigurationRevision
	extensions.afterRead = func() {
		if _, updateErr := database.Exec(ctx, `UPDATE plugin_idc_resource_interface_channel_binding SET current_configuration_revision='rci_22222222222222222222222222222222'`); updateErr != nil {
			t.Error(updateErr)
		}
	}
	if _, err = source.ReadImportedBillingTerms(ctx, request); err == nil {
		t.Fatal("interface changed while source was read")
	}
}
