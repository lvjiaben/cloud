package account

import (
	accountapi "lina-plugin-idc-resource/backend/api/account"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type ControllerV1 struct {
	servers resourcecap.CurrentCustomerServerService
}

func NewV1(servers resourcecap.CurrentCustomerServerService) accountapi.IAccountV1 {
	return &ControllerV1{servers: servers}
}
