package commerce

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
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
	for n := 0; n < 25; n++ {
		_, err = service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", ManualDiscountMinor: 1000, PaymentTermCode: AssistedPaymentTermStandardPrepay, Reason: "numbered pagination test", ValiditySeconds: 3600, IdempotencyKey: fmt.Sprintf("numbered-assisted-%d", n), Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}}})
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

func TestAssistedQuoteApprovalAndCheckoutPostgreSQL(t *testing.T) {
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
	authorizer := &assistedQuoteAuthorizerStub{actor: 1001}
	audit := &assistedQuoteAuditStub{}
	service, err := NewAssistedQuoteService(
		database, resolver, commerceService, audit, authorizer,
		[]byte("0123456789abcdef0123456789abcdef"),
	)
	if err != nil {
		t.Fatalf("create assisted quote service: %v", err)
	}
	created, err := service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		ManualDiscountMinor: 1_000, PaymentTermCode: AssistedPaymentTermStandardPrepay,
		Reason: "customer retention assisted quotation", InternalNote: "staff only",
		ValiditySeconds: 3600, IdempotencyKey: "assisted-create-0001",
		Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
	})
	if err != nil || created == nil || created.StatusCode != AssistedQuoteStatusPendingApproval ||
		created.PaymentDueSeconds < 300 || len(created.Items) != 1 || created.Reused {
		t.Fatalf("create assisted quote result=%+v err=%v", created, err)
	}
	replayed, err := service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		ManualDiscountMinor: 1_000, PaymentTermCode: AssistedPaymentTermStandardPrepay,
		Reason: "customer retention assisted quotation", InternalNote: "staff only",
		ValiditySeconds: 3600, IdempotencyKey: "assisted-create-0001",
		Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
	})
	if err != nil || replayed == nil || !replayed.Reused || replayed.RequestNo != created.RequestNo {
		t.Fatalf("replay assisted quote result=%+v err=%v", replayed, err)
	}
	_, err = service.DecideAssistedQuote(ctx, AssistedQuoteDecisionInput{
		RequestNo: created.RequestNo, DecisionCode: AssistedQuoteDecisionApproved,
		DecisionReason: "pricing and inventory reviewed", IdempotencyKey: "assisted-approve-0001",
	})
	if !bizerr.Is(err, CodeAssistedQuoteApprovalRequired) {
		t.Fatalf("self approval error=%v", err)
	}
	authorizer.actor = 1002
	approved, err := service.DecideAssistedQuote(ctx, AssistedQuoteDecisionInput{
		RequestNo: created.RequestNo, DecisionCode: AssistedQuoteDecisionApproved,
		DecisionReason: "pricing and inventory reviewed", IdempotencyKey: "assisted-approve-0001",
	})
	if err != nil || approved == nil || approved.StatusCode != AssistedQuoteStatusApproved ||
		approved.QuoteNo == "" || approved.DiscountMinor != 1_000 || approved.TotalAmountMinor <= 0 ||
		approved.DecisionBy != 1002 || !approved.CanCheckout {
		t.Fatalf("approve assisted quote result=%+v err=%v", approved, err)
	}
	decisionEvidence, err := database.GetValue(ctx, `
SELECT decision_evidence_digest
FROM plugin_idc_core_commerce_assisted_quote_request
WHERE request_no = ?`, created.RequestNo)
	if err != nil || !quoteDigestPattern.MatchString(decisionEvidence.String()) {
		t.Fatalf("server-derived assisted decision evidence=%q err=%v", decisionEvidence.String(), err)
	}
	authorizer.actor = 1003
	const checkoutWorkers = 8
	start := make(chan struct{})
	checkoutResults := make([]*AssistedQuoteRequest, checkoutWorkers)
	checkoutErrors := make([]error, checkoutWorkers)
	var checkoutWait sync.WaitGroup
	for index := range checkoutWorkers {
		checkoutWait.Add(1)
		go func(index int) {
			defer checkoutWait.Done()
			<-start
			checkoutResults[index], checkoutErrors[index] = service.CheckoutAssistedQuote(
				ctx,
				AssistedQuoteCheckoutInput{
					RequestNo: created.RequestNo, IdempotencyKey: "assisted-checkout-0001",
				},
			)
		}(index)
	}
	close(start)
	checkoutWait.Wait()
	ordered := checkoutResults[0]
	for index := range checkoutWorkers {
		candidate := checkoutResults[index]
		if checkoutErrors[index] != nil || candidate == nil || candidate.StatusCode != AssistedQuoteStatusOrdered ||
			candidate.OrderNo == "" || candidate.OrderStatusCode == "paid" || candidate.OrderStatusCode == "active" ||
			ordered == nil || candidate.OrderNo != ordered.OrderNo {
			t.Fatalf("concurrent assisted checkout %d result=%+v err=%v", index, candidate, checkoutErrors[index])
		}
	}
	checkoutReplay, err := service.CheckoutAssistedQuote(ctx, AssistedQuoteCheckoutInput{
		RequestNo: created.RequestNo, IdempotencyKey: "assisted-checkout-0001",
	})
	if err != nil || checkoutReplay == nil || !checkoutReplay.Reused ||
		checkoutReplay.OrderNo != ordered.OrderNo {
		t.Fatalf("replay assisted checkout result=%+v err=%v", checkoutReplay, err)
	}
	var persisted struct {
		SourceCode        string `orm:"source_code"`
		StatusCode        string `orm:"status_code"`
		AssistedRequestNo string `orm:"assisted_request_no"`
		PaymentTermCode   string `orm:"payment_term_code"`
		Paid              bool   `orm:"paid"`
	}
	if err = database.GetScan(ctx, &persisted, `
SELECT source_code, status_code, assisted_request_no, payment_term_code,
       (paid_at IS NOT NULL) AS paid
FROM plugin_idc_core_commerce_order WHERE order_no = ?`, ordered.OrderNo); err != nil {
		t.Fatalf("load assisted order evidence: %v", err)
	}
	if persisted.SourceCode != "admin" || persisted.AssistedRequestNo != created.RequestNo ||
		persisted.PaymentTermCode != "standard_prepay" || persisted.Paid ||
		(persisted.StatusCode != "pending_payment" && persisted.StatusCode != "manual_review") {
		t.Fatalf("unsafe assisted order evidence=%+v", persisted)
	}
	if len(audit.transitions) != 3 || audit.transitions[0].OperationCode != "commerce.assisted-quote.create" ||
		audit.transitions[1].OperationCode != "commerce.assisted-quote.approve" ||
		audit.transitions[2].OperationCode != "commerce.assisted-quote.checkout" {
		t.Fatalf("assisted quote audit transitions=%+v", audit.transitions)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_order (
    order_no, customer_no, quote_no, currency_code, source_code, status_code,
    risk_decision_code, requires_contract, item_count, subtotal_minor,
    discount_minor, net_amount_minor, tax_amount_minor, total_amount_minor,
    checkout_idempotency_digest, checkout_request_digest, payment_due_at
) SELECT 'unsafe-admin-order', customer_no, quote_no, currency_code, 'admin',
         'pending_payment', 'approved', FALSE, item_count, subtotal_minor,
         discount_minor, net_amount_minor, tax_amount_minor, total_amount_minor,
         repeat('1', 64), repeat('2', 64), CURRENT_TIMESTAMP + INTERVAL '1 hour'
	  FROM plugin_idc_core_commerce_order WHERE order_no = ?`, ordered.OrderNo); err == nil {
		t.Fatal("database accepted an admin order without an approved assisted request")
	} else if !strings.Contains(err.Error(), "ck_idc_com_order_assisted_source") {
		t.Fatalf("unsafe admin order failed for the wrong reason: %v", err)
	}

	authorizer.actor = 1004
	expired, err := service.CreateAssistedQuote(ctx, AssistedQuoteCreateInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY",
		PaymentTermCode: AssistedPaymentTermExtendedPrepay, PaymentDueSeconds: 172800,
		Reason: "expired assisted quotation must fail closed", ValiditySeconds: 900,
		IdempotencyKey: "assisted-expiry-0001",
		Items:          []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
	})
	if err != nil || expired == nil {
		t.Fatalf("create expiry fixture result=%+v err=%v", expired, err)
	}
	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
DISABLE TRIGGER trg_idc_com_assisted_request_identity_immutable`); err != nil {
		t.Fatalf("disable assisted request identity trigger: %v", err)
	}
	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_assisted_quote_request
