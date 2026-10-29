// This file defines direct browser access to public customer avatars through random opaque keys.

package v1

import "github.com/gogf/gf/v2/frame/g"

// CustomerAvatarOpenReq reads an avatar; no user identifier or arbitrary storage path is accepted.
type CustomerAvatarOpenReq struct {
	g.Meta `path:"/public/customer-avatars/{key}" method:"get" tags:"IDC Customer Account" summary:"Read customer avatar" dc:"Streams a validated avatar using its unpredictable public image key, without exposing account identifiers or arbitrary storage objects"`
	Key    string `json:"key" v:"required|length:52,53" dc:"192-bit random avatar key with PNG, JPEG or WebP suffix" eg:"1234567890abcdef1234567890abcdef1234567890abcdef.png"`
}

// CustomerAvatarOpenRes is streamed image content, not a JSON envelope.
type CustomerAvatarOpenRes struct {
	g.Meta `mime:"image/png,image/jpeg,image/webp"`
}
