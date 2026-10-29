// This file validates independent notification and local browser return origins.
package channelconfig

import (
	"lina-core/pkg/bizerr"
	"testing"
)

func TestNormalizeIndependentPaymentPublicAddresses(t *testing.T) {
	for _, customer := range []string{"https://customer.example", "http://localhost:6032", "http://127.0.0.1:6032", "http://[::1]:6032"} {
		value, err := NormalizePublicAddress(PublicAddress{NotificationOrigin: "https://notify.example/", CustomerReturnOrigin: customer})
		if err != nil || value.NotificationOrigin != "https://notify.example" || value.CustomerReturnOrigin != customer {
			t.Fatalf("independent addresses rejected: %+v %v", value, err)
		}
	}
	for _, notification := range []string{"http://notify.example", "https://user:secret@notify.example", "https://notify.example/path", "https://notify.example?key=x", "https://notify.example#x", "https://localhost.invalid"} {
		if _, err := NormalizePublicAddress(PublicAddress{NotificationOrigin: notification, CustomerReturnOrigin: "https://customer.example"}); !bizerr.Is(err, CodePublicAddressInvalid) {
			t.Fatalf("invalid notification origin accepted: %q err=%v", notification, err)
		}
	}
	if _, err := NormalizePublicAddress(PublicAddress{NotificationOrigin: "https://notify.example", CustomerReturnOrigin: "http://customer.example"}); !bizerr.Is(err, CodePublicAddressInvalid) {
		t.Fatalf("remote plaintext browser return accepted: %v", err)
	}
	if _, err := NormalizePublicAddress(PublicAddress{}); !bizerr.Is(err, CodePublicAddressUnconfigured) {
		t.Fatalf("empty origins accepted: %v", err)
	}
}
