package account

import (
	accountapi "lina-plugin-idc-payment/backend/api/account"
	"lina-plugin-idc-payment/backend/internal/service/paymentflow"
)

type ControllerV1 struct {
	flow *paymentflow.Service
}

func NewV1(flow *paymentflow.Service) accountapi.IAccountV1 {
	return &ControllerV1{flow: flow}
}
