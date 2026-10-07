// This file verifies order query authorization, pagination and safe projections.

package commerce

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestOrderCursorRejectsTamperingAndFilterReuse(t *testing.T) {
	codec, err := newOrderCursorCodec(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatalf("create order cursor: %v", err)
	}
	digest := sha256.Sum256([]byte("customer=one&status=pending_payment"))
	encoded, err := codec.encode(orderCursorBoundary{
		CreatedAtMicroseconds: time.Now().UnixMicro(), ID: 91,
	}, digest)
	if err != nil {
		t.Fatalf("encode order cursor: %v", err)
	}
	decoded, err := codec.decode(encoded, digest)
	if err != nil || decoded.ID != 91 {
		t.Fatalf("decode order cursor=%+v err=%v", decoded, err)
	}
	tamperedBytes := []byte(encoded)
	if tamperedBytes[len(tamperedBytes)/2] == 'A' {
		tamperedBytes[len(tamperedBytes)/2] = 'B'
	} else {
		tamperedBytes[len(tamperedBytes)/2] = 'A'
	}
	tampered := string(tamperedBytes)
	if _, err = codec.decode(tampered, digest); err == nil {
		t.Fatal("tampered cursor must be rejected")
	}
	otherDigest := sha256.Sum256([]byte("customer=two&status=pending_payment"))
	if _, err = codec.decode(encoded, otherDigest); err == nil {
		t.Fatal("cursor must be bound to customer and filters")
	}
}

func TestManagementOrderAuthorizerKeepsListAndDetailIndependent(t *testing.T) {
	current := riskReviewBizCtxStub{current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2}}
	staff := riskReviewPrincipalStub{audience: &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	authorizer, err := NewManagementOrderAuthorizer(current, staff, riskReviewAuthzStub{
		grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionOrderDirectoryList): true},
	})
	if err != nil {
		t.Fatalf("create order authorizer: %v", err)
	}
	if err = authorizer.AuthorizeOrderList(context.Background()); err != nil {
		t.Fatalf("authorize order list: %v", err)
	}
	if err = authorizer.AuthorizeOrderDetail(context.Background(), "ORD_0123456789abcdef0123456789abcdef"); !bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
		t.Fatalf("list permission leaked into detail: %v", err)
	}
}

func TestManagementOrderAuthorizerFailsClosedForUnsafeActors(t *testing.T) {
	staff := &principal.Audience{
		UserID: 71, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}
	tests := []struct {
		name     string
		current  bizctxcap.CurrentContext
		audience *principal.Audience
		allow    bool
	}{
		{name: "direct staff", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1}, audience: staff, allow: true},
		{name: "customer audience", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1}, audience: &principal.Audience{UserID: 71, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive}},
		{name: "impersonation", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1, IsImpersonation: true}, audience: staff},
		{name: "acting user", current: bizctxcap.CurrentContext{UserID: 71, ActingUserID: 72, TenantID: 0, DataScope: 1}, audience: staff},
		{name: "acting tenant", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 1, ActingAsTenant: true}, audience: staff},
		{name: "unsupported scope", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 0, DataScope: 2, DataScopeUnsupported: true}, audience: staff},
		{name: "foreign tenant", current: bizctxcap.CurrentContext{UserID: 71, TenantID: 9, DataScope: 1}, audience: staff},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			authorizer, err := NewManagementOrderAuthorizer(
				riskReviewBizCtxStub{current: testCase.current},
				riskReviewPrincipalStub{audience: testCase.audience},
				riskReviewAuthzStub{grants: map[authz.PermissionKey]bool{
					authz.PermissionKey(permissionOrderDirectoryList): true,
				}},
			)
			if err != nil {
				t.Fatalf("create order authorizer: %v", err)
			}
			authorizeErr := authorizer.AuthorizeOrderList(context.Background())
			if testCase.allow {
				if authorizeErr != nil {
					t.Fatalf("expected direct staff to be allowed: %v", authorizeErr)
				}
				return
			}
			if !bizerr.Is(authorizeErr, CodeOrderDirectoryNotAccessible) {
				t.Fatalf("unsafe actor was not denied: %v", authorizeErr)
			}
		})
	}
}

type managementOrderAuthorizerStub struct{}

func (managementOrderAuthorizerStub) AuthorizeOrderList(context.Context) error           { return nil }
func (managementOrderAuthorizerStub) AuthorizeOrderDetail(context.Context, string) error { return nil }

