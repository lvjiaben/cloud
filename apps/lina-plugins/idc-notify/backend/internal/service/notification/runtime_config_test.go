package notification

import "testing"

func TestLoadRuntimeConfigUsesBoundedDailyDefaultAndExplicitOrigin(t *testing.T) {
	t.Setenv(publicCustomerOriginEnv, "https://customer.example.com/")
	config, err := LoadRuntimeConfig()
	if err != nil {
		t.Fatalf("load runtime config: %v", err)
	}
	if config.SMSVerificationDailyLimit != 1000 || config.PublicCustomerOrigin != "https://customer.example.com" {
		t.Fatalf("unexpected runtime config: %+v", config)
	}
}

func TestLoadRuntimeConfigRejectsExplicitInvalidDailyLimit(t *testing.T) {
	for _, value := range []string{"", "0", "100001", " 1000", "abc"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(smsDailyLimitEnv, value)
			if _, err := LoadRuntimeConfig(); err == nil {
				t.Fatalf("explicit invalid limit %q was accepted", value)
			}
		})
	}
}

func TestPublicCustomerOriginNeverTrustsUnsafeOrDecoratedOrigins(t *testing.T) {
	for _, value := range []string{
		"http://customer.example.com", "https://127.0.0.1", "https://10.0.0.2",
		"https://user@example.com", "https://example.com/path", "https://example.com?q=1", "https://example.com/#x",
	} {
		if normalized, ok := normalizePublicCustomerOrigin(value); ok || normalized != "" {
			t.Fatalf("unsafe origin %q was accepted as %q", value, normalized)
		}
	}
	if normalized, ok := normalizePublicCustomerOrigin("http://127.0.0.1:5173"); !ok || normalized != "http://127.0.0.1:5173" {
		t.Fatalf("explicit development loopback was rejected: %q %v", normalized, ok)
	}
}

func TestLoadRuntimeConfigKeepsMissingOrInvalidOriginUnavailable(t *testing.T) {
	for _, value := range []string{"", "http://customer.example.com", "https://10.0.0.2", " https://customer.example.com"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(publicCustomerOriginEnv, value)
			config, err := LoadRuntimeConfig()
			if err != nil {
				t.Fatalf("invalid public origin must not stop the service: %v", err)
			}
			if config.PublicCustomerOrigin != "" {
				t.Fatalf("invalid public origin became available: %q", config.PublicCustomerOrigin)
			}
		})
	}
}
