// This file defines the notice projection returned by the source-plugin service.

package notice

import "time"

// NoticeEntity mirrors the plugin-owned notice fields without importing DAO or
// generated database packages into the WASM guest.
type NoticeEntity struct {
	Id        int64      `json:"id"`
	TenantId  int        `json:"tenantId"`
	Title     string     `json:"title"`
	Type      int        `json:"type"`
	Content   string     `json:"content"`
	FileIds   string     `json:"fileIds"`
	Status    int        `json:"status"`
	Remark    string     `json:"remark"`
	CreatedBy int64      `json:"createdBy"`
	UpdatedBy int64      `json:"updatedBy"`
	CreatedAt *time.Time `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
}
