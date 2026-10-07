// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AccessorySnapshotItem is the golang structure for table accessory_snapshot_item.
type AccessorySnapshotItem struct {
	Id            int64      `json:"id"            orm:"id"             description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"      description:""`
	SnapshotNo    string     `json:"snapshotNo"    orm:"snapshot_no"    description:""`
	ObjectNo      string     `json:"objectNo"      orm:"object_no"      description:"Local opaque object number exposed to callers instead of Provider identifiers"`
	AccessoryKind string     `json:"accessoryKind" orm:"accessory_kind" description:""`
	ExternalRef   string     `json:"externalRef"   orm:"external_ref"   description:"Resource-owner private Provider reference; never returned, logged, or audited"`
	DisplayName   string     `json:"displayName"   orm:"display_name"   description:""`
	StateCode     string     `json:"stateCode"     orm:"state_code"     description:""`
	Projection    string     `json:"projection"    orm:"projection"     description:""`
	ObservedAt    *time.Time `json:"observedAt"    orm:"observed_at"    description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"     description:""`
}
