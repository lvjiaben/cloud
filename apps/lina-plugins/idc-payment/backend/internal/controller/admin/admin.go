package admin

import (
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/filecap"
	adminapi "lina-plugin-idc-payment/backend/api/admin"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
	"lina-plugin-idc-payment/backend/internal/service/exceptionfund"
	"lina-plugin-idc-payment/backend/internal/service/reconciliation"
)

type ControllerV1 struct {
	reconciliation  *reconciliation.Service
	exceptionFunds  *exceptionfund.Service
	businessContext bizctxcap.Service
	files           filecap.Service
	channels        *channelconfig.Service
}

func NewV1(service *reconciliation.Service, exceptionFunds *exceptionfund.Service, channels *channelconfig.Service, businessContext bizctxcap.Service, files filecap.Service) adminapi.IAdminV1 {
	return &ControllerV1{reconciliation: service, exceptionFunds: exceptionFunds, channels: channels, businessContext: businessContext, files: files}
}
