package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-baota/backend/internal/baota"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func NewGuestHandler(executor confidentialoutboundcap.Service) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if executor == nil || request == nil || request.Route == nil || request.Request == nil || request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("BT Panel invocation was not found"), nil
		}
		var invocation capmodel.BusinessExtensionBridgeRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Request.Body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&invocation) != nil || decoder.Decode(&struct{}{}) != io.EOF || invocation.ProviderPluginID != "idc-baota" || invocation.RuntimeContext == nil || invocation.RuntimeContext.ContextID == "" {
			return protocol.NewBadRequestResponse("BT Panel invocation is invalid"), nil
		}
		var result json.RawMessage
		var err error
		if invocation.Point == resourcecap.DynamicProviderPoint && invocation.OwnerPluginID == resourcecap.OwnerPluginID {
			var payload json.RawMessage
			if _, err = resourcecap.DecodeDynamicProviderInvocationEnvelope(invocation.Payload, &payload); err == nil {
				result, err = baota.ExecuteResource(executor, invocation.RuntimeContext.ContextID, invocation.Method, payload)
			}
		} else {
			err = errors.New("BT Panel business extension is unsupported")
		}
		if err != nil {
			return protocol.NewBadRequestResponse(err.Error()), nil
		}
		body, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: result})
		if err != nil {
			return nil, errors.New("encode BT Panel response")
		}
		return protocol.NewJSONResponse(200, body), nil
	}
}
