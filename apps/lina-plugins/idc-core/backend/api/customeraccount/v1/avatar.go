// This file defines customer-owned avatar uploads. Identity and scene come from trusted server context.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// AvatarUploadReq uploads one image for the current active customer, without backoffice file permissions.
type AvatarUploadReq struct {
	g.Meta `path:"/account/customer/avatar-uploads" method:"post" mime:"multipart/form-data" tags:"IDC Customer Account" summary:"Upload my avatar" dc:"Stores one validated image owned by the current active customer; does not accept an actor or business scene override"`
	File   *ghttp.UploadFile `json:"file" type:"file" v:"required" dc:"PNG, JPEG or WebP avatar, at most 5 MiB" eg:"avatar.png"`
}

// AvatarUploadRes returns the governed image URL for the existing self-profile avatar update endpoint.
type AvatarUploadRes struct {
	URL string `json:"url" dc:"Uploaded avatar URL" eg:"/uploads/avatar/example.png"`
}
