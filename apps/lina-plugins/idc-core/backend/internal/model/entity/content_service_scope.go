// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ContentServiceScope is the golang structure for table content_service_scope.
type ContentServiceScope struct {
	Id        int64      `json:"id"        orm:"id"         description:""`
	TenantId  int64      `json:"tenantId"  orm:"tenant_id"  description:""`
	ContentNo string     `json:"contentNo" orm:"content_no" description:""`
	ServiceNo string     `json:"serviceNo" orm:"service_no" description:""`
	CreatedAt *time.Time `json:"createdAt" orm:"created_at" description:""`
}
