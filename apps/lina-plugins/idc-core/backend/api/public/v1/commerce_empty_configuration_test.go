package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"strings"
	"testing"
)

// Products without governed fields legitimately quote and submit an empty array.
func TestCartAcceptsProductWithoutConfigurationFields(t *testing.T) {
	req := &GuestCartItemAddReq{Quantity: 1, PublicSlug: "no-fields", ConfigurationRevision: strings.Repeat("a", 43), BillingTermToken: "term", CurrencyCode: "CNY", PurchaseCycles: 1, Values: []*PurchaseConfigurationValue{}}
	if err := g.Validator().Data(req).Run(t.Context()); err != nil {
		t.Fatalf("empty governed field set rejected: %v", err)
	}
	req.ConfigurationRevision = ""
	if err := g.Validator().Data(req).Run(t.Context()); err == nil {
		t.Fatal("unsigned configuration must still be rejected")
	}
}
