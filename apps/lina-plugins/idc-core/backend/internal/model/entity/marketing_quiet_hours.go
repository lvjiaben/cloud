// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingQuietHours is the golang structure for table marketing_quiet_hours.
type MarketingQuietHours struct {
	Id           int64      `json:"id"           orm:"id"            description:""`
	TenantId     int        `json:"tenantId"     orm:"tenant_id"     description:""`
	CustomerNo   string     `json:"customerNo"   orm:"customer_no"   description:""`
	TimezoneName string     `json:"timezoneName" orm:"timezone_name" description:""`
	StartMinute  int        `json:"startMinute"  orm:"start_minute"  description:""`
	EndMinute    int        `json:"endMinute"    orm:"end_minute"    description:""`
	Enabled      bool       `json:"enabled"      orm:"enabled"       description:""`
	Version      int64      `json:"version"      orm:"version"       description:""`
	CreatedAt    *time.Time `json:"createdAt"    orm:"created_at"    description:""`
	UpdatedAt    *time.Time `json:"updatedAt"    orm:"updated_at"    description:""`
}
