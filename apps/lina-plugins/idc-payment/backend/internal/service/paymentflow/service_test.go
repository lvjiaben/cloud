// This file verifies public-origin validation without database or supplier access.

package paymentflow

import (
	"testing"

	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/paymentattempt"
)

// TestNewRequiresSiteOrigin prevents API prefixes from leaking into browser returns.
func TestNewRequiresSiteOrigin(t *testing.T) {
	t.Parallel()
	for _, input := range []struct {
		origin string
		valid  bool
	}{
		{"https://merchant.example", true},
		{"https://merchant.example/", true},
		{"https://merchant.example:8443", true},
		{"https://localhost.invalid", true},
		{"http://merchant.example", false},
		{"https://merchant.example/x/idc-payment", false},
		{"https://merchant.example/%2f", false},
		{"https://user:secret@merchant.example", false},
		{"https://merchant.example?token=value", false},
		{"https://merchant.example#fragment", false},
	} {
		t.Run(input.origin, func(t *testing.T) {
			svc, err := New(&paymentattempt.Store{}, &channelconfig.Service{}, Config{PublicOrigin: input.origin})
			if (err == nil) != input.valid {
				t.Fatalf("origin %q: valid=%v error=%v", input.origin, input.valid, err)
			}
			if input.valid && svc == nil {
				t.Fatal("valid origin did not construct a service")
			}
		})
	}
}
