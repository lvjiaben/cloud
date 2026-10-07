package admin

import (
	"lina-plugin-idc-portal/backend/api/admin"
	"lina-plugin-idc-portal/backend/internal/service/brand"
	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

type ControllerV1 struct {
	brands        *brand.Service
	externalMenus *externalmenu.Service
}

func NewV1(brands *brand.Service, externalMenus *externalmenu.Service) admin.IAdminV1 {
	return &ControllerV1{brands: brands, externalMenus: externalMenus}
}
