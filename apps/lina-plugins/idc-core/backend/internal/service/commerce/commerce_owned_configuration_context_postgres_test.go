// Read-only context uses real compiler output and owner DDL; altered identities
// and revisions must fail without creating sales or execution documents.
package commerce

import (
	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOwnedConfigurationContextCurrentSCFAndNoWritesPostgreSQL(t *testing.T) {
	f := newTargetQuoteFixture(t)
	_, source, _, _ := runtime.Caller(0)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "manifest", "sql", "152-native-priced-server-operation.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range dialect.SplitSQLStatements(string(data))[:5] {
		if _, err = f.database.Exec(f.ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	public, err := catalog.NewPublicCatalogReaderConfigured(f.database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, public)
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(public)
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := freezer.FreezePurchaseConfiguration(f.ctx, catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values})
	if err != nil {
		t.Fatal(err)
	}
	var interfaceNo, interfaceVersion any
	if frozen.InterfaceInstanceNo != "" {
		interfaceNo = frozen.InterfaceInstanceNo
		interfaceVersion = frozen.InterfaceConfigurationRevision
	}
	_, err = f.database.Exec(f.ctx, `INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest,order_no,order_item_no,bound_at)
 SELECT 'SCF_context_parent',e.customer_no,101,o.quote_no,i.quote_item_no,e.product_no,e.version_no,?,?,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),repeat('e',64),e.order_no,e.order_item_no,CURRENT_TIMESTAMP FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_commerce_order o ON o.order_no=e.order_no JOIN plugin_idc_core_commerce_order_item i ON i.order_item_no=e.order_item_no WHERE e.entitlement_no=?`, frozen.ConfigurationRevision, interfaceNo, interfaceVersion, frozen.CustomerDeclarationDigest, frozen.CandidateSnapshotDigest, frozen.CustomerValueDigest, string(frozen.CandidateSnapshotJSON), string(frozen.TypedValueSnapshotJSON), f.entitlementNo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET configuration_snapshot_no='SCF_context_parent' WHERE entitlement_no=?`, f.entitlementNo); err != nil {
		t.Fatal(err)
	}
	var input catalog.OwnedPurchaseConfigurationInput
	input.EntitlementNo = f.entitlementNo
	input.CustomerNo = "customer-commerce-service-1"
	input.SnapshotNo = "SCF_context_parent"
	input.PublicSlug = f.purchase.PublicSlug
	input.CurrencyCode = "CNY"
	input.PurchaseCycles = 1
	if err = f.database.GetScan(f.ctx, &input, `SELECT product_no,version_no,billing_rule_no,period_unit_code,period_count FROM plugin_idc_core_service_entitlement WHERE entitlement_no=?`, f.entitlementNo); err != nil {
		t.Fatal(err)
	}
	reader, ok := public.(catalog.OwnedPurchaseConfigurationReader)
	if !ok {
		t.Fatal("owned configuration reader missing")
	}
	contextView, err := reader.ReadOwnedPurchaseConfiguration(f.ctx, input)
	if err != nil || contextView == nil || !contextView.Valid || contextView.BillingTermToken == "" {
		t.Fatalf("owned context=%+v / %+v", contextView, err)
	}
	for _, value := range contextView.Values {
		if len(value.FileTokens) > 0 {
			t.Fatal("context exposed saved file credentials")
		}
	}

	edit := input
	edit.ConfigurationRevision = contextView.ConfigurationRevision
	edit.Values = contextView.Values
	if _, err = reader.ReadOwnedPurchaseConfiguration(f.ctx, edit); err != nil {
		t.Fatalf("public edit reevaluation: %+v", err)
	}
	for _, test := range []struct {
		name   string
		change func(*catalog.OwnedPurchaseConfigurationInput)
	}{
		{"foreign", func(in *catalog.OwnedPurchaseConfigurationInput) { in.CustomerNo = "customer-commerce-service-2" }},
		{"other-ent", func(in *catalog.OwnedPurchaseConfigurationInput) { in.EntitlementNo = "ENT_other" }},
		{"forged-scf", func(in *catalog.OwnedPurchaseConfigurationInput) { in.SnapshotNo = "SCF_forged" }},
		{"version", func(in *catalog.OwnedPurchaseConfigurationInput) { in.VersionNo = "version-foreign" }},
		{"revision", func(in *catalog.OwnedPurchaseConfigurationInput) { in.ConfigurationRevision = "revision-forged" }},
		{"candidate", func(in *catalog.OwnedPurchaseConfigurationInput) {
			in.ConfigurationRevision = contextView.ConfigurationRevision
			in.Values = []catalog.PublicPurchaseConfigurationValue{{FieldToken: "forged", Type: "single_select", OptionTokens: []string{"forged"}}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := input
			test.change(&copy)
			if view, err := reader.ReadOwnedPurchaseConfiguration(f.ctx, copy); err == nil || view != nil {
				t.Fatalf("unsafe context=%+v / %+v", view, err)
			}
		})
	}
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_commerce_quote_snapshot", 2)
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_commerce_order", 1)
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_native_server_target_quote", 0)
	target, err := f.service.QuoteAccountTarget(f.ctx, AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: contextView.ConfigurationRevision, BillingTermToken: contextView.BillingTermToken, CurrencyCode: input.CurrencyCode, PurchaseCycles: input.PurchaseCycles, Values: contextView.Values}})
	if err != nil || target == nil || len(target.Items) != 1 {
		t.Fatalf("read context was not a usable owned target: %+v / %+v", target, err)
	}

}
