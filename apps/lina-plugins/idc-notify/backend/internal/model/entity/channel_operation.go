// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelOperation is the golang structure for table channel_operation.
type ChannelOperation struct {
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	OperationNo       string     `json:"operationNo"       orm:"operation_no"        description:""`
	ChannelCode       string     `json:"channelCode"       orm:"channel_code"        description:""`
	OperationCode     string     `json:"operationCode"     orm:"operation_code"      description:""`
	ActorUserId       int64      `json:"actorUserId"       orm:"actor_user_id"       description:""`
	ReasonCode        string     `json:"reasonCode"        orm:"reason_code"         description:""`
	BeforeVersion     int64      `json:"beforeVersion"     orm:"before_version"      description:""`
	AfterVersion      int64      `json:"afterVersion"      orm:"after_version"       description:""`
	AffectedTaskCount int        `json:"affectedTaskCount" orm:"affected_task_count" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
}
