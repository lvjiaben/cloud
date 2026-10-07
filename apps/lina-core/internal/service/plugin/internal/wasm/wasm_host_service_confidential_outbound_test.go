// This file verifies the confidential outbound dispatcher is available only
// to exact internal business-extension executions with explicit authorization.

package wasm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/internal/service/plugin/internal/confidentialoutbound"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-core/pkg/plugin/pluginhost"
)

func TestConfidentialOutboundDispatcherUsesExactOrderedContext(t *testing.T) {
	registry := confidentialoutbound.New()
	contextID, revoke, err := registry.Register(confidentialoutbound.Registration{
		OwnerPluginID: "idc-resource",
		Input: pluginhost.ConfidentialOutboundContextInput{
			ProviderPluginID: "zjmfcloud", Point: "product-fulfillment", ContractVersion: "v1",
			ProviderMethod: "server.query", OutboundMethod: "zjmfcloud.server.query",
			ReleaseVersion: "1.0.0", ReleaseDigest: strings.Repeat("a", 64),
			DeclarationDigest: strings.Repeat("b", 64), InterfaceInstanceID: "iface-a",
			Purpose:             capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
			AllowedResultFields: []string{"state"},
			Handler: func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
				return &pluginhost.ConfidentialOutboundResult{Payload: json.RawMessage(`{"state":"running"}`)}, nil
			},
		},
	})
	if err != nil {
		t.Fatalf("register context failed: %v", err)
	}
	defer revoke()
	hcc := &hostCallContext{
		runtime: &hostServiceRuntime{confidentialOutbound: registry}, pluginID: "zjmfcloud",
		capabilities:    map[string]struct{}{protocol.CapabilityConfidentialOutbound: {}},
		hostServices:    []*protocol.HostServiceSpec{{Service: protocol.HostServiceConfidentialOutbound, Methods: []string{protocol.HostServiceMethodConfidentialOutboundExecute}}},
		executionSource: contract.ExecutionSourceBusinessExtension,
		confidentialOutboundInvocation: &confidentialoutbound.Invocation{
			OwnerPluginID: "idc-resource", ProviderPluginID: "zjmfcloud", Point: "product-fulfillment",
			ContractVersion: "v1", ProviderMethod: "server.query", ReleaseVersion: "1.0.0",
			ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
			Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, ContextID: contextID,
		},
	}
	response := invokeConfidentialOutbound(t, hcc, contextID, 1)
	if response.Status != protocol.HostCallStatusSuccess {
		t.Fatalf("expected success, got status=%d payload=%s", response.Status, response.Payload)
	}
	decoded, err := protocol.UnmarshalHostServiceJSONResponse(response.Payload)
	if err != nil || !strings.Contains(string(decoded.Value), `"state":"running"`) {
		t.Fatalf("unexpected safe projection value=%s err=%v", decoded.Value, err)
	}
	if replay := invokeConfidentialOutbound(t, hcc, contextID, 1); replay.Status == protocol.HostCallStatusSuccess {
		t.Fatal("expected replay to fail closed")
	}
}

func TestConfidentialOutboundDispatcherRejectsOrdinaryRouteExecution(t *testing.T) {
	hcc := &hostCallContext{
		runtime: &hostServiceRuntime{confidentialOutbound: confidentialoutbound.New()}, pluginID: "zjmfcloud",
		capabilities:    map[string]struct{}{protocol.CapabilityConfidentialOutbound: {}},
		hostServices:    []*protocol.HostServiceSpec{{Service: protocol.HostServiceConfidentialOutbound, Methods: []string{protocol.HostServiceMethodConfidentialOutboundExecute}}},
		executionSource: contract.ExecutionSourceRoute,
	}
	response := invokeConfidentialOutbound(t, hcc, "ctx_abcdefghijklmnopqrstuvwxyzABCDEFG123456", 1)
	if response.Status != protocol.HostCallStatusCapabilityDenied {
		t.Fatalf("expected ordinary route to be denied, got status=%d", response.Status)
	}
}

func invokeConfidentialOutbound(t *testing.T, hcc *hostCallContext, contextID string, sequence uint8) *protocol.HostCallResponseEnvelope {
	t.Helper()
	content, err := json.Marshal(struct {
		ContextID string          `json:"contextId"`
		Sequence  uint8           `json:"sequence"`
		Payload   json.RawMessage `json:"payload"`
	}{ContextID: contextID, Sequence: sequence, Payload: json.RawMessage(`{"resource":"srv-1"}`)})
	if err != nil {
		t.Fatalf("encode request failed: %v", err)
	}
	return handleHostServiceInvoke(context.Background(), withTestHostCallRuntime(t, hcc), protocol.MarshalHostServiceRequestEnvelope(&protocol.HostServiceRequestEnvelope{
		Service: protocol.HostServiceConfidentialOutbound, Method: protocol.HostServiceMethodConfidentialOutboundExecute,
		Payload: protocol.MarshalHostServiceJSONRequest(&protocol.HostServiceJSONRequest{Value: content}),
	}))
}
