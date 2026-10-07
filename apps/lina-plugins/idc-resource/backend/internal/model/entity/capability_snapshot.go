// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CapabilitySnapshot is the golang structure for table capability_snapshot.
type CapabilitySnapshot struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	SnapshotNo     string     `json:"snapshotNo"     orm:"snapshot_no"     description:""`
	ChannelNo      string     `json:"channelNo"      orm:"channel_no"      description:""`
	AdapterVersion string     `json:"adapterVersion" orm:"adapter_version" description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"     description:""`
	ItemCount      int        `json:"itemCount"      orm:"item_count"      description:""`
	ObservedAt     *time.Time `json:"observedAt"     orm:"observed_at"     description:""`
	CompletedAt    *time.Time `json:"completedAt"    orm:"completed_at"    description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
}
