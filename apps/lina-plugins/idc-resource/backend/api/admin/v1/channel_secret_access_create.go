package v1

import "github.com/gogf/gf/v2/frame/g"

type ChannelSecretAccessCreateReq struct {
	g.Meta            `path:"/admin/resource/channels/{channelNo}/secrets/access-contexts" method:"post" tags:"IDC Resource Channels" summary:"Authorize one channel-secret operation" dc:"Issue a short-lived, signed and single-use context after exact staff authorization and a current active session. The context is bound to one immutable operation preview." permission:"idc:admin:resource:channel-secret:manage"`
	ChannelNo         string `json:"channelNo" in:"path" v:"required|max-length:128"`
	Operation         string `json:"operation" v:"required|in:create,replace,revoke,rotate"`
	BindingName       string `json:"bindingName" v:"max-length:128"`
	SecretDigest      string `json:"secretDigest" v:"max-length:64" dc:"Lowercase SHA-256 digest calculated from the exact secret bytes; the mutation endpoint recomputes and verifies it"`
	ExpectedUpdatedAt int64  `json:"expectedUpdatedAt" dc:"Current secret lifecycle fence in Unix milliseconds"`
	RotateLimit       int    `json:"rotateLimit" dc:"Maximum envelopes to rewrap, from 1 to 100"`
	IdempotencyKey    string `json:"idempotencyKey" v:"required|max-length:128"`
}

type ChannelSecretAccessCreateRes struct {
	Item ChannelSecretAccessContext `json:"item"`
}
