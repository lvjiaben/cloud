// Manual commercial definitions must remain independent of supplier identity
// while retaining the same bounded type and choice checks as automated goods.
package productautomation

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestManualOfferSchemaPreservesGenericFieldsAndOrder(t *testing.T) {
	profile := &capmodel.ConfigurationFieldDeclaration{Key: "local_profile", Type: capmodel.ConfigurationFieldSingleSelect,
		Label:            &capmodel.LocalizedText{ZhCN: "验证方式", EnUS: "Validation method"},
		StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "dns", Label: &capmodel.LocalizedText{ZhCN: "自动验证 | 推荐", EnUS: "Automatic DNS"}}, {Value: "file", Label: &capmodel.LocalizedText{ZhCN: "文件验证", EnUS: "File validation"}}}}
	selections := []CustomerFieldSelection{
		{FieldKey: profile.Key, LocalField: profile, SortOrder: 2},
		{FieldKey: "local_domain", LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_domain", Type: capmodel.ConfigurationFieldSingleLine, Label: &capmodel.LocalizedText{ZhCN: "域名", EnUS: "Domain"}, Required: true, MaximumLength: 253}, SortOrder: 1},
	}
	before, _ := json.Marshal(selections)
	schema, err := manualCustomerSchema(selections)
	if err != nil || len(schema.Fields) != 2 || schema.Fields[0].Key != "local_domain" || schema.Fields[1].StaticCandidates[0].Label.ZhCN != "自动验证 | 推荐" {
		t.Fatalf("manual commercial fields changed: schema=%+v err=%v", schema, err)
	}
	schema.Fields[1].Label.ZhCN = "Changed result"
	after, _ := json.Marshal(selections)
	if string(before) != string(after) {
		t.Fatal("commercial compilation mutated merchant input")
	}
	if schema, err = manualCustomerSchema(nil); err != nil || schema == nil || len(schema.Fields) != 0 {
		t.Fatal("a manual product without questions must not require an interface")
	}
}

func TestManualOfferRejectsSupplierCapabilitiesAndInvalidChoices(t *testing.T) {
	field := func() *capmodel.ConfigurationFieldDeclaration {
		return &capmodel.ConfigurationFieldDeclaration{Key: "local_profile", Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{ZhCN: "档位", EnUS: "Profile"}, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "a", Label: &capmodel.LocalizedText{ZhCN: "A", EnUS: "A"}}}}
	}
	for _, mode := range []string{"parameter", "dynamic", "secret", "duplicate", "identity", "label"} {
		t.Run(mode, func(t *testing.T) {
			item := CustomerFieldSelection{FieldKey: "local_profile", LocalField: field()}
			items := []CustomerFieldSelection{item}
			switch mode {
			case "parameter":
				items[0].ProductFieldKey = "unverified_parameter"
			case "dynamic":
				item.LocalField.DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}
			case "secret":
				item.LocalField.Type = capmodel.ConfigurationFieldSecret
			case "duplicate":
				items = append(items, item)
			case "identity":
				items[0].FieldKey = "provider_field"
			case "label":
				item.LocalField.Label.ZhCN = strings.Repeat("长", 129)
			}
			if _, err := manualCustomerSchema(items); err == nil {
				t.Fatal("manual configuration accepted unsupported supplier capability or malformed input")
			}
		})
	}
	if _, err := manualCustomerSchema(make([]CustomerFieldSelection, 129)); err == nil {
		t.Fatal("unbounded manual fields accepted")
	}
}
