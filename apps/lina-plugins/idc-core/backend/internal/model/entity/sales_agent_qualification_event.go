// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentQualificationEvent is the golang structure for table sales_agent_qualification_event.
type SalesAgentQualificationEvent struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	EventNo         string     `json:"eventNo"         orm:"event_no"         description:""`
	QualificationNo string     `json:"qualificationNo" orm:"qualification_no" description:""`
	EventSequence   int64      `json:"eventSequence"   orm:"event_sequence"   description:""`
	FromStatusCode  string     `json:"fromStatusCode"  orm:"from_status_code" description:""`
	ToStatusCode    string     `json:"toStatusCode"    orm:"to_status_code"   description:""`
	FromLevelNo     string     `json:"fromLevelNo"     orm:"from_level_no"    description:""`
	ToLevelNo       string     `json:"toLevelNo"       orm:"to_level_no"      description:""`
	ReasonCode      string     `json:"reasonCode"      orm:"reason_code"      description:""`
	ActorReference  string     `json:"actorReference"  orm:"actor_reference"  description:""`
	EvidenceDigest  string     `json:"evidenceDigest"  orm:"evidence_digest"  description:""`
	OccurredAt      *time.Time `json:"occurredAt"      orm:"occurred_at"      description:""`
}
