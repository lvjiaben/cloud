// This file defines the callback contract that completes the current staff member's WeCom binding.

package v1

import "github.com/gogf/gf/v2/frame/g"

// WeComOAuthCallbackGetReq carries the official one-time callback code and state.
type WeComOAuthCallbackGetReq struct {
	g.Meta `path:"/staff/notifications/wechat-work/oauth-callback" method:"get" tags:"IDC Staff WeCom Binding" summary:"Complete my WeCom authorization" dc:"Consumes the official single-use code only for the same authenticated staff session and OAuth state."`
	Code   string `json:"code" in:"query" v:"required|max-length:512" dc:"Official single-use authorization code" eg:"provider-code"`
	State  string `json:"state" in:"query" v:"required|max-length:128" dc:"Server-issued single-use state" eg:"state"`
}

// WeComOAuthCallbackGetRes returns only the safe binding projection.
type WeComOAuthCallbackGetRes struct {
	Item *WeComBindingItem `json:"item" dc:"Binding status after the callback" eg:"{}"`
}
