// Commercial choice rules share the normal evaluation/compile boundary. These
// cases exercise a non-server product and the empty-candidate rejection case.
package configurationcontract

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func commercialChoiceFixture() (*capmodel.ConfigurationSchemaDeclaration, map[string]Presentation) {
	field := func(key string, values ...string) *capmodel.ConfigurationFieldDeclaration {
		result := &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: key, EnUS: key}}
		for _, value := range values {
			result.StaticCandidates = append(result.StaticCandidates, &capmodel.ConfigurationCandidate{Value: value, Label: &capmodel.LocalizedText{ZhCN: value, EnUS: value}})
		}
		return result
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{field("certificate", "single", "wildcard", "unavailable"), field("validation", "dns", "file")}}
	rule := func(parent string, choices ...string) *capmodel.ConfigurationRuleDeclaration {
		return &capmodel.ConfigurationRuleDeclaration{When: &capmodel.ConfigurationConditionDeclaration{Field: "certificate", Operator: capmodel.ConfigurationConditionEqual, Value: parent}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionFilterCandidates, Field: "validation", Values: choices}}}
	}
	return schema, map[string]Presentation{"validation": {ChoiceRules: []*capmodel.ConfigurationRuleDeclaration{rule("wildcard", "dns"), rule("unavailable")}}}
}

func TestCommercialChoiceRulesProtectEvaluationAndCompile(t *testing.T) {
	schema, presentations := commercialChoiceFixture()
	before, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := WithPresentationChoiceRules(schema, presentations)
	if err != nil || ValidatePresentations(schema, presentations) != nil {
		t.Fatal("commercial restrictions rejected", err)
	}
	for _, test := range []struct {
		parent, child string
		valid         bool
		count         int
	}{
		{"single", "file", true, 2}, {"wildcard", "dns", true, 1}, {"wildcard", "file", false, 1}, {"unavailable", "dns", false, 0},
	} {
		values := []ConfigurationValue{{FieldKey: "certificate", OptionTokens: []string{test.parent}}, {FieldKey: "validation", OptionTokens: []string{test.child}}}
		result, evaluateErr := EvaluateResolved(effective, values, SurfaceCustomer, nil, nil)
		if evaluateErr != nil || result.Valid != test.valid || len(result.States["validation"].CandidateValues) != test.count {
			t.Fatalf("%s/%s: result=%+v err=%v", test.parent, test.child, result, evaluateErr)
		}
		if _, compileErr := CompileResolved(effective, values, SurfaceCustomer, nil, nil); (compileErr == nil) != test.valid {
			t.Fatalf("%s/%s bypassed compile: %v", test.parent, test.child, compileErr)
		}
	}
	after, err := json.Marshal(schema)
	if err != nil || string(before) != string(after) {
		t.Fatal("evaluation changed the provider's frozen declaration")
	}
}

func TestCommercialChoiceRulesCannotRelaxProviderControls(t *testing.T) {
	for _, mode := range []string{"hide", "other-field", "unknown-parent", "bound"} {
		t.Run(mode, func(t *testing.T) {
			schema, presentations := commercialChoiceFixture()
			p := presentations["validation"]
			switch mode {
			case "hide":
				p.ChoiceRules[0].Actions[0].Action = capmodel.ConfigurationActionHide
			case "other-field":
				p.ChoiceRules[0].Actions[0].Field = "certificate"
			case "unknown-parent":
				p.ChoiceRules[0].When.Field = "not_declared"
			case "bound":
				for len(p.ChoiceRules) < 129 {
					p.ChoiceRules = append(p.ChoiceRules, p.ChoiceRules[0])
				}
			}
			presentations["validation"] = p
			if _, err := WithPresentationChoiceRules(schema, presentations); err == nil {
				t.Fatal("commercial rule expanded provider authority")
			}
		})
	}
}
