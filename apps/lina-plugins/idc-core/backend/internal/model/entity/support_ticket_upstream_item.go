// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketUpstreamItem is the golang structure for table support_ticket_upstream_item.
type SupportTicketUpstreamItem struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	ForwardNo string     `json:"forwardNo" orm:"forward_no" description:""`
	ItemType  string     `json:"itemType"  orm:"item_type"  description:""`
	ItemNo    string     `json:"itemNo"    orm:"item_no"    description:""`
	SortOrder int        `json:"sortOrder" orm:"sort_order" description:""`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:""`
}
