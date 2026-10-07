package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSecretRevokeReq struct {
	g.Meta            `path:"/admin/resource/channels/{channelNo}/secrets/{bindingName}" method:"delete" tags:"IDC Resource Channels" summary:"Revoke a channel secret" dc:"Permanently revoke one exact channel-scoped secret and retain its non-secret lifecycle history." permission:"idc:admin:resource:channel-secret:manage"`
	ChannelNo         string `json:"channelNo" in:"path" v:"required|max-length:128"`
	BindingName       string `json:"bindingName" in:"path" v:"required|max-length:128"`
	ExpectedUpdatedAt int64  `json:"expectedUpdatedAt" v:"required|min:1"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|max-length:128"`
	AccessContext     string `json:"accessContext" v:"required|max-length:16384"`
}

type ChannelSecretRevokeRes struct {
	Item ChannelSecretStatus `json:"item"`
}
