// This file checks canonical guest exceptions without weakening protected routes.
package backend

import "testing"

func TestRecoveryGuardPublicPathBoundary(t *testing.T) {
	for _, value := range []string{"/api/v1/user/profile", "/x/idc-core/api/v1/account/billing/overview", "/x/idc-core/api/v1/account/products/public/operations", "/x/idc-core/api/v1/public/../account/billing/overview", "/api/v1/auth/refresh"} {
		if !protectedRecoveryPath(value) {
			t.Fatalf("protected route bypasses denial: %s", value)
		}
	}
	for _, value := range []string{"/api/v1/auth/login", "/api/v1/auth/logout", "/api/v1/config/public/frontend", "/x/idc-notify/api/v1/public/account/password-recovery/password-resets", "/x/idc-notify/api/v1/account/password-reset/session-cleanups"} {
		if protectedRecoveryPath(value) {
			t.Fatalf("guest or narrow cleanup entry blocked: %s", value)
		}
	}
}
