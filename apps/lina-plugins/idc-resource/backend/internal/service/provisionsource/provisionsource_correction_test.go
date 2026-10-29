// These tests reject malformed nonsecret password overrides before any task or
// secret is created and preserve paid values when applying a valid correction.
package provisionsource

import (
	"encoding/json"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"testing"
)

func TestRecoveryPasswordRules(t *testing.T) {
	for _, test := range []struct {
		length     int
		characters string
	}{
		{0, "digits"}, {5, "digits"}, {21, "digits"}, {12, ""}, {12, "digits,digits"}, {12, "unknown"}, {12, "uppercase, digits"},
	} {
		values := map[string]json.RawMessage{"cpu": json.RawMessage(`2`)}
		if err := applyRecoveryPasswordRules(values, provisionresourcecap.RecoveryConfiguration{PasswordLength: test.length, PasswordCharacters: test.characters}); err == nil {
			t.Fatal("invalid password rule accepted")
		}
		if len(values) != 1 {
			t.Fatal("invalid rule partially mutated configuration")
		}
	}
	values := map[string]json.RawMessage{"cpu": json.RawMessage(`2`), "password_length": json.RawMessage(`20`)}
	if err := applyRecoveryPasswordRules(values, provisionresourcecap.RecoveryConfiguration{}); err != nil || string(values["password_length"]) != "20" {
		t.Fatal("empty correction changed original rules")
	}
	if err := applyRecoveryPasswordRules(values, provisionresourcecap.RecoveryConfiguration{PasswordLength: 12, PasswordCharacters: "uppercase,digits"}); err != nil {
		t.Fatal(err)
	}
	if string(values["cpu"]) != "2" || string(values["password_length"]) != "12" || string(values["password_characters"]) != `["uppercase","digits"]` {
		t.Fatal("incorrect correction")
	}
}
