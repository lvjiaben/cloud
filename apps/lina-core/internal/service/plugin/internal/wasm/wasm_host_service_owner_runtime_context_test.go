// This file verifies owner host-service calls can use only the exact
// short-lived runtime context bound to an internal business-extension call.

package wasm

import (
	"context"
	"testing"

	"lina-core/internal/service/plugin/internal/confidentialoutbound"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/capregistry"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginbridge/protocol"
)

const (
	runtimeContextOwner   = "idc-core"
	runtimeContextService = "supplier-interface-runtime"
	runtimeContextVersion = "v1"
	runtimeContextMethod  = "outbound.http.execute"
	runtimeContextID      = "src_0123456789abcdef0123456789abcdef"
	runtimeProviderID     = "idc-zjmfcloud"
)

type runtimeContextOwnerInvoker struct {
	calls      int
	invocation capregistry.Invocation
}

func (i *runtimeContextOwnerInvoker) Invoke(
	_ context.Context,
	invocation capregistry.Invocation,
) (*capregistry.InvocationResult, error) {
	i.calls++
	i.invocation = invocation
	return &capregistry.InvocationResult{Payload: []byte(`{"status":"accepted"}`)}, nil
}

func TestOwnerHostServiceAllowsExactBusinessExtensionRuntimeContext(t *testing.T) {
	hcc, invoker := newRuntimeContextHostCallContext(t)
	response := dispatchOwnerHostService(context.Background(), hcc, runtimeContextOwnerRequest())

	if response.Status != protocol.HostCallStatusSuccess {
		t.Fatalf("exact business-extension runtime context was rejected: status=%d payload=%s", response.Status, response.Payload)
	}
	if invoker.calls != 1 {
		t.Fatalf("owner invoker calls=%d, want 1", invoker.calls)
	}
	if invoker.invocation.CallerPluginID != runtimeProviderID ||
		invoker.invocation.OwnerPluginID != runtimeContextOwner ||
		invoker.invocation.Service != runtimeContextService ||
		invoker.invocation.Version != runtimeContextVersion ||
		invoker.invocation.Method != runtimeContextMethod ||
		invoker.invocation.ResourceRef != runtimeContextID ||
		invoker.invocation.Table != "" {
		t.Fatalf("owner invoker received an inexact runtime binding: %#v", invoker.invocation)
	}
}

func TestOwnerHostServiceRejectsInexactBusinessExtensionRuntimeContext(t *testing.T) {
	tests := map[string]func(*hostCallContext, *protocol.HostServiceRequestEnvelope){
		"wrong context": func(_ *hostCallContext, request *protocol.HostServiceRequestEnvelope) {
			request.ResourceRef = "src_fedcba9876543210fedcba9876543210"
		},
		"wrong plugin": func(hcc *hostCallContext, _ *protocol.HostServiceRequestEnvelope) {
			hcc.pluginID = "other-provider"
		},
		"wrong owner": func(_ *hostCallContext, request *protocol.HostServiceRequestEnvelope) {
			request.Owner = "other-core"
		},
		"wrong method": func(_ *hostCallContext, request *protocol.HostServiceRequestEnvelope) {
			request.Method = "outbound.http.delete"
		},
		"ordinary route": func(hcc *hostCallContext, _ *protocol.HostServiceRequestEnvelope) {
			hcc.executionSource = bridgecontract.ExecutionSourceRoute
		},
		"table scoped": func(_ *hostCallContext, request *protocol.HostServiceRequestEnvelope) {
			request.Table = "private_supplier_state"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			hcc, invoker := newRuntimeContextHostCallContext(t)
			request := runtimeContextOwnerRequest()
			mutate(hcc, request)

			response := dispatchOwnerHostService(context.Background(), hcc, request)
			if response.Status == protocol.HostCallStatusSuccess {
				t.Fatalf("inexact runtime context was accepted: payload=%s", response.Payload)
			}
			if invoker.calls != 0 {
				t.Fatalf("inexact runtime context reached owner invoker: calls=%d invocation=%#v", invoker.calls, invoker.invocation)
			}
		})
	}
}

func newRuntimeContextHostCallContext(t *testing.T) (*hostCallContext, *runtimeContextOwnerInvoker) {
	t.Helper()
	invoker := &runtimeContextOwnerInvoker{}
	registry := capregistry.NewRegistry()
	if err := registry.Register(capregistry.Descriptor{
		OwnerPluginID:   runtimeContextOwner,
		Service:         runtimeContextService,
		Version:         runtimeContextVersion,
		SourceContract:  "lina-plugin-idc-core/backend/cap/supplierinterfacecap",
		DynamicContract: "lina-plugin-idc-core/backend/cap/supplierinterfacecap/bridge",
		Invoker:         invoker,
		Methods: []capregistry.MethodDescriptor{{
			Method:       runtimeContextMethod,
			Capability:   "host:idc:supplier-interface.outbound.http.execute",
			Risk:         capregistry.RiskLevelExecute,
			ResourceKind: capregistry.ResourceKindRef,
		}},
	}); err != nil {
		t.Fatalf("register owner runtime capability: %v", err)
	}

	spec := &protocol.HostServiceSpec{
		Owner: runtimeContextOwner, Service: runtimeContextService, Version: runtimeContextVersion,
		Methods: []string{runtimeContextMethod},
	}
	hcc := &hostCallContext{
		runtime:         newTestHostServiceRuntime(withTestOwnerCapabilities(registry)),
		pluginID:        runtimeProviderID,
		hostServices:    []*protocol.HostServiceSpec{spec},
		executionSource: bridgecontract.ExecutionSourceBusinessExtension,
		confidentialOutboundInvocation: &confidentialoutbound.Invocation{
			OwnerPluginID: runtimeContextOwner, ProviderPluginID: runtimeProviderID,
			Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1",
			ProviderMethod: "provision", Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness,
			ContextID: runtimeContextID,
		},
	}
	return hcc, invoker
}

func runtimeContextOwnerRequest() *protocol.HostServiceRequestEnvelope {
	return &protocol.HostServiceRequestEnvelope{
		Owner: runtimeContextOwner, Service: runtimeContextService, Version: runtimeContextVersion,
		Method: runtimeContextMethod, ResourceRef: runtimeContextID, Payload: []byte(`{"path":"/clouds"}`),
	}
}
