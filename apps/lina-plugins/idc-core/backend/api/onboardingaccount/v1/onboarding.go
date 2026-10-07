package v1

import "github.com/gogf/gf/v2/frame/g"

type StatusReq struct {
	g.Meta `path:"/account/onboarding/status" method:"get" tags:"IDC Customer Onboarding" summary:"Get the current identity onboarding status"`
}
type StatusRes struct {
	Required   bool   `json:"required"`
	CustomerNo string `json:"customerNo,omitempty"`
}

type RedeemReq struct {
	g.Meta                 `path:"/account/onboarding/redeem" method:"post" tags:"IDC Customer Onboarding" summary:"Redeem one verified customer onboarding intent"`
	Token                  string `json:"token" v:"required|max-length:2048"`
	BrowserNonce           string `json:"browserNonce" v:"required|min-length:16|max-length:128"`
	DeviceId               string `json:"deviceId" v:"required|min-length:16|max-length:128" dc:"Browser-generated random device identifier; the server persists only a purpose-bound digest"`
	HostRegistrationUserID int    `json:"hostRegistrationUserId" v:"required|min:1"`
}
type RedeemRes struct {
	CustomerNo   string `json:"customerNo"`
	CustomerType string `json:"customerType"`
	Reused       bool   `json:"reused"`
}
