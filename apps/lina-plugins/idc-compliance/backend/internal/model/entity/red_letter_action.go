// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// RedLetterAction is the golang structure for table red_letter_action.
type RedLetterAction struct {
	Id                 int64      `json:"id"                 orm:"id"                  description:""`
	TenantId           int        `json:"tenantId"           orm:"tenant_id"           description:""`
	ActionNo           string     `json:"actionNo"           orm:"action_no"           description:""`
	TaskNo             string     `json:"taskNo"             orm:"task_no"             description:""`
	ActionCode         string     `json:"actionCode"         orm:"action_code"         description:""`
	IdempotencyDigest  string     `json:"idempotencyDigest"  orm:"idempotency_digest"  description:""`
	RequestFingerprint string     `json:"requestFingerprint" orm:"request_fingerprint" description:""`
	ActorTypeCode      string     `json:"actorTypeCode"      orm:"actor_type_code"     description:""`
	ActorUserId        int        `json:"actorUserId"        orm:"actor_user_id"       description:""`
	BeforeStatusCode   string     `json:"beforeStatusCode"   orm:"before_status_code"  description:""`
	AfterStatusCode    string     `json:"afterStatusCode"    orm:"after_status_code"   description:""`
	BeforeVersion      int64      `json:"beforeVersion"      orm:"before_version"      description:""`
	AfterVersion       int64      `json:"afterVersion"       orm:"after_version"       description:""`
	EvidenceNo         string     `json:"evidenceNo"         orm:"evidence_no"         description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"          description:""`
}
