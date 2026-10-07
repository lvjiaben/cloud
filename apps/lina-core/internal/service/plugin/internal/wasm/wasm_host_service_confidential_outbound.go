// This file dispatches one-use, host-managed supplier operations for exact
// dynamic business-extension invocations without exposing raw network material.

package wasm

import (
	"context"
	"encoding/json"
	"strings"

	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	bridgehostcall "lina-core/pkg/plugin/pluginbridge/protocol"
	bridgehostservice "lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-core/pkg/plugin/pluginhost"
)

func dispatchConfidentialoutboundHostService(
	ctx context.Context,
	hcc *hostCallContext,
	_ string,
	method string,
	payload []byte,
) *bridgehostcall.HostCallResponseEnvelope {
	if method != bridgehostservice.HostServiceMethodConfidentialOutboundExecute {
		return bridgehostcall.NewHostCallErrorResponse(bridgehostcall.HostCallStatusNotFound, "unsupported confidential outbound method")
	}
	var wireRequest struct {
		ContextID string          `json:"contextId"`
		Sequence  uint8           `json:"sequence"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := decodeCapabilityJSONRequest(payload, &wireRequest); err != nil {
		return hostCallErrorFromError(bridgehostcall.HostCallStatusInvalidRequest, err)
	}
	contextID := strings.TrimSpace(wireRequest.ContextID)
	if hcc == nil || hcc.runtime == nil || hcc.runtime.confidentialOutbound == nil ||
		hcc.executionSource != bridgecontract.ExecutionSourceBusinessExtension ||
		hcc.confidentialOutboundInvocation == nil ||
		strings.TrimSpace(contextID) == "" || hcc.confidentialOutboundInvocation.ContextID != contextID {
		return bridgehostcall.NewHostCallErrorResponse(bridgehostcall.HostCallStatusCapabilityDenied, "confidential outbound context is not authorized")
	}
	if !hcc.hasHostServiceAccess(bridgehostservice.HostServiceConfidentialOutbound, method, "", "") {
		return bridgehostcall.NewHostCallErrorResponse(bridgehostcall.HostCallStatusCapabilityDenied, "confidential outbound host service is not authorized")
	}
	request := &pluginhost.ConfidentialOutboundRequest{Sequence: wireRequest.Sequence, Payload: wireRequest.Payload}
	result, err := hcc.runtime.confidentialOutbound.Execute(ctx, *hcc.confidentialOutboundInvocation, request)
	if err != nil {
		return bridgehostcall.NewHostCallErrorResponse(bridgehostcall.HostCallStatusCapabilityDenied, "confidential outbound operation is unavailable")
	}
	return capabilityJSONResponse(result)
}
