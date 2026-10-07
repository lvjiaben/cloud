package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-portal/backend/internal/service/brand"
)

type BrandGetReq struct {
	g.Meta `path:"/public/brand" method:"get" tags:"Website" summary:"Get published website brand"`
}
type BrandGetRes struct {
	Brand *brand.Config `json:"brand,omitempty"`
}