// TestOrderNumberedPaginationPostgreSQL uses real checkout-created orders and
// confirms scoped totals, fixed query cost, page boundaries and input refusal.
func TestOrderNumberedPaginationPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 100)
	for index := 0; index < 4; index++ {
		current := scenario
		if index > 0 {
			customerNo, userID := "customer-commerce-service-1", 101
			if index == 3 {
				customerNo, userID = "customer-commerce-service-2", 102
			}
			current = newCheckoutCustomerOnDatabase(t, scenario, customerNo, userID, 1)
		}
		if _, err := current.service.CheckoutAccountQuote(current.ctx, CheckoutRequest{
			QuoteNo: current.quote.QuoteNo, IdempotencyKey: fmt.Sprintf("numbered-orders-checkout-%04d", index),
		}); err != nil {
			t.Fatal(err)
		}
	}
	key := bytes.Repeat([]byte{0x55}, 32)
	reader, err := NewManagementOrderReader(scenario.database, managementOrderAuthorizerStub{}, key)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		page, limit, count, rows int
		customer                 string
	}{
		{1, 2, 4, 2, ""}, {2, 2, 4, 2, ""}, {3, 2, 4, 0, ""},
		{1, 100, 4, 4, ""}, {2, 2, 3, 1, "customer-commerce-service-1"},
		{1, 10, 0, 0, "customer-missing"},
	} {
		page, readErr := reader.ListManagementOrders(scenario.ctx, OrderDirectoryFilter{Page: test.page, Limit: test.limit, CustomerNo: test.customer})
		if readErr != nil {
			t.Fatal(readErr)
		}
		if page.Total != test.count || len(page.Orders) != test.rows {
			t.Fatalf("case %+v got %+v", test, page)
		}
	}
	account, err := NewAccountOrderReader(scenario.database, scenario.service.principal, key)
	if err != nil {
		t.Fatal(err)
	}
	page, err := account.ListAccountOrders(scenario.ctx, OrderDirectoryFilter{Page: 2, Limit: 2, CustomerNo: "customer-commerce-service-2"})
	if err != nil || page.Total != 3 || len(page.Orders) != 1 {
		t.Fatalf("account ownership count = %+v, %v", page, err)
	}
	for _, input := range []OrderDirectoryFilter{{Page: -1}, {Page: 1, Cursor: "forbidden-mixed"}, {Page: math.MaxInt, Limit: 100}} {
		if _, err = reader.ListManagementOrders(scenario.ctx, input); !bizerr.Is(err, CodeOrderDirectoryInvalid) {
			t.Fatalf("invalid pagination accepted: %+v, %v", input, err)
		}
	}
}

func TestOrderDirectoryAggregatePostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, false, 0)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-order-directory-0001",
	})
	if err != nil {
		t.Fatalf("checkout order directory scenario: %v", err)
	}
	paymentPreparation, err := NewPaymentPreparationService(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatalf("create payment preparation: %v", err)
	}
	if _, err = paymentPreparation.PrepareAccountOrderPayment(scenario.ctx, checkout.OrderNo); err != nil {
		t.Fatalf("prepare order payment: %v", err)
	}
	key := bytes.Repeat([]byte{0x55}, 32)
	reader, err := NewAccountOrderReader(scenario.database, scenario.service.principal, key)
	if err != nil {
		t.Fatalf("create account order reader: %v", err)
	}
	page, err := reader.ListAccountOrders(scenario.ctx, OrderDirectoryFilter{Limit: 20})
	if err != nil || len(page.Orders) != 1 || page.Orders[0].OrderNo != checkout.OrderNo ||
		page.Orders[0].CustomerNo != "" || !page.Orders[0].CanPreparePayment {
		t.Fatalf("account order page=%+v err=%v", page, err)
	}
	queryLogger := &checkoutBenchmarkQueryLogger{
		ILogger: scenario.database.GetLogger(), counts: make(map[int]int),
	}
	previousLogger, previousDebug := scenario.database.GetLogger(), scenario.database.GetDebug()
	scenario.database.SetLogger(queryLogger)
	scenario.database.SetDebug(true)
	t.Cleanup(func() {
		scenario.database.SetDebug(previousDebug)
		scenario.database.SetLogger(previousLogger)
	})
	detailContext := context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9301)
	detail, err := reader.GetAccountOrder(detailContext, checkout.OrderNo)
	if err != nil || detail == nil || detail.Invoice == nil || len(detail.Items) != 1 ||
		detail.PaymentPreparation == nil || detail.Summary.CustomerNo != "" {
		t.Fatalf("account order detail=%+v err=%v", detail, err)
	}
	if queries := queryLogger.count(9301); queries != 15 {
		t.Fatalf("account order detail query budget=%d, want 15", queries)
	}
	otherReader, err := NewAccountOrderReader(scenario.database, commercePrincipalStub{audience: &principal.Audience{
		UserID: 202, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-2",
	}}, key)
	if err != nil {
		t.Fatalf("create other account order reader: %v", err)
	}
	if _, err = otherReader.GetAccountOrder(scenario.ctx, checkout.OrderNo); !bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
		t.Fatalf("cross-customer order detail error=%v", err)
	}
	managementReader, err := NewManagementOrderReader(scenario.database, managementOrderAuthorizerStub{}, key)
	if err != nil {
		t.Fatalf("create management order reader: %v", err)
	}
	managementDetail, err := managementReader.GetManagementOrder(scenario.ctx, checkout.OrderNo)
	if err != nil || managementDetail.Summary.CustomerNo != "customer-commerce-service-1" {
		t.Fatalf("management order detail=%+v err=%v", managementDetail, err)
	}
}
