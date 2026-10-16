// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package backend

import (
	"bytes"
	"encoding/json"
	"github.com/gogf/gf/v2/errors/gerror"
	"io"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/renewalownercap"
	"lina-plugin-idc-peer/backend/internal/service/peer"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const pluginID = "idc-peer"

// NewGuestHandler validates startup dependencies before exposing the connector entry point.
func NewGuestHandler(executor confidentialoutboundcap.Service) (pluginbridge.GuestHandler, error) {
	if executor == nil {
		return nil, gerror.New("confidential outbound executor is required")
	}
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if request == nil || request.Route == nil || request.Request == nil || request.Route.RequestType != capmodel.BusinessExtensionBridgeRequestType {
			return protocol.NewNotFoundResponse("Connector invocation not found"), nil
		}
		var invocation capmodel.BusinessExtensionBridgeRequest
		decoder := json.NewDecoder(bytes.NewReader(request.Request.Body))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&invocation) != nil || decoder.Decode(new(any)) != io.EOF || !validInvocation(&invocation) {
			return protocol.NewBadRequestResponse("Connector invocation is invalid"), nil
		}
		result, err := dispatch(executor, &invocation)
		if err != nil {
			return protocol.NewInternalErrorResponse("Upstream operation could not be completed"), nil
		}
		body, err := json.Marshal(capmodel.BusinessExtensionBridgeResponse{Version: capmodel.BusinessExtensionInvocationProtocolV1, Payload: result})
		if err != nil {
			return protocol.NewInternalErrorResponse("Connector response is invalid"), nil
		}
		return protocol.NewJSONResponse(200, body), nil
	}, nil
}

func dispatch(executor confidentialoutboundcap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	if invocation.Point == resourcecap.DynamicProviderPoint {
		var payload json.RawMessage
		if _, err := resourcecap.DecodeDynamicProviderInvocationEnvelope(invocation.Payload, &payload); err != nil {
			return nil, err
		}
		return peer.ExecuteResource(executor, invocation.RuntimeContext.ContextID, invocation.Method, payload)
	}
	return dispatchFulfillment(executor, invocation)
}

func dispatchFulfillment(executor confidentialoutboundcap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	if invocation.Method == "peer.configuration.candidates" {
		return peer.ReadImportCandidates(executor, invocation.RuntimeContext.ContextID, invocation.Payload)
	}
	if invocation.Method == renewalownercap.PeerRenewMethod || invocation.Method == renewalownercap.PeerRenewQueryMethod {
		return dispatchRenewal(executor, invocation)
	}
	var input struct {
		productfulfillmentcap.Request
		PeerPurchase *channelcommercecap.Purchase `json:"peerPurchase"`
	}
	if json.Unmarshal(invocation.Payload, &input) != nil {
		return nil, gerror.New("invalid fulfillment payload")
	}
	if err := productfulfillmentcap.ValidateRequest(&input.Request); err != nil {
		return nil, err
	}
	var output *productfulfillmentcap.Result
	var err error
	if invocation.Method == "query" {
		output, err = peer.QueryPurchase(executor, invocation.RuntimeContext.ContextID, input.IdempotencyKey)
	} else {
		if input.PeerPurchase == nil {
			return nil, gerror.New("frozen upstream purchase required")
		}
		output, err = peer.PurchaseForSchedule(executor, invocation.RuntimeContext.ContextID, input.IdempotencyKey, *input.PeerPurchase, input.Billing)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(output)
}

func validInvocation(input *capmodel.BusinessExtensionBridgeRequest) bool {
	if input == nil || input.Version != capmodel.BusinessExtensionInvocationProtocolV1 || input.ProviderPluginID != pluginID || input.RuntimeContext == nil || input.RuntimeContext.ContextID == "" || input.ReleaseVersion == "" || len(input.ReleaseDigest) != 64 || len(input.DeclarationDigest) != 64 || input.ContractVersion != "v1" {
		return false
	}
	if input.Point == capmodel.BusinessExtensionProductFulfillment {
		if input.Method == "peer.configuration.candidates" {
			return input.OwnerPluginID == "idc-core" && input.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness
		}
		return input.OwnerPluginID == "idc-core" && input.Purpose == capmodel.BusinessExtensionInvocationPurposeExistingBusiness && (input.Method == "provision" || input.Method == "query" || input.Method == renewalownercap.PeerRenewMethod || input.Method == renewalownercap.PeerRenewQueryMethod)
	}
	if input.Point != resourcecap.DynamicProviderPoint || input.OwnerPluginID != resourcecap.OwnerPluginID {
		return false
	}
	switch input.Method {
	case resourcecap.DynamicProviderMethodListCatalog, resourcecap.DynamicProviderMethodReadCatalogProduct, resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, resourcecap.DynamicProviderMethodCatalogBillingTerms, resourcecap.DynamicProviderMethodConfigurationCandidates:
		return input.Purpose == capmodel.BusinessExtensionInvocationPurposeNewBusiness
	default:
		if input.Purpose != capmodel.BusinessExtensionInvocationPurposeExistingBusiness {
			return false
		}
		for _, method := range resourcecap.DynamicProviderMethods() {
			if input.Method == method {
				return true
			}
		}
	}
	return false
}

// dispatchRenewal keeps the renewal envelope separate from purchase payloads.
func dispatchRenewal(executor confidentialoutboundcap.Service, invocation *capmodel.BusinessExtensionBridgeRequest) (json.RawMessage, error) {
	var request renewalownercap.PeerRequest
	if json.Unmarshal(invocation.Payload, &request) != nil || renewalownercap.ValidatePeerRequest(request) != nil {
		return nil, gerror.New("invalid renewal payload")
	}
	var result *renewalownercap.Task
	var err error
	if invocation.Method == renewalownercap.PeerRenewMethod {
		result, err = peer.Renew(executor, invocation.RuntimeContext.ContextID, request)
	} else {
		result, err = peer.QueryRenewal(executor, invocation.RuntimeContext.ContextID, request)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
