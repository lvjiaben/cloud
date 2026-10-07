package backend

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-v10/backend/internal/v10"
)

const pluginID = "idc-v10"

func NewGuestHandler(executor confidentialoutboundcap.Service) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if executor == nil || request == nil || request.Route == nil || request.Request == nil || request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("V10 connector invocation was not found"), nil
		}
		var invocation capmodel.BusinessExtensionBridgeRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Request.Body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&invocation) != nil || decoder.Decode(&struct{}{}) != io.EOF || !validInvocation(&invocation) {
			return protocol.NewBadRequestResponse("V10 connector invocation is invalid"), nil
		}
		var payload json.RawMessage
		if _, err := resourcecap.DecodeDynamicProviderInvocationEnvelope(invocation.Payload, &payload); err != nil {
			return protocol.NewBadRequestResponse("V10 connector envelope is invalid"), nil
		}
		result, err := v10.ExecuteDynamic(executor, invocation.RuntimeContext.ContextID, invocation.Method, payload)
		if err != nil {
			return protocol.NewInternalErrorResponse("V10 supplier operation could not be completed"), nil
		}
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: result})
		if err != nil {
			return protocol.NewInternalErrorResponse("V10 connector result could not be encoded"), nil
		}
		return protocol.NewJSONResponse(200, body), nil
	}
}

func validInvocation(value *capmodel.BusinessExtensionBridgeRequest) bool {
	if value == nil || value.Version != capmodel.BusinessExtensionInvocationProtocolV1 || value.ProviderPluginID != pluginID ||
		value.OwnerPluginID != resourcecap.OwnerPluginID || value.Point != resourcecap.DynamicProviderPoint ||
		value.ContractVersion != resourcecap.DynamicProviderContractVersion || value.RuntimeContext == nil ||
		strings.TrimSpace(value.RuntimeContext.ContextID) == "" || strings.TrimSpace(value.ReleaseVersion) == "" ||
		len(value.ReleaseDigest) != 64 || len(value.DeclarationDigest) != 64 {
		return false
	}
	if value.Method == resourcecap.DynamicProviderMethodConfigurationCandidates ||
		value.Method == resourcecap.DynamicProviderMethodListCatalog ||
		value.Method == resourcecap.DynamicProviderMethodReadCatalogProduct {
		return value.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness
	}
	if value.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
		return false
	}
	for _, method := range resourcecap.DynamicProviderMethods() {
		if value.Method == method {
			return true
		}
	}
	return false
}
