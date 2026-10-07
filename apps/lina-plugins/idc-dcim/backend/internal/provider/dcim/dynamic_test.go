package dcim

import (
	"encoding/json"
	"errors"
	"testing"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type recordedExecutor struct {
	requests []resourcecap.ConfidentialHTTPRequest
	results  []json.RawMessage
	err      error
}

func (e *recordedExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	var request resourcecap.ConfidentialHTTPRequest
	if json.Unmarshal(payload, &request) != nil {
		return nil, errors.New("invalid request")
	}
	e.requests = append(e.requests, request)
	if e.err != nil {
		return nil, e.err
	}
	result := e.results[0]
	e.results = e.results[1:]
	return result, nil
}

func response(status int, data string) json.RawMessage {
	raw, _ := json.Marshal(resourcecap.ConfidentialHTTPResponse{StatusCode: status, Data: json.RawMessage(data)})
	return raw
}

func TestBMSProvisionUsesHostSecretSlotAndRelativeRoute(t *testing.T) {
	t.Parallel()
	executor := &recordedExecutor{results: []json.RawMessage{response(200, `{"code":200,"data":77}`)}}
	parameters, err := buildProvisionParameters(ProtocolBMS, validPlan())
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(resourcecap.DynamicOperationRequest{
		RequestID: "request-1", Capability: resourcecap.CapabilityProvision, Parameters: parameters,
		ConfidentialInputNames: []string{"password"},
	})
	result, err := ExecuteDynamic(executor, "context-1", json.RawMessage(`{"protocol":"bms"}`), resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Path != "/bms/source" || executor.requests[0].SecretFormFields["password"] != "password" {
		t.Fatalf("unexpected confidential request: %#v", executor.requests)
	}
	encoded, _ := json.Marshal(executor.requests[0])
	if containsAny(string(encoded), "access_token", "access-user", "context-1") {
		t.Fatalf("credential material leaked into guest request: %s", encoded)
	}
	var operation resourcecap.OperationResult
	if json.Unmarshal(result, &operation) != nil || operation.Status != resourcecap.TaskStatusSucceeded || operation.ExternalResourceRef != "77" {
		t.Fatalf("unexpected result: %s", result)
	}
}

func TestBMSNetworkFailureReturnsUnknown(t *testing.T) {
	t.Parallel()
	executor := &recordedExecutor{err: errors.New("timeout")}
	parameters, _ := buildProvisionParameters(ProtocolBMS, validPlan())
	payload, _ := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "request-2", Capability: resourcecap.CapabilityProvision, Parameters: parameters, ConfidentialInputNames: []string{"password"}})
	result, err := ExecuteDynamic(executor, "context-2", json.RawMessage(`{"protocol":"bms"}`), resourcecap.DynamicProviderMethodExecute, payload)
	if err != nil {
		t.Fatal(err)
	}
	var operation resourcecap.OperationResult
	if json.Unmarshal(result, &operation) != nil || operation.Status != resourcecap.TaskStatusUnknown || operation.Failure == nil || !operation.Failure.ResultUnknown {
		t.Fatalf("network ambiguity was not preserved: %s", result)
	}
}

func TestLegacyOperationMakesNoOutboundCall(t *testing.T) {
	t.Parallel()
	executor := &recordedExecutor{}
	payload, _ := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "request-3", Capability: resourcecap.CapabilityStart, ResourceRef: "5"})
	if _, err := ExecuteDynamic(executor, "context-3", json.RawMessage(`{"protocol":"legacy"}`), resourcecap.DynamicProviderMethodExecute, payload); err == nil {
		t.Fatal("legacy lifecycle must be unsupported")
	}
	if len(executor.requests) != 0 {
		t.Fatalf("legacy lifecycle performed outbound calls: %#v", executor.requests)
	}
}

func TestBMSResourceQueryUsesProvenStatusRoute(t *testing.T) {
	t.Parallel()
	executor := &recordedExecutor{results: []json.RawMessage{response(200, `{"code":200,"status":"on"}`)}}
	payload, _ := json.Marshal(resourcecap.ResourceQuery{ExternalResourceRef: "91"})
	result, err := ExecuteDynamic(executor, "context-5", json.RawMessage(`{"protocol":"bms"}`), resourcecap.DynamicProviderMethodQueryResource, payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 1 || executor.requests[0].Path != "/bms/source/91/status" {
		t.Fatalf("unexpected status request: %#v", executor.requests)
	}
	var snapshot resourcecap.ResourceSnapshot
	if json.Unmarshal(result, &snapshot) != nil || snapshot.State != resourcecap.ResourceStateRunning {
		t.Fatalf("unexpected resource snapshot: %s", result)
	}
}
