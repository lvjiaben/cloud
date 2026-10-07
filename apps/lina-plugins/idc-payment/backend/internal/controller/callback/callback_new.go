package callback

import (
	callbackapi "lina-plugin-idc-payment/backend/api/callback"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

type ControllerV1 struct {
	flow *paymentflow.Service
}

func NewV1(flow *paymentflow.Service) callbackapi.ICallbackV1 {
	return &ControllerV1{flow: flow}
}
