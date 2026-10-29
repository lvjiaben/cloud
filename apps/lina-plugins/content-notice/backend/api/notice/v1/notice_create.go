// This file declares the create-notice request/response DTOs used by the
// content-notice source plugin.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Notice Create API

// CreateReq defines the request for creating a notice.
type CreateReq struct {
	g.Meta    `path:"/notice" method:"post" tags:"Announcements" summary:"Create announcement" dc:"Create an announcement as either a draft or a published item, with images in the content." permission:"content-notice:notice:add" pageBridgeCapability:"content-notice.admin.v1" pageBridgeHost:"admin"`
	Title     string `json:"title" v:"required#gf.gvalid.rule.required" dc:"Announcement title" eg:"System maintenance notification"`
	Content   string `json:"content" v:"required#gf.gvalid.rule.required" dc:"Announcement content (supports rich text HTML)" eg:"<p>The system will be undergoing maintenance and upgrade tonight</p>"`
	SortOrder int    `json:"sortOrder" v:"min:0" dc:"Sort order, smaller values appear first" eg:"0"`
	Status    *int   `json:"status" d:"0" v:"in:0,1" dc:"Announcement status: 0=Draft 1=Published" eg:"1"`
}

// CreateRes Notice create response
type CreateRes struct {
	Id int64 `json:"id" dc:"Announcement ID" eg:"1"`
}
