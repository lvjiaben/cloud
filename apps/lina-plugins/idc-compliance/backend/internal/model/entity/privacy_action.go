// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyAction is the golang structure for table privacy_action.
type PrivacyAction struct {
	Id                 int64      `json:"id"                 orm:"id"                  description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"           description:""`
	ActionNo           string     `json:"actionNo"           orm:"action_no"           description:""`
	ActionCode         string     `json:"actionCode"         orm:"action_code"         description:""`
	TargetNo           string     `json:"targetNo"           orm:"target_no"           description:""`
	IdempotencyDigest  string     `json:"idempotencyDigest"  orm:"idempotency_digest"  description:""`
	RequestFingerprint string     `json:"requestFingerprint" orm:"request_fingerprint" description:""`
	ResultReference    string     `json:"resultReference"    orm:"result_reference"    description:""`
	ActorUserId        int        `json:"actorUserId"        orm:"actor_user_id"       description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"          description:""`
}
