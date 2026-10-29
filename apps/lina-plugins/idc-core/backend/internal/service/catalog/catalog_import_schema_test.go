// This file verifies that portable schemas preserve purchase conditions and
// hierarchy without leaking provider keys or accepting tokens from other revisions.

package catalog

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestImportSchemaPreservesConditionsAndHierarchy(t *testing.T) {
	t.Parallel()
	var (
		label  = &capmodel.LocalizedText{EnUS: "Choice", ZhCN: "选择"}
		schema = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "region", Type: capmodel.ConfigurationFieldTreeSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{
				{Value: "parent", Label: label}, {Value: "child", ParentValue: "parent", Label: label},
			}},
			{Key: "name", Type: capmodel.ConfigurationFieldSingleLine, Label: label},
			{Key: "size", Type: capmodel.ConfigurationFieldInteger, Label: label},
		}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{
			When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionAnd, Children: []*capmodel.ConfigurationConditionDeclaration{
				{Operator: capmodel.ConfigurationConditionEqual, Field: "region", Value: "child"},
				{Operator: capmodel.ConfigurationConditionEqual, Field: "name", Value: "child"},
				{Operator: capmodel.ConfigurationConditionGreaterThan, Field: "size", Value: "2"},
			}},
			Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionFilterCandidates, Field: "region", Values: []string{"child"}}},
		}}}
	)
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	projected, err := projectImportSchema(codec, &publicStructuredConfigurationSnapshot{Revision: "revision-a", Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("export mutated the published schema")
	}
	field := projected.schema.Fields[0]
	if field.Key == "region" || field.StaticCandidates[0].Value == "parent" || field.StaticCandidates[1].ParentValue != field.StaticCandidates[0].Value {
		t.Fatal("opaque projection lost candidate hierarchy")
	}
	condition := projected.schema.Conditions[0]
	if condition.When.Children[0].Value != field.StaticCandidates[1].Value || condition.When.Children[0].Field != field.Key ||
		condition.When.Children[1].Value != "child" || condition.When.Children[2].Value != "2" ||
		condition.Actions[0].Field != field.Key || condition.Actions[0].Values[0] != field.StaticCandidates[1].Value {
		t.Fatal("condition projection changed purchase semantics")
	}
	for index, mapping := range projected.mappings[0].Options {
		decoded, decodeErr := codec.decodeStructuredOption(mapping.OptionToken, "revision-a", "region")
		if decodeErr != nil || decoded != schema.Fields[0].StaticCandidates[index].Value {
			t.Fatalf("invalid purchase mapping: %s, %v", decoded, decodeErr)
		}
		if _, decodeErr = codec.decodeStructuredOption(mapping.OptionToken, "revision-b", "region"); decodeErr == nil {
			t.Fatal("cross-revision token accepted")
		}
	}
	other, err := projectImportSchema(codec, &publicStructuredConfigurationSnapshot{Revision: "revision-b", Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(other.mappings, projected.mappings) || other.schema.Fields[0].Key == field.Key {
		t.Fatal("revision change reused portable identities")
	}
}

func TestImportSchemaPreservesDynamicResolution(t *testing.T) {
	t.Parallel()
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatal(err)
	}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
		Key: "location", Type: capmodel.ConfigurationFieldSingleSelect,
		Label:             &capmodel.LocalizedText{EnUS: "Location", ZhCN: "位置"},
		DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "private.provider.locations"},
	}}}
	projected, err := projectImportSchema(codec, &publicStructuredConfigurationSnapshot{Revision: "revision-a", Schema: schema})
	if err != nil {
		t.Fatal(err)
	}
	field := projected.schema.Fields[0]
	if field.DynamicCandidates == nil || field.DynamicCandidates.Method != importedCandidateMethod || len(field.StaticCandidates) != 0 || len(projected.mappings[0].Options) != 0 {
		t.Fatal("dynamic field was converted into incomplete static choices")
	}
	if schema.Fields[0].DynamicCandidates.Method != "private.provider.locations" {
		t.Fatal("source method was mutated")
	}
}

func TestImportSchemaProjectsHiddenDefaultsAcrossResale(t *testing.T) {
	codec, err := newPublicConfigurationRevisionCodec(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	label := &capmodel.LocalizedText{ZhCN: "配置", EnUS: "Configuration"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "local_plan", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "paid", Label: label}}},
		{Key: "quantity", Type: capmodel.ConfigurationFieldInteger, Label: label, Minimum: "0", Maximum: "10"},
		{Key: "flag", Type: capmodel.ConfigurationFieldBoolean, Label: label},
	}}
	policy := `{"local_plan":{"customerVisible":false,"defaultValue":"paid","productFieldKey":"private","productChoiceBindings":{"paid":"private-provider-option"},"descriptionEnUS":"local"},"quantity":{"customerVisible":false,"defaultValue":0},"flag":{"customerVisible":false,"defaultValue":false}}`
	first, err := projectImportSchema(codec, &publicStructuredConfigurationSnapshot{Revision: "first", Schema: schema, Target: publicStructuredConfigurationTarget{CustomerPresentation: policy}})
	if err != nil {
		t.Fatal(err)
	}
	if string(first.mappings[0].DefaultValue) != `"`+first.schema.Fields[0].StaticCandidates[0].Value+`"` || string(first.mappings[1].DefaultValue) != "0" || string(first.mappings[2].DefaultValue) != "false" {
		t.Fatalf("default projection: %+v", first.mappings)
	}
	portablePolicy := map[string]map[string]any{}
	for _, m := range first.mappings {
		portablePolicy[m.FieldKey] = map[string]any{"customerVisible": m.CustomerVisible, "defaultValue": m.DefaultValue}
	}
	raw, err := json.Marshal(portablePolicy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := projectImportSchema(codec, &publicStructuredConfigurationSnapshot{Revision: "second", Schema: first.schema, Target: publicStructuredConfigurationTarget{CustomerPresentation: string(raw)}})
	if err != nil {
		t.Fatal(err)
	}
	if string(second.mappings[0].DefaultValue) != `"`+second.schema.Fields[0].StaticCandidates[0].Value+`"` || second.schema.Fields[0].StaticCandidates[0].Value == first.schema.Fields[0].StaticCandidates[0].Value || *second.mappings[0].CustomerVisible {
		t.Fatal("second resale changed hidden default policy")
	}
	encoded, _ := json.Marshal(second.mappings)
	if bytes.Contains(encoded, []byte("descriptionEnUS")) || bytes.Contains(encoded, []byte("productFieldKey")) {
		t.Fatal("private presentation metadata leaked")
	}
	if schema.Fields[0].StaticCandidates[0].Value != "paid" {
		t.Fatal("source mutated")
	}
}
