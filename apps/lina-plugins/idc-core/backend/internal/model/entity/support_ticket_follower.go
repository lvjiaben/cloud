// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketFollower is the golang structure for table support_ticket_follower.
type SupportTicketFollower struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	TicketNo  string     `json:"ticketNo"  orm:"ticket_no"  description:""`
	UserId    int64      `json:"userId"    orm:"user_id"    description:""`
	CreatedBy int64      `json:"createdBy" orm:"created_by" description:""`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:""`
}
