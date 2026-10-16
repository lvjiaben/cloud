// These tests cover configured currency choices and refusal before assisted quote persistence.
package commerce

import (
	"context"
	"crypto/rand"
	"lina-core/pkg/bizerr"
	"reflect"
	"testing"
	"time"
)

type assistedCurrencyStub struct{ enabled bool }

func (s *assistedCurrencyStub) Enabled(context.Context) ([]string, error) {
	return []string{"CNY", "USD"}, nil
}
func (s *assistedCurrencyStub) IsEnabled(context.Context, string) (bool, error) {
	return s.enabled, nil
}
func TestAssistedConfiguredCurrenciesPostgreSQL(t *testing.T) {
	ctx, db := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, db)
	seedCommerceServiceCatalog(t, ctx, db)
	resolver, purchase := newAssistedQuoteTestPurchase(t, ctx, db)
	owner, err := newService(db, resolver, commercePrincipalStub{}, rand.Reader, commerceSettlementStub{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAssistedQuoteService(db, resolver, owner, &assistedQuoteAuditStub{}, &assistedQuoteAuthorizerStub{actor: 1001}, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	source := &assistedCurrencyStub{}
	if err = BindAssistedCurrencies(service, source); err != nil {
		t.Fatal(err)
	}
	codes, err := service.AssistedOrderCurrencies(ctx, "customer-commerce-service-1")
	if err != nil || !reflect.DeepEqual(codes, []string{"CNY", "USD"}) {
		t.Fatalf("configured choices: %v %v", codes, err)
	}
	input := AssistedQuoteCreateInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(), IdempotencyKey: "configured-currency-quote", Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}}}
	if _, err = service.CreateAssistedQuote(ctx, input); !bizerr.Is(err, CodeAssistedQuoteInvalid) {
		t.Fatalf("disabled currency admitted: %v", err)
	}
	source.enabled = true
	if _, err = service.CreateAssistedQuote(ctx, input); err != nil {
		t.Fatalf("enabled quote failed: %v", err)
	}
}
