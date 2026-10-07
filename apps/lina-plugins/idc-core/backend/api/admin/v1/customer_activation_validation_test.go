// This file verifies the public validation contract for customer activation requests.

package v1

import (
	"context"
	"testing"

	"github.com/gogf/gf/v2/frame/g"
)

func TestCustomerDirectAccountCreateCountryCodeValidation(t *testing.T) {
	valid := &CustomerDirectAccountCreateReq{
		CustomerType: "individual", DisplayName: "Validation Customer",
		CountryCode: "CN", PreferredLocale: "zh-CN",
		Email: "validation@example.test", InitialPassword: "StrongPassw0rd!",
		Reason:         "Validate direct customer country input.",
		IdempotencyKey: "customer-direct-validation-0001",
	}
	if err := g.Validator().Data(valid).Run(context.Background()); err != nil {
		t.Fatalf("two-letter country code should pass: %v", err)
	}

	for _, countryCode := range []string{"C", "CHN"} {
		invalid := *valid
		invalid.CountryCode = countryCode
		if err := g.Validator().Data(&invalid).Run(context.Background()); err == nil {
			t.Fatalf("country code %q should be rejected", countryCode)
		}
	}
}
