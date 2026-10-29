// Hidden fields retain typed defaults without allowing customer overrides.
package configurationcontract

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

func TestHiddenRequiredDefaultIsAuthoritative(t *testing.T) {
	hidden := false
	field := &capmodel.ConfigurationFieldDeclaration{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "1", Maximum: "16"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{field}}
	policies := map[string]Presentation{"cpu": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`4`)}}
	if err := ValidatePresentations(schema, policies); err != nil {
		t.Fatal(err)
	}
	tampered := int64(16)
	values, err := ApplyHiddenDefaults(schema, policies, []ConfigurationValue{{FieldKey: "cpu", IntegerValue: &tampered}})
	if err != nil || len(values) != 1 || values[0].IntegerValue == nil || *values[0].IntegerValue != 4 {
		t.Fatal("hidden default overridden", values, err)
	}
	if err = ValidateHiddenValues(schema, policies, values); err != nil {
		t.Fatal(err)
	}
	if err = ValidateHiddenValues(schema, policies, []ConfigurationValue{{FieldKey: "cpu", IntegerValue: &tampered}}); err == nil {
		t.Fatal("frozen override accepted")
	}
	if err = ValidateHiddenValues(schema, policies, nil); err == nil {
		t.Fatal("missing frozen default accepted")
	}
	for _, raw := range []string{`17`, `"4"`, `null`, `[]`, `0`} {
		policies["cpu"] = Presentation{CustomerVisible: &hidden, DefaultValue: json.RawMessage(raw)}
		if ValidatePresentations(schema, policies) == nil {
			t.Fatalf("invalid default accepted: %s", raw)
		}
	}
}
func TestHiddenChoiceRequiresSellableDefault(t *testing.T) {
	hidden := false
	field := &capmodel.ConfigurationFieldDeclaration{Key: "local_tier", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "basic"}, {Value: "fast"}}}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{field}}
	for _, raw := range []string{`"fast"`, `"absent"`, `""`} {
		p := map[string]Presentation{field.Key: {CustomerVisible: &hidden, DefaultValue: json.RawMessage(raw)}}
		err := ValidatePresentations(schema, p)
		if (err == nil) != (raw == `"fast"`) {
			t.Fatalf("default %s: %v", raw, err)
		}
	}
}

func TestHiddenDefaultsKeepZeroFalseAndVerifyResolvedMembership(t *testing.T) {
	hidden := false
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "count", Label: &capmodel.LocalizedText{ZhCN: "数量", EnUS: "Count"}, Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "0", Maximum: "16"},
		{Key: "backup", Label: &capmodel.LocalizedText{ZhCN: "备份", EnUS: "Backup"}, Type: capmodel.ConfigurationFieldBoolean, Required: true},
		{Key: "os", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: "系统", EnUS: "OS"}, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.os"}},
	}}
	policies := map[string]Presentation{"count": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`0`)}, "backup": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`false`)}, "os": {CustomerVisible: &hidden, DefaultValue: json.RawMessage(`"linux"`)}}
	values, err := ApplyHiddenDefaults(schema, policies, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 3 || values[0].BooleanValue == nil || *values[0].BooleanValue || values[1].IntegerValue == nil || *values[1].IntegerValue != 0 {
		t.Fatalf("zero/false lost: %+v", values)
	}
	for _, token := range []string{"linux", "windows"} {
		resolved := ResolvedCandidates{"os": []*capmodel.ConfigurationCandidate{{Value: token, Label: &capmodel.LocalizedText{ZhCN: token, EnUS: token}}}}
		evaluation, evaluateErr := EvaluateResolved(schema, values, SurfaceCustomer, []string{"candidate.os"}, resolved)
		if evaluateErr != nil || evaluation.Valid != (token == "linux") {
			t.Fatalf("hidden candidate %s admission: %+v %v", token, evaluation, evaluateErr)
		}
	}
}
