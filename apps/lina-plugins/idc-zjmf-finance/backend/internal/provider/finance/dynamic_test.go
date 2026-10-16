package finance

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type recordingExecutor struct {
	requests []resourcecap.ConfidentialHTTPRequest
	results  []resourcecap.ConfidentialHTTPResponse
}

func (e *recordingExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	e.requests = append(e.requests, request)
	result := e.results[len(e.requests)-1]
	encoded, err := json.Marshal(result)
	return json.RawMessage(encoded), err
}

var _ confidentialoutboundcap.Service = (*recordingExecutor)(nil)

func TestNativeAuthenticationAndProductCandidatesUseOnlyVerifiedRoutes(t *testing.T) {
	executor := &recordingExecutor{results: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Authenticated: true},
		{StatusCode: 200, Data: json.RawMessage(`{"data":{"products":[{"name":"云服务","products":[{"id":41,"name":"KVM A"}]}]}}`)},
	}}
	payload := json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"upstream_product_id","parentValues":[],"selectedValues":[]}]}`)
	result, err := ExecuteDynamic(executor, "ctx-1", resourcecap.DynamicProviderMethodConfigurationCandidates, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 2 || executor.requests[0].Path != "/zjmf_api_login" || executor.requests[1].Path != "/cart/all" {
		t.Fatalf("unexpected native routes: %#v", executor.requests)
	}
	if len(executor.requests[0].CredentialFields) != 0 {
		t.Fatal("login does not use the resource-owned credential slots")
	}
	if string(result) == "" {
		t.Fatal("candidate result is empty")
	}
}

func TestMultipleConfigurationFieldsShareOneProductScopedRead(t *testing.T) {
	executor := &recordingExecutor{results: []resourcecap.ConfidentialHTTPResponse{
		{StatusCode: 200, Authenticated: true},
		{StatusCode: 200, Data: json.RawMessage(`{"data":{"config_groups":[{"options":[{"id":11,"option_name":"区域","sub":[{"id":101,"option_name":"香港"}]},{"id":12,"option_name":"系统","sub":[{"id":201,"option_name":"Linux"}]}]}]}}`)},
	}}
	payload := json.RawMessage(`{"schemaVersion":"v1","currentSelections":[{"fieldKey":"upstream_product_id","values":["41"]},{"fieldKey":"upstream_region_option_id","values":["11"]},{"fieldKey":"upstream_os_option_id","values":["12"]}],"requests":[{"fieldKey":"region","parentValues":[],"selectedValues":[]},{"fieldKey":"operating_system","parentValues":[],"selectedValues":[]}]}`)
	if _, err := ExecuteDynamic(executor, "ctx-1", resourcecap.DynamicProviderMethodConfigurationCandidates, payload); err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 2 || executor.requests[1].Path != "/cart/get_product_config" || executor.requests[1].Query["pid"][0] != "41" {
		t.Fatalf("configuration discovery was not one product-scoped read: %#v", executor.requests)
	}
}

func TestTransportFailureReturnsUnknownInsteadOfRetryableSuccess(t *testing.T) {
	result := unknownResult("req-1", "91")
	if result.Status != resourcecap.TaskStatusUnknown || result.Failure == nil || !result.Failure.ResultUnknown || result.Failure.Retryable {
		t.Fatalf("unsafe unknown outcome: %#v", result)
	}
}

func TestResourceStateReadsNestedBusinessDataInsteadOfEnvelopeStatus(t *testing.T) {
	raw := json.RawMessage(`{"status":200,"data":{"status":"on"}}`)
	if got := nestedDataString(raw, "status"); got != "on" {
		t.Fatalf("nested resource status became %q", got)
	}
	if !successfulBusinessResponse(raw) {
		t.Fatal("successful response envelope was rejected")
	}
}
