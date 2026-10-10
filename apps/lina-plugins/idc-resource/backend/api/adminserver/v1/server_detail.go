// This file defines one fixed-provider staff server detail read.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
)

type CloudServerDetailReq struct {
	g.Meta   `path:"/admin/resource/zjmfcloud-servers/{serverNo}" method:"get" tags:"IDC Server Directory" summary:"Get a ZJMFCloud server" dc:"Recheck direct staff access and the fixed connector before returning a safe local projection" permission:"idc:admin:resource:server-batch:view" pageBridgeCapability:"zjmfcloud-servers.admin.v1" pageBridgeHost:"admin"`
	ServerNo string `json:"serverNo" in:"path" v:"required|regex:^SVR-[0-9A-F]{32}$" dc:"Public server number selected from the authorized list"`
}
type CloudServerDetailRes struct {
	Item *accountv1.ServerItem `json:"item" dc:"One non-secret local server projection"`
}
