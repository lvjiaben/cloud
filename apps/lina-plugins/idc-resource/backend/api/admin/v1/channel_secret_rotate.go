package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSecretRotateReq struct {
	g.Meta         `path:"/admin/resource/channels/{channelNo}/secrets/rotation" method:"post" tags:"IDC Resource Channels" summary:"Rotate channel-secret wrapping" dc:"Rewrap a bounded batch in the exact channel scope without listing or returning secret references." permission:"idc:admin:resource:channel-secret:manage"`
	ChannelNo      string `json:"channelNo" in:"path" v:"required|max-length:128"`
	Limit          int    `json:"limit" v:"required|between:1,100"`
	IdempotencyKey string `json:"idempotencyKey" v:"required|max-length:128"`
	AccessContext  string `json:"accessContext" v:"required|max-length:16384"`
}

type ChannelSecretRotateRes struct {
	Rewrapped int  `json:"rewrapped"`
	Remaining bool `json:"remaining"`
}
