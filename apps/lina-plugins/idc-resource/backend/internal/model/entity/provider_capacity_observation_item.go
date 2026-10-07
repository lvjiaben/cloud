// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ProviderCapacityObservationItem is the golang structure for table provider_capacity_observation_item.
type ProviderCapacityObservationItem struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	ObservationNo string     `json:"observationNo" orm:"observation_no" description:""`
	CapacityKind  string     `json:"capacityKind"  orm:"capacity_kind"  description:""`
	ExternalRef   string     `json:"externalRef"   orm:"external_ref"   description:""`
	Projection    string     `json:"projection"    orm:"projection"     description:"Normalized non-secret projection; raw Provider responses are forbidden"`
	ObservedAt    *time.Time `json:"observedAt"    orm:"observed_at"    description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
}
