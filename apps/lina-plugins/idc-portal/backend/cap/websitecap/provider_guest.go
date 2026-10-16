// This file implements the read-only website descriptor guest contract.
package websitecap

import (
	"bytes"
	"encoding/json"
	"io"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"strings"
)

// NewProviderGuestHandler returns only a public template descriptor after exact invocation validation.
func NewProviderGuestHandler(pluginID string) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if request == nil || request.Route == nil || request.Request == nil || request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("Website description was not found"), nil
		}
		decoder := json.NewDecoder(bytes.NewReader(request.Request.Body))
		decoder.DisallowUnknownFields()
		var invocation capmodel.BusinessExtensionBridgeRequest
		if err := decoder.Decode(&invocation); err != nil {
			return protocol.NewBadRequestResponse("Website description request is invalid"), nil
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF || !validInvocation(&invocation, pluginID) {
			return protocol.NewBadRequestResponse("Website description request is invalid"), nil
		}
		payload, err := json.Marshal(struct {
			PluginID string `json:"pluginId"`
			Version  string `json:"version"`
		}{pluginID, Version})
		if err != nil {
			return protocol.NewInternalErrorResponse("Website description could not be encoded"), nil
		}
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: payload})
		if err != nil {
			return protocol.NewInternalErrorResponse("Website description could not be encoded"), nil
		}
		return protocol.NewJSONResponse(200, body), nil
	}
}

// validInvocation binds the descriptor call to host-frozen provider identity and lifecycle context.
func validInvocation(invocation *capmodel.BusinessExtensionBridgeRequest, pluginID string) bool {
	if invocation.Version != capmodel.BusinessExtensionInvocationProtocolV1 || invocation.OwnerPluginID != Owner || invocation.ProviderPluginID != pluginID || invocation.Point != Point || invocation.ContractVersion != Version || invocation.Method != MethodDescribe {
		return false
	}
	if invocation.RuntimeContext == nil || strings.TrimSpace(invocation.RuntimeContext.ContextID) == "" || strings.TrimSpace(invocation.ReleaseVersion) == "" || len(invocation.ReleaseDigest) != 64 || len(invocation.DeclarationDigest) != 64 {
		return false
	}
	return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness || invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
}
