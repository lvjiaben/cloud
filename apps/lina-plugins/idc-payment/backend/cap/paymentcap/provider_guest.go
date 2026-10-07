package paymentcap

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
)

// NewDynamicProviderGuestHandler returns the complete guest boundary needed by
// a declarative payment-method plugin. Operational payment and refund truth
// remains inside idc-payment; this method only confirms the frozen capability
// descriptor selected by the owner.
func NewDynamicProviderGuestHandler(pluginID string) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if request == nil || request.Route == nil || request.Request == nil ||
			request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("Payment method invocation was not found"), nil
		}
		decoder := json.NewDecoder(bytes.NewReader(request.Request.Body))
		decoder.DisallowUnknownFields()
		var invocation capmodel.BusinessExtensionBridgeRequest
		if err := decoder.Decode(&invocation); err != nil {
			return protocol.NewBadRequestResponse("Payment method invocation is invalid"), nil
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF || !validDynamicProviderInvocation(&invocation, pluginID) {
			return protocol.NewBadRequestResponse("Payment method invocation is invalid"), nil
		}
		payload, _ := json.Marshal(map[string]string{"version": DynamicProviderContractVersion})
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{
			Version: capmodel.BusinessExtensionInvocationProtocolV1,
			Payload: payload,
		})
		if err != nil {
			return protocol.NewInternalErrorResponse("Payment method result could not be encoded"), nil
		}
		return protocol.NewJSONResponse(200, body), nil
	}
}

func validDynamicProviderInvocation(invocation *capmodel.BusinessExtensionBridgeRequest, pluginID string) bool {
	if invocation == nil || invocation.Version != capmodel.BusinessExtensionInvocationProtocolV1 ||
		invocation.OwnerPluginID != OwnerPluginID || invocation.ProviderPluginID != strings.TrimSpace(pluginID) ||
		invocation.Point != DynamicProviderPoint || invocation.ContractVersion != DynamicProviderContractVersion ||
		invocation.Method != DynamicProviderMethodDescribe || invocation.RuntimeContext == nil ||
		strings.TrimSpace(invocation.RuntimeContext.ContextID) == "" || strings.TrimSpace(invocation.ReleaseVersion) == "" ||
		len(invocation.ReleaseDigest) != 64 || len(invocation.DeclarationDigest) != 64 {
		return false
	}
	return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness ||
		invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
}
