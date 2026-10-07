// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AsyncTask is the golang structure for table async_task.
type AsyncTask struct {
	Id                   int64      `json:"id"                   orm:"id"                      description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"               description:""`
	TaskNo               string     `json:"taskNo"               orm:"task_no"                 description:"Opaque task business number"`
	TaskType             string     `json:"taskType"             orm:"task_type"               description:""`
	SubjectType          string     `json:"subjectType"          orm:"subject_type"            description:""`
	SubjectNo            string     `json:"subjectNo"            orm:"subject_no"              description:""`
	CustomerNo           string     `json:"customerNo"           orm:"customer_no"             description:"Validated customer ownership projection for database-side permission filtering"`
	ExternalReferenceNo  string     `json:"externalReferenceNo"  orm:"external_reference_no"   description:"Current normalized external task or operation reference"`
	IdempotencyKeyDigest string     `json:"idempotencyKeyDigest" orm:"idempotency_key_digest"  description:"SHA-256 digest of the task idempotency key"`
	StatusCode           string     `json:"statusCode"           orm:"status_code"             description:""`
	Priority             int        `json:"priority"             orm:"priority"                description:""`
	Payload              string     `json:"payload"              orm:"payload"                 description:"Versioned redacted task input without secrets"`
	ResultSnapshot       string     `json:"resultSnapshot"       orm:"result_snapshot"         description:"Redacted normalized result used for status and recovery"`
	AvailableAt          *time.Time `json:"availableAt"          orm:"available_at"            description:""`
	DeadlineAt           *time.Time `json:"deadlineAt"           orm:"deadline_at"             description:""`
	LockedBy             string     `json:"lockedBy"             orm:"locked_by"               description:""`
	LeaseExpiresAt       *time.Time `json:"leaseExpiresAt"       orm:"lease_expires_at"        description:""`
	AttemptCount         int        `json:"attemptCount"         orm:"attempt_count"           description:""`
	MaxAttempts          int        `json:"maxAttempts"          orm:"max_attempts"            description:""`
	CompletedAt          *time.Time `json:"completedAt"          orm:"completed_at"            description:""`
	ManualTakeoverAt     *time.Time `json:"manualTakeoverAt"     orm:"manual_takeover_at"      description:"Time when automated execution yielded to an operator"`
	ManualAssigneeUserId int        `json:"manualAssigneeUserId" orm:"manual_assignee_user_id" description:"Host staff identity assigned to manual takeover"`
	ManualReason         string     `json:"manualReason"         orm:"manual_reason"           description:"Redacted operator-facing reason for manual takeover"`
	LastErrorCode        string     `json:"lastErrorCode"        orm:"last_error_code"         description:""`
	LastErrorSummary     string     `json:"lastErrorSummary"     orm:"last_error_summary"      description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"              description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"              description:""`
}
