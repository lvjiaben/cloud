package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-portal/backend/internal/service/externalmenu"
)

type ExternalMenuListReq struct {
	g.Meta `path:"/admin/customer-external-menus" method:"get" tags:"Customer External Menu" summary:"List customer external menus" permission:"idc:admin:portal:external-menu:view"`
}
type ExternalMenuListRes struct {
	Items []externalmenu.Item `json:"items"`
}

type ExternalMenuSaveReq struct {
	g.Meta    `path:"/admin/customer-external-menus" method:"post" tags:"Customer External Menu" summary:"Create or update a customer external menu" permission:"idc:admin:portal:external-menu:manage"`
	ID        int64  `json:"id"`
	NameZh    string `json:"nameZh" v:"required|length:1,80"`
	NameEn    string `json:"nameEn" v:"required|length:1,80"`
	Icon      string `json:"icon" v:"required|length:1,64"`
	URL       string `json:"url" v:"required|length:1,2048"`
	SortOrder int    `json:"sortOrder" v:"between:0,9999"`
	Version   int    `json:"version"`
}
type ExternalMenuSaveRes struct {
	Item externalmenu.Item `json:"item"`
}

type ExternalMenuStatusReq struct {
	g.Meta  `path:"/admin/customer-external-menus/{id}/status" method:"put" tags:"Customer External Menu" summary:"Enable or disable a customer external menu" permission:"idc:admin:portal:external-menu:manage"`
	ID      int64 `json:"id" v:"required|min:1"`
	Version int   `json:"version" v:"required|min:1"`
	Enabled bool  `json:"enabled"`
}
type ExternalMenuStatusRes struct {
	Item externalmenu.Item `json:"item"`
}
