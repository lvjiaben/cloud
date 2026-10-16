// This file verifies dynamic resource Provider dispatch and proves supplier
// secrets and raw tokens are absent from the guest-visible payload.

package backend

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type resourceOutboundStub struct {
	payloads [][]byte
	result   []byte
	results  map[uint8][]byte
}

func (s *resourceOutboundStub) Execute(_ string, sequence uint8, payload json.RawMessage) (json.RawMessage, error) {
	s.payloads = append(s.payloads, append([]byte(nil), payload...))
	if sequence == 1 {
		return json.RawMessage(`{"StatusCode":200,"Authenticated":true}`), nil
	}
	if result := s.results[sequence]; result != nil {
		return append(json.RawMessage(nil), result...), nil
	}
	return append(json.RawMessage(nil), s.result...), nil
}

var _ confidentialoutboundcap.Service = (*resourceOutboundStub)(nil)

func TestManagedGuestResolvesCandidatesOnlyThroughResourceProvider(t *testing.T) {
	t.Parallel()
	executor := &resourceOutboundStub{results: map[uint8][]byte{
		2: []byte(`{"StatusCode":200,"Data":{"data":[{"id":"cn-east","name":"华东"}]}}`),
		3: []byte(`{"StatusCode":200,"Data":{"data":[{"id":"premium","name":"精品线路"}]}}`),
		4: []byte(`{"StatusCode":200,"Data":{"data":[{"id":"debian-12","name":"Debian 12"}]}}`),
	}}
	payload := json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"region","parentValues":[],"selectedValues":[]},{"fieldKey":"line","parentValues":["cn-east"],"selectedValues":[]},{"fieldKey":"operating_system","parentValues":[],"selectedValues":[]}]}`)
	payload = managedInvocationEnvelope(t, payload)
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: resourcecap.OwnerPluginID,
		ProviderPluginID: pluginID, Point: resourcecap.DynamicProviderPoint,
		ContractVersion: resourcecap.DynamicProviderContractVersion,
		Method:          resourcecap.DynamicProviderMethodConfigurationCandidates,
		ReleaseVersion:  "v0.2.2", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose:        capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)}, Payload: payload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID, Route: &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request: &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != 200 || len(executor.payloads) != 4 {
		t.Fatalf("resource candidates response=%#v outbound=%q err=%v", response, executor.payloads, err)
	}
	outbound := string(bytes.Join(executor.payloads, nil))
	if !strings.Contains(outbound, `"path":"/areas"`) || !strings.Contains(outbound, `"path":"/nodes"`) || !strings.Contains(outbound, `"path":"/image"`) ||
		!strings.Contains(outbound, `"area_id":["cn-east"]`) {
		t.Fatalf("resource candidate paths were not constrained: %s", outbound)
	}
	var bridge capmodel.BusinessExtensionBridgeResponse
	if json.Unmarshal(response.Body, &bridge) != nil || !strings.Contains(string(bridge.Payload), `"FieldKey":"region"`) ||
		!strings.Contains(string(bridge.Payload), `"LabelZhCN":"华东"`) {
		t.Fatalf("resource candidate projection is invalid: %s", response.Body)
	}
}

func TestManagedGuestRejectsInvalidResourceCandidatesBeforeOutbound(t *testing.T) {
	t.Parallel()
	for _, payload := range []json.RawMessage{
		json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"password","parentValues":[],"selectedValues":[]}]}`),
		json.RawMessage(`{"schemaVersion":"v1","currentSelections":[],"requests":[{"fieldKey":"region","parentValues":[],"selectedValues":[],"secret":"bad"}]}`),
	} {
		payload = managedInvocationEnvelope(t, payload)
		executor := &resourceOutboundStub{}
		invocation := &capmodel.BusinessExtensionBridgeRequest{
			Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: resourcecap.OwnerPluginID,
			ProviderPluginID: pluginID, Point: resourcecap.DynamicProviderPoint,
			ContractVersion: resourcecap.DynamicProviderContractVersion,
			Method:          resourcecap.DynamicProviderMethodConfigurationCandidates,
			ReleaseVersion:  "v0.2.2", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
			Purpose:        capmodel.BusinessExtensionInvocationPurposeNewBusiness,
			RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)}, Payload: payload,
		}
		body, err := json.Marshal(invocation)
		if err != nil {
			t.Fatal(err)
		}
		response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
			PluginID: pluginID, Route: &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
			Request: &protocol.HTTPRequestSnapshotV1{Body: body},
		})
		if err != nil || response.StatusCode != 500 || len(executor.payloads) != 0 {
			t.Fatalf("invalid candidate request crossed outbound: response=%#v payloads=%q err=%v", response, executor.payloads, err)
		}
	}
}

