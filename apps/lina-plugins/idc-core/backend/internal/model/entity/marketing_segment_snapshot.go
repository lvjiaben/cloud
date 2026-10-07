// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingSegmentSnapshot is the golang structure for table marketing_segment_snapshot.
type MarketingSegmentSnapshot struct {
	Id               int64      `json:"id"               orm:"id"                description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"         description:""`
	SnapshotNo       string     `json:"snapshotNo"       orm:"snapshot_no"       description:""`
	SegmentNo        string     `json:"segmentNo"        orm:"segment_no"        description:""`
	SegmentVersion   int64      `json:"segmentVersion"   orm:"segment_version"   description:""`
	DefinitionDigest string     `json:"definitionDigest" orm:"definition_digest" description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"       description:""`
	MemberCount      int64      `json:"memberCount"      orm:"member_count"      description:""`
	ContentDigest    *string    `json:"contentDigest"    orm:"content_digest"    description:""`
	StartedAt        *time.Time `json:"startedAt"        orm:"started_at"        description:""`
	CompletedAt      *time.Time `json:"completedAt"      orm:"completed_at"      description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"        description:""`
}
