// This file validates governed extension invocations and keeps every product
// extension method away from supplier outbound access. Only idc-resource may
// invoke the confidential resource-provider extension.

package backend

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	pluginID                 = "idc-zjmfcloud"
	fulfillmentOwnerPluginID = "idc-core"
	candidateMethod          = "configuration.candidates"
	provisionMethod          = "provision"
	queryMethod              = "query"
)

var errResourceOwnerRequired = errors.New("server fulfillment must be routed through the resource owner")

// NewBusinessExtensionHandler creates the guest dispatcher. Product-point
// declarations are authorization evidence only and can never cross outbound.
func NewBusinessExtensionHandler(resourceExecutor confidentialoutboundcap.Service) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if resourceExecutor == nil || request == nil || request.Route == nil || request.Request == nil ||
			request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("Managed connector invocation was not found"), nil
		}
		invocation, err := decodeBusinessExtensionInvocation(request.Request.Body)
		if err != nil || !validInvocationEvidence(invocation) {
			return protocol.NewBadRequestResponse("Managed connector invocation is invalid"), nil
		}
		result, err := executeInvocation(resourceExecutor, invocation)
		if err != nil {
			if errors.Is(err, errResourceOwnerRequired) {
				return protocol.NewBadRequestResponse("Server fulfillment must be routed through the resource owner"), nil
			}
			return protocol.NewInternalErrorResponse("Supplier operation could not be completed"), nil
		}
		responseBody, err := json.Marshal(&capmodel.BusinessExtensionBridgeResponse{
			Version: capmodel.BusinessExtensionInvocationProtocolV1,
			Payload: result,
		})
		if err != nil {
			return protocol.NewInternalErrorResponse("Managed connector result could not be encoded"), nil
		}
		return protocol.NewJSONResponse(200, responseBody), nil
	}
}

func decodeBusinessExtensionInvocation(content []byte) (*capmodel.BusinessExtensionBridgeRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var invocation capmodel.BusinessExtensionBridgeRequest
	if err := decoder.Decode(&invocation); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, err
	}
	return &invocation, nil
}

func validInvocationEvidence(invocation *capmodel.BusinessExtensionBridgeRequest) bool {
	if invocation == nil || invocation.Version != capmodel.BusinessExtensionInvocationProtocolV1 ||
		invocation.ProviderPluginID != pluginID || invocation.RuntimeContext == nil ||
		strings.TrimSpace(invocation.RuntimeContext.ContextID) == "" ||
		strings.TrimSpace(invocation.ReleaseVersion) == "" || len(invocation.ReleaseDigest) != 64 ||
		len(invocation.DeclarationDigest) != 64 {
		return false
	}
	if invocation.Point == resourcecap.DynamicProviderPoint {
		return validResourceProviderInvocation(invocation)
	}
	if invocation.OwnerPluginID != fulfillmentOwnerPluginID ||
		invocation.Point != capmodel.BusinessExtensionProductFulfillment ||
		invocation.ContractVersion != productfulfillmentcap.ContractVersion {
		return false
	}
	switch invocation.Method {
	case candidateMethod, productfulfillmentcap.MethodValidateProductConfiguration:
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness ||
			invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	case provisionMethod, queryMethod:
		return invocation.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness
	default:
		return false
	}
}

func executeInvocation(resourceExecutor confidentialoutboundcap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	if invocation.Point == resourcecap.DynamicProviderPoint {
		var payload json.RawMessage
		if _, err := resourcecap.DecodeDynamicProviderInvocationEnvelope(invocation.Payload, &payload); err != nil {
			return nil, err
		}
		invocation.Payload = payload
		return executeResourceProviderInvocation(resourceExecutor, invocation)
	}
	if invocation.Method == productfulfillmentcap.MethodValidateProductConfiguration {
		var request productfulfillmentcap.ConfigurationValidationRequest
		decoder := json.NewDecoder(bytes.NewReader(invocation.Payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || request.Version != productfulfillmentcap.ContractVersion {
			return nil, io.ErrUnexpectedEOF
		}
		_, validationErr := resourcecap.ParseServerGenerationPolicy(request.ProductConfiguration)
		return json.Marshal(productfulfillmentcap.ConfigurationValidationResult{Version: productfulfillmentcap.ContractVersion, Valid: validationErr == nil})
	}
	return nil, errResourceOwnerRequired
}
