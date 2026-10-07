// This file binds the isolated customer-page detail ticket to one exact public
// server path. The resource owner still rechecks customer ownership on execution.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
)

// ServerDetailReq cannot redirect its signed path through a query parameter.
type ServerDetailReq struct {
	g.Meta   `path:"/account/plugin-servers/{serverNo}" method:"get" tags:"IDC Account Servers" summary:"Get my server" dc:"Return one server owned by the current authenticated customer" pageBridgeCapability:"customer-servers.read" pageBridgeHost:"customer"`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number" eg:"SVR-0123456789ABCDEF0123456789ABCDEF"`
}

// ServerDetailRes contains only the existing safe customer projection.
type ServerDetailRes struct {
	Item *accountv1.ServerItem `json:"item" dc:"Customer-owned server detail"`
}
