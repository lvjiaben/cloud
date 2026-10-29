// This file proves native procurement is rebuilt from paid configuration and
// exact import evidence, then reaches only the typed resource-owner boundary.
package productfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/do"
)

type nativePurchaseRegistrar struct {
	pluginhost.BusinessExtensionRegistrar
	exact *capmodel.BusinessExtensionProviderEvidence
}

func (r *nativePurchaseRegistrar) GetProviderExact(_ context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if input.PluginID != r.exact.PluginID || input.ReleaseDigest != r.exact.ReleaseDigest || input.DeclarationDigest != r.exact.DeclarationDigest {
		return nil, errors.New("wrong exact provider")
	}
	return r.exact, nil
}

func TestNativeWorkerRebuildsPurchasePostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	rollback := errors.New("rollback")
	err = db.Transaction(t.Context(), func(ctx context.Context, _ gdb.TX) error {
		if _, err := db.Exec(ctx, `CREATE TEMP TABLE plugin_idc_core_catalog_import_contract(tenant_id INT,version_no TEXT,interface_instance_no TEXT,snapshot JSONB,snapshot_digest TEXT,read_evidence JSONB);
 CREATE TEMP TABLE plugin_idc_core_catalog_import_term(tenant_id INT,version_no TEXT,billing_rule_no TEXT,currency_code TEXT,upstream_term_reference TEXT);
 CREATE TEMP TABLE plugin_idc_core_catalog_import_candidate(tenant_id INT,version_no TEXT,field_key TEXT,candidate_value TEXT,option_token TEXT);
 INSERT INTO plugin_idc_core_catalog_import_term VALUES(0,'PV_1','monthly','CNY','native-monthly');`); err != nil {
			return err
		}
		label := &capmodel.LocalizedText{ZhCN: "规格", EnUS: "Specification"}
		snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "41"}, ConfigurationRevision: strings.Repeat("d", 64), PricingComplete: true, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cfg_8", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "12", Label: label}}}}}, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "cfg_8", FieldToken: "config:8", Options: []channelcommercecap.ImportOptionMapping{{Value: "12", OptionToken: "12"}}}}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "native-monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}}}, Fulfillment: channelcommercecap.ImportFulfillment{MaximumPurchaseCycles: 1, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable}}
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		declaration := &productfulfillmentcap.ProviderDeclaration{SaleImportProtocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Capability: productfulfillmentcap.ProviderCapability{ZhCN: "原生转售", EnUS: "Native sale"}, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingManagement: true, EntitlementOwnerID: provisionresourcecap.OwnerID, RenewalSupport: productfulfillmentcap.RenewalSupportUnsupported, ExpiryManagement: productfulfillmentcap.ExpiryManagementUnsupported, MethodRoles: productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query"}}
		metadata, err := productfulfillmentcap.NewProviderMetadata(declaration)
		if err != nil {
			return err
		}
		claim, request := resourceWorkerFixture(productfulfillmentcap.ActionSubmit)
		claim.PluginID = "unlisted-native"
		claim.ReleaseVersion = "v0.1.0"
		claim.ContractVersion = "v1"
		claim.ReleaseDigest = strings.Repeat("a", 64)
		claim.DeclarationDigest = strings.Repeat("b", 64)
		claim.ProvisionMethod = "provision"
		claim.QueryMethod = "query"
		claim.VersionNo = "PV_1"
		claim.BillingRuleNo = "monthly"
		claim.CurrencyCode = "CNY"
		claim.PurchaseCycles = 1
		claim.Quantity = 3
		claim.CustomerConfiguration = json.RawMessage(`{"cfg_8":"12"}`)
		request.ProductConfiguration = json.RawMessage(`{"upstream_product_id":"41","upstream_configuration_revision":"` + snapshot.ConfigurationRevision + `"}`)
		request.CustomerConfiguration = claim.CustomerConfiguration
		request.Quantity = 3
		exact := &capmodel.BusinessExtensionProviderEvidence{PluginID: claim.PluginID, ReleaseVersion: claim.ReleaseVersion, ContractVersion: claim.ContractVersion, ReleaseDigest: claim.ReleaseDigest, DeclarationDigest: claim.DeclarationDigest, AuthorizedMethods: []string{"provision", "query"}, ProviderMetadata: metadata}
		read := resourceinterfaceprojectioncap.CatalogReadEvidence{SaleImportProtocol: declaration.SaleImportProtocol, InterfaceInstanceNo: claim.InterfaceInstanceNo, OwnerID: "idc-resource", ResourceChannelRef: "CHN_native", InterfaceConfigurationRevision: claim.ResourceInterfaceConfigurationRevision, PluginID: claim.PluginID, PluginReleaseVersion: claim.ReleaseVersion, ContractVersion: claim.ContractVersion, ReleaseDigest: claim.ReleaseDigest, DeclarationDigest: claim.DeclarationDigest, AuthorizedMethods: exact.AuthorizedMethods}
		readRaw, err := json.Marshal(read)
		if err != nil {
			return err
		}
		if _, err = db.Model(do.CatalogImportContract{}).Ctx(ctx).Data(do.CatalogImportContract{TenantId: 0, VersionNo: claim.VersionNo, InterfaceInstanceNo: claim.InterfaceInstanceNo, Snapshot: string(raw), SnapshotDigest: sha256Hex(raw), ReadEvidence: string(readRaw)}).Insert(); err != nil {
			return err
		}
		source := &resourceSourceStub{task: validResourceTask(provisionresourcecap.TaskStatusPending)}
		worker := &fulfillmentWorker{database: db, extensions: &nativePurchaseRegistrar{exact: exact}, resourceSource: source}
		result := worker.invokeResourceOwner(ctx, claim, request)
		if result.failed || len(source.created) != 1 {
			t.Fatalf("native task was not handed to owner: %+v", result)
		}
		forwarded := source.created[0]
		if forwarded.NativePurchase == nil || forwarded.NativePurchase.Quantity != 1 || forwarded.Fulfillment.Quantity != 3 || forwarded.NativePurchase.BillingTermToken != "native-monthly" || len(forwarded.NativePurchase.Values) != 1 || forwarded.NativePurchase.Values[0].FieldToken != "config:8" || forwarded.NativePurchase.Values[0].OptionTokens[0] != "12" || provisionresourcecap.ValidateTaskRequest(forwarded) != nil {
			t.Fatalf("wrong frozen purchase: %+v", forwarded)
		}
		claim.PurchaseCycles = 2
		rejected := worker.invokeResourceOwner(ctx, claim, request)
		if !rejected.failed || len(source.created) != 1 {
			t.Fatal("native cycles bypass reached owner")
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}
