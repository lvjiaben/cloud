// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// DeliveryAudit is the golang structure for table delivery_audit.
type DeliveryAudit struct {
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	AuditNo        string     `json:"auditNo"        orm:"audit_no"        description:""`
	TaskNo         string     `json:"taskNo"         orm:"task_no"         description:""`
	ActionCode     string     `json:"actionCode"     orm:"action_code"     description:""`
	ActorType      string     `json:"actorType"      orm:"actor_type"      description:""`
	ActorUserId    any        `json:"actorUserId"    orm:"actor_user_id"   description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"     description:""`
	PreviousStatus string     `json:"previousStatus" orm:"previous_status" description:""`
	CurrentStatus  string     `json:"currentStatus"  orm:"current_status"  description:""`
	TaskVersion    int64      `json:"taskVersion"    orm:"task_version"    description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
