package finance

import (
	"encoding/json"
	"testing"
)

func TestCandidateCallsRequireUpstreamProductForProductConfiguration(t *testing.T) {
	request, err := DecodeCandidateRequest(json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"region","parentValues":[],"selectedValues":[]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CandidateCalls(request); err == nil {
		t.Fatal("product configuration candidates were accepted without upstream_product_id")
	}
}

func TestCandidateCallsKeepDifferentProductsSeparate(t *testing.T) {
	build := func(product string) CandidateCall {
		raw := json.RawMessage(`{"schemaVersion":"v1","currentSelections":[{"fieldKey":"upstream_product_id","values":["` + product + `"]}],"requests":[{"fieldKey":"operating_system","parentValues":[],"selectedValues":[]}]}`)
		request, err := DecodeCandidateRequest(raw)
		if err != nil {
			t.Fatal(err)
		}
		calls, err := CandidateCalls(request)
		if err != nil || len(calls) != 1 {
			t.Fatalf("unexpected calls: %#v err=%v", calls, err)
		}
		return calls[0]
	}
	first, second := build("41"), build("42")
	if first.Query.Get("pid") != "41" || second.Query.Get("pid") != "42" || first.Query.Encode() == second.Query.Encode() {
		t.Fatalf("candidate lookup reused another product: %q %q", first.Query.Encode(), second.Query.Encode())
	}
}

func TestProductCandidateProjectionIsBoundedAndSafe(t *testing.T) {
	items, err := ProjectProductCandidates(json.RawMessage(`{"data":{"products":[{"name":"云服务","products":[{"id":41,"name":"KVM A"}]}]}}`))
	if err != nil || len(items) != 1 || items[0].Value != "41" || items[0].LabelZhCN != "KVM A" {
		t.Fatalf("unexpected candidates: %#v err=%v", items, err)
	}
}

func TestConfigurationCandidatesRequireExplicitSemanticOptionMapping(t *testing.T) {
	raw := json.RawMessage(`{"data":{"config_groups":[{"options":[{"id":11,"option_name":"机房","sub":[{"id":101,"option_name":"香港"}]}]}]}}`)
	if _, err := ProjectConfigurationCandidates("region", nil, raw); err == nil {
		t.Fatal("region candidates were guessed without an administrator-selected option")
	}
	items, err := ProjectConfigurationCandidates("region", []CandidateSelection{{FieldKey: "upstream_region_option_id", Values: []string{"11"}}}, raw)
	if err != nil || len(items) != 1 || items[0].Value != "101" || items[0].LabelZhCN != "香港" {
		t.Fatalf("unexpected mapped candidates: %#v err=%v", items, err)
	}
}
