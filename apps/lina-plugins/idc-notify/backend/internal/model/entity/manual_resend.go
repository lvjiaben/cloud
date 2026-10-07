// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ManualResend is the golang structure for table manual_resend.
type ManualResend struct {
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	OperationNo       string     `json:"operationNo"       orm:"operation_no"        description:""`
	ChainNo           string     `json:"chainNo"           orm:"chain_no"            description:""`
	SourceTaskNo      string     `json:"sourceTaskNo"      orm:"source_task_no"      description:""`
	CreatedTaskNo     string     `json:"createdTaskNo"     orm:"created_task_no"     description:""`
	ActorUserId       int64      `json:"actorUserId"       orm:"actor_user_id"       description:""`
	ReasonText        string     `json:"reasonText"        orm:"reason_text"         description:""`
	SourceTaskVersion int64      `json:"sourceTaskVersion" orm:"source_task_version" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
