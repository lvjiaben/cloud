// Partial imports cannot acquire full-validation status through a local save.
// Presence checks reuse existing rules without fabricating live directory data.
package configurationcontract

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestStoredCompletenessChecksRequiredAndConditionalInputsOffline(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "配置", EnUS: "Configuration"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "plan", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "plan.choices"}},
		{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Label: label, Required: true, Minimum: "1"},
		{Key: "addons", Type: capmodel.ConfigurationFieldMultiSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "validation", Label: label}}},
		{Key: "callback", Type: capmodel.ConfigurationFieldSingleLine, Label: label},
	}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Field: "addons", Operator: capmodel.ConfigurationConditionContains, Value: "validation"}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionRequire, Field: "callback"}}}}}
	for _, test := range []struct {
		name     string
		document map[string]any
		valid    bool
	}{
		{"missing-plan", map[string]any{"cpu": json.Number("2")}, false},
		{"missing-cpu", map[string]any{"plan": "stored-plan"}, false},
		{"missing-conditional-input", map[string]any{"plan": "stored-plan", "cpu": json.Number("2"), "addons": []any{"validation"}}, false},
		{"empty-conditional-input", map[string]any{"plan": "stored-plan", "cpu": json.Number("2"), "addons": []any{"validation"}, "callback": ""}, false},
		{"complete", map[string]any{"plan": "stored-plan", "cpu": json.Number("2"), "addons": []any{"validation"}, "callback": "https://example.test/callback"}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			before, err := json.Marshal(test.document)
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidateStoredCompleteness(schema, test.document, SurfaceProduct, []string{"plan.choices"}); (err == nil) != test.valid {
				t.Fatalf("stored completeness accepted=%t, want=%t: %v", err == nil, test.valid, err)
			}
			after, err := json.Marshal(test.document)
			if err != nil || string(before) != string(after) {
				t.Fatal("presence check changed saved configuration")
			}
		})
	}
}
