// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketStatusHistory is the golang structure for table support_ticket_status_history.
type SupportTicketStatusHistory struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"        description:""`
	HistoryNo      string     `json:"historyNo"      orm:"history_no"       description:""`
	TicketNo       string     `json:"ticketNo"       orm:"ticket_no"        description:""`
	FromBaseStage  string     `json:"fromBaseStage"  orm:"from_base_stage"  description:""`
	ToBaseStage    string     `json:"toBaseStage"    orm:"to_base_stage"    description:""`
	CustomStatusNo string     `json:"customStatusNo" orm:"custom_status_no" description:""`
	SourceCode     string     `json:"sourceCode"     orm:"source_code"      description:""`
	ActorUserId    int64      `json:"actorUserId"    orm:"actor_user_id"    description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"      description:""`
	ReasonText     string     `json:"reasonText"     orm:"reason_text"      description:""`
	RuleVersion    int64      `json:"ruleVersion"    orm:"rule_version"     description:""`
	OccurredAt     *time.Time `json:"occurredAt"     orm:"occurred_at"      description:""`
}
