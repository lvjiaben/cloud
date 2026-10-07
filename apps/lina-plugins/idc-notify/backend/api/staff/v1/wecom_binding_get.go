// This file defines the read-only contract for the current staff member's WeCom binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

// WeComBindingGetReq requests the caller's own binding state.
type WeComBindingGetReq struct {
	g.Meta `path:"/staff/notifications/wechat-work/binding" method:"get" tags:"IDC Staff WeCom Binding" summary:"Get my WeCom binding" dc:"Returns only the current active direct staff member's own WeCom binding status. It never returns a WeCom UserID."`
}

// WeComBindingGetRes returns the caller's safe binding projection.
type WeComBindingGetRes struct {
	Item *WeComBindingItem `json:"item" dc:"Current staff member's own binding status" eg:"{}"`
}
