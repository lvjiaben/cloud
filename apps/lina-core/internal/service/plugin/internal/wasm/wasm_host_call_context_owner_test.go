package wasm

import (
	"testing"

	"lina-core/internal/service/plugin/internal/confidentialoutbound"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	bridgehostservice "lina-core/pkg/plugin/pluginbridge/protocol"
)

func TestOwnerHostServiceAllowsOwnerValidatedRuntimeResource(t *testing.T) {
	t.Parallel()
	context := &hostCallContext{
		pluginID:        "idc-zjmfcloud",
		executionSource: bridgecontract.ExecutionSourceBusinessExtension,
		confidentialOutboundInvocation: &confidentialoutbound.Invocation{
			OwnerPluginID: "idc-core", ProviderPluginID: "idc-zjmfcloud",
			ContextID: "src_0123456789abcdef0123456789abcdef",
		},
		hostServices: []*bridgehostservice.HostServiceSpec{{
			Owner: "idc-core", Service: "supplier-interface-runtime", Version: "v1",
			Methods: []string{"outbound.http.execute"},
		}}}

	if !context.hasOwnerHostServiceAccess(
		"idc-core", "supplier-interface-runtime", "v1", "outbound.http.execute",
		"src_0123456789abcdef0123456789abcdef", "",
	) {
		t.Fatal("owner-validated runtime resource was rejected before owner dispatch")
	}
	if context.hasOwnerHostServiceAccess(
		"idc-core", "supplier-interface-runtime", "v1", "configuration.get",
		"src_0123456789abcdef0123456789abcdef", "",
	) {
		t.Fatal("undeclared owner method was authorized")
	}
	if context.hasOwnerHostServiceAccess(
		"idc-core", "supplier-interface-runtime", "v1", "outbound.http.execute",
		"src_ffffffffffffffffffffffffffffffff", "",
	) {
		t.Fatal("a different runtime context was authorized")
	}
	context.executionSource = bridgecontract.ExecutionSourceRoute
	if context.hasOwnerHostServiceAccess(
		"idc-core", "supplier-interface-runtime", "v1", "outbound.http.execute",
		"src_0123456789abcdef0123456789abcdef", "",
	) {
		t.Fatal("a normal route reused a business-extension runtime context")
	}
}

func TestOwnerHostServiceKeepsFixedResourceBoundary(t *testing.T) {
	t.Parallel()
	context := &hostCallContext{hostServices: []*bridgehostservice.HostServiceSpec{{
		Owner: "payment-core", Service: "payment-account", Version: "v1",
		Methods:   []string{"account.get"},
		Resources: []*bridgehostservice.HostServiceResourceSpec{{Ref: "account_allowed"}},
	}}}

	if !context.hasOwnerHostServiceAccess(
		"payment-core", "payment-account", "v1", "account.get", "account_allowed", "",
	) {
		t.Fatal("declared fixed owner resource was rejected")
	}
	if context.hasOwnerHostServiceAccess(
		"payment-core", "payment-account", "v1", "account.get", "account_other", "",
	) {
		t.Fatal("undeclared fixed owner resource was authorized")
	}
}
