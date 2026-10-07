// This file verifies source plugins can register one executable governed
// business-extension dispatcher and cannot replace it silently.

package pluginhost

import (
	"context"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestSourcePluginRegistersBusinessExtensionMethodHandler(t *testing.T) {
	declarations := NewDeclarations("source-provider-test")
	handler := func(ctx context.Context, invocation *capmodel.BusinessExtensionMethodInvocation) (*capmodel.BusinessExtensionMethodResult, error) {
		if caller, ok := SourceBusinessExtensionCallerPluginID(ctx); !ok || caller != "source-provider-test" {
			t.Fatalf("handler did not receive host-bound source identity: caller=%q ok=%v", caller, ok)
		}
		return &capmodel.BusinessExtensionMethodResult{Version: invocation.Version, Payload: []byte(`{"items":[]}`)}, nil
	}
	if err := declarations.Providers().ProvideBusinessExtensionMethods(handler); err != nil {
		t.Fatalf("register source provider method handler: %v", err)
	}
	definition := declarations.(SourcePluginDefinition)
	result, err := definition.GetBusinessExtensionMethodHandler()(context.Background(), &capmodel.BusinessExtensionMethodInvocation{
		Version: capmodel.BusinessExtensionInvocationProtocolV1,
	})
	if err != nil || result == nil || result.Version != capmodel.BusinessExtensionInvocationProtocolV1 {
		t.Fatalf("execute registered source provider handler: result=%#v err=%v", result, err)
	}
	if err = declarations.Providers().ProvideBusinessExtensionMethods(handler); err == nil {
		t.Fatal("expected duplicate source provider method handler registration to fail")
	}
}

func TestSourcePluginBusinessExtensionMethodHandlerRecoversPanic(t *testing.T) {
	declarations := NewDeclarations("source-provider-panic")
	if err := declarations.Providers().ProvideBusinessExtensionMethods(func(context.Context, *capmodel.BusinessExtensionMethodInvocation) (*capmodel.BusinessExtensionMethodResult, error) {
		panic("sensitive provider detail")
	}); err != nil {
		t.Fatalf("register source provider method handler: %v", err)
	}
	result, err := declarations.(SourcePluginDefinition).GetBusinessExtensionMethodHandler()(context.Background(), &capmodel.BusinessExtensionMethodInvocation{})
	if result != nil || err == nil {
		t.Fatalf("panic was not converted into a closed failure: result=%#v err=%v", result, err)
	}
	if strings.Contains(err.Error(), "sensitive provider detail") {
		t.Fatalf("panic detail leaked through the host boundary: %v", err)
	}
}

func TestSourceBusinessExtensionCallerIdentityIsUnavailableOutsideWrapper(t *testing.T) {
	if caller, ok := SourceBusinessExtensionCallerPluginID(context.Background()); ok || caller != "" {
		t.Fatalf("ordinary context exposed a source identity: caller=%q ok=%v", caller, ok)
	}
}
