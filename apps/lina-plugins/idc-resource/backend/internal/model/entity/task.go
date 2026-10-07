// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Task is the golang structure for table task.
type Task struct {
	Id                      int64      `json:"id"                      orm:"id"                        description:""`
	TenantId                int        `json:"tenantId"                orm:"tenant_id"                 description:""`
	TaskNo                  string     `json:"taskNo"                  orm:"task_no"                   description:""`
	RequestIdDigest         string     `json:"requestIdDigest"         orm:"request_id_digest"         description:"SHA-256 digest of the caller idempotency key; raw keys are not persisted"`
	CapabilityCode          string     `json:"capabilityCode"          orm:"capability_code"           description:""`
	LocalResourceNo         string     `json:"localResourceNo"         orm:"local_resource_no"         description:""`
	ChannelNo               string     `json:"channelNo"               orm:"channel_no"                description:""`
	PoolNo                  string     `json:"poolNo"                  orm:"pool_no"                   description:""`
	StatusCode              string     `json:"statusCode"              orm:"status_code"               description:""`
	Priority                int        `json:"priority"                orm:"priority"                  description:""`
	SanitizedParameters     string     `json:"sanitizedParameters"     orm:"sanitized_parameters"      description:"Validated non-secret operation parameters; credential handles and plaintext are forbidden"`
	AttemptCount            int        `json:"attemptCount"            orm:"attempt_count"             description:""`
	MaxAttempts             int        `json:"maxAttempts"             orm:"max_attempts"              description:""`
	AvailableAt             *time.Time `json:"availableAt"             orm:"available_at"              description:""`
	LockedBy                string     `json:"lockedBy"                orm:"locked_by"                 description:""`
	LeaseExpiresAt          *time.Time `json:"leaseExpiresAt"          orm:"lease_expires_at"          description:""`
	NextObserveAt           *time.Time `json:"nextObserveAt"           orm:"next_observe_at"           description:""`
	AcceptedAt              *time.Time `json:"acceptedAt"              orm:"accepted_at"               description:""`
	FinishedAt              *time.Time `json:"finishedAt"              orm:"finished_at"               description:""`
	FailureCode             string     `json:"failureCode"             orm:"failure_code"              description:""`
	FailureSummary          string     `json:"failureSummary"          orm:"failure_summary"           description:""`
	ResultUnknown           bool       `json:"resultUnknown"           orm:"result_unknown"            description:"Hard replay fence requiring observation or manual reconciliation"`
	Version                 int64      `json:"version"                 orm:"version"                   description:""`
	CreatedAt               *time.Time `json:"createdAt"               orm:"created_at"                description:""`
	UpdatedAt               *time.Time `json:"updatedAt"               orm:"updated_at"                description:""`
	ObservationClaimedAt    *time.Time `json:"observationClaimedAt"    orm:"observation_claimed_at"    description:"PostgreSQL time of the latest QueryResource observation claim; never derived from updated_at"`
	OwnerMetadata           string     `json:"ownerMetadata"           orm:"owner_metadata"            description:"Validated non-secret owner intent retained for replay checks and never sent to the Provider"`
	ObservationAttemptCount int        `json:"observationAttemptCount" orm:"observation_attempt_count" description:"Consecutive read-only reconciliation attempts; never authorizes Provider write replay"`
}
