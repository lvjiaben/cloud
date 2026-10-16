// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProductFulfillmentTask is the golang structure for table product_fulfillment_task.
type ProductFulfillmentTask struct {
	TenantId           int        `json:"tenantId"           orm:"tenant_id"            description:""`
	TaskNo             string     `json:"taskNo"             orm:"task_no"              description:""`
	IntentNo           string     `json:"intentNo"           orm:"intent_no"            description:""`
	StatusCode         string     `json:"statusCode"         orm:"status_code"          description:""`
	PhaseCode          string     `json:"phaseCode"          orm:"phase_code"           description:""`
	AttemptCount       int        `json:"attemptCount"       orm:"attempt_count"        description:""`
	QueryAttemptCount  int        `json:"queryAttemptCount"  orm:"query_attempt_count"  description:""`
	SubmitAttemptCount int        `json:"submitAttemptCount" orm:"submit_attempt_count" description:""`
	MaximumAttempts    int        `json:"maximumAttempts"    orm:"maximum_attempts"     description:""`
	RetryBaseSeconds   int        `json:"retryBaseSeconds"   orm:"retry_base_seconds"   description:""`
	NextAttemptAt      *time.Time `json:"nextAttemptAt"      orm:"next_attempt_at"      description:""`
	LeaseToken         string     `json:"leaseToken"         orm:"lease_token"          description:""`
	LeaseExpiresAt     *time.Time `json:"leaseExpiresAt"     orm:"lease_expires_at"     description:""`
	ExternalReference  string     `json:"externalReference"  orm:"external_reference"   description:""`
	LastResultCode     string     `json:"lastResultCode"     orm:"last_result_code"     description:""`
	TakeoverRequested  bool       `json:"takeoverRequested"  orm:"takeover_requested"   description:""`
	CancelRequested    bool       `json:"cancelRequested"    orm:"cancel_requested"     description:""`
	Version            int64      `json:"version"            orm:"version"              description:""`
	CreatedAt          *time.Time `json:"createdAt"          orm:"created_at"           description:""`
	UpdatedAt          *time.Time `json:"updatedAt"          orm:"updated_at"           description:""`
	UpstreamDeliveries string     `json:"upstreamDeliveries" orm:"upstream_deliveries"  description:""`
}
