// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// InterfaceCandidateSnapshot is the golang structure for table interface_candidate_snapshot.
type InterfaceCandidateSnapshot struct {
	RequestKey            string     `json:"requestKey"            orm:"request_key"            description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"              description:""`
	InterfaceInstanceNo   string     `json:"interfaceInstanceNo"   orm:"interface_instance_no"  description:""`
	ConfigurationRevision string     `json:"configurationRevision" orm:"configuration_revision" description:""`
	RequestPayload        string     `json:"requestPayload"        orm:"request_payload"        description:""`
	ResultPayload         string     `json:"resultPayload"         orm:"result_payload"         description:""`
	ObservedAt            *time.Time `json:"observedAt"            orm:"observed_at"            description:""`
	RefreshAfter          *time.Time `json:"refreshAfter"          orm:"refresh_after"          description:""`
	LeaseUntil            *time.Time `json:"leaseUntil"            orm:"lease_until"            description:""`
	LastErrorCode         string     `json:"lastErrorCode"         orm:"last_error_code"        description:""`
}
