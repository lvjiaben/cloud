// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyIncidentEvent is the golang structure for table privacy_incident_event.
type PrivacyIncidentEvent struct {
	Id          int64      `json:"id"          orm:"id"            description:""`
	TenantId    int        `json:"tenantId"    orm:"tenant_id"     description:""`
	EventNo     string     `json:"eventNo"     orm:"event_no"      description:""`
	IncidentNo  string     `json:"incidentNo"  orm:"incident_no"   description:""`
	StageCode   string     `json:"stageCode"   orm:"stage_code"    description:""`
	SafeNote    string     `json:"safeNote"    orm:"safe_note"     description:""`
	ActorUserId int        `json:"actorUserId" orm:"actor_user_id" description:""`
	OccurredAt  *time.Time `json:"occurredAt"  orm:"occurred_at"   description:""`
	CreatedAt   *time.Time `json:"createdAt"   orm:"created_at"    description:""`
}
