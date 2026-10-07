package interfacecandidate

import (
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestProjectCandidateResultRequiresBoundedBilingualServerFields(t *testing.T) {
	result, err := projectCandidateResult([]byte(`{
  "Fields":[{"FieldKey":"region","Items":[
    {"Value":"cn-hz","LabelZhCN":"杭州","LabelEnUS":"Hangzhou"}
  ]}]
}`))
	if err != nil || result == nil || len(result.Fields) != 1 || result.Fields[0].Items[0].Value != "cn-hz" {
		t.Fatalf("project safe candidate result=%+v err=%v", result, err)
	}

	invalid := []string{
		`{"Fields":[{"FieldKey":"password","Items":[{"Value":"secret","LabelZhCN":"秘密","LabelEnUS":"Secret"}]}]}`,
		`{"Fields":[{"FieldKey":"region","Items":[{"Value":"cn-hz","LabelZhCN":"杭州","LabelEnUS":""}]}]}`,
		`{"Fields":[{"FieldKey":"region","Items":[{"Value":"cn-hz","LabelZhCN":"杭州","LabelEnUS":"Hangzhou"},{"Value":"cn-hz","LabelZhCN":"杭州二","LabelEnUS":"Hangzhou 2"}]}]}`,
	}
	for _, payload := range invalid {
		if _, err = projectCandidateResult([]byte(payload)); err == nil {
			t.Fatalf("unsafe candidate payload was accepted: %s", payload)
		}
	}
}

func TestMatchesEvidenceRejectsWrongPluginContractAndDeclaration(t *testing.T) {
	value := binding{
		PluginID: "server-connector", PluginReleaseVersion: "v1.0.0",
		ContractVersion: "v1", AuthorizedMethods: []string{resourcecap.DynamicProviderMethodConfigurationCandidates},
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
	}
	evidence := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: value.PluginID, ReleaseVersion: value.PluginReleaseVersion,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: value.ContractVersion,
		AuthorizedMethods: append([]string(nil), value.AuthorizedMethods...),
		ReleaseDigest:     value.ReleaseDigest, DeclarationDigest: value.DeclarationDigest,
	}
	if !matchesEvidence(value, evidence, resourcecap.DynamicProviderMethodConfigurationCandidates) {
		t.Fatal("exact resource Provider evidence was rejected")
	}
	mutations := []func(*capmodel.BusinessExtensionProviderEvidence){
		func(item *capmodel.BusinessExtensionProviderEvidence) { item.PluginID = "other-connector" },
		func(item *capmodel.BusinessExtensionProviderEvidence) { item.ContractVersion = "v2" },
		func(item *capmodel.BusinessExtensionProviderEvidence) {
			item.DeclarationDigest = strings.Repeat("c", 64)
		},
	}
	for _, mutate := range mutations {
		copy := *evidence
		mutate(&copy)
		if matchesEvidence(value, &copy, resourcecap.DynamicProviderMethodConfigurationCandidates) {
			t.Fatalf("mismatched Provider evidence was accepted: %+v", copy)
		}
	}
}

func TestLazySourceRejectsWrongPFIAndMethodBeforeRuntimeAccess(t *testing.T) {
	source := NewLazy()
	requests := []resourceinterfaceprojectioncap.CandidateRequest{
		{InterfaceInstanceNo: "CHN_not_a_pfi", Method: "configuration.candidates", Values: []byte(`{}`)},
		{InterfaceInstanceNo: "pfi_11111111111111111111111111111111", Method: "operation.execute", Values: []byte(`{}`)},
	}
	for _, request := range requests {
		if _, err := source.ResolveCandidates(t.Context(), request); err == nil {
			t.Fatalf("invalid candidate request was accepted: %+v", request)
		}
	}
}
