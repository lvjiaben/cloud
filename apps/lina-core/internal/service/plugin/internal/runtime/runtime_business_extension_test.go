// This file verifies internal business-extension execution reuses the isolated
// WASM bridge while withholding host identity and public-route exposure.

package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/wasm"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	bridgeprotocol "lina-core/pkg/plugin/pluginbridge/protocol"
)

type businessExtensionWasmRuntime struct {
	input   wasm.ExecutionInput
	request *bridgecontract.BridgeRequestEnvelopeV1
}

func (r *businessExtensionWasmRuntime) ExecuteBridge(_ context.Context, input wasm.ExecutionInput, content []byte) (*bridgecontract.BridgeResponseEnvelopeV1, error) {
	request, err := bridgeprotocol.DecodeRequestEnvelope(content)
	if err != nil {
		return nil, err
	}
	r.input = input
	r.request = request
	return bridgeprotocol.NewJSONResponse(200, []byte(`{"version":"v1","payload":{"items":[]}}`)), nil
}

func (*businessExtensionWasmRuntime) InvalidateCache(context.Context, string) {}
func (*businessExtensionWasmRuntime) InvalidateAllCache(context.Context)      {}

func TestExecuteBusinessExtensionMethodUsesIsolatedInternalBridge(t *testing.T) {
	fakeRuntime := &businessExtensionWasmRuntime{}
	service := &serviceImpl{wasmRuntime: fakeRuntime}
	manifest := &catalog.Manifest{
		ID: "ssl-provider", Version: "v1.0.0",
		BridgeSpec:      &bridgecontract.BridgeSpec{RuntimeKind: bridgecontract.RuntimeKindWasm, RouteExecution: true},
		RuntimeArtifact: &catalog.ArtifactSpec{Path: "ssl-provider.wasm", Checksum: "digest"},
	}
	requestBody := []byte(`{"version":"v1","ownerPluginId":"idc-core","providerPluginId":"ssl-provider","point":"product-fulfillment","contractVersion":"v1","method":"configuration.candidates.list","releaseVersion":"v1.0.0","releaseDigest":"digest","declarationDigest":"declaration","purpose":"new-business","runtimeContext":{"contextId":"ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E"},"payload":{"field":"region"}}`)
	response, err := service.ExecuteBusinessExtensionMethod(context.Background(), manifest, requestBody)
	if err != nil || response == nil || response.StatusCode != 200 {
		t.Fatalf("execute isolated provider bridge: response=%#v err=%v", response, err)
	}
	if fakeRuntime.input.ExecutionSource != bridgecontract.ExecutionSourceBusinessExtension ||
		fakeRuntime.input.RoutePath != businessExtensionInternalPath {
		t.Fatalf("expected governed internal execution source, got %#v", fakeRuntime.input)
	}
	if fakeRuntime.input.Identity != nil || fakeRuntime.request.Identity != nil {
		t.Fatal("business extension invocation must not expose a host identity or token snapshot")
	}
	if fakeRuntime.request.Route == nil || fakeRuntime.request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType ||
		fakeRuntime.request.Route.PublicPath != "" {
		t.Fatalf("expected fixed non-public guest request type, got %#v", fakeRuntime.request.Route)
	}
	decoded := &capmodel.BusinessExtensionBridgeRequest{}
	if err = json.Unmarshal(fakeRuntime.request.Request.Body, decoded); err != nil ||
		decoded.OwnerPluginID != "idc-core" || decoded.Method != "configuration.candidates.list" ||
		decoded.RuntimeContext == nil || decoded.RuntimeContext.ContextID != "ctx_7gSQ8cKYbF2wVh9pR4mX6nJ3dT5zQa1E" {
		t.Fatalf("expected exact versioned provider request body, decoded=%#v err=%v", decoded, err)
	}
}
