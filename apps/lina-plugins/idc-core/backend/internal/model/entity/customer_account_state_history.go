// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CustomerAccountStateHistory is the golang structure for table customer_account_state_history.
type CustomerAccountStateHistory struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	TransitionNo         string     `json:"transitionNo"         orm:"transition_no"          description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"            description:""`
	FromStatusCode       string     `json:"fromStatusCode"       orm:"from_status_code"       description:""`
	ToStatusCode         string     `json:"toStatusCode"         orm:"to_status_code"         description:""`
	ReasonCode           string     `json:"reasonCode"           orm:"reason_code"            description:""`
	SourceCode           string     `json:"sourceCode"           orm:"source_code"            description:""`
	SourceNo             string     `json:"sourceNo"             orm:"source_no"              description:""`
	ActorUserId          int        `json:"actorUserId"          orm:"actor_user_id"          description:""`
	SubjectVersion       int64      `json:"subjectVersion"       orm:"subject_version"        description:""`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest" description:""`
	RequestFingerprint   string     `json:"requestFingerprint"   orm:"request_fingerprint"    description:""`
	OccurredAt           *time.Time `json:"occurredAt"           orm:"occurred_at"            description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
}
