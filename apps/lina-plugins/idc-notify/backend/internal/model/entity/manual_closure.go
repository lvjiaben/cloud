// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ManualClosure is the golang structure for table manual_closure.
type ManualClosure struct {
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	ClosureNo           string     `json:"closureNo"           orm:"closure_no"            description:""`
	TaskNo              string     `json:"taskNo"              orm:"task_no"               description:""`
	ActorUserId         int64      `json:"actorUserId"         orm:"actor_user_id"         description:""`
	ReasonText          string     `json:"reasonText"          orm:"reason_text"           description:""`
	ExpectedTaskVersion int64      `json:"expectedTaskVersion" orm:"expected_task_version" description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
