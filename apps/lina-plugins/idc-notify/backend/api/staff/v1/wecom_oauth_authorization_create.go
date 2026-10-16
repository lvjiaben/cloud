// This file defines the contract for starting the current staff member's WeCom authorization.

package v1

import "github.com/gogf/gf/v2/frame/g"

// WeComOAuthAuthorizationCreateReq starts one session-bound authorization attempt.
type WeComOAuthAuthorizationCreateReq struct {
	g.Meta         `path:"/staff/notifications/wechat-work/oauth-authorizations" method:"post" tags:"IDC Staff WeCom Binding" summary:"Start my WeCom authorization" dc:"Creates one five-minute, current-session-bound official WeCom OAuth and QR authorization after a current active session."`
	IdempotencyKey string `json:"idempotencyKey" v:"required|min-length:8|max-length:64" dc:"Caller-generated idempotency key" eg:"wecom-bind-20260815-001"`
}

// WeComOAuthAuthorizationCreateRes returns official authorization destinations and their expiry.
type WeComOAuthAuthorizationCreateRes struct {
	AuthorizationURL string `json:"authorizationUrl" dc:"Official WeCom mobile OAuth URL" eg:"https://open.weixin.qq.com/connect/oauth2/authorize?..."`
	QRLoginURL       string `json:"qrLoginUrl" dc:"Official WeCom desktop QR login URL" eg:"https://login.work.weixin.qq.com/wwlogin/sso/login?..."`
	ExpiresAt        int64  `json:"expiresAt" dc:"Authorization expiry as a Unix timestamp in milliseconds" eg:"1786752300000"`
	Reused           bool   `json:"reused" dc:"Whether the same pending authorization was reused" eg:"false"`
}
