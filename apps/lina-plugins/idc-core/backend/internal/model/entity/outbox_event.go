// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// OutboxEvent is the golang structure for table outbox_event.
type OutboxEvent struct {
	Id                  int64      `json:"id"                  orm:"id"                   description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"            description:""`
	EventNo             string     `json:"eventNo"             orm:"event_no"             description:"Opaque outbox event business number"`
	EventType           string     `json:"eventType"           orm:"event_type"           description:""`
	EventVersion        int        `json:"eventVersion"        orm:"event_version"        description:"Positive version of the published event contract"`
	Topic               string     `json:"topic"               orm:"topic"                description:"Stable logical delivery topic"`
	AggregateType       string     `json:"aggregateType"       orm:"aggregate_type"       description:""`
	AggregateNo         string     `json:"aggregateNo"         orm:"aggregate_no"         description:""`
	CorrelationNo       string     `json:"correlationNo"       orm:"correlation_no"       description:"Opaque business correlation number"`
	DeduplicationDigest string     `json:"deduplicationDigest" orm:"deduplication_digest" description:"Digest that prevents duplicate events of the same type"`
	StatusCode          string     `json:"statusCode"          orm:"status_code"          description:""`
	Payload             string     `json:"payload"             orm:"payload"              description:"Versioned redacted event payload without secrets"`
	Metadata            string     `json:"metadata"            orm:"metadata"             description:"Versioned delivery metadata without secrets"`
	OccurredAt          *time.Time `json:"occurredAt"          orm:"occurred_at"          description:""`
	AvailableAt         *time.Time `json:"availableAt"         orm:"available_at"         description:"Earliest time at which the event may be claimed"`
	LockedBy            string     `json:"lockedBy"            orm:"locked_by"            description:""`
	LeaseExpiresAt      *time.Time `json:"leaseExpiresAt"      orm:"lease_expires_at"     description:"Claim lease expiry used for crash recovery"`
	AttemptCount        int        `json:"attemptCount"        orm:"attempt_count"        description:""`
	MaxAttempts         int        `json:"maxAttempts"         orm:"max_attempts"         description:""`
	PublishedAt         *time.Time `json:"publishedAt"         orm:"published_at"         description:""`
	LastErrorCode       string     `json:"lastErrorCode"       orm:"last_error_code"      description:""`
	LastErrorSummary    string     `json:"lastErrorSummary"    orm:"last_error_summary"   description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"           description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"           description:""`
}
