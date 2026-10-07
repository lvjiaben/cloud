// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingSnapshotMember is the golang structure for table marketing_snapshot_member.
type MarketingSnapshotMember struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int        `json:"tenantId"   orm:"tenant_id"   description:""`
	SnapshotNo string     `json:"snapshotNo" orm:"snapshot_no" description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	CreatedAt  *time.Time `json:"createdAt"  orm:"created_at"  description:""`
}
