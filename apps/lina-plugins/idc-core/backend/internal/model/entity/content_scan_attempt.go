// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentScanAttempt is the golang structure for table content_scan_attempt.
type ContentScanAttempt struct {
	Id                  int64      `json:"id"                  orm:"id"                   description:""`
	TenantId            int64      `json:"tenantId"            orm:"tenant_id"            description:""`
	AttemptNo           string     `json:"attemptNo"           orm:"attempt_no"           description:""`
	FileNo              string     `json:"fileNo"              orm:"file_no"              description:""`
	CandidateGeneration int64      `json:"candidateGeneration" orm:"candidate_generation" description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"          description:""`
	QueuedAt            *time.Time `json:"queuedAt"            orm:"queued_at"            description:""`
	StartedAt           *time.Time `json:"startedAt"           orm:"started_at"           description:""`
	LeaseExpiresAt      *time.Time `json:"leaseExpiresAt"      orm:"lease_expires_at"     description:""`
	FinishedAt          *time.Time `json:"finishedAt"          orm:"finished_at"          description:""`
	RiskCode            string     `json:"riskCode"            orm:"risk_code"            description:""`
	CreatedBy           int64      `json:"createdBy"           orm:"created_by"           description:""`
	Version             int64      `json:"version"             orm:"version"              description:""`
}
