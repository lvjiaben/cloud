// This file declares the delete-notice request/response DTOs used by the
// content-notice source plugin.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Notice Delete API

// DeleteReq defines the request for deleting notices.
type DeleteReq struct {
	g.Meta `path:"/notice" method:"delete" tags:"Announcements" summary:"Delete announcement" dc:"Delete one or more announcements by query array ids[]" permission:"content-notice:notice:remove" pageBridgeCapability:"content-notice.admin.v1" pageBridgeHost:"admin"`
	Ids    []int64 `json:"ids" v:"required|min-length:1" dc:"Announcement ID list as a query array, e.g. ids[]=1&ids[]=2&ids[]=3" eg:"[1,2,3]"`
}

// DeleteRes Notice delete response
type DeleteRes struct{}
