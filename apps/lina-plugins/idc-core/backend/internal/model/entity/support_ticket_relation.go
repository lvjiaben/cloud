// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SupportTicketRelation is the golang structure for table support_ticket_relation.
type SupportTicketRelation struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"        description:""`
	RelationNo     string     `json:"relationNo"     orm:"relation_no"      description:""`
	RelationType   string     `json:"relationType"   orm:"relation_type"    description:""`
	SourceTicketNo string     `json:"sourceTicketNo" orm:"source_ticket_no" description:""`
	TargetTicketNo string     `json:"targetTicketNo" orm:"target_ticket_no" description:""`
	ActorUserId    int64      `json:"actorUserId"    orm:"actor_user_id"    description:""`
	Reason         string     `json:"reason"         orm:"reason"           description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
}
