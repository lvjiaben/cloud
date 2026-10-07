package configurationcontract

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestDynamicCandidateBatchesGroupFieldsByAuthorizedMethod(t *testing.T) {
	schema := dynamicTestSchema()
	values := []ConfigurationValue{
		{FieldKey: "country", OptionTokens: []string{"cn"}},
		{FieldKey: "region", AddressOptionTokens: []string{"cn", "cn-zj"}},
	}
	batches, err := BuildDynamicCandidateBatches(schema, values, SurfaceCustomer, []string{"candidate.location"})
	if err != nil {
		t.Fatalf("build batches: %v", err)
	}
	if len(batches) != 1 || batches[0].Method != "candidate.location" || len(batches[0].Fields) != 2 {
		t.Fatalf("fields were not grouped into one method call: %#v", batches)
	}
	var request DynamicCandidateRequest
	if err = json.Unmarshal(batches[0].Payload, &request); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(request.Requests) != 2 || request.Requests[0].FieldKey != "region" ||
		len(request.Requests[0].ParentValues) != 1 || request.Requests[0].ParentValues[0] != "cn" ||
		len(request.Requests[0].SelectedValues) != 2 {
		t.Fatalf("unexpected canonical request: %#v", request)
	}
	if len(request.CurrentSelections) != 2 || request.CurrentSelections[0].FieldKey != "country" ||
		request.CurrentSelections[0].Values[0] != "cn" || request.CurrentSelections[1].FieldKey != "region" {
		t.Fatalf("request omitted the safe cross-field current selections: %#v", request.CurrentSelections)
	}
}

func TestDynamicCandidateRequestMatchesHostPayloadBound(t *testing.T) {
	schema := dynamicTestSchema()
	oversized := make([]string, MaximumDynamicCandidatesPerField)
	for index := range oversized {
		oversized[index] = strings.Repeat("x", 240) + string(rune('a'+index%26)) + string(rune('A'+index/26%26))
	}
	_, err := BuildDynamicCandidateBatches(
		schema,
		[]ConfigurationValue{{FieldKey: "region", AddressOptionTokens: oversized}},
		SurfaceCustomer,
		[]string{"candidate.location"},
	)
	if err == nil {
		t.Fatal("request larger than the host 60 KiB payload allowance was accepted")
	}
}

func TestDynamicCandidateResponseRejectsUnsafeShapes(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "浙江", EnUS: "Zhejiang"}
	valid := DynamicCandidateResponse{SchemaVersion: DynamicCandidateProtocolV1, Items: []DynamicCandidateItem{
		{FieldKey: "region", Candidates: []*capmodel.ConfigurationCandidate{{Value: "cn", Label: label}, {Value: "cn-zj", ParentValue: "cn", Label: label}}},
	}}
	encode := func(value any) []byte {
		payload, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return payload
	}
	if _, err := DecodeDynamicCandidateResponse(encode(valid), []DynamicCandidateQuery{{FieldKey: "region"}}); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	tests := map[string][]byte{
		"unknown field":   encode(DynamicCandidateResponse{SchemaVersion: "v1", Items: []DynamicCandidateItem{{FieldKey: "other"}}}),
		"duplicate field": encode(DynamicCandidateResponse{SchemaVersion: "v1", Items: []DynamicCandidateItem{{FieldKey: "region"}, {FieldKey: "region"}}}),
		"unknown member":  []byte(`{"schemaVersion":"v1","items":[{"fieldKey":"region","candidates":[],"private":"x"}]}`),
		"trailing value":  append(encode(valid), []byte(` {}`)...),
		"oversize":        []byte(strings.Repeat("x", MaximumDynamicCandidatePayload+1)),
		"duplicate candidate": encode(DynamicCandidateResponse{SchemaVersion: "v1", Items: []DynamicCandidateItem{{FieldKey: "region", Candidates: []*capmodel.ConfigurationCandidate{
			{Value: "same", Label: label}, {Value: "same", Label: label},
		}}}}),
		"invalid parent": encode(DynamicCandidateResponse{SchemaVersion: "v1", Items: []DynamicCandidateItem{{FieldKey: "region", Candidates: []*capmodel.ConfigurationCandidate{
			{Value: "child", ParentValue: "missing", Label: label},
		}}}}),
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			fields := []DynamicCandidateQuery{{FieldKey: "region"}}
			if name == "duplicate field" {
				fields = []DynamicCandidateQuery{{FieldKey: "region"}, {FieldKey: "zone"}}
			}
			if _, err := DecodeDynamicCandidateResponse(payload, fields); err == nil {
				t.Fatal("unsafe dynamic response accepted")
			}
		})
	}
}

func TestCompileResolvedValidatesHierarchyAndSelectedValue(t *testing.T) {
	schema := dynamicTestSchema()
	resolved := ResolvedCandidates{
		"region": {{Value: "cn", Label: dynamicLabel("中国", "China")}, {Value: "cn-zj", ParentValue: "cn", Label: dynamicLabel("浙江", "Zhejiang")}},
		"zone":   {{Value: "a", Label: dynamicLabel("可用区 A", "Zone A")}},
	}
	values := []ConfigurationValue{{FieldKey: "region", AddressOptionTokens: []string{"cn", "cn-zj"}}}
	if _, err := CompileResolved(schema, values, SurfaceCustomer, []string{"candidate.location"}, resolved); err != nil {
		t.Fatalf("compile resolved candidates: %v", err)
	}
	values[0].AddressOptionTokens[1] = "unknown"
	if _, err := CompileResolved(schema, values, SurfaceCustomer, []string{"candidate.location"}, resolved); err == nil {
		t.Fatal("unknown selected candidate accepted")
	}
	resolved["region"][1].ParentValue = "missing"
	if _, err := CompileResolved(schema, nil, SurfaceCustomer, []string{"candidate.location"}, resolved); err == nil {
		t.Fatal("invalid parent relation accepted")
	}
}

func dynamicTestSchema() *capmodel.ConfigurationSchemaDeclaration {
	return &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
		{Key: "country", Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicLabel("国家", "Country"), StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "cn", Label: dynamicLabel("中国", "China")}}},
		{Key: "region", Type: capmodel.ConfigurationFieldAddress, Label: dynamicLabel("地区", "Region"), DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.location"}},
		{Key: "zone", Type: capmodel.ConfigurationFieldSingleSelect, Label: dynamicLabel("可用区", "Zone"), DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "candidate.location"}},
	}}
}

func dynamicLabel(zhCN, enUS string) *capmodel.LocalizedText {
	return &capmodel.LocalizedText{ZhCN: zhCN, EnUS: enUS}
}
