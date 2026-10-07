// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketMessage is the golang structure for table support_ticket_message.
type SupportTicketMessage struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"       description:""`
	MessageNo      string     `json:"messageNo"      orm:"message_no"      description:""`
	TicketNo       string     `json:"ticketNo"       orm:"ticket_no"       description:""`
	MessageType    string     `json:"messageType"    orm:"message_type"    description:""`
	VisibilityCode string     `json:"visibilityCode" orm:"visibility_code" description:""`
	AuthorType     string     `json:"authorType"     orm:"author_type"     description:""`
	AuthorUserId   int64      `json:"authorUserId"   orm:"author_user_id"  description:""`
	Body           string     `json:"body"           orm:"body"            description:""`
	Automated      bool       `json:"automated"      orm:"automated"       description:""`
	RuleVersion    int64      `json:"ruleVersion"    orm:"rule_version"    description:""`
	HiddenAt       *time.Time `json:"hiddenAt"       orm:"hidden_at"       description:""`
	HiddenBy       int64      `json:"hiddenBy"       orm:"hidden_by"       description:""`
	HiddenReason   string     `json:"hiddenReason"   orm:"hidden_reason"   description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
