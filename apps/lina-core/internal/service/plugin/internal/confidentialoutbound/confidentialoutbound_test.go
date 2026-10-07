// This file verifies exact binding, expiry, ordered execution, and safe result projection.

package confidentialoutbound

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

func TestRegistryExecutesExactBindingInOrderAndRejectsReplay(t *testing.T) {
	registry := New()
	calls := 0
	contextID, revoke, err := registry.Register(testRegistration(func(_ context.Context, request *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		calls++
		if string(request.Payload) != `{"resource":"srv-1"}` {
			t.Fatalf("unexpected safe request: %s", request.Payload)
		}
		return &pluginhost.ConfidentialOutboundResult{Payload: json.RawMessage(`{"state":"running"}`)}, nil
	}))
	if err != nil {
		t.Fatalf("register context failed: %v", err)
	}
	defer revoke()
	invocation := testInvocation(contextID)
	result, err := registry.Execute(context.Background(), invocation, &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: json.RawMessage(`{"resource":"srv-1"}`)})
	if err != nil || string(result.Payload) != `{"state":"running"}` || calls != 1 {
		t.Fatalf("exact execution failed result=%#v calls=%d err=%v", result, calls, err)
	}
	if _, err = registry.Execute(context.Background(), invocation, &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: json.RawMessage(`{"resource":"srv-1"}`)}); err == nil {
		t.Fatal("expected sequence replay to fail closed")
	}
	if calls != 1 {
		t.Fatalf("replay reached supplier handler: calls=%d", calls)
	}
}

func TestRegistryRejectsCrossPluginAndExpiredContexts(t *testing.T) {
	registry := New()
	now := time.Unix(100, 0)
	registry.now = func() time.Time { return now }
	contextID, _, err := registry.Register(testRegistration(testSafeHandler))
	if err != nil {
		t.Fatalf("register context failed: %v", err)
	}
	wrong := testInvocation(contextID)
	wrong.ProviderPluginID = "other-provider"
	if _, err = registry.Execute(context.Background(), wrong, &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: json.RawMessage(`{"resource":"srv-1"}`)}); err == nil {
		t.Fatal("expected cross-plugin use to fail closed")
	}

	contextID, _, err = registry.Register(testRegistration(testSafeHandler))
	if err != nil {
		t.Fatalf("register expiring context failed: %v", err)
	}
	now = now.Add(11 * time.Second)
	if _, err = registry.Execute(context.Background(), testInvocation(contextID), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: json.RawMessage(`{"resource":"srv-1"}`)}); err == nil {
		t.Fatal("expected expired context use to fail closed")
	}
}

func TestRegistryRejectsSecretOrUnapprovedResultProjection(t *testing.T) {
	for _, result := range []json.RawMessage{
		json.RawMessage(`{"token":"plain-text"}`),
		json.RawMessage(`{"state":{"client_secret":"plain-text"}}`),
		json.RawMessage(`{"unapproved":"value"}`),
	} {
		registry := New()
		contextID, _, err := registry.Register(testRegistration(func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
			return &pluginhost.ConfidentialOutboundResult{Payload: result}, nil
		}))
		if err != nil {
			t.Fatalf("register context failed: %v", err)
		}
		if _, err = registry.Execute(context.Background(), testInvocation(contextID), &pluginhost.ConfidentialOutboundRequest{Sequence: 1, Payload: json.RawMessage(`{"resource":"srv-1"}`)}); err == nil {
			t.Fatalf("expected unsafe result to fail closed: %s", result)
		}
	}
}

func TestRegistryRejectsGuestSelectedInterfaceInstance(t *testing.T) {
	registry := New()
	calls := 0
	registration := testRegistration(func(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		calls++
		return &pluginhost.ConfidentialOutboundResult{Payload: json.RawMessage(`{"state":"running"}`)}, nil
	})
	contextID, _, err := registry.Register(registration)
	if err != nil {
		t.Fatalf("register context failed: %v", err)
	}
	_, err = registry.Execute(context.Background(), testInvocation(contextID), &pluginhost.ConfidentialOutboundRequest{
		Sequence: 1, Payload: json.RawMessage(`{"interfaceInstanceId":"iface-b"}`),
	})
	if err == nil {
		t.Fatal("expected guest-selected interface instance to fail closed")
	}
	if calls != 0 {
		t.Fatalf("unsafe request reached supplier handler: calls=%d", calls)
	}
}

func testRegistration(handler pluginhost.ConfidentialOutboundHandler) Registration {
	return Registration{OwnerPluginID: "idc-resource", Input: pluginhost.ConfidentialOutboundContextInput{
		ProviderPluginID: "zjmfcloud", Point: "product-fulfillment", ContractVersion: "v1",
		ProviderMethod: "server.provision", OutboundMethod: "zjmfcloud.server.provision",
		ReleaseVersion: "1.0.0", ReleaseDigest: strings.Repeat("a", 64),
		DeclarationDigest: strings.Repeat("b", 64), InterfaceInstanceID: "iface-a",
		Purpose:             capmodel.BusinessExtensionInvocationPurposeNewBusiness,
		AllowedResultFields: []string{"state"}, Handler: handler,
	}}
}

func testInvocation(contextID string) Invocation {
	return Invocation{
		OwnerPluginID: "idc-resource", ProviderPluginID: "zjmfcloud", Point: "product-fulfillment",
		ContractVersion: "v1", ProviderMethod: "server.provision", ReleaseVersion: "1.0.0",
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
		Purpose: capmodel.BusinessExtensionInvocationPurposeNewBusiness, ContextID: contextID,
	}
}

func testSafeHandler(context.Context, *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
	return &pluginhost.ConfidentialOutboundResult{Payload: json.RawMessage(`{"state":"running"}`)}, nil
}
