package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSecretCreateReq struct {
	g.Meta         `path:"/admin/resource/channels/{channelNo}/secrets" method:"post" tags:"IDC Resource Channels" summary:"Create a channel secret" dc:"Encrypt one new channel-scoped secret and persist only its opaque binding. Secret bytes are accepted as base64 JSON, never returned, logged or written to audit snapshots." permission:"idc:admin:resource:channel-secret:manage"`
	ChannelNo      string `json:"channelNo" in:"path" v:"required|max-length:128"`
	BindingName    string `json:"bindingName" v:"required|max-length:128"`
	Secret         []byte `json:"secret" v:"required"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|max-length:128"`
	AccessContext  string `json:"accessContext" v:"required|max-length:16384"`
}

type ChannelSecretCreateRes struct {
	Item ChannelSecretStatus `json:"item"`
}
