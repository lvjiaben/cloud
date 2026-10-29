package runtimeconfig

import "testing"

func TestLoadPersonalVerificationReturnURLUsesOnlyCustomerHTTPSOrigin(t *testing.T) {
	t.Setenv(customerPublicOriginEnvironment, "https://customer.example.test/")
	value, err := LoadPersonalVerificationReturnURL()
	if err != nil || value != "https://customer.example.test/account/identity?verification_return=1" {
		t.Fatalf("value=%q err=%v", value, err)
	}
	for _, invalid := range []string{
		"", "http://customer.example.test", "https://user@customer.example.test",
		"https://:443",
		"https://customer.example.test/base", "https://customer.example.test/?next=evil",
		"https://customer.example.test/#fragment",
	} {
		t.Setenv(customerPublicOriginEnvironment, invalid)
		if value, loadErr := LoadPersonalVerificationReturnURL(); loadErr == nil || value != "" {
			t.Fatalf("invalid origin %q returned value=%q err=%v", invalid, value, loadErr)
		}
	}
}

func TestLoadReturnURLsAllowExplicitLoopbackHTTPForLocalDevelopment(t *testing.T) {
	t.Setenv(customerPublicOriginEnvironment, "http://127.0.0.1:6032")
	identityValue, err := LoadPersonalVerificationReturnURL()
	if err != nil || identityValue != "http://127.0.0.1:6032/account/identity?verification_return=1" {
		t.Fatalf("identity value=%q err=%v", identityValue, err)
	}
}
