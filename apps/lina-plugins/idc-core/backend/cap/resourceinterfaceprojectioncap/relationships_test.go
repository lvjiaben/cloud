// Relationship tests use non-server fields to keep the contract independent
// of any cloud category, and reject malformed or contradictory graph facts.
package resourceinterfaceprojectioncap

import (
	"encoding/json"
	"testing"
)

func relationshipFixture() CandidateResult {
	return CandidateResult{Fields: []CandidateField{
		{FieldKey: "certificate", Items: []Candidate{{Value: "single"}, {Value: "wildcard"}}},
		{FieldKey: "validation", Items: []Candidate{{Value: "dns"}, {Value: "file"}}},
	}, Relationships: []ChoiceRelationship{{ParentFieldKey: "certificate", ChildFieldKey: "validation", Links: []ChoiceLink{
		{ParentValue: "single", ChildValue: "dns"}, {ParentValue: "single", ChildValue: "file"}, {ParentValue: "wildcard", ChildValue: "dns"},
	}}}}
}

func TestChoiceRelationshipHasNoProductTypeAssumption(t *testing.T) {
	value := relationshipFixture()
	before, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = value.ValidateRelationships(); err != nil {
		t.Fatal(err)
	}
	after, err := json.Marshal(value)
	if err != nil || string(before) != string(after) {
		t.Fatal("validation mutated the provider projection")
	}
	value.Fields = value.Fields[1:]
	if err = value.ValidateRelationships(); err != nil {
		t.Fatal("partial child query lost its explicit relationship", err)
	}
	value.Relationships = nil
	if err = value.ValidateRelationships(); err != nil {
		t.Fatal("legacy directory incorrectly required relationships", err)
	}
}

func TestChoiceRelationshipRejectsUnprovableGraph(t *testing.T) {
	for _, mode := range []string{"unknown-child", "unknown-parent-value", "unknown-child-value", "duplicate-link", "duplicate-relation", "self", "cycle", "control", "bound"} {
		t.Run(mode, func(t *testing.T) {
			value := relationshipFixture()
			relation := &value.Relationships[0]
			switch mode {
			case "unknown-child":
				relation.ChildFieldKey = "not_declared"
			case "unknown-parent-value":
				relation.Links[0].ParentValue = "not_declared"
			case "unknown-child-value":
				relation.Links[0].ChildValue = "not_declared"
			case "duplicate-link":
				relation.Links = append(relation.Links, relation.Links[0])
			case "duplicate-relation":
				value.Relationships = append(value.Relationships, *relation)
			case "self":
				relation.ParentFieldKey = relation.ChildFieldKey
			case "cycle":
				value.Relationships = append(value.Relationships, ChoiceRelationship{ParentFieldKey: "validation", ChildFieldKey: "certificate", Links: []ChoiceLink{{ParentValue: "dns", ChildValue: "single"}}})
			case "control":
				relation.Links[0].ParentValue = "s\x00ingle"
			case "bound":
				value.Relationships = make([]ChoiceRelationship, 17)
			}
			if err := value.ValidateRelationships(); err == nil {
				t.Fatal("invalid relationship accepted")
			}
		})
	}
}
