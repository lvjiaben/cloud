package commerce

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type assistedQuoteAuthorizerStub struct {
	actor int
}

func TestAssistedQuoteNumberedPagesPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, database)
	seedCommerceServiceCatalog(t, ctx, database)
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, database)
	commerceService, err := newService(database, resolver, commercePrincipalStub{}, rand.Reader, commerceSettlementStub{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAssistedQuoteService(database, resolver, commerceService, &assistedQuoteAuditStub{}, &assistedQuoteAuthorizerStub{actor: 1001}, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory SET sellable_quantity=1000`); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 25; n++ {
		_, err = service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", ManualDiscountMinor: 1000, PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(), Reason: "numbered pagination test", IdempotencyKey: fmt.Sprintf("numbered-assisted-%d", n), Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}}})
		if err != nil {
			t.Fatalf("create quote %d: %v", n, err)
		}
	}
	seen := map[string]bool{}
	for _, pageNumber := range []int{1, 2, 3, 50} {
		page, readErr := service.ListAssistedQuotes(ctx, AssistedQuoteFilter{Page: pageNumber, Limit: 10, CustomerNo: "customer-commerce-service-1"})
		want := 10
		if pageNumber == 3 {
			want = 5
		}
		if pageNumber == 50 {
			want = 0
		}
		if readErr != nil || page == nil || page.Total != 25 || len(page.Requests) != want {
			t.Fatalf("page%d: %+v err=%v", pageNumber, page, readErr)
		}
		for _, item := range page.Requests {
			if seen[item.RequestNo] {
				t.Fatalf("duplicate quote %s", item.RequestNo)
			}
			seen[item.RequestNo] = true
		}
	}
	large, err := service.ListAssistedQuotes(ctx, AssistedQuoteFilter{Page: 1, Limit: 100})
	if err != nil || large.Total != 25 || len(large.Requests) != 25 {
		t.Fatalf("page size100: %+v %v", large, err)
	}
	empty, err := service.ListAssistedQuotes(ctx, AssistedQuoteFilter{Page: 2, Limit: 10, StatusCode: AssistedQuoteStatusApproved})
	if err != nil || empty.Total != 0 || len(empty.Requests) != 0 {
		t.Fatalf("filtered empty: %+v %v", empty, err)
	}
}

func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteCreate(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteList(context.Context) (int, error) {
	return s.actor, nil
}
func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteDetail(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteApprove(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteReject(context.Context, string) (int, error) {
	return s.actor, nil
}
func (s *assistedQuoteAuthorizerStub) AuthorizeAssistedQuoteCheckout(context.Context, string) (int, error) {
	return s.actor, nil
}

type assistedQuoteAuditStub struct {
	fail        bool
	transitions []idccap.BusinessAuditTransition
}

func (s *assistedQuoteAuditStub) AppendBusinessTransition(
	_ context.Context,
	input idccap.BusinessAuditTransition,
) error {
	if s.fail {
		return errors.New("injected assisted quote audit failure")
	}
	s.transitions = append(s.transitions, input)
	return nil
}

func (*assistedQuoteAuditStub) AppendBusinessBatchTransition(
	context.Context,
	idccap.BusinessAuditBatchTransition,
) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, nil
}

func TestAssistedDirectOrderAndConcurrentReplayPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, database)
	seedCommerceServiceCatalog(t, ctx, database)
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, database)
	commerceService, err := newService(database, resolver, commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1", UserID: 2001,
	}}, rand.Reader, commerceSettlementStub{})
	if err != nil {
		t.Fatalf("create assisted commerce service: %v", err)
	}
	// Fixed/manual products must remain purchasable with production structured binding enabled.
	freezer, ok := resolver.(catalog.PurchaseConfigurationFreezer)
	if !ok {
		t.Fatal("catalog reader does not implement freezing")
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(database, commerceService.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(commerceService, freezer, files); err != nil {
		t.Fatal(err)
	}
	authorizer := &assistedQuoteAuthorizerStub{actor: 1001}
	audit := &assistedQuoteAuditStub{}
	service, err := NewAssistedQuoteService(
		database, resolver, commerceService, audit, authorizer,
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create assisted quote service: %v", err)
	}
	input := AssistedQuoteCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", ManualDiscountMinor: 1000, PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(), Reason: "", IdempotencyKey: "assisted-direct-create-1", Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}}}
	const workers = 8
	results := make([]*AssistedQuoteRequest, workers)
	failures := make([]error, workers)
	start := make(chan struct{})
	var wait sync.WaitGroup
	for index := range workers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], failures[index] = service.CreateAssistedQuote(ctx, input)
		}(index)
	}
	close(start)
	wait.Wait()
	created := results[0]
	for index, result := range results {
		if failures[index] != nil || result == nil || result.StatusCode != AssistedQuoteStatusOrdered || result.OrderNo == "" || result.OrderNo != created.OrderNo || result.CanApprove || result.CanCheckout || result.DecisionBy != 0 {
			t.Fatalf("direct result %d=%+v err=%v", index, result, failures[index])
		}
	}
	var evidence struct {
		Paid      bool
		DueAt     int64
		Decisions int
		Orders    int
		Debits    int
	}
	if err = database.GetScan(ctx, &evidence, `SELECT (o.paid_at IS NOT NULL) AS paid,(extract(epoch FROM o.payment_due_at)*1000)::bigint AS due_at,
 (SELECT count(*) FROM plugin_idc_core_commerce_assisted_quote_request WHERE decision_by IS NOT NULL)::int AS decisions,
 (SELECT count(*) FROM plugin_idc_core_commerce_order)::int AS orders,
 (SELECT count(*) FROM plugin_idc_core_commerce_order WHERE total_amount_minor=0)::int AS debits
 FROM plugin_idc_core_commerce_order o WHERE o.order_no=?`, created.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.Paid || evidence.DueAt != input.PaymentDueAt || evidence.Decisions != 0 || evidence.Orders != 1 || created.DiscountMinor != 1000 {
		t.Fatalf("evidence=%+v created=%+v", evidence, created)
	}
	if len(audit.transitions) != 1 || audit.transitions[0].OperationCode != "commerce.assisted-quote.create" {
		t.Fatalf("audit=%+v", audit.transitions)
	}
	input.ManualDiscountMinor++
	if _, err = service.CreateAssistedQuote(ctx, input); !bizerr.Is(err, CodeAssistedQuoteConflict) {
		t.Fatalf("changed replay=%v", err)
	}
	input.IdempotencyKey = "assisted-direct-expired"
	input.PaymentDueAt = time.Now().Add(-time.Minute).UnixMilli()
	if _, err = service.CreateAssistedQuote(ctx, input); !bizerr.Is(err, CodeAssistedQuoteInvalid) {
		t.Fatalf("expired=%v", err)
	}
}

func TestAssistedQuoteAuditFailureRollsBackPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, database)
	seedCommerceServiceCatalog(t, ctx, database)
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, database)
	commerceService, err := newService(
		database, resolver, commercePrincipalStub{}, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create commerce service: %v", err)
	}
	service, err := NewAssistedQuoteService(
		database, resolver, commerceService, &assistedQuoteAuditStub{fail: true},
		&assistedQuoteAuthorizerStub{actor: 1001},
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create assisted quote service: %v", err)
	}
	_, err = service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(),
		Reason:       "audit rollback assisted quotation", IdempotencyKey: "assisted-audit-fail-0001",
		Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
	})
	if !bizerr.Is(err, CodeAssistedQuoteUnavailable) {
		t.Fatalf("audit failure error=%v", err)
	}
	count, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_commerce_assisted_quote_request`)
	if err != nil || count.Int64() != 0 {
		t.Fatalf("audit failure escaped transaction count=%d err=%v", count.Int64(), err)
	}
}

func newAssistedQuoteTestPurchase(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) (catalog.PurchaseConfigurationResolver, catalog.PurchaseConfigurationInput) {
	t.Helper()
	reader, err := catalog.NewPublicCatalogReader(
		database, []byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create assisted quote catalog reader: %v", err)
	}
	bindTestPublicBillingLabels(t, reader)
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatalf("create assisted quote resolver: %v", err)
	}
	configuration, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatalf("load assisted quote configuration: %v", err)
	}
	quote, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil || quote == nil || len(quote.Terms) != 1 {
		t.Fatalf("load assisted public quote result=%+v err=%v", quote, err)
	}
	return resolver, catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quote.Terms[0].BillingTermToken, CurrencyCode: "CNY",
		PurchaseCycles: 1,
	}
}

func applyAssistedQuoteTestMigration(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve assisted quote test path")
	}
	path := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql",
		"026-commerce-assisted-order.sql")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read assisted quote migration: %v", err)
	}
	if _, err = database.Exec(ctx, string(content)); err != nil {
		t.Fatalf("execute assisted quote migration: %v", err)
	}
	for _, name := range []string{"114-assisted-purchase-configuration.sql", "115-direct-assisted-orders.sql"} {
		content, err = os.ReadFile(filepath.Join(filepath.Dir(path), name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = database.Exec(ctx, string(content)); err != nil {
			t.Fatal(err)
		}
	}
}
