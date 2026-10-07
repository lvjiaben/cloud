// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketSlaPause is the golang structure for table support_ticket_sla_pause.
type SupportTicketSlaPause struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int64      `json:"tenantId"   orm:"tenant_id"   description:""`
	TicketNo   string     `json:"ticketNo"   orm:"ticket_no"   description:""`
	RoundNo    int        `json:"roundNo"    orm:"round_no"    description:""`
	StartedAt  *time.Time `json:"startedAt"  orm:"started_at"  description:""`
	EndedAt    *time.Time `json:"endedAt"    orm:"ended_at"    description:""`
	ReasonCode string     `json:"reasonCode" orm:"reason_code" description:""`
}
