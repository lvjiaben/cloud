// Package v1 contains read-only staff projections for the ZJMFCloud surface.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
)

type CloudServerListReq struct {
	g.Meta `path:"/admin/resource/zjmfcloud-servers" method:"get" tags:"IDC Server Directory" summary:"List ZJMFCloud servers" dc:"Return one bounded staff-authorized page of local ZJMFCloud projections without credentials or supplier payloads" permission:"idc:admin:resource:server-batch:view" pageBridgeCapability:"zjmfcloud-servers.admin.v1" pageBridgeHost:"admin"`
	Page   int `json:"page" d:"1" v:"between:1,1000000" dc:"One-based page number"`
	Limit  int `json:"limit" d:"20" v:"between:1,100" dc:"Maximum local servers in one page"`
}
type CloudServerListRes struct {
	Total      int                     `json:"total" dc:"Exact count within the same staff and fixed connector scope"`
	Items      []*accountv1.ServerItem `json:"items" dc:"Bounded non-secret server projections"`
	NextCursor string                  `json:"nextCursor" dc:"Continuation cursor, empty on the final page"`
}
