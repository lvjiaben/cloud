// This file composes business-extension and customer-page requests on the WASM guest boundary.

package backend

import (
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-core/pkg/plugin/pluginbridge"
	"lina-core/pkg/plugin/pluginbridge/protocol"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	customerapi "lina-plugin-idc-zjmfcloud/backend/api/customer/v1"
	customercontroller "lina-plugin-idc-zjmfcloud/backend/internal/controller/customer"
)

// NewGuestHandler injects startup-owned host-service clients into one guest dispatcher.
func NewGuestHandler(resourceExecutor confidentialoutboundcap.Service, servers resourcecap.CurrentCustomerServerService) pluginbridge.GuestHandler {
	businessHandler := NewBusinessExtensionHandler(resourceExecutor)
	customerHandler := newCustomerPageHandler(customercontroller.NewWithService(servers))
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if request != nil && request.Route != nil && request.Route.RequestType == capmodel.BusinessExtensionBridgeRequestType {
			return businessHandler(request)
		}
		return customerHandler(request)
	}
}

func newCustomerPageHandler(controller *customercontroller.Controller) pluginbridge.GuestHandler {
	return func(request *protocol.BridgeRequestEnvelopeV1) (*protocol.BridgeResponseEnvelopeV1, error) {
		if request == nil || request.Route == nil || controller == nil {
			return protocol.NewBadRequestResponse("Customer server request is invalid"), nil
		}
		ctx := pluginbridge.NewGuestControllerContext(request)
		switch request.Route.RequestType {
		case "ServerListReq":
			response, err := controller.ServerList(ctx, &customerapi.ServerListReq{Cursor: pluginbridge.QueryValue(request, "cursor"), Limit: pluginbridge.QueryInt(request, "limit")})
			if err != nil {
				return protocol.NewInternalErrorResponse("Customer server directory is unavailable"), nil
			}
			return pluginbridge.WriteJSON(200, response)
		case "ServerDetailReq":
			response, err := controller.ServerDetail(ctx, &customerapi.ServerDetailReq{ServerNo: pluginbridge.QueryValue(request, "serverNo")})
			if err != nil {
				return protocol.NewInternalErrorResponse("Customer server is unavailable"), nil
			}
			return pluginbridge.WriteJSON(200, response)
		default:
			return protocol.NewNotFoundResponse("Managed connector route was not found"), nil
		}
	}
}
