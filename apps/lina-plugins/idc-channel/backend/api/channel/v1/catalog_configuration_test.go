// These tests exercise the same GoFrame validation rules used by the HTTP route.
package v1

import (
	"context"
	"github.com/gogf/gf/v2/util/gvalid"
	"testing"
)

func TestCatalogBillingTermsAcceptsISOCurrency(t *testing.T) {
	for _, code := range []string{"CNY", "USD", "JPY", "", "CN", "CNYX"} {
		request := CatalogBillingTermsReq{PublicSlug: "test-product", ConfigurationRevision: "revision", CurrencyCode: code}
		err := gvalid.New().Data(request).Run(context.Background())
		if (err == nil) != (len(code) == 3) {
			t.Fatalf("unexpected currency validation for %q: %v", code, err)
		}
	}
}
