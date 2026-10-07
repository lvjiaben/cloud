// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketEscalation is the golang structure for table support_ticket_escalation.
type SupportTicketEscalation struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int64      `json:"tenantId"     orm:"tenant_id"     description:""`
	EscalationNo string     `json:"escalationNo" orm:"escalation_no" description:""`
	TicketNo     string     `json:"ticketNo"     orm:"ticket_no"     description:""`
	FromUserId   int64      `json:"fromUserId"   orm:"from_user_id"  description:""`
	ToUserId     int64      `json:"toUserId"     orm:"to_user_id"    description:""`
	Reason       string     `json:"reason"       orm:"reason"        description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
}
