// These tests exercise real structured configuration approval and checkout.
package commerce

import (
	"crypto/rand"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

func TestAssistedStructuredConfigurationCheckoutPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, database)
	applyAssistedQuoteTestMigration(t, ctx, database)
	seedCommerceServiceCatalog(t, ctx, database)
	prepareStructuredCheckoutDatabase(t, ctx, database)
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_configuration DISABLE TRIGGER trg_idc_guard_product_version_configuration;
 UPDATE plugin_idc_core_catalog_product_version_configuration SET customer_declaration_snapshot='{"version":"v1","fields":[{"key":"domain","type":"single_line","required":true,"label":{"zhCN":"域名","enUS":"Domain"}}]}'::jsonb WHERE version_no='version-commerce-service-1';
 ALTER TABLE plugin_idc_core_catalog_product_version_configuration ENABLE TRIGGER trg_idc_guard_product_version_configuration;`); err != nil {
		t.Fatal(err)
	}
	actor := &assistedQuoteAuthorizerStub{actor: 1001}
	writer, database := realAssistedAuditWriter(t, ctx, database, actor)
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	config, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil || len(config.Fields) != 1 {
		t.Fatalf("configuration=%+v err=%v", config, err)
	}
	domain := "example.com"
	values := []catalog.PublicPurchaseConfigurationValue{{FieldToken: config.Fields[0].FieldToken, Type: "single_line", TextValue: &domain}}
	pricing, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: config.ConfigurationRevision, Values: values}, CurrencyCode: "CNY"})
	if err != nil || len(pricing.Terms) != 1 {
		t.Fatalf("pricing=%+v err=%v", pricing, err)
	}
	// Connectors must use the authoritative interval, never parse translated labels.
	if pricing.Terms[0].ChargeType != catalog.ChargeTypeRecurring || pricing.Terms[0].PeriodUnit != catalog.PeriodUnitMonth || pricing.Terms[0].PeriodCount != 1 {
		t.Fatalf("billing interval was lost in purchase configuration: %+v", pricing.Terms[0])
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	service, err := newService(database, resolver, commercePrincipalStub{}, rand.Reader, commerceSettlementStub{})
	if err != nil {
		t.Fatal(err)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(database, service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(service, freezer, files); err != nil {
		t.Fatal(err)
	}
	assisted, err := NewAssistedQuoteService(database, resolver, service, writer, actor, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	input := AssistedQuoteCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(), Reason: "Customer requested a configured service", IdempotencyKey: "assisted-structured-create-1", Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: config.ConfigurationRevision, BillingTermToken: pricing.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1, Values: values}}}}
	created, err := assisted.CreateAssistedQuote(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := assisted.CreateAssistedQuote(ctx, input)
	if err != nil || replayed.RequestNo != created.RequestNo || !replayed.Reused {
		t.Fatalf("create replay=%+v err=%v", replayed, err)
	}
	var snapshot struct {
		CustomerNo         string
		UploaderUserID     int
		TypedValueSnapshot string
	}
	if err = database.GetScan(ctx, &snapshot, `SELECT customer_no,uploader_user_id,typed_value_snapshot FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE quote_no=?`, created.QuoteNo); err != nil {
		t.Fatal(err)
	}
	if snapshot.CustomerNo != input.CustomerNo || snapshot.UploaderUserID != 1001 || snapshot.TypedValueSnapshot == "" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	if created.StatusCode != AssistedQuoteStatusOrdered || created.OrderNo == "" || created.DecisionBy != 0 {
		t.Fatalf("direct=%+v", created)
	}
	var order struct{ CreatedBy int }
	if err = database.GetScan(ctx, &order, `SELECT created_by FROM plugin_idc_core_commerce_order WHERE order_no=?`, created.OrderNo); err != nil || order.CreatedBy != 1001 {
		t.Fatalf("actor=%+v err=%v", order, err)
	}
	var auditCount struct{ Count int }
	if err = database.GetScan(ctx, &auditCount, `SELECT count(*) FROM plugin_idc_core_business_audit_event WHERE target_no=? AND actor_user_id=1001 AND operation_code='commerce.assisted-quote.create'`, created.RequestNo); err != nil || auditCount.Count != 1 {
		t.Fatalf("audit=%+v err=%v", auditCount, err)
	}
	input.IdempotencyKey = "assisted-structured-stale"
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_configuration DISABLE TRIGGER trg_idc_guard_product_version_configuration; UPDATE plugin_idc_core_catalog_product_version_configuration SET customer_declaration_snapshot='{"version":"v1","fields":[]}'::jsonb WHERE version_no='version-commerce-service-1'; ALTER TABLE plugin_idc_core_catalog_product_version_configuration ENABLE TRIGGER trg_idc_guard_product_version_configuration;`); err != nil {
		t.Fatal(err)
	}
	if _, err = assisted.CreateAssistedQuote(ctx, input); err == nil {
		t.Fatal("stale configuration accepted")
	}
	var count struct{ Count int }
	if err = database.GetScan(ctx, &count, `SELECT count(*) FROM plugin_idc_core_commerce_order`); err != nil || count.Count != 1 {
		t.Fatalf("failed creation escaped transaction count=%+v err=%v", count, err)
	}
}

func TestAssistedConfigurationRejectsCustomerFiles(t *testing.T) {
	item := AssistedQuoteItemInput{Quantity: 1, Purchase: catalog.PurchaseConfigurationInput{PublicSlug: "cloud", CurrencyCode: "CNY", ConfigurationRevision: "revision", BillingTermToken: "term", PurchaseCycles: 1, Values: []catalog.PublicPurchaseConfigurationValue{{FileTokens: []string{"customer-private-file"}}}}}
	if err := normalizeAssistedQuoteItem(&item, "CNY"); !bizerr.Is(err, CodeAssistedQuoteInvalid) {
		t.Fatalf("private customer file accepted: %v", err)
	}
}
