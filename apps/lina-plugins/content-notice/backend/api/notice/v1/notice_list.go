// This file declares the list-notice request/response DTOs used by the
// content-notice source plugin.

package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// Notice List API

// ListReq defines the request for listing notices.
type ListReq struct {
	g.Meta    `path:"/notice" method:"get" tags:"Announcements" summary:"Get announcement list" dc:"Query announcements by page, with filtering by title and creator." permission:"content-notice:notice:query" pageBridgeCapability:"content-notice.admin.v1" pageBridgeHost:"admin"`
	PageNum   int    `json:"pageNum" d:"1" v:"min:1" dc:"Page number" eg:"1"`
	PageSize  int    `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Number of items per page" eg:"10"`
	Title     string `json:"title" dc:"Filter by title (fuzzy match)" eg:"System maintenance"`
	CreatedBy string `json:"createdBy" dc:"Filter by creator username" eg:"admin"`
}

// ListRes Notice list response
type ListRes struct {
	List  []*ListItem `json:"list" dc:"Announcement list" eg:"[]"`
	Total int         `json:"total" dc:"Total number of items" eg:"20"`
}

// ListItem Notice list item
type ListItem struct {
	NoticeItem
	CreatedByName string `json:"createdByName" dc:"Creator username" eg:"admin"`
}
