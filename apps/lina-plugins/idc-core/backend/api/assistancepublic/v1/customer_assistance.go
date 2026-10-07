package v1

import "github.com/gogf/gf/v2/frame/g"

type CustomerAssistanceSnapshotReq struct {
	g.Meta      `path:"/public/customer-assistance/access-verifications" method:"post" tags:"IDC Customer Assistance" summary:"Exchange a customer assistance launch token" dc:"Consume one live fifteen-minute assistance launch token and return the delegated customer access token once."`
	AccessToken string `json:"accessToken" v:"required|min-length:32|max-length:128" dc:"Short-lived assistance access token carried only in the request body"`
}

type CustomerAssistanceSnapshotRes struct {
	SessionNo   string `json:"sessionNo"`
	AccessToken string `json:"accessToken"`
	HomePath    string `json:"homePath"`
	ExpiresAt   int64  `json:"expiresAt"`
}
