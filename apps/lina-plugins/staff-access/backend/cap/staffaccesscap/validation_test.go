package staffaccesscap

import "testing"

func TestValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		userID     int64
		capability string
		scopeType  string
		scopeKey   string
		wantError  bool
	}{
		{name: "global", userID: 7, capability: "support.agent", scopeType: ScopeGlobal},
		{name: "department", userID: 8, capability: "support.department-leader", scopeType: ScopeDepartment, scopeKey: "DPT_12"},
		{name: "missing target", capability: "support.agent", scopeType: ScopeGlobal, wantError: true},
		{name: "invalid capability", userID: 7, capability: "Support Agent", scopeType: ScopeGlobal, wantError: true},
		{name: "global with key", userID: 7, capability: "support.agent", scopeType: ScopeGlobal, scopeKey: "DPT_12", wantError: true},
		{name: "department without key", userID: 7, capability: "support.agent", scopeType: ScopeDepartment, wantError: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := Validate(test.userID, test.capability, test.scopeType, test.scopeKey)
			if (err != nil) != test.wantError {
				t.Fatalf("Validate() error = %v, wantError %v", err, test.wantError)
			}
		})
	}
}
