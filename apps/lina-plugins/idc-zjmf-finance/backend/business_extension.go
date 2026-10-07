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
	"lina-plugin-idc-zjmf-finance/backend/internal/provider/finance"
)

const pluginID = "idc-zjmf-finance"

func NewGuestHandler(executor confidentialoutboundcap.Service) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if executor == nil || request == nil || request.Route == nil || request.Request == nil ||
			request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("Managed connector invocation was not found"), nil
		}
		invocation, err := decodeInvocation(request.Request.Body)
		if err != nil || !validInvocation(invocation) {
			return protocol.NewBadRequestResponse("Managed connector invocation is invalid"), nil
		}
		var payload json.RawMessage
		if _, err = resourcecap.DecodeDynamicProviderInvocationEnvelope(invocation.Payload, &payload); err != nil {
			return protocol.NewBadRequestResponse("Managed connector interface settings are unavailable"), nil
		}
		result, err := finance.ExecuteDynamic(executor, invocation.RuntimeContext.ContextID, invocation.Method, payload)
		if err != nil {
			return protocol.NewInternalErrorResponse("Supplier operation could not be completed"), nil
		}
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: result})
		if err != nil {
			return protocol.NewInternalErrorResponse("Managed connector result could not be encoded"), nil
		}
		return protocol.NewJSONResponse(200, body), nil
	}
}

func decodeInvocation(raw []byte) (*capmodel.BusinessExtensionBridgeRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var invocation capmodel.BusinessExtensionBridgeRequest
	if decoder.Decode(&invocation) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	return &invocation, nil
}

func validInvocation(invocation *capmodel.BusinessExtensionBridgeRequest) bool {
	if invocation == nil || invocation.Version != capmodel.BusinessExtensionInvocationProtocolV1 ||
		invocation.ProviderPluginID != pluginID || invocation.RuntimeContext == nil ||
		strings.TrimSpace(invocation.RuntimeContext.ContextID) == "" || strings.TrimSpace(invocation.ReleaseVersion) == "" ||
		len(invocation.ReleaseDigest) != 64 || len(invocation.DeclarationDigest) != 64 {
		return false
	}
	if invocation.Point != resourcecap.DynamicProviderPoint || invocation.OwnerPluginID != resourcecap.OwnerPluginID ||
		invocation.ContractVersion != resourcecap.DynamicProviderContractVersion {
		return false
	}
	if invocation.Method == resourcecap.DynamicProviderMethodConfigurationCandidates ||
		invocation.Method == resourcecap.DynamicProviderMethodListCatalog ||
		invocation.Method == resourcecap.DynamicProviderMethodReadCatalogProduct {
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness
	}
	if invocation.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
		return false
	}
	for _, method := range resourcecap.DynamicProviderMethods() {
		if invocation.Method == method {
			return true
		}
	}
	return false
}
