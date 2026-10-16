// This file verifies the managed guest contract and confidential supplier-call boundary.

package backend

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

type fakeConfidentialOutbound struct {
	contextID string
	payload   json.RawMessage
	result    json.RawMessage
	err       error
	calls     int
}

func (f *fakeConfidentialOutbound) Execute(contextID string, _ uint8, payload json.RawMessage) (json.RawMessage, error) {
	f.calls++
	f.contextID = contextID
	f.payload = append(json.RawMessage(nil), payload...)
	return append(json.RawMessage(nil), f.result...), f.err
}

func TestManagedGuestRejectsDirectProductFulfillmentBeforeOutbound(t *testing.T) {
	t.Parallel()
	for _, method := range []string{candidateMethod, provisionMethod, queryMethod} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			executor := &fakeConfidentialOutbound{}
			response, err := NewBusinessExtensionHandler(executor)(businessExtensionEnvelope(t, method, json.RawMessage(`{"version":"v1"}`)))
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 400 || response.Failure == nil || executor.calls != 0 {
				t.Fatalf("direct product fulfillment crossed an outbound boundary: response=%#v executor=%#v", response, executor)
			}
		})
	}
}

// TestGenerationValidationIsPureBeforePayment proves invalid cross-field
// naming rules fail before payment and never use supplier outbound access.
func TestGenerationValidationIsPureBeforePayment(t *testing.T) {
	t.Parallel()
	for _, configuration := range []string{`{"hostname_prefix":"Xxun","hostname_length":12,"password_length":12}`, `{"hostname_prefix":"longprefix","hostname_length":4}`} {
		request := productfulfillmentcap.ConfigurationValidationRequest{Version: "v1", ProductConfiguration: json.RawMessage(configuration)}
		payload, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		envelope := businessExtensionEnvelope(t, productfulfillmentcap.MethodValidateProductConfiguration, payload)
		var invocation capmodel.BusinessExtensionBridgeRequest
		if err = json.Unmarshal(envelope.Request.Body, &invocation); err != nil {
			t.Fatal(err)
		}
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
		envelope.Request.Body, err = json.Marshal(invocation)
		if err != nil {
			t.Fatal(err)
		}
		executor := &fakeConfidentialOutbound{}
		response, err := NewBusinessExtensionHandler(executor)(envelope)
		if err != nil || response.StatusCode != 200 || executor.calls != 0 {
			t.Fatalf("validation crossed boundary: %#v %v calls=%d", response, err, executor.calls)
		}
		var result capmodel.BusinessExtensionBridgeResponse
		if err = json.Unmarshal(response.Body, &result); err != nil {
			t.Fatal(err)
		}
		expected := strings.Contains(configuration, `"hostname_length":12`)
		var validation struct {
			Valid bool `json:"valid"`
		}
		if err = json.Unmarshal(result.Payload, &validation); err != nil || validation.Valid != expected {
			t.Fatalf("wrong validation result: %s %v", result.Payload, err)
		}
	}
}

func TestManagedGuestRejectsInvalidEvidenceBeforeOutbound(t *testing.T) {
	executor := &fakeConfidentialOutbound{result: json.RawMessage(`{"version":"v1","status":"not_accepted"}`)}
	envelope := businessExtensionEnvelope(t, provisionMethod, json.RawMessage(`{"version":"v1"}`))
	var invocation capmodel.BusinessExtensionBridgeRequest
	if err := json.Unmarshal(envelope.Request.Body, &invocation); err != nil {
		t.Fatal(err)
	}
	invocation.ProviderPluginID = "another-plugin"
	envelope.Request.Body, _ = json.Marshal(&invocation)
	response, err := NewBusinessExtensionHandler(executor)(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 400 || executor.calls != 0 {
		t.Fatalf("invalid evidence crossed the confidential boundary: response=%#v calls=%d", response, executor.calls)
	}
}

func TestManagedGuestRejectsProvisionForNewBusinessBeforePayment(t *testing.T) {
	for _, method := range []string{candidateMethod, provisionMethod} {
		executor := &fakeConfidentialOutbound{}
		envelope := businessExtensionEnvelope(t, method, json.RawMessage(`{"version":"v1"}`))
		var invocation capmodel.BusinessExtensionBridgeRequest
		if err := json.Unmarshal(envelope.Request.Body, &invocation); err != nil {
			t.Fatal(err)
		}
		invocation.Purpose = capmodel.BusinessExtensionInvocationPurposeNewBusiness
		envelope.Request.Body, _ = json.Marshal(&invocation)
		response, err := NewBusinessExtensionHandler(executor)(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 400 || executor.calls != 0 {
			t.Fatalf("new-business product method %s crossed the outbound boundary: response=%#v calls=%d", method, response, executor.calls)
		}
	}
}

func businessExtensionEnvelope(t *testing.T, method string, payload json.RawMessage) *protocol.BridgeRequestEnvelopeV1 {
	t.Helper()
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version:           capmodel.BusinessExtensionInvocationProtocolV1,
		OwnerPluginID:     fulfillmentOwnerPluginID,
		ProviderPluginID:  pluginID,
		Point:             capmodel.BusinessExtensionProductFulfillment,
		ContractVersion:   productfulfillmentcap.ContractVersion,
		Method:            method,
		ReleaseVersion:    "v0.2.0",
		ReleaseDigest:     strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64),
		Purpose:           capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext:    &capmodel.BusinessExtensionRuntimeContext{ContextID: "src_" + strings.Repeat("c", 44)},
		Payload:           payload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	return &protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request:  &protocol.HTTPRequestSnapshotV1{Body: body},
	}
}
