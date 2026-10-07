package runtimeconfig

import "testing"

func TestCustomerPublicURLUsesOnlyExplicitSafeOrigin(t *testing.T) {
	t.Setenv(customerPublicOriginEnvironment, "https://customer.example.test/")
	got, err := CustomerPublicURL("/account/assistance?token=safe-token")
	if err != nil || got != "https://customer.example.test/account/assistance?token=safe-token" {
		t.Fatalf("customer URL=%q err=%v", got, err)
	}

	t.Setenv(customerPublicOriginEnvironment, "http://127.0.0.1:6677")
	if _, err = CustomerPublicURL("/account/assistance?token=safe-token"); err != nil {
		t.Fatalf("loopback development origin rejected: %v", err)
	}

	for _, invalid := range []string{
		"", "http://customer.example.test", "https://user@customer.example.test",
		"https://customer.example.test/base", "https://customer.example.test/?source=admin",
	} {
		t.Setenv(customerPublicOriginEnvironment, invalid)
		if _, invalidErr := CustomerPublicURL("/account/assistance?token=safe-token"); invalidErr == nil {
			t.Fatalf("unsafe origin accepted: %q", invalid)
		}
	}

	t.Setenv(customerPublicOriginEnvironment, "https://customer.example.test")
	if _, err = CustomerPublicURL("//attacker.example/account/assistance"); err == nil {
		t.Fatal("network-path reference was accepted")
	}
}

func TestCustomerPublicURLAllowsExactActivationRoute(t *testing.T) {
	t.Setenv(customerPublicOriginEnvironment, "https://customer.example.com")
	value, err := CustomerPublicURL("/auth/customer-activation?token=opaque")
	if err != nil || value != "https://customer.example.com/auth/customer-activation?token=opaque" {
		t.Fatalf("activation URL=%q err=%v", value, err)
	}
}
