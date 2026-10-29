// Mapping tests use certificate options and unrelated supplier parameter names
// to prove that relationship freezing has no native-server type dependency.
package productautomation

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func commercialRelationshipFixture() (*capmodel.ConfigurationSchemaDeclaration, []CustomerFieldSelection, []resourceinterfaceprojectioncap.ChoiceRelationship) {
	field := func(key string, choices ...string) *capmodel.ConfigurationFieldDeclaration {
		result := &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: &capmodel.LocalizedText{ZhCN: key, EnUS: key}}
		for _, value := range choices {
			result.StaticCandidates = append(result.StaticCandidates, &capmodel.ConfigurationCandidate{Value: value, Label: &capmodel.LocalizedText{ZhCN: value, EnUS: value}})
		}
		return result
	}
	parent, child := field("local_certificate", "single", "wildcard", "retired"), field("local_validation", "file", "dns")
	return &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{parent, child}}, []CustomerFieldSelection{
		{FieldKey: parent.Key, ProductFieldKey: "sku_family", LocalField: parent},
		{FieldKey: child.Key, ProductFieldKey: "dcv_method", LocalField: child},
	}, []resourceinterfaceprojectioncap.ChoiceRelationship{{ParentFieldKey: "sku_family", ChildFieldKey: "dcv_method", Links: []resourceinterfaceprojectioncap.ChoiceLink{{ParentValue: "single", ChildValue: "file"}, {ParentValue: "single", ChildValue: "dns"}, {ParentValue: "wildcard", ChildValue: "dns"}}}}
}

