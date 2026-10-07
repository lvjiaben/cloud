package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

type ExternalMenuListReq struct {
	g.Meta `path:"/customer/external-menus" method:"get" tags:"Customer Center" summary:"List enabled customer external menus"`
}
type ExternalMenuListRes struct {
	Items []externalmenu.Item `json:"items"`
}
