// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// PublicAddress is the golang structure for table public_address.
type PublicAddress struct {
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	NotificationOrigin   string     `json:"notificationOrigin"   orm:"notification_origin"    description:""`
	CustomerReturnOrigin string     `json:"customerReturnOrigin" orm:"customer_return_origin" description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	UpdatedBy            int        `json:"updatedBy"            orm:"updated_by"             description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
