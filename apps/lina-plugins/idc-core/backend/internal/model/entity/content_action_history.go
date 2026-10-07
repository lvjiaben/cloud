// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentActionHistory is the golang structure for table content_action_history.
type ContentActionHistory struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int64      `json:"tenantId"       orm:"tenant_id"       description:""`
	ActionNo       string     `json:"actionNo"       orm:"action_no"       description:""`
	ContentNo      string     `json:"contentNo"      orm:"content_no"      description:""`
	ActionCode     string     `json:"actionCode"     orm:"action_code"     description:""`
	ActorUserId    int64      `json:"actorUserId"    orm:"actor_user_id"   description:""`
	ContentVersion int64      `json:"contentVersion" orm:"content_version" description:""`
	OccurredAt     *time.Time `json:"occurredAt"     orm:"occurred_at"     description:""`
}
