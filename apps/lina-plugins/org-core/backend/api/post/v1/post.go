// This file defines shared post response DTOs for the org-core API.
package v1

// PostItem exposes position fields visible through org-core APIs.
type PostItem struct {
	Id        int    `json:"id" dc:"Position ID" eg:"1"`
	DeptId    int    `json:"deptId" dc:"Department ID" eg:"100"`
	Code      string `json:"code" dc:"Position code" eg:"dev"`
	Name      string `json:"name" dc:"Position name" eg:"Development Engineer"`
	Sort      int    `json:"sort" dc:"Sort order" eg:"1"`
	Status    int    `json:"status" dc:"Status: 1=normal 0=disabled" eg:"1"`
	Remark    string `json:"remark" dc:"Remark" eg:"Responsible for system development"`
	CreatedAt *int64 `json:"createdAt" dc:"Creation time as Unix timestamp in milliseconds" eg:"1776756000000"`
	UpdatedAt *int64 `json:"updatedAt" dc:"Update time as Unix timestamp in milliseconds" eg:"1776757800000"`
}