SET created_at = CURRENT_TIMESTAMP - INTERVAL '2 hours',
    expires_at = CURRENT_TIMESTAMP - INTERVAL '1 hour',
    updated_at = CURRENT_TIMESTAMP
WHERE request_no = ?`, expired.RequestNo); err != nil {
		t.Fatalf("expire assisted quote fixture: %v", err)
	}
	if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_core_commerce_assisted_quote_request
ENABLE TRIGGER trg_idc_com_assisted_request_identity_immutable`); err != nil {
		t.Fatalf("enable assisted request identity trigger: %v", err)
	}
	authorizer.actor = 1005
	_, err = service.DecideAssistedQuote(ctx, AssistedQuoteDecisionInput{
		RequestNo: expired.RequestNo, DecisionCode: AssistedQuoteDecisionApproved,
		DecisionReason: "expired request reviewed", IdempotencyKey: "assisted-expiry-approve-0001",
	})
	if !bizerr.Is(err, CodeAssistedQuoteConflict) {
		t.Fatalf("expired assisted quote approval error=%v", err)
	}
	expiredView, err := service.GetAssistedQuote(ctx, expired.RequestNo)
	if err != nil || expiredView == nil || expiredView.CanApprove || expiredView.CanCheckout ||
		expiredView.StatusCode != AssistedQuoteStatusPendingApproval {
		t.Fatalf("expired assisted quote projection=%+v err=%v", expiredView, err)
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
		PaymentTermCode: AssistedPaymentTermStandardPrepay,
		Reason:          "audit rollback assisted quotation", ValiditySeconds: 3600,
		IdempotencyKey: "assisted-audit-fail-0001",
		Items:          []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
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
}
