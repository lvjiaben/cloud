// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// NotificationPreferenceProfile is the golang structure for table notification_preference_profile.
type NotificationPreferenceProfile struct {
	Id         int64      `json:"id"         orm:"id"          description:""`
	TenantId   int64      `json:"tenantId"   orm:"tenant_id"   description:""`
	CustomerNo string     `json:"customerNo" orm:"customer_no" description:""`
	Version    int64      `json:"version"    orm:"version"     description:""`
	UpdatedAt  *time.Time `json:"updatedAt"  orm:"updated_at"  description:""`
}
