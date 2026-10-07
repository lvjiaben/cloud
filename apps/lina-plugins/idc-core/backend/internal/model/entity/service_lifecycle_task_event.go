// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceLifecycleTaskEvent is the golang structure for table service_lifecycle_task_event.
type ServiceLifecycleTaskEvent struct {
	Id             int64      `json:"id"             orm:"id"               description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"        description:""`
	EventNo        string     `json:"eventNo"        orm:"event_no"         description:""`
	TaskNo         string     `json:"taskNo"         orm:"task_no"          description:""`
	ServiceNo      string     `json:"serviceNo"      orm:"service_no"       description:""`
	EventSequence  int        `json:"eventSequence"  orm:"event_sequence"   description:""`
	FromStatusCode *string    `json:"fromStatusCode" orm:"from_status_code" description:""`
	ToStatusCode   string     `json:"toStatusCode"   orm:"to_status_code"   description:""`
	ReasonCode     string     `json:"reasonCode"     orm:"reason_code"      description:""`
	ActorTypeCode  string     `json:"actorTypeCode"  orm:"actor_type_code"  description:""`
	ActorReference string     `json:"actorReference" orm:"actor_reference"  description:""`
	EvidenceDigest string     `json:"evidenceDigest" orm:"evidence_digest"  description:""`
	ContentDigest  string     `json:"contentDigest"  orm:"content_digest"   description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"       description:""`
}
