package configurationcontract

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

func TestProductChoiceBindingsRequireExplicitCommercialSelect(t *testing.T) {
	field := &capmodel.ConfigurationFieldDeclaration{Key: "local_storage", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "standard"}, {Value: "archive"}}}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{field}}
	valid := Presentation{ProductFieldKey: "storage_profile", ProductChoiceBindings: map[string]string{"archive": "vendor-7"}}
	if err := ValidatePresentations(schema, map[string]Presentation{field.Key: valid}); err != nil {
		t.Fatal(err)
	}
	if ProductChoiceValue(valid.ProductChoiceBindings, "standard") != "standard" || ProductChoiceValue(valid.ProductChoiceBindings, "archive") != "vendor-7" || field.StaticCandidates[1].Value != "archive" {
		t.Fatal("mapping changed a commercial identity or legacy fallback")
	}
	for _, invalid := range []Presentation{
		{ProductChoiceBindings: valid.ProductChoiceBindings},
		{ProductFieldKey: "storage_profile", ProductChoiceBindings: map[string]string{"unknown": "vendor-7"}},
		{ProductFieldKey: "storage_profile", ProductChoiceBindings: map[string]string{"archive": ""}},
		{ProductFieldKey: "storage_profile", ProductChoiceBindings: map[string]string{"archive": "hidden\nvalue"}},
		{ProductFieldKey: "storage_profile", ProductChoiceBindings: map[string]string{"archive": " vendor-7"}},
	} {
		if err := ValidatePresentations(schema, map[string]Presentation{field.Key: invalid}); err == nil {
			t.Fatalf("invalid choice bindings accepted: %+v", invalid)
		}
	}
	field.Required = false
	if err := ValidatePresentations(schema, map[string]Presentation{field.Key: valid}); err == nil {
		t.Fatal("optional information field acquired a provider mapping")
	}
}

func TestLocalPurchaseControlsPreserveNumericContract(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Minimum: "2", Maximum: "16"}}}
	for _, control := range []string{ControlInput, ControlSlider, ControlSelect, ControlRadio} {
		t.Run(control, func(t *testing.T) {
			p := Presentation{Control: control, Step: "2"}
			if control == ControlSelect || control == ControlRadio {
				p.NumericChoices = []string{"2", "4", "8"}
			}
			presentations := map[string]Presentation{"cpu": p}
			if err := ValidatePresentations(schema, presentations); err != nil {
				t.Fatal(err)
			}
			for _, n := range []int64{2, 4, 3, 6} {
				issues := PresentationIssues(schema, presentations, []ConfigurationValue{{FieldKey: "cpu", IntegerValue: &n}})
				valid := n == 2 || n == 4 || (n == 6 && len(p.NumericChoices) == 0)
				if (len(issues) == 0) != valid {
					t.Fatalf("value %d accepted=%t, want %t", n, len(issues) == 0, valid)
				}
			}
		})
	}
	for _, p := range []Presentation{{Control: ControlSlider, Step: "0"}, {Control: ControlSlider, Step: "-2"}, {Control: ControlSlider, Step: "1.5"}, {Control: ControlSelect}, {Control: ControlSelect, NumericChoices: []string{"2", "2"}}, {Control: ControlRadio, Step: "2", NumericChoices: []string{"3"}}, {Control: ControlRadio, NumericChoices: []string{"17"}}, {Control: ControlRadio, NumericChoices: []string{"02"}}, {Control: ControlInput, NumericChoices: []string{"2"}}, {Control: "unknown"}} {
		if err := ValidatePresentations(schema, map[string]Presentation{"cpu": p}); err == nil {
			t.Fatalf("invalid control accepted: %+v", p)
		}
	}
	if err := ValidatePresentations(schema, map[string]Presentation{"unknown": {Control: ControlSlider}}); err == nil {
		t.Fatal("unknown field accepted")
	}
}

func TestLocalPurchaseControlsDecodeAndLegacy(t *testing.T) {
	for _, raw := range []string{"", `{}`} {
		p, err := DecodePresentations(raw)
		if err != nil || len(p) != 0 {
			t.Fatalf("legacy %q: %v", raw, err)
		}
	}
	for _, raw := range []string{`null`, `[]`, `{"cpu":null`, `{"cpu":"slider"}`} {
		if _, err := DecodePresentations(raw); err == nil {
			t.Fatalf("malformed metadata accepted: %s", raw)
		}
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "os", Type: capmodel.ConfigurationFieldSingleSelect}}}
	for _, control := range []string{ControlAuto, ControlSelect, ControlRadio} {
		if err := ValidatePresentations(schema, map[string]Presentation{"os": {Control: control}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := ValidatePresentations(schema, map[string]Presentation{"os": {Control: ControlSlider}}); err == nil {
		t.Fatal("slider accepted on provider option token")
	}
}
