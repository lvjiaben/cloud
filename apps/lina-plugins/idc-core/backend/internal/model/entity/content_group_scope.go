// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentGroupScope is the golang structure for table content_group_scope.
type ContentGroupScope struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	ContentNo string     `json:"contentNo" orm:"content_no" description:""`
	GroupNo   string     `json:"groupNo"   orm:"group_no"   description:""`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:""`
}
