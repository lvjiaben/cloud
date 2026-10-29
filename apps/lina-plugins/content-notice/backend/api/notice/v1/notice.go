// This file defines shared notice response DTOs for the content-notice API.
package v1

// NoticeItem exposes announcement fields visible to callers.
type NoticeItem struct {
	Id        int64  `json:"id" dc:"Announcement ID" eg:"1"`
	Title     string `json:"title" dc:"Announcement title" eg:"System maintenance notification"`
	Content   string `json:"content" dc:"Announcement content, supports rich text HTML" eg:"<p>The system will be undergoing maintenance and upgrade tonight</p>"`
	SortOrder int    `json:"sortOrder" dc:"Sort order, smaller values appear first" eg:"0"`
	Status    int    `json:"status" dc:"Announcement status: 0=Draft 1=Published" eg:"1"`
	CreatedBy int64  `json:"createdBy" dc:"Creator user ID" eg:"1"`
	UpdatedBy int64  `json:"updatedBy" dc:"Last updated user ID" eg:"1"`
	CreatedAt *int64 `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1776756000000"`
	UpdatedAt *int64 `json:"updatedAt" dc:"Last updated time as Unix timestamp in milliseconds" eg:"1776757800000"`
}