func TestManagedGuestDispatchesResourceProviderWithoutSecretMaterial(t *testing.T) {
	t.Parallel()
	result, err := json.Marshal(map[string]any{
		"StatusCode": 200,
		"Data":       map[string]any{"data": map[string]any{"id": 41, "status": "on"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	executor := &resourceOutboundStub{result: result}
	requestPayload, err := json.Marshal(resourcecap.ResourceQuery{ExternalResourceRef: "41"})
	if err != nil {
		t.Fatal(err)
	}
	requestPayload = managedInvocationEnvelope(t, requestPayload)
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version:       capmodel.BusinessExtensionInvocationProtocolV1,
		OwnerPluginID: resourcecap.OwnerPluginID, ProviderPluginID: pluginID,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion,
		Method: resourcecap.DynamicProviderMethodQueryResource, ReleaseVersion: "v0.2.0",
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose:        capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)},
		Payload:        requestPayload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request:  &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != 200 {
		t.Fatalf("dispatch resource Provider invocation: response=%#v err=%v", response, err)
	}
	guestPayload := strings.ToLower(string(bytes.Join(executor.payloads, nil)))
	for _, forbidden := range []string{"opaque-encrypted-reference", "access-token", "authorization", "secretref", "secrethandle", "credentialfields", `"password":`} {
		if strings.Contains(guestPayload, forbidden) {
			t.Fatalf("guest payload contains forbidden supplier material %q: %s", forbidden, guestPayload)
		}
	}
	if !strings.Contains(guestPayload, `"kind":"authenticate"`) ||
		!strings.Contains(guestPayload, "/clouds/41") {
		t.Fatalf("guest payload lost the governed login or relative business request: %s", guestPayload)
	}
}

func TestManagedGuestRejectsCrossOwnerResourceInvocation(t *testing.T) {
	t.Parallel()
	executor := &resourceOutboundStub{result: []byte(`{}`)}
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version:       capmodel.BusinessExtensionInvocationProtocolV1,
		OwnerPluginID: "another-owner", ProviderPluginID: pluginID,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion,
		Method: resourcecap.DynamicProviderMethodDiscoverCapabilities, ReleaseVersion: "v0.2.0",
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose:        capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)},
		Payload:        json.RawMessage(`{}`),
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request:  &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != 400 || len(executor.payloads) != 0 {
		t.Fatalf("cross-owner invocation was not rejected: response=%#v payload=%q err=%v", response, executor.payloads, err)
	}
}

