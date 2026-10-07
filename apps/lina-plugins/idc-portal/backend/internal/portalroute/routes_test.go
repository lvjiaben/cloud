// This file verifies exact reset-route ownership and safe customer-origin handling.
package portalroute

import (
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/pluginhost"
)

// recordingHTTPRegistrar captures global middleware registration without starting a server.
type recordingHTTPRegistrar struct {
	scopes []pluginhost.MiddlewareScope
}

// Routes is unused by the password-reset bridge.
func (*recordingHTTPRegistrar) Routes() pluginhost.RouteRegistrar { return nil }

// GlobalMiddlewares returns the recorder itself.
func (registrar *recordingHTTPRegistrar) GlobalMiddlewares() pluginhost.GlobalMiddlewareRegistrar {
	return registrar
}

// Services is unused by the password-reset bridge.
func (*recordingHTTPRegistrar) Services() capability.Services { return nil }

// BusinessExtensions is unused by the password-reset bridge.
func (*recordingHTTPRegistrar) BusinessExtensions() pluginhost.BusinessExtensionRegistrar { return nil }

// Bind records one scope and rejects invalid handlers in the test double.
func (registrar *recordingHTTPRegistrar) Bind(scope pluginhost.MiddlewareScope, handler pluginhost.MiddlewareHandler) error {
	if handler == nil {
		panic("password-reset route handler must not be nil")
	}
	registrar.scopes = append(registrar.scopes, scope)
	return nil
}

// TestRegisterBindsOnlyExactResetPath verifies that no broad Portal shell route remains.
func TestRegisterBindsOnlyExactResetPath(t *testing.T) {
	t.Setenv(customerPublicOriginEnvironment, "https://customer.example.test")
	registrar := &recordingHTTPRegistrar{}
	if err := Register(registrar); err != nil {
		t.Fatalf("Register returned error: %v", err)
	}
	if len(registrar.scopes) != 1 || registrar.scopes[0] != hostResetPasswordPath {
		t.Fatalf("unexpected scopes: %#v", registrar.scopes)
	}
}

// TestRegisterRejectsMissingInputs verifies startup validation fails closed.
func TestRegisterRejectsMissingInputs(t *testing.T) {
	if err := Register(nil); err == nil {
		t.Fatal("expected a nil registrar to fail")
	}
	t.Setenv(customerPublicOriginEnvironment, "")
	if err := Register(&recordingHTTPRegistrar{}); err == nil {
		t.Fatal("expected an empty customer origin to fail")
	}
}

// TestParseCustomerPublicOriginRejectsUnsafeValues verifies request data cannot choose the redirect host.
func TestParseCustomerPublicOriginRejectsUnsafeValues(t *testing.T) {
	for _, invalid := range []string{
		"",
		"http://customer.example.test",
		"https://user@customer.example.test",
		"https://customer.example.test/base",
		"https://customer.example.test/?source=admin",
	} {
		if _, err := parseCustomerPublicOrigin(invalid); err == nil {
			t.Errorf("unsafe origin accepted: %q", invalid)
		}
	}
	for _, valid := range []string{
		"https://customer.example.test",
		"http://127.0.0.1:6677",
		"http://localhost:6677",
	} {
		if _, err := parseCustomerPublicOrigin(valid); err != nil {
			t.Errorf("safe origin %q rejected: %v", valid, err)
		}
	}
}

// TestResetPasswordTargetPreservesOnlyOneToken verifies unrelated and ambiguous query data is dropped.
func TestResetPasswordTargetPreservesOnlyOneToken(t *testing.T) {
	origin, err := parseCustomerPublicOrigin("https://customer.example.test")
	if err != nil {
		t.Fatalf("parse safe origin: %v", err)
	}
	if got := resetPasswordTarget(origin, "token=opaque%2Bvalue&redirect=https%3A%2F%2Fattacker.test"); got != "https://customer.example.test/auth/reset-password?token=opaque%2Bvalue" {
		t.Fatalf("unexpected reset target: %q", got)
	}
	if got := resetPasswordTarget(origin, "token=one&token=two"); got != "https://customer.example.test/auth/reset-password" {
		t.Fatalf("duplicate token was preserved: %q", got)
	}
}
