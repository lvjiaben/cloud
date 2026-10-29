// This file verifies that resource-owned dynamic candidates use the owner
// proxy contract and return only safe bilingual values.

package productautomation

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

type resourceCandidateSourceStub struct {
	request    resourceinterfaceprojectioncap.CandidateRequest
	catalogIDs []string
	catalog    map[string]bool
	result     *resourceinterfaceprojectioncap.CandidateResult
}

func (stub *resourceCandidateSourceStub) ResolveCandidates(_ context.Context, request resourceinterfaceprojectioncap.CandidateRequest) (*resourceinterfaceprojectioncap.CandidateResult, error) {
	stub.request = request
	if stub.result != nil {
		return stub.result, nil
	}
	return &resourceinterfaceprojectioncap.CandidateResult{Fields: []resourceinterfaceprojectioncap.CandidateField{{
		FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "cn-east", LabelZhCN: "华东", LabelEnUS: "East China"}},
	}}}, nil
}

func TestResolveResourceCandidatesRejectsDuplicateOwnerData(t *testing.T) {
	t.Parallel()
	for _, fields := range [][]resourceinterfaceprojectioncap.CandidateField{
		{
			{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "cn", LabelZhCN: "中国", LabelEnUS: "China"}}},
			{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "us", LabelZhCN: "美国", LabelEnUS: "USA"}}},
		},
		{{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{
			{Value: "cn", LabelZhCN: "中国", LabelEnUS: "China"},
			{Value: "cn", LabelZhCN: "中国二", LabelEnUS: "China 2"},
		}}},
	} {
		source := &resourceCandidateSourceStub{result: &resourceinterfaceprojectioncap.CandidateResult{Fields: fields}}
		service := &serviceImpl{resourceCandidates: source}
		_, err := service.resolveResourceCandidates(context.Background(), "pfi_11111111111111111111111111111111", "", "candidate.region", []byte(`{"version":"v1"}`), []configurationcontract.DynamicCandidateQuery{{FieldKey: "region"}}, nil, nil)
		if err == nil {
			t.Fatal("resolveResourceCandidates() accepted duplicate owner data")
		}
	}
}

func TestResolveResourceCandidatesUsesPFIOwnerProxy(t *testing.T) {
	t.Parallel()
	source := &resourceCandidateSourceStub{}
	service := &serviceImpl{resourceCandidates: source}
	queries := []configurationcontract.DynamicCandidateQuery{{FieldKey: "region"}}
	const revision = "rci_11111111111111111111111111111111"
	resolved, err := service.resolveResourceCandidates(context.Background(), "pfi_11111111111111111111111111111111", revision, "candidate.region", []byte(`{"queries":[{"fieldKey":"region"}]}`), queries, nil, nil)
	if err != nil || source.request.InterfaceInstanceNo != "pfi_11111111111111111111111111111111" ||
		source.request.Method != "candidate.region" || len(resolved["region"]) != 1 ||
		source.request.ExpectedConfigurationRevision != revision ||
		resolved["region"][0].Label.ZhCN != "华东" {
		t.Fatalf("resource candidate proxy: request=%+v resolved=%+v err=%v", source.request, resolved, err)
	}
}

func TestResolveResourceCandidatesPreservesSupplierFailureStatus(t *testing.T) {
	t.Parallel()
	source := &resourceCandidateSourceStub{result: &resourceinterfaceprojectioncap.CandidateResult{SyncState: resourceinterfaceprojectioncap.CandidateSyncRetained, Fields: []resourceinterfaceprojectioncap.CandidateField{{FieldKey: "region", Items: []resourceinterfaceprojectioncap.Candidate{{Value: "1", LabelZhCN: "陕西", LabelEnUS: "Shaanxi"}}}}}}
	service := &serviceImpl{resourceCandidates: source}
	state := resourceinterfaceprojectioncap.CandidateSyncReady
	_, err := service.resolveResourceCandidates(t.Context(), "pfi_11111111111111111111111111111111", "", "candidate.region", []byte(`{}`), []configurationcontract.DynamicCandidateQuery{{FieldKey: "region"}}, &state, nil)
	if err != nil || state != resourceinterfaceprojectioncap.CandidateSyncRetained {
		t.Fatalf("supplier status lost: state=%s err=%v", state, err)
	}
	source.result.SyncState = resourceinterfaceprojectioncap.CandidateSyncReady
	_, err = service.resolveResourceCandidates(t.Context(), "pfi_11111111111111111111111111111111", "", "candidate.region", []byte(`{}`), []configurationcontract.DynamicCandidateQuery{{FieldKey: "region"}}, &state, nil)
	if err != nil || state != resourceinterfaceprojectioncap.CandidateSyncRetained {
		t.Fatal("healthy field erased failed supplier status")
	}
}

func (stub *resourceCandidateSourceStub) CatalogAvailability(_ context.Context, ids []string) (map[string]bool, error) {
	stub.catalogIDs = append([]string(nil), ids...)
	return stub.catalog, nil
}
