// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingSegmentMember is the golang structure for table marketing_segment_member.
type MarketingSegmentMember struct {
	Id             int64      `json:"id"             orm:"id"              description:""`
	TenantId       int        `json:"tenantId"       orm:"tenant_id"       description:""`
	SegmentNo      string     `json:"segmentNo"      orm:"segment_no"      description:""`
	CustomerNo     string     `json:"customerNo"     orm:"customer_no"     description:""`
	SourceCode     string     `json:"sourceCode"     orm:"source_code"     description:""`
	SegmentVersion int64      `json:"segmentVersion" orm:"segment_version" description:""`
	StatusCode     string     `json:"statusCode"     orm:"status_code"     description:""`
	EvaluatedAt    *time.Time `json:"evaluatedAt"    orm:"evaluated_at"    description:""`
	RevokedAt      *time.Time `json:"revokedAt"      orm:"revoked_at"      description:""`
	CreatedAt      *time.Time `json:"createdAt"      orm:"created_at"      description:""`
	UpdatedAt      *time.Time `json:"updatedAt"      orm:"updated_at"      description:""`
}
