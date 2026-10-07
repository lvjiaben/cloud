// This file routes governed business-extension calls into active dynamic WASM
// releases without exposing a public HTTP endpoint or host identity snapshot.

package runtime

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/confidentialoutbound"
	"lina-core/internal/service/plugin/internal/wasm"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	bridgeprotocol "lina-core/pkg/plugin/pluginbridge/protocol"
)

const businessExtensionInternalPath = "/__lina/business-extension/invoke"

// ExecuteBusinessExtensionMethod runs one internal provider call through the
// active artifact's ordinary bridge executor and host-service isolation.
func (s *serviceImpl) ExecuteBusinessExtensionMethod(
	ctx context.Context,
	manifest *catalog.Manifest,
	requestBody []byte,
) (*bridgecontract.BridgeResponseEnvelopeV1, error) {
	if s == nil || s.wasmRuntime == nil {
		return nil, gerror.New("dynamic wasm runtime is not configured")
	}
	if manifest == nil || manifest.RuntimeArtifact == nil || manifest.BridgeSpec == nil {
		return nil, gerror.New("dynamic business extension manifest is incomplete")
	}
	if !manifest.BridgeSpec.RouteExecution || manifest.BridgeSpec.RuntimeKind != bridgecontract.RuntimeKindWasm {
		return nil, gerror.New("dynamic business extension runtime is not executable")
	}
	invocation := &capmodel.BusinessExtensionBridgeRequest{}
	if err := json.Unmarshal(requestBody, invocation); err != nil {
		return nil, gerror.Wrap(err, "decode confidential outbound invocation binding failed")
	}
	request := &bridgecontract.BridgeRequestEnvelopeV1{
		PluginID: manifest.ID,
		Route: &bridgecontract.RouteMatchSnapshotV1{
			Method:       http.MethodPost,
			InternalPath: businessExtensionInternalPath,
			RoutePath:    businessExtensionInternalPath,
			RequestType:  capmodel.BusinessExtensionBridgeRequestType,
		},
		Request: &bridgecontract.HTTPRequestSnapshotV1{
			Method:       http.MethodPost,
			InternalPath: businessExtensionInternalPath,
			ContentType:  "application/json",
			Body:         append([]byte(nil), requestBody...),
		},
	}
	requestContent, err := bridgeprotocol.EncodeRequestEnvelope(request)
	if err != nil {
		return nil, err
	}
	return s.wasmRuntime.ExecuteBridge(ctx, wasm.ExecutionInput{
		PluginID:                       manifest.ID,
		ArtifactPath:                   manifest.RuntimeArtifact.Path,
		BridgeSpec:                     manifest.BridgeSpec,
		Capabilities:                   manifest.HostCapabilities,
		HostServices:                   manifest.HostServices,
		ArtifactDefaultConfig:          buildArtifactDefaultConfig(manifest),
		ArtifactManifestResources:      buildArtifactManifestResources(manifest),
		ExecutionSource:                bridgecontract.ExecutionSourceBusinessExtension,
		RoutePath:                      businessExtensionInternalPath,
		RequestID:                      request.RequestID,
		ConfidentialOutboundInvocation: confidentialOutboundInvocationFromBridge(invocation),
	}, requestContent)
}

func confidentialOutboundInvocationFromBridge(input *capmodel.BusinessExtensionBridgeRequest) *confidentialoutbound.Invocation {
	if input == nil || input.RuntimeContext == nil {
		return nil
	}
	return &confidentialoutbound.Invocation{
		OwnerPluginID: input.OwnerPluginID, ProviderPluginID: input.ProviderPluginID,
		Point: input.Point, ContractVersion: input.ContractVersion, ProviderMethod: input.Method,
		ReleaseVersion: input.ReleaseVersion, ReleaseDigest: input.ReleaseDigest,
		DeclarationDigest: input.DeclarationDigest, Purpose: input.Purpose,
		ContextID: input.RuntimeContext.ContextID,
	}
}
