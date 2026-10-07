// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketAssignmentHistory is the golang structure for table support_ticket_assignment_history.
type SupportTicketAssignmentHistory struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int64      `json:"tenantId"         orm:"tenant_id"          description:""`
	HistoryNo        string     `json:"historyNo"        orm:"history_no"         description:""`
	TicketNo         string     `json:"ticketNo"         orm:"ticket_no"          description:""`
	ActionCode       string     `json:"actionCode"       orm:"action_code"        description:""`
	FromDepartmentNo string     `json:"fromDepartmentNo" orm:"from_department_no" description:""`
	ToDepartmentNo   string     `json:"toDepartmentNo"   orm:"to_department_no"   description:""`
	FromUserId       int64      `json:"fromUserId"       orm:"from_user_id"       description:""`
	ToUserId         int64      `json:"toUserId"         orm:"to_user_id"         description:""`
	ActorUserId      int64      `json:"actorUserId"      orm:"actor_user_id"      description:""`
	ReasonCode       string     `json:"reasonCode"       orm:"reason_code"        description:""`
	ReasonText       string     `json:"reasonText"       orm:"reason_text"        description:""`
	OccurredAt       *time.Time `json:"occurredAt"       orm:"occurred_at"        description:""`
}
