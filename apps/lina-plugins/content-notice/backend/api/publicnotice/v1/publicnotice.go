// Package v1 declares public website projections from the single announcement source.
package v1

import "github.com/gogf/gf/v2/frame/g"

// Item exposes a published announcement without account or management data.
type Item struct {
	Id        int64  `json:"id" dc:"Announcement ID" eg:"1"`
	Title     string `json:"title" dc:"Announcement title" eg:"Maintenance"`
	Content   string `json:"content" dc:"Announcement content with inline images" eg:"<p>Maintenance</p>"`
	SortOrder int    `json:"sortOrder" dc:"Sort order, smaller values appear first" eg:"0"`
	CreatedAt *int64 `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1776756000000"`
	UpdatedAt *int64 `json:"updatedAt" dc:"Last updated time as Unix timestamp in milliseconds" eg:"1776756000000"`
}

// ListReq defines the public announcement page.
type ListReq struct {
	g.Meta   `path:"/public/announcements" method:"get" tags:"Announcements" summary:"List public announcements" dc:"List published announcements for the public website."`
	PageNum  int `json:"pageNum" d:"1" v:"min:1" dc:"Page number" eg:"1"`
	PageSize int `json:"pageSize" d:"10" v:"min:1|max:100" dc:"Number of items per page" eg:"10"`
}

// ListRes returns published announcements in their saved sort order.
type ListRes struct {
	List  []*Item `json:"list" dc:"Published announcements" eg:"[]"`
	Total int     `json:"total" dc:"Total number of items" eg:"20"`
}

// GetReq selects one published announcement.
type GetReq struct {
	g.Meta `path:"/public/announcements/{id}" method:"get" tags:"Announcements" summary:"Get public announcement" dc:"Get a published announcement for the public website."`
	Id     int64 `json:"id" v:"required|min:1" dc:"Announcement ID" eg:"1"`
}

// GetRes returns a public announcement.
type GetRes struct{ Item }
