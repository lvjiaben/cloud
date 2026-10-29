// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductFulfillmentCommand is the golang structure for table product_fulfillment_command.
type ProductFulfillmentCommand struct {
	TenantId              int        `json:"tenantId"              orm:"tenant_id"              description:""`
	CommandNo             string     `json:"commandNo"             orm:"command_no"             description:""`
	IntentNo              string     `json:"intentNo"              orm:"intent_no"              description:""`
	ActionCode            string     `json:"actionCode"            orm:"action_code"            description:""`
	IdempotencyKey        string     `json:"idempotencyKey"        orm:"idempotency_key"        description:""`
	RequestDigest         string     `json:"requestDigest"         orm:"request_digest"         description:""`
	ReasonCode            string     `json:"reasonCode"            orm:"reason_code"            description:""`
	ReasonLabelZhCn       string     `json:"reasonLabelZhCn"       orm:"reason_label_zh_cn"     description:""`
	ReasonLabelEnUs       string     `json:"reasonLabelEnUs"       orm:"reason_label_en_us"     description:""`
	SafeNote              string     `json:"safeNote"              orm:"safe_note"              description:""`
	ActorUserId           int64      `json:"actorUserId"           orm:"actor_user_id"          description:""`
	ResultingVersion      int64      `json:"resultingVersion"      orm:"resulting_version"      description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"             description:""`
	RecoveryTaskVersion   int64      `json:"recoveryTaskVersion"   orm:"recovery_task_version"  description:""`
	RecoveryQueryCount    int        `json:"recoveryQueryCount"    orm:"recovery_query_count"   description:""`
	RecoverySubmitCount   int        `json:"recoverySubmitCount"   orm:"recovery_submit_count"  description:""`
	RecoveryConfiguration string     `json:"recoveryConfiguration" orm:"recovery_configuration" description:""`
}
