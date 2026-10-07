// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// WecomBindingAudit is the golang structure for table wecom_binding_audit.
type WecomBindingAudit struct {
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	AuditNo        string     `json:"auditNo"        orm:"audit_no"        description:""`
	BindingNo      string     `json:"bindingNo"      orm:"binding_no"      description:""`
	ActorUserId    int64      `json:"actorUserId"    orm:"actor_user_id"   description:""`
	ActionCode     string     `json:"actionCode"     orm:"action_code"     description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"     description:""`
	PreviousStatus string     `json:"previousStatus" orm:"previous_status" description:""`
	CurrentStatus  string     `json:"currentStatus"  orm:"current_status"  description:""`
	BindingVersion int64      `json:"bindingVersion" orm:"binding_version" description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
