// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PrivacyRetentionHold is the golang structure for table privacy_retention_hold.
type PrivacyRetentionHold struct {
	Id              int64      `json:"id"              orm:"id"                description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"         description:""`
	HoldNo          string     `json:"holdNo"          orm:"hold_no"           description:""`
	TargetTypeCode  string     `json:"targetTypeCode"  orm:"target_type_code"  description:""`
	TargetNo        string     `json:"targetNo"        orm:"target_no"         description:""`
	ReasonCode      string     `json:"reasonCode"      orm:"reason_code"       description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"       description:""`
	CreatedByOwner  string     `json:"createdByOwner"  orm:"created_by_owner"  description:""`
	ReleasedByOwner string     `json:"releasedByOwner" orm:"released_by_owner" description:""`
	ReleasedAt      *time.Time `json:"releasedAt"      orm:"released_at"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"        description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"        description:""`
}