// TestManagedGuestBuildsProvisionParametersWithoutOutbound proves the new
// resource-owner method executes the connector mapper inside the guest and
// never opens a supplier session.
func TestManagedGuestBuildsProvisionParametersWithoutOutbound(t *testing.T) {
	t.Parallel()
	executor := &resourceOutboundStub{}
	plan := resourcecap.ServerProvisionPlan{
		ExternalPrincipalRef:  "91",
		Hostname:              "srv-001.example",
		ProductConfiguration:  json.RawMessage(`{}`),
		CustomerConfiguration: json.RawMessage(`{}`),
		Selector:              json.RawMessage(`{"provision":{"provider":"zjmfcloud-v1","areas":{},"nodes":{},"node_groups":{},"images":{"debian-12":"5"},"defaults":{}}}`),
		Components: []resourcecap.ServerProvisionComponent{
			{Type: resourcecap.ServerProvisionComponentCPU, Cores: 2, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentMemory, Megabytes: 2048, SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentImage, Code: "debian-12", SelectionQuantity: 1},
			{Type: resourcecap.ServerProvisionComponentDataDisk, Gigabytes: 100, SelectionQuantity: 1},
		},
	}
	payload, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	payload = managedInvocationEnvelope(t, payload)
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version: capmodel.BusinessExtensionInvocationProtocolV1, OwnerPluginID: resourcecap.OwnerPluginID,
		ProviderPluginID: pluginID, Point: resourcecap.DynamicProviderPoint,
		ContractVersion: resourcecap.DynamicProviderContractVersion,
		Method:          resourcecap.DynamicProviderMethodBuildProvisionParameters,
		ReleaseVersion:  "v0.2.2", ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose:        capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)}, Payload: payload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID, Route: &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request: &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != 200 || len(executor.payloads) != 0 {
		t.Fatalf("provision mapper response=%#v outbound=%q err=%v", response, executor.payloads, err)
	}
	var bridge capmodel.BusinessExtensionBridgeResponse
	if json.Unmarshal(response.Body, &bridge) != nil || !strings.Contains(string(bridge.Payload), `"cpu":2`) ||
		!strings.Contains(string(bridge.Payload), `"os":"5"`) ||
		!strings.Contains(string(bridge.Payload), `"data_disk_size":100`) {
		t.Fatalf("provision mapper projection is invalid: %s", response.Body)
	}
}

func TestManagedGuestRejectsResourceProvisionWithoutPasswordBeforeOutbound(t *testing.T) {
	t.Parallel()
	executor := &resourceOutboundStub{}
	requestPayload, err := json.Marshal(resourcecap.DynamicOperationRequest{
		RequestID:  "provision-without-password",
		Capability: resourcecap.CapabilityProvision,
		Parameters: json.RawMessage(`{"cpu":2,"memory":2048,"os":5,"client":91,"hostname":"srv-001.example"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	requestPayload = managedInvocationEnvelope(t, requestPayload)
	invocation := &capmodel.BusinessExtensionBridgeRequest{
		Version:       capmodel.BusinessExtensionInvocationProtocolV1,
		OwnerPluginID: resourcecap.OwnerPluginID, ProviderPluginID: pluginID,
		Point: resourcecap.DynamicProviderPoint, ContractVersion: resourcecap.DynamicProviderContractVersion,
		Method: resourcecap.DynamicProviderMethodExecute, ReleaseVersion: "v0.2.0",
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose:        capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
		RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: strings.Repeat("c", 48)},
		Payload:        requestPayload,
	}
	body, err := json.Marshal(invocation)
	if err != nil {
		t.Fatal(err)
	}
	response, err := NewBusinessExtensionHandler(executor)(&protocol.BridgeRequestEnvelopeV1{
		PluginID: pluginID,
		Route:    &protocol.RouteMatchSnapshotV1{RequestType: capmodel.BusinessExtensionBridgeRequestType},
		Request:  &protocol.HTTPRequestSnapshotV1{Body: body},
	})
	if err != nil || response == nil || response.StatusCode != 500 || len(executor.payloads) != 0 {
		t.Fatalf("provision without the owner-bound password was not rejected before outbound: response=%#v payloads=%q err=%v", response, executor.payloads, err)
	}
}

func managedInvocationEnvelope(t *testing.T, payload json.RawMessage) json.RawMessage {
	t.Helper()
	result, err := resourcecap.NewDynamicProviderInvocationEnvelope(json.RawMessage(`{}`), payload)
	if err != nil {
		t.Fatalf("build managed invocation envelope: %v", err)
	}
	return result
}