func TestCommercialRelationshipsUseExplicitMappingsAndMerchantOrder(t *testing.T) {
	schema, selections, relationships := commercialRelationshipFixture()
	result, err := commercialChoiceSelections(&capmodel.BusinessExtensionProviderEvidence{}, schema, selections, []byte(`{}`), relationships)
	if err != nil || len(result[1].ChoiceRules) != 3 {
		t.Fatal("commercial relationships were not compiled", err)
	}
	if len(selections[1].ChoiceRules) != 0 || schema.Fields[1].StaticCandidates[0].Value != "file" {
		t.Fatal("relationship mapping changed merchant input or choice order")
	}
	presentation, err := compileCustomerPresentations(schema, result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := configurationcontract.DecodePresentations(presentation)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := configurationcontract.WithPresentationChoiceRules(schema, decoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		parent, child string
		valid         bool
	}{{"single", "file", true}, {"wildcard", "dns", true}, {"wildcard", "file", false}, {"retired", "dns", false}} {
		values := []configurationcontract.ConfigurationValue{{FieldKey: "local_certificate", OptionTokens: []string{test.parent}}, {FieldKey: "local_validation", OptionTokens: []string{test.child}}}
		if _, compileErr := configurationcontract.CompileResolved(effective, values, configurationcontract.SurfaceCustomer, nil, nil); (compileErr == nil) != test.valid {
			t.Fatalf("mapped pair %s/%s accepted=%t", test.parent, test.child, compileErr == nil)
		}
	}
	selections[1].ProductFieldKey = ""
	result, err = commercialChoiceSelections(&capmodel.BusinessExtensionProviderEvidence{}, schema, selections, []byte(`{}`), relationships)
	if err != nil || len(result[1].ChoiceRules) != 0 {
		t.Fatal("an ordinary customer information field acquired an implicit supplier binding", err)
	}
}

func TestCommercialChoiceBindingsTranslateRelationsWithoutChangingRetailValues(t *testing.T) {
	schema, selections, relationships := commercialRelationshipFixture()
	selections[0].ProductChoiceBindings = map[string]string{"single": "vendor-dv", "wildcard": "vendor-wildcard"}
	selections[1].ProductChoiceBindings = map[string]string{"file": "http-01", "dns": "dns-01"}
	relationships[0].Links = []resourceinterfaceprojectioncap.ChoiceLink{{ParentValue: "vendor-dv", ChildValue: "http-01"}, {ParentValue: "vendor-dv", ChildValue: "dns-01"}, {ParentValue: "vendor-wildcard", ChildValue: "dns-01"}}
	result, err := commercialChoiceSelections(&capmodel.BusinessExtensionProviderEvidence{}, schema, selections, []byte(`{}`), relationships)
	if err != nil {
		t.Fatal(err)
	}
	if got := result[1].ChoiceRules[1]; got.When.Value != "wildcard" || len(got.Actions[0].Values) != 1 || got.Actions[0].Values[0] != "dns" {
		t.Fatalf("provider relation was not converted to retail identities: %+v", got)
	}
	if len(result[1].ChoiceRules[2].Actions[0].Values) != 0 || schema.Fields[0].StaticCandidates[0].Value != "single" {
		t.Fatal("empty eligibility or commercial identities changed")
	}
	current := map[string]configurationcontract.Presentation{
		"local_certificate": {ProductFieldKey: "sku_family", ProductChoiceBindings: selections[0].ProductChoiceBindings},
		"local_validation":  {ProductFieldKey: "dcv_method", ProductChoiceBindings: selections[1].ProductChoiceBindings},
	}
	condition := result[1].ChoiceRules[0].When
	if observedChoiceRule(condition, "local_validation", current, nil) || !observedChoiceRule(condition, "local_validation", current, [][]resourceinterfaceprojectioncap.ChoiceRelationship{relationships}) {
		t.Fatal("changed mapping either erased restrictions without proof or rejected matching proof")
	}
	old := map[string]configurationcontract.Presentation{"local_certificate": {ProductFieldKey: "sku_family"}}
	if sameChoiceRuleBindings(condition, map[string]bool{"local_certificate": true}, current, old) {
		t.Fatal("old restriction reused after provider meaning changed")
	}
	old["local_validation"] = configurationcontract.Presentation{ProductFieldKey: "dcv_method"}
	if !choiceRuleAliasesChanged(condition, "local_validation", current, old) {
		t.Fatal("changed aliases did not require rebuilding a saved restriction")
	}
	current["local_validation"] = configurationcontract.Presentation{}
	if choiceRuleAliasesChanged(condition, "local_validation", current, old) {
		t.Fatal("an explicit information-only decision acquired an alias-only restriction")
	}
}

func TestCommercialRelationshipsValidateFixedParentWithoutDiscardingPricesOrChoices(t *testing.T) {
	schema, selections, relationships := commercialRelationshipFixture()
	schema.Fields, selections = schema.Fields[1:], selections[1:]
	for _, test := range []struct {
		merchant string
		valid    bool
	}{{`{"sku_family":"single"}`, true}, {`{"sku_family":"wildcard"}`, false}, {`{}`, false}} {
		result, err := commercialChoiceSelections(&capmodel.BusinessExtensionProviderEvidence{}, schema, selections, []byte(test.merchant), relationships)
		if (err == nil) != test.valid {
			t.Fatalf("fixed parent accepted=%t, want=%t", err == nil, test.valid)
		}
		if err == nil && len(result[0].LocalField.StaticCandidates) != 2 {
			t.Fatal("validation silently removed a merchant's sellable choice")
		}
	}
}

func TestCandidateRelationshipCollectionCopiesAndRejectsContradictions(t *testing.T) {
	_, _, input := commercialRelationshipFixture()
	var collected []resourceinterfaceprojectioncap.ChoiceRelationship
	if err := appendCandidateRelationships(&collected, input); err != nil {
		t.Fatal(err)
	}
	input[0].Links[0], input[0].Links[1] = input[0].Links[1], input[0].Links[0]
	if err := appendCandidateRelationships(&collected, input); err != nil || len(collected) != 1 {
		t.Fatal("equivalent directory order caused a conflict or duplicate", err)
	}
	input[0].Links[0].ParentValue = "another"
	if collected[0].Links[1].ParentValue != "single" {
		t.Fatal("captured directory shares mutable caller data")
	}
	if err := appendCandidateRelationships(&collected, input); err == nil {
		t.Fatal("contradictory directory facts were combined")
	}
}
