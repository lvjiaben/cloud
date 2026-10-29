// This file declares the signed-in announcement list, detail and read-state APIs.

package v1

import "github.com/gogf/gf/v2/frame/g"

// AnnouncementItem contains the published content and current reader state.
type AnnouncementItem struct {
	Id        int64  `json:"id" dc:"Announcement ID" eg:"1"`
	Title     string `json:"title" dc:"Announcement title" eg:"Maintenance"`
	Content   string `json:"content" dc:"Announcement content with inline images" eg:"<p>Maintenance</p>"`
	SortOrder int    `json:"sortOrder" dc:"Sort order, smaller values appear first" eg:"0"`
	CreatedAt *int64 `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1776756000000"`
	UpdatedAt *int64 `json:"updatedAt" dc:"Last updated time as Unix timestamp in milliseconds" eg:"1776756000000"`
	Read      bool   `json:"read" dc:"Whether the current user has read this announcement" eg:"false"`
}

// AnnouncementsReq lists published announcements for the signed-in user.
type AnnouncementsReq struct {
	g.Meta   `path:"/notice/announcements" method:"get" tags:"Announcements" summary:"List published announcements" dc:"List all published announcements with the current user's read state."`
	PageNum  int `json:"pageNum" d:"1" v:"min:1" dc:"Page number" eg:"1"`
	PageSize int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Number of items per page" eg:"10"`
}

// AnnouncementsRes returns one announcement page.
type AnnouncementsRes struct {
	List  []*AnnouncementItem `json:"list" dc:"Published announcements" eg:"[]"`
	Total int                 `json:"total" dc:"Total number of items" eg:"20"`
}

// AnnouncementReq fetches one published announcement without changing read state.
type AnnouncementReq struct {
	g.Meta `path:"/notice/announcements/{id}" method:"get" tags:"Announcements" summary:"Get published announcement" dc:"Get a published announcement with the current user's read state."`
	Id     int64 `json:"id" v:"required|min:1" dc:"Announcement ID" eg:"1"`
}

// AnnouncementRes returns the published announcement.
type AnnouncementRes struct{ AnnouncementItem }

// AnnouncementReadReq marks a published announcement read for the current user.
type AnnouncementReadReq struct {
	g.Meta `path:"/notice/announcements/{id}/read" method:"put" tags:"Announcements" summary:"Mark announcement read" dc:"Idempotently mark a published announcement read for the signed-in user."`
	Id     int64 `json:"id" v:"required|min:1" dc:"Announcement ID" eg:"1"`
}

// AnnouncementReadRes acknowledges persisted read state.
type AnnouncementReadRes struct{}
