// This file prevents unchanged JSONB settings from invalidating product bindings.
package providerconfiguration

import (
	"encoding/json"
	"testing"
)

func TestSameNonSecretSettingsIgnoresFormattingAndOrder(t *testing.T) {
	for _, tc := range []struct {
		name, left, right string
		equal             bool
	}{
		{"jsonb formatting", `{"region": "2", "protocol": "v1"}`, `{"protocol":"v1","region":"2"}`, true},
		{"unchanged empty", "{}", "{ }", true},
		{"changed protocol", `{"protocol":"v1"}`, `{"protocol":"v2"}`, false},
		{"precise integers", `{"limit":9007199254740992}`, `{"limit":9007199254740993}`, false},
		{"invalid", "{", "{", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameNonSecretSettings(json.RawMessage(tc.left), json.RawMessage(tc.right)); got != tc.equal {
				t.Fatalf("equality=%v want=%v", got, tc.equal)
			}
		})
	}
}
