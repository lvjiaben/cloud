package public

import (
	"lina-plugin-idc-portal/backend/api/public"
	"lina-plugin-idc-portal/backend/internal/service/brand"
	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

type ControllerV1 struct {
	brands        *brand.Service
	externalMenus *externalmenu.Service
}

func NewV1(brands *brand.Service, externalMenus *externalmenu.Service) public.IPublicV1 {
	return &ControllerV1{brands: brands, externalMenus: externalMenus}
}
