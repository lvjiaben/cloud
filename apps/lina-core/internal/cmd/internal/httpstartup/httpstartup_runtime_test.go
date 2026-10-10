// This file verifies startup rejects public or missing JWT signing secrets.

package httpstartup

import "testing"

// TestValidateJWTSigningSecret covers insecure templates and configured secrets.
func TestValidateJWTSigningSecret(t *testing.T) {
	for _, test := range []struct {
		name, secret string
		rejected     bool
	}{
		{"empty", "", true}, {"whitespace", " ", true},
		{"template", "lina-jwt-secret-key-change-in-production", true},
		{"padded template", " lina-jwt-secret-key-change-in-production ", true},
		{"configured", "independently-configured-test-secret", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := validateJWTSigningSecret(test.secret); (err != nil) != test.rejected {
				t.Fatalf("unexpected validation outcome: %v", err)
			}
		})
	}
}
