// These tests verify upgrade switches and target ranges affect real changed values.
package configurationcontract

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

func TestUpgradePolicyRejectsDisabledChangedFieldsAndOutOfRangeTargets(t *testing.T) {
	disabled := false
	policy := map[string]Presentation{"cpu": {UpgradeEnabled: &disabled}, "memory": {UpgradeMinimum: "4", UpgradeMaximum: "16"}, "disk": {UpgradeCandidateValues: []string{"fast", "premium"}}}
	for _, tc := range []struct {
		name, current, target string
		valid                 bool
	}{
		{"unchanged disabled field", `{"cpu":2,"memory":2}`, `{"cpu":2,"memory":4}`, true},
		{"disabled changed field", `{"cpu":2}`, `{"cpu":4}`, false},
		{"disabled removed field", `{"cpu":2}`, `{}`, false},
		{"below minimum", `{"memory":2}`, `{"memory":3}`, false},
		{"above maximum", `{"memory":8}`, `{"memory":17}`, false},
		{"allowed choice", `{"disk":"slow"}`, `{"disk":"fast"}`, true},
		{"forbidden choice", `{"disk":"fast"}`, `{"disk":"slow"}`, false},
		{"unchanged old value", `{"memory":2}`, `{"memory":2}`, true},
		{"legacy default", `{"other":1}`, `{"other":2}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUpgradeChanges(json.RawMessage(tc.current), json.RawMessage(tc.target), policy)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestUpgradeRangeMustStayWithinSellableRange(t *testing.T) {
	field := &capmodel.ConfigurationFieldDeclaration{Type: capmodel.ConfigurationFieldInteger, Minimum: "2", Maximum: "32"}
	for _, p := range []Presentation{{UpgradeMinimum: "1"}, {UpgradeMaximum: "33"}, {UpgradeMinimum: "8", UpgradeMaximum: "4"}, {UpgradeMinimum: "4.5"}} {
		if err := validateUpgradeMetadata(field, p); err == nil {
			t.Fatal("invalid upgrade bounds accepted")
		}
	}
	if err := validateUpgradeMetadata(field, Presentation{UpgradeMinimum: "4", UpgradeMaximum: "16"}); err != nil {
		t.Fatal(err)
	}
}
