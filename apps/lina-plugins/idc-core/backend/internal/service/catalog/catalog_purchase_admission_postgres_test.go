// These tests keep final purchase admission authoritative while proving that
// resolving/freezing one configuration does not scan plugin discovery twice.
package catalog

import (
	"context"
	"os"
	"testing"
)

func TestPurchaseResolveAndFreezePerformOneFinalAdmissionPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	ctx := context.Background()
	database := configureFulfillmentPolicyTestDatabase(t, ctx, dsn)
	if _, err := database.Exec(ctx, publicNumberedFixtureSQL+importExportFixtureSQL+`
ALTER TABLE plugin_idc_core_catalog_product_version_configuration ADD COLUMN interface_instance_no TEXT,ADD COLUMN interface_configuration_revision BIGINT;
CREATE TABLE plugin_idc_core_product_automation_interface_instance(tenant_id INT,interface_instance_no TEXT,interface_origin_code TEXT);
CREATE TABLE plugin_idc_core_product_automation_interface_configuration_revision(tenant_id INT,interface_instance_no TEXT,revision BIGINT);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding VALUES(0,'v1');
INSERT INTO plugin_idc_core_product_automation_interface_instance VALUES(0,'interface-one','local');
INSERT INTO plugin_idc_core_catalog_product_version_configuration(tenant_id,version_no,version,customer_declaration_snapshot,customer_presentation_snapshot,declaration_digest,interface_instance_no,interface_configuration_revision) VALUES(0,'v1',1,'{"version":"v1","fields":[{"key":"note","type":"single_line","required":true,"label":{"zhCN":"备注","enUS":"Note"}}]}','{}','digest-one','interface-one',1);`); err != nil {
		t.Fatal(err)
	}
	availability := &publicPurchaseAvailabilityStub{available: true}
	publicReader, err := newPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"), availability, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reader := publicReader.(*publicCatalogReaderImpl)
	reader.billingLabels = PublicBillingLabelResources{"period.month": {ZhCN: "月", EnUS: "month"}}
	view, err := reader.GetPurchaseConfiguration(ctx, "alpha-one")
	if err != nil || len(view.Fields) != 1 {
		t.Fatal("initial local projection failed", err)
	}
	note := "customer choice"
	input := PublicPurchaseConfigurationInput{PublicSlug: "alpha-one", ConfigurationRevision: view.ConfigurationRevision, Values: []PublicPurchaseConfigurationValue{{FieldToken: view.Fields[0].FieldToken, Type: view.Fields[0].Type, TextValue: &note}}}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: input, CurrencyCode: "CNY"})
	if err != nil || len(quoted.Terms) != 1 || availability.calls != 0 {
		t.Fatal("local preview performed plugin admission", err)
	}
	purchase := PurchaseConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: input.Values, CurrencyCode: "CNY", BillingTermToken: quoted.Terms[0].BillingTermToken, PurchaseCycles: 1}
	for _, accepting := range []bool{true, false} {
		availability.available = accepting
		availability.calls = 0
		resolved, resolveErr := reader.ResolvePurchaseConfiguration(ctx, purchase)
		if availability.calls != 1 || (resolveErr == nil) != accepting || (resolved != nil) != accepting {
			t.Fatalf("resolve acceptance=%t calls=%d err=%v", accepting, availability.calls, resolveErr)
		}
		availability.calls = 0
		frozen, freezeErr := reader.FreezePurchaseConfiguration(ctx, input)
		if availability.calls != 1 || (freezeErr == nil) != accepting || (frozen != nil) != accepting {
			t.Fatalf("freeze acceptance=%t calls=%d err=%v", accepting, availability.calls, freezeErr)
		}
	}
	availability.available = true
	availability.calls = 0
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product SET status_code='suspended' WHERE product_no='p1'`); err != nil {
		t.Fatal(err)
	}
	if frozen, freezeErr := reader.FreezePurchaseConfiguration(ctx, input); freezeErr == nil || frozen != nil || availability.calls != 0 {
		t.Fatal("unavailable local product reached plugin discovery")
	}
}
