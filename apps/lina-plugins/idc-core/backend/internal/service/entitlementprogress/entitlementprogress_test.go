// These tests keep local management readiness separate from business outcomes
// and prevent unknown persisted messages from escaping into reader diagnostics.
package entitlementprogress

import "testing"

func TestManagementProgress(t *testing.T) {
	for input, want := range map[string]Status{"": "", "pending": Preparing, "retry_wait": Preparing, "delivering": Preparing, "delivered": Ready, "manual_attention": Attention, "unknown-private-state": Attention} {
		if got := FromDelivery(input); got != want {
			t.Fatalf("state %q: got %q want %q", input, got, want)
		}
	}
	for _, code := range []string{"entitlement_owner_unavailable", "owner_delivery_unavailable", "owner_delivery_result_unknown"} {
		if SafeFailure(code) != code {
			t.Fatalf("lost reviewed category %s", code)
		}
	}
	for _, code := range []string{"", "password=private", "https://provider/?token=private", "owner_delivery_unavailable\nprivate"} {
		if SafeFailure(code) != "" {
			t.Fatal("unreviewed failure text escaped")
		}
	}
}
