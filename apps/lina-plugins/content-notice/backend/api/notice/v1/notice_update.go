// This file declares the update-notice request/response DTOs used by the
// content-notice source plugin.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Notice Update API

// UpdateReq defines the request for updating a notice.
type UpdateReq struct {
	g.Meta    `path:"/notice/{id}" method:"put" tags:"Announcements" summary:"Update announcement" dc:"Update the specified announcement. All fields are optional." permission:"content-notice:notice:edit" pageBridgeCapability:"content-notice.admin.v1" pageBridgeHost:"admin"`
	Id        int64   `json:"id" v:"required" dc:"Announcement ID" eg:"1"`
	Title     *string `json:"title" dc:"Announcement title" eg:"System maintenance notification (update)"`
	Content   *string `json:"content" dc:"Announcement content (supports rich text HTML)" eg:"<p>Updated content</p>"`
	SortOrder *int    `json:"sortOrder" v:"min:0" dc:"Sort order, smaller values appear first" eg:"0"`
	Status    *int    `json:"status" v:"in:0,1" dc:"Announcement status: 0=Draft 1=Published" eg:"1"`
}

// UpdateRes Notice update response
type UpdateRes struct{}
