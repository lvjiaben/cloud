// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AsyncTaskAttempt is the golang structure for table async_task_attempt.
type AsyncTaskAttempt struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	TaskNo              string     `json:"taskNo"              orm:"task_no"               description:""`
	AttemptNo           int        `json:"attemptNo"           orm:"attempt_no"            description:"Monotonic attempt number within one task"`
	WorkerId            string     `json:"workerId"            orm:"worker_id"             description:""`
	ExternalReferenceNo string     `json:"externalReferenceNo" orm:"external_reference_no" description:"External reference returned for this execution attempt"`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	RequestSnapshot     string     `json:"requestSnapshot"     orm:"request_snapshot"      description:"Redacted normalized request evidence"`
	ResponseSnapshot    string     `json:"responseSnapshot"    orm:"response_snapshot"     description:"Redacted normalized response evidence"`
	StartedAt           *time.Time `json:"startedAt"           orm:"started_at"            description:""`
	EndedAt             *time.Time `json:"endedAt"             orm:"ended_at"              description:""`
	ErrorCode           string     `json:"errorCode"           orm:"error_code"            description:""`
	ErrorSummary        string     `json:"errorSummary"        orm:"error_summary"         description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
