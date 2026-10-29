// This file verifies distinct transport rules for server notifications and
// browser returns, including the local customer-center development origin.
package paymentcap

import (
	"testing"
	"time"
)

func TestCreateRequestNotificationAndBrowserReturnTransports(t *testing.T) {
	for _, scenario := range []struct {
		name, notify, customer string
		allowed                bool
	}{
		{"https", "https://notify.example/callback", "https://customer.example/payment/return/PAT_test", true},
		{"local IPv4", "https://baidu.com/callback", "http://127.0.0.1:6032/payment/return/PAT_test", true},
		{"local IPv6", "https://notify.example/callback", "http://[::1]:6032/payment/return/PAT_test", true},
		{"local hostname", "https://notify.example/callback", "http://localhost:6032/payment/return/PAT_test", true},
		{"remote http", "https://notify.example/callback", "http://customer.example/payment/return", false},
		{"lookalike local hostname", "https://notify.example/callback", "http://localhost.example/payment/return", false},
		{"lookalike IP hostname", "https://notify.example/callback", "http://127.0.0.1.example/payment/return", false},
		{"plaintext notify", "http://notify.example/callback", "https://customer.example/payment/return", false},
		{"local plaintext notify", "http://127.0.0.1/callback", "http://127.0.0.1/payment/return", false},
		{"return userinfo", "https://notify.example/callback", "https://user:secret@customer.example/payment/return", false},
		{"no origin", "https://notify.example/callback", "/payment/return", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			err := ValidateCreateRequest(CreateRequest{AttemptNo: "PAT_test", InvoiceNo: "INV_test", Currency: "CNY", AmountMinor: 100, Subject: "isolated payment", ExpiresAt: time.Now().Add(time.Hour), NotifyURL: scenario.notify, ReturnURL: scenario.customer})
			if (err == nil) != scenario.allowed {
				t.Fatalf("transport acceptance=%t want=%t err=%v", err == nil, scenario.allowed, err)
			}
		})
	}
}
