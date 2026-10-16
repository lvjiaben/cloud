// This file reproduces governed JSON rejection for password-named map keys
// and verifies login plus reset-password now use only safe wire field names.

package zjmfcloud

import (
	"encoding/json"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"strings"
	"testing"
)

type secretBindingWireExecutor struct {
	requests []resourcecap.ConfidentialHTTPRequest
}

func (executor *secretBindingWireExecutor) Execute(_ string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	if strings.Contains(string(payload), `"password":`) {
		return nil, errInvalidConfig
	}
	var request resourcecap.ConfidentialHTTPRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return nil, err
	}
	executor.requests = append(executor.requests, request)
	if request.Kind == resourcecap.ConfidentialHTTPKindAuthenticate {
		return json.RawMessage(`{"StatusCode":200,"Authenticated":true}`), nil
	}
	if request.Method == "GET" {
		return json.RawMessage(`{"StatusCode":200,"Data":{"data":{"id":41}}}`), nil
	}
	return json.RawMessage(`{"StatusCode":200,"Data":{"data":{"taskid":71}}}`), nil
}

func TestDynamicPasswordOperationUsesSecretBindingArray(t *testing.T) {
	executor := &secretBindingWireExecutor{}
	payload, err := json.Marshal(resourcecap.DynamicOperationRequest{RequestID: "reset-password-wire", Capability: resourcecap.CapabilityResetPassword, ResourceRef: "41", ConfidentialInputNames: []string{"password"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ExecuteDynamicResource(executor, "context-wire", resourcecap.DynamicProviderMethodExecute, payload); err != nil {
		t.Fatal(err)
	}
	if len(executor.requests) != 3 {
		t.Fatalf("unexpected steps: %d", len(executor.requests))
	}
	if len(executor.requests[0].CredentialFields) != 0 {
		t.Fatal("guest must not send owner authentication mapping")
	}
	operation := executor.requests[2]
	if len(operation.SecretFormFields) != 0 || len(operation.SecretFormBindings) != 1 || operation.SecretFormBindings[0] != (resourcecap.ConfidentialSecretFormBinding{Field: "password", Slot: "password"}) {
		t.Fatalf("unexpected operation projection: %#v", operation)
	}
}
