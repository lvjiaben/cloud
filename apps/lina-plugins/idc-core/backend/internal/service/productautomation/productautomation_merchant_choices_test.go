// These tests prove merchant image subsets retain labels and order in frozen
// sales while preventing static IDs and requiredness from being forged.
package productautomation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"testing"
)

func TestMerchantImageChoicesFreezeOrderNamesAndConstraints(t *testing.T) {
	exact := customerSelectionEvidence(t)
	original := exact.CustomerConfiguration.Fields[0]
	original.StaticCandidates = append(original.StaticCandidates, &capmodel.ConfigurationCandidate{Value: "windows", Label: &capmodel.LocalizedText{ZhCN: "Windows", EnUS: "Windows"}})
	selections := []CustomerFieldSelection{{FieldKey: "operating_system", CandidateValues: []string{"windows", "linux"}, CandidateLabels: map[string]*capmodel.LocalizedText{"linux": {ZhCN: "自选 Linux", EnUS: "Custom Linux"}}}}
	frozen, err := selectCustomerSchema(exact, selections)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Fields[0].StaticCandidates[0].Value != "windows" || frozen.Fields[0].StaticCandidates[1].Label.ZhCN != "自选 Linux" || !frozen.Fields[0].Required {
		t.Fatal("sale choices lost order, label or requirement")
	}
	if _, err = frozenCustomerSelections(exact, frozen); err != nil {
		t.Fatal(err)
	}
	if original.StaticCandidates[0].Label.ZhCN == "自选 Linux" {
		t.Fatal("provider mutated")
	}
	frozen.Fields[0].Required = false
	if _, err = frozenCustomerSelections(exact, frozen); err == nil {
		t.Fatal("forged requiredness accepted")
	}
	selections[0].CandidateValues = []string{"unknown"}
	if _, err = selectCustomerSchema(exact, selections); err == nil {
		t.Fatal("undeclared static image accepted")
	}
}

func TestMerchantDynamicImageChoicesBecomeFrozenSubset(t *testing.T) {
	exact := customerSelectionEvidence(t)
	original := exact.CustomerConfiguration.Fields[0]
	original.StaticCandidates = nil
	original.DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}
	exact.AuthorizedMethods = append(exact.AuthorizedMethods, "configuration.candidates")
	frozen, err := selectCustomerSchema(exact, []CustomerFieldSelection{{FieldKey: "operating_system", CandidateValues: []string{"97"}, CandidateLabels: map[string]*capmodel.LocalizedText{"97": {ZhCN: "Ubuntu", EnUS: "Ubuntu"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Fields[0].DynamicCandidates != nil || len(frozen.Fields[0].StaticCandidates) != 1 {
		t.Fatal("image whitelist not frozen")
	}
	if _, err = frozenCustomerSelections(exact, frozen); err != nil {
		t.Fatal(err)
	}
	if original.DynamicCandidates == nil {
		t.Fatal("original source removed")
	}
	legacy, err := selectCustomerSchema(exact, nil)
	if err != nil || legacy.Fields[0].DynamicCandidates == nil {
		t.Fatal("legacy dynamic images changed")
	}
}

func TestCompositeProfilePriceIdentitiesKeepWholeValues(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "性能", EnUS: "Performance"}
	choices := []*capmodel.ConfigurationCandidate{{Value: "0,0,0,0", Label: label}, {Value: "500,500,2000,2000", Label: label}}
	candidates, err := configurationPriceCandidates("ver_profiles", &VersionConfiguration{Version: 1, CustomerDeclaration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "local_disk", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: choices}}}})
	if err != nil || len(candidates) != 2 {
		t.Fatalf("profiles were split: %+v %v", candidates, err)
	}
	if candidates[0].candidateValue != "0,0,0,0" || candidates[1].candidateValue != "500,500,2000,2000" || candidates[0].CandidateToken == candidates[1].CandidateToken {
		t.Fatal("profile pricing identities overlap")
	}
}

func TestMerchantChoicesRejectUnknownDirectoryIDsInOneBatch(t *testing.T) {
	exact := dynamicCandidateEvidence("1.0.0")
	ext := &automationTestExtensions{}
	ext.invoke = func(input pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
		var req configurationcontract.DynamicCandidateRequest
		if err := json.Unmarshal(input.Payload, &req); err != nil {
			return nil, err
		}
		if len(req.Requests) != 2 {
			t.Fatal("directory fields were fetched separately")
		}
		items := make([]configurationcontract.DynamicCandidateItem, 0, len(req.Requests))
		for _, q := range req.Requests {
			items = append(items, configurationcontract.DynamicCandidateItem{FieldKey: q.FieldKey, Candidates: []*capmodel.ConfigurationCandidate{{Value: q.FieldKey + "-a", Label: &capmodel.LocalizedText{ZhCN: "选项", EnUS: "Option"}}}})
		}
		raw, err := json.Marshal(configurationcontract.DynamicCandidateResponse{SchemaVersion: "v1", Items: items})
		return &capmodel.BusinessExtensionMethodResult{Version: "v1", Payload: raw}, err
	}
	service := &serviceImpl{extensions: ext, dynamicCandidates: newDynamicCandidateCache(), runtimeContexts: newSupplierRuntimeContextStore(rand.Reader)}
	input := SaveVersionConfigurationInput{CustomerFieldSelections: []CustomerFieldSelection{{FieldKey: "region", CandidateValues: []string{"region-a"}}}}
	scope := dynamicCandidateScope{interfaceInstanceNo: "pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", configurationRevision: 1}
	if err := service.validateCustomerChoiceCandidates(context.Background(), exact, input, scope); err != nil {
		t.Fatal(err)
	}
	input.CustomerFieldSelections[0].CandidateValues = []string{"forged"}
	if err := service.validateCustomerChoiceCandidates(context.Background(), exact, input, scope); err == nil {
		t.Fatal("unknown dynamic choice accepted")
	}
	if ext.invokeCalls != 1 {
		t.Fatal("same local directory was not reused")
	}
}
