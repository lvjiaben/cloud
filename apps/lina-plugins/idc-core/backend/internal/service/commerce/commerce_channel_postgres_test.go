// This file validates the public API quote lifecycle against real commerce
// tables: frozen prices, replay conflicts, customer isolation and checkout.
package commerce

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

func TestChannelFrozenQuoteCheckoutPostgreSQL(t *testing.T) {
	ctx, db := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, db)
	seedCommerceServiceCatalog(t, ctx, db)
	content, err := os.ReadFile(filepath.Join("../../../../manifest/sql", "102-channel-commerce-attribution.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(content)); err != nil {
		t.Fatal(err)
	}
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, db)
	current := commercePrincipalStub{audience: &principal.Audience{UserID: 101, CustomerNo: "customer-commerce-service-1", Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}}
	owner, err := NewWithContractTemplates(db, resolver, current, commerceSettlementStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	freezer, ok := resolver.(catalog.PurchaseConfigurationFreezer)
	if !ok {
		t.Fatal("missing freezer")
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(db, current)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(owner, freezer, files); err != nil {
		t.Fatal(err)
	}
	channel, err := NewChannelOrderService(db, resolver, owner)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := channel.CreateQuote(ctx, "app_test", "customer-commerce-service-1", "quote-request-one", purchase, 1)
	if err != nil || quote == nil || quote.TotalAmountMinor <= 0 {
		t.Fatalf("freeze quote: %+v %+v", quote, err)
	}
	replay, err := channel.CreateQuote(ctx, "app_test", "customer-commerce-service-1", "quote-request-one", purchase, 1)
	if err != nil || replay.QuoteNo != quote.QuoteNo || replay.TotalAmountMinor != quote.TotalAmountMinor {
		t.Fatalf("quote replay: %+v %v", replay, err)
	}
	if _, err = channel.CreateQuote(ctx, "app_test", "customer-commerce-service-1", "quote-request-one", purchase, 2); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("changed quote payload accepted: %v", err)
	}
	input := ChannelOrderInput{QuoteNo: quote.QuoteNo, ChannelID: "app_test", OwnershipMode: "channel_owned", PayerCustomerNo: "customer-commerce-service-1", ServiceOwnerCustomerNo: "customer-commerce-service-1", IdempotencyKey: "order-request-one"}
	order, err := channel.Checkout(ctx, input)
	if err != nil || order == nil || order.TotalAmountMinor != quote.TotalAmountMinor {
		t.Fatalf("checkout frozen quote: %+v %v", order, err)
	}
	again, err := channel.Checkout(ctx, input)
	if err != nil || again.OrderNo != order.OrderNo || !again.Reused {
		t.Fatalf("checkout replay: %+v %v", again, err)
	}
	input.PayerCustomerNo = "different-customer"
	input.ServiceOwnerCustomerNo = "different-customer"
	input.IdempotencyKey = "other-customer-request"
	if _, err = channel.Checkout(ctx, input); err == nil {
		t.Fatal("another customer checked out quote")
	}
	count, err := db.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order WHERE quote_no=?`, quote.QuoteNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("order replay duplicated: %v %v", count, err)
	}
}

func TestChannelCheckoutConsumesExactUploaderFilesPostgreSQL(t *testing.T) {
	ctx, db := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, db)
	seedCommerceServiceCatalog(t, ctx, db)
	content, err := os.ReadFile(filepath.Join("../../../../manifest/sql", "102-channel-commerce-attribution.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(content)); err != nil {
		t.Fatal(err)
	}
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, db)
	current := commercePrincipalStub{audience: &principal.Audience{UserID: 101, CustomerNo: "customer-commerce-service-1", Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}}
	owner, err := NewWithContractTemplates(db, resolver, current, commerceSettlementStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := NewChannelOrderService(db, resolver, owner)
	if err != nil {
		t.Fatal(err)
	}
	quote, err := channel.CreateQuote(ctx, "app_file", "customer-commerce-service-1", "file-quote", purchase, 1)
	if err != nil {
		t.Fatal(err)
	}
	scenario := checkoutTestScenario{ctx: ctx, database: db, resolver: resolver, purchase: purchase, service: owner.(*serviceImpl), quote: quote}
	prepareStructuredCheckoutSchema(t, scenario)
	item := loadStructuredQuoteItem(t, scenario, quote.QuoteNo)
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{ID: "customer.identity.v1", MaximumFiles: 1, MaximumFileBytes: 1024, MaximumTotalBytes: 1024, AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}}
	target := &purchaseconfigurationfile.Target{ProductVersionNo: item.VersionNo, ConfigurationRevision: "cfg-structured-1", FieldKey: "identity_document", Policy: policy}
	storage := &channelUploadStorage{}
	consumer, err := purchaseconfigurationfile.New(db, current, storage, channelUploadTarget{target}, channelUploadScanner{})
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	reservation, err := consumer.Reserve(ctx, purchaseconfigurationfile.ReserveInput{PublicSlug: purchase.PublicSlug, ConfigurationRevision: "public", FieldToken: "field", FileName: "identity.pdf", SizeBytes: int64(len(pdf))})
	if err != nil {
		t.Fatal(err)
	}
	upload, err := consumer.Upload(ctx, reservation.UploadToken, "application/pdf", bytes.NewReader(pdf))
	if err != nil {
		t.Fatal(err)
	}
	var digests struct{ SelectionDigest, CustomerValueDigest string }
	if err = db.GetScan(ctx, &digests, `SELECT configuration.selection_digest,configuration.customer_value_digest FROM plugin_idc_core_commerce_quote_item_snapshot item JOIN plugin_idc_core_commerce_configuration_snapshot configuration ON configuration.configuration_snapshot_no=item.configuration_snapshot_no WHERE item.quote_item_no=?`, item.QuoteItemNo); err != nil {
		t.Fatal(err)
	}
	frozen := &catalog.FrozenPurchaseConfiguration{ProductNo: item.ProductNo, VersionNo: item.VersionNo, ConfigurationRevision: target.ConfigurationRevision, InterfaceInstanceNo: "pfi_11111111111111111111111111111111", InterfaceConfigurationRevision: 1, CustomerDeclarationDigest: strings.Repeat("8", 64), CandidateSnapshotDigest: strings.Repeat("9", 64), CustomerValueDigest: digests.CustomerValueDigest, PriceSelectionDigest: digests.SelectionDigest, CandidateSnapshotJSON: []byte("{}"), TypedValueSnapshotJSON: []byte(`{"identity_document":["` + upload.FileToken + `"]}`), FileFields: []purchaseconfigurationfile.VerifyInput{{Target: target, FileTokens: []string{upload.FileToken}}}}
	if err = BindStructuredConfigurationSnapshots(owner, structuredBindingFreezerStub{frozen}, consumer); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.FreezeAccountQuoteItemConfiguration(ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: item.QuoteItemNo}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy DISABLE TRIGGER trg_idc_guard_fulfillment_policy;
UPDATE plugin_idc_core_catalog_product_version_fulfillment_policy SET strategy_code='payment_confirmed_plugin',granularity_code='per_order_item',ongoing_management=false,renewal_mode_code='unavailable',provision_method='provision',query_method='query',maximum_attempts=10,retry_base_seconds=30,policy_digest=repeat('f',64) WHERE version_no='version-commerce-service-1';
ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy ENABLE TRIGGER trg_idc_guard_fulfillment_policy`); err != nil {
		t.Fatal(err)
	}
	input := ChannelOrderInput{QuoteNo: quote.QuoteNo, ChannelID: "app_file", OwnershipMode: "channel_owned", PayerCustomerNo: "customer-commerce-service-1", ServiceOwnerCustomerNo: "customer-commerce-service-1", IdempotencyKey: "file-checkout"}
	order, err := channel.Checkout(ctx, input)
	if err != nil || order == nil {
		t.Fatalf("channel file checkout: %+v %v", order, err)
	}
	assertStructuredCheckoutEvidence(t, scenario, "consumed", order.OrderNo, order.OrderNo)
	replay, err := channel.Checkout(ctx, input)
	if err != nil || !replay.Reused || replay.OrderNo != order.OrderNo {
		t.Fatalf("channel file replay: %+v %v", replay, err)
	}
	seedFinancialSettlementMerchant(t, scenario)
	insertVerifiedIndividualPaymentIdentity(t, scenario, input.PayerCustomerNo)
	balance, err := billingservice.NewBalanceService(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, billingservice.ConfirmedRechargeInput{CustomerNo: input.PayerCustomerNo, CurrencyCode: "CNY", AmountMinor: order.TotalAmountMinor, SourceReference: "file-funding", SourceSnapshotDigest: strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billingservice.NewInvoiceService(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(ctx, billingservice.InvoiceBalancePaymentInput{InvoiceNo: order.InvoiceNo, SourceReference: "file-payment", SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(ctx, order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	fulfillment, err := productfulfillmentservice.NewFullConfigured(db, structuredFulfillmentEvidenceValidator{}, fulfillmentGateStub{}, fulfillmentSystemReasons(t), fulfillmentAuthorizerStub{}, fulfillmentAuditStub{}, fulfillmentActorsStub{}, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fulfillment.ConsumeFinanciallySettled(ctx, productfulfillmentservice.SettlementInput{SettlementNo: settlement.SettlementNo, OrderNo: order.OrderNo}); err != nil {
		t.Fatal(err)
	}
	var scope purchaseconfigurationfile.ConsumedFileScope
	if err = db.GetScan(ctx, &scope, `SELECT intent_no,order_no,order_item_no,configuration_snapshot_no,plugin_id,plugin_release_version AS release_version,contract_version,release_digest,declaration_digest,provision_method AS method,interface_instance_no,interface_configuration_revision FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, order.OrderNo); err != nil {
		t.Fatal(err)
	}
	reader, err := purchaseconfigurationfile.NewConsumedFileReader(db, storage)
	if err != nil {
		t.Fatal(err)
	}
	files, err := reader.ListConsumedField(ctx, scope, target.FieldKey)
	if err != nil || len(files) != 1 || files[0].SizeBytes != int64(len(pdf)) {
		t.Fatalf("consumed fulfillment scope: %+v %v", files, err)
	}
	visits := 0
	if err = reader.VisitConsumedField(ctx, scope, target.FieldKey, func(file purchaseconfigurationfile.ConsumedFile, body io.Reader) error {
		content, readErr := io.ReadAll(body)
		if readErr != nil {
			return readErr
		}
		if file.ObjectRef != "" || !bytes.Equal(content, pdf) {
			t.Fatal("file transfer leaked storage path or changed bytes")
		}
		visits++
		return nil
	}); err != nil || visits != 1 {
		t.Fatalf("stream consumed file: visits=%d err=%v", visits, err)
	}
	scope.OrderNo = "different-order"
	if files, err = reader.ListConsumedField(ctx, scope, target.FieldKey); err == nil && len(files) != 0 {
		t.Fatal("foreign order read consumed file")
	}
}

// These narrow in-memory dependencies keep the PostgreSQL test focused on
// real reservation, upload inspection, quote freezing and order consumption.
type channelUploadTarget struct {
	target *purchaseconfigurationfile.Target
}

func (r channelUploadTarget) ResolvePurchaseFileTarget(context.Context, string, string, string) (*purchaseconfigurationfile.Target, error) {
	return r.target, nil
}

type channelUploadScanner struct{}

func (channelUploadScanner) Ready() bool { return true }
func (channelUploadScanner) Scan(context.Context, io.Reader, int64) (string, error) {
	return "clean", nil
}

type channelUploadStorage struct {
	storagecap.Service
	objects map[string][]byte
}

func (s *channelUploadStorage) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	s.objects[input.Path] = data
	return &storagecap.PutOutput{Object: &storagecap.Object{Path: input.Path, Size: int64(len(data)), ContentType: input.ContentType, Visibility: storagecap.VisibilityPrivate}}, nil
}

func (s *channelUploadStorage) Get(_ context.Context, input storagecap.GetInput) (*storagecap.GetOutput, error) {
	data, found := s.objects[input.Path]
	if !found {
		return &storagecap.GetOutput{}, nil
	}
	return &storagecap.GetOutput{Found: true, Object: &storagecap.Object{Path: input.Path, Size: int64(len(data)), ContentType: "application/pdf", Visibility: storagecap.VisibilityPrivate}, Body: io.NopCloser(bytes.NewReader(data))}, nil
}
