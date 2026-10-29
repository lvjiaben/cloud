// Supplier replacement keeps commercial identities and rules independently,
// without inventing directory choices or carrying over executable mappings.
package productautomation

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func TestDetachedOfferPreservesCommercialChoicesRulesAndIdentity(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "证书验证方式", EnUS: "Certificate validation"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "validation", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "options.validation"}},
		{Key: "local_domain", Type: capmodel.ConfigurationFieldSingleLine, Label: label, MaximumLength: 128},
	}, Conditions: []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Operator: capmodel.ConfigurationConditionEqual, Field: "validation", Value: "dns"}, Actions: []*capmodel.ConfigurationActionDeclaration{{Action: capmodel.ConfigurationActionRequire, Field: "local_domain"}}}}}
	presentation := map[string]configurationcontract.Presentation{"validation": {SortOrder: 4, Unit: "份", Control: configurationcontract.ControlSelect}, "local_domain": {SortOrder: 7}}
	row := detachedOfferTestRow(t, schema, presentation)
	// JSONB returns different spacing/key order from the original compiler.
	// Large exact numbers must survive canonicalization without float64.
	row.MerchantValueSnapshot = `{ "z": true, "a": 9007199254740993 }`
	row.MerchantValueDigest = configurationDigest([]byte(`{"a":9007199254740993,"z":true}`))
	before, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	choices := configurationcontract.ResolvedCandidates{"validation": {{Value: "dns", Label: label}, {Value: "file", Label: label}}}
	detached, err := detachedCommercialDefinition(row, choices)
	if err != nil {
		t.Fatal(err)
	}
	key := detached.identities["validation"]
	if !strings.HasPrefix(key, "local_retained_") || detached.identities["local_domain"] != "local_domain" || len(detached.schema.Fields) != 2 || detached.schema.Fields[0].DynamicCandidates != nil || len(detached.schema.Fields[0].StaticCandidates) != 2 {
		t.Fatal("commercial fields or choices were not made independent")
	}
	if detached.schema.Conditions[0].When.Field != key || detached.schema.Conditions[0].Actions[0].Field != "local_domain" || detached.presentations[key].SortOrder != 4 || detached.presentations[key].Unit != "份" || !detached.presentations[key].AutomationMappingPending || !detached.presentations[key].BasePriceIncluded || detached.presentations["local_domain"].AutomationMappingPending {
		t.Fatal("rules, presentation or explicit mapping requirement were lost")
	}
	choices["validation"][0].Value = "changed-by-caller"
	if detached.schema.Fields[0].StaticCandidates[0].Value != "dns" {
		t.Fatal("detached definition aliases directory input")
	}
	after, err := json.Marshal(row)
	if err != nil || string(before) != string(after) {
		t.Fatal("detachment rewrote source evidence", err)
	}
	manualRow := detachedOfferTestRow(t, detached.schema, detached.presentations)
	again, err := detachedCommercialDefinition(manualRow, nil)
	if err != nil || again.identities[key] != key || !again.presentations[key].AutomationMappingPending {
		t.Fatal("repeat conversion changed the commercial identity", err)
	}
}

func TestDetachedOfferClearsSupplierChoiceBindingsAndKeepsRetailChoices(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "存储等级", EnUS: "Storage profile"}
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "local_storage", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "archive", Label: label}}}}}
	row := detachedOfferTestRow(t, schema, map[string]configurationcontract.Presentation{"local_storage": {ProductFieldKey: "supplier_profile", ProductChoiceBindings: map[string]string{"archive": "vendor-old"}}})
	result, err := detachedCommercialDefinition(row, nil)
	if err != nil {
		t.Fatal(err)
	}
	presentation := result.presentations["local_storage"]
	if presentation.ProductFieldKey != "" || len(presentation.ProductChoiceBindings) != 0 || !presentation.AutomationMappingPending || result.schema.Fields[0].StaticCandidates[0].Value != "archive" {
		t.Fatal("supplier meaning survived detach or commercial choice was removed")
	}
}

func TestDetachedOfferRejectsMissingChoicesProtectedFieldsAndCorruptEvidence(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "交付配置", EnUS: "Delivery configuration"}
	for _, kind := range []string{capmodel.ConfigurationFieldSecret, capmodel.ConfigurationFieldFile} {
		row := detachedOfferTestRow(t, &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "protected", Type: kind, Label: label}}}, nil)
		if _, err := detachedCommercialDefinition(row, nil); !bizerr.Is(err, codeOfferDetachUnsupported) {
			t.Fatal("protected policy was silently discarded", kind, err)
		}
	}
	row := detachedOfferTestRow(t, &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "choice", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "options.choice"}}}}, nil)
	if _, err := detachedCommercialDefinition(row, nil); !bizerr.Is(err, codeOfferOptionsRequired) {
		t.Fatal("missing supplier choices were guessed", err)
	}
	row.DeclarationDigest = strings.Repeat("0", 64)
	if _, err := detachedCommercialDefinition(row, nil); !bizerr.Is(err, catalog.CodeManagementConflict) {
		t.Fatal("corrupt source evidence converted", err)
	}
}

func detachedOfferTestRow(t *testing.T, schema *capmodel.ConfigurationSchemaDeclaration, presentations map[string]configurationcontract.Presentation) *entity.CatalogProductVersionConfiguration {
	t.Helper()
	if presentations == nil {
		presentations = map[string]configurationcontract.Presentation{}
	}
	productJSON, err := configurationDeclarationJSON(emptyOfferSchema())
	if err != nil {
		t.Fatal(err)
	}
	customerJSON, err := configurationDeclarationJSON(schema)
	if err != nil {
		t.Fatal(err)
	}
	presentationJSON, err := json.Marshal(presentations)
	if err != nil {
		t.Fatal(err)
	}
	return &entity.CatalogProductVersionConfiguration{Version: 3, InterfaceInstanceNo: "pfi_" + strings.Repeat("a", 32), CustomerDeclarationSnapshot: string(customerJSON), ProductDeclarationSnapshot: string(productJSON), CustomerPresentationSnapshot: string(presentationJSON), MerchantValueSnapshot: "{}", DeclarationDigest: configurationDigest(append(append([]byte(nil), productJSON...), customerJSON...)), MerchantValueDigest: configurationDigest([]byte("{}"))}
}
