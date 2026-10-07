package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSecretReplaceReq struct {
	g.Meta            `path:"/admin/resource/channels/{channelNo}/secrets/{bindingName}" method:"put" tags:"IDC Resource Channels" summary:"Replace a channel secret" dc:"Replace one exact channel-scoped secret behind an optimistic lifecycle fence. Secret bytes are accepted as base64 JSON and are never returned." permission:"idc:admin:resource:channel-secret:manage"`
	ChannelNo         string `json:"channelNo" in:"path" v:"required|max-length:128"`
	BindingName       string `json:"bindingName" in:"path" v:"required|max-length:128"`
	Secret            []byte `json:"secret" v:"required"`
	ExpectedUpdatedAt int64  `json:"expectedUpdatedAt" v:"required|min:1"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|max-length:128"`
	AccessContext     string `json:"accessContext" v:"required|max-length:16384"`
}

type ChannelSecretReplaceRes struct {
	Item ChannelSecretStatus `json:"item"`
}
