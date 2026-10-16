package public

import (
	"lina-plugin-idc-portal/backend/api/public"
	"lina-plugin-idc-portal/backend/internal/service/customerconfiguration"
	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
	"lina-plugin-idc-portal/backend/internal/service/siteconfiguration"
)

type ControllerV1 struct {
	externalMenus *externalmenu.Service
	sites         siteconfiguration.Service
	customers     customerconfiguration.Service
}

func NewV1(externalMenus *externalmenu.Service, sites siteconfiguration.Service, customers customerconfiguration.Service) public.IPublicV1 {
	return &ControllerV1{externalMenus: externalMenus, sites: sites, customers: customers}
}
