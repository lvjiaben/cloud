// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProviderCapacityObservation is the golang structure for table provider_capacity_observation.
type ProviderCapacityObservation struct {
	Id                     int64      `json:"id"                     orm:"id"                      description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"               description:""`
	ObservationNo          string     `json:"observationNo"          orm:"observation_no"          description:""`
	ChannelNo              string     `json:"channelNo"              orm:"channel_no"              description:""`
	CapacityKind           string     `json:"capacityKind"           orm:"capacity_kind"           description:""`
	StatusCode             string     `json:"statusCode"             orm:"status_code"             description:""`
	ObservationFingerprint string     `json:"observationFingerprint" orm:"observation_fingerprint" description:""`
	ItemCount              int        `json:"itemCount"              orm:"item_count"              description:""`
	ObservedAt             *time.Time `json:"observedAt"             orm:"observed_at"             description:""`
	CompletedAt            *time.Time `json:"completedAt"            orm:"completed_at"            description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"              description:""`
	PageSize               int        `json:"pageSize"               orm:"page_size"               description:"Fixed Provider page size for the exact collecting observation"`
	NextCursor             string     `json:"nextCursor"             orm:"next_cursor"             description:"Exact opaque Provider cursor required for the next staged page"`
	PageCount              int        `json:"pageCount"              orm:"page_count"              description:"Number of validated pages durably staged into this observation"`
	TriggerCode            string     `json:"triggerCode"            orm:"trigger_code"            description:"Trusted operation source: system or direct staff manual request"`
	RequestedBy            int64      `json:"requestedBy"            orm:"requested_by"            description:"Trusted host staff user identifier for a manual request; zero for system work"`
	AttemptCount           int        `json:"attemptCount"           orm:"attempt_count"           description:"Bounded number of claimed Provider page attempts for this observation"`
	NextAttemptAt          *time.Time `json:"nextAttemptAt"          orm:"next_attempt_at"         description:"PostgreSQL due time for the next single-page attempt; null means not scheduled"`
	LeaseTokenDigest       string     `json:"leaseTokenDigest"       orm:"lease_token_digest"      description:"SHA-256 digest of an ephemeral worker lease token; the token itself is never stored"`
	LeaseExpiresAt         *time.Time `json:"leaseExpiresAt"         orm:"lease_expires_at"        description:"PostgreSQL time after which another worker may reclaim this collecting observation"`
	FailureCode            string     `json:"failureCode"            orm:"failure_code"            description:"Stable non-secret terminal failure code; raw Provider errors are forbidden"`
}
