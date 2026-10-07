// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CapabilitySnapshotItem is the golang structure for table capability_snapshot_item.
type CapabilitySnapshotItem struct {
	Id                int64      `json:"id"                orm:"id"                 description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"          description:""`
	SnapshotNo        string     `json:"snapshotNo"        orm:"snapshot_no"        description:""`
	CapabilityCode    string     `json:"capabilityCode"    orm:"capability_code"    description:""`
	SupportCode       string     `json:"supportCode"       orm:"support_code"       description:""`
	Constraints       string     `json:"constraints"       orm:"constraints"        description:""`
	UnsupportedReason string     `json:"unsupportedReason" orm:"unsupported_reason" description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"         description:""`
}
