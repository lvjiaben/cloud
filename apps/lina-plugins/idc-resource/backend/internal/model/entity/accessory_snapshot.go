// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// AccessorySnapshot is the golang structure for table accessory_snapshot.
type AccessorySnapshot struct {
	Id                  int64      `json:"id"                  orm:"id"                   description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"            description:""`
	SnapshotNo          string     `json:"snapshotNo"          orm:"snapshot_no"          description:""`
	MappingNo           string     `json:"mappingNo"           orm:"mapping_no"           description:""`
	LocalResourceNo     string     `json:"localResourceNo"     orm:"local_resource_no"    description:""`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"           description:""`
	StatusCode          string     `json:"statusCode"          orm:"status_code"          description:""`
	SnapshotFingerprint string     `json:"snapshotFingerprint" orm:"snapshot_fingerprint" description:""`
	ItemCount           int        `json:"itemCount"           orm:"item_count"           description:""`
	ObservedAt          *time.Time `json:"observedAt"          orm:"observed_at"          description:""`
	ExpiresAt           *time.Time `json:"expiresAt"           orm:"expires_at"           description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"           description:""`
}
