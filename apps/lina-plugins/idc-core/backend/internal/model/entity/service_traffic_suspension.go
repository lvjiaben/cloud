// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceTrafficSuspension is the golang structure for table service_traffic_suspension.
type ServiceTrafficSuspension struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	ServiceNo         string     `json:"serviceNo"         orm:"service_no"          description:""`
	SourceOperationNo string     `json:"sourceOperationNo" orm:"source_operation_no" description:""`
	Active            bool       `json:"active"            orm:"active"              description:""`
	SuspendedAt       *time.Time `json:"suspendedAt"       orm:"suspended_at"        description:""`
	ClearedAt         *time.Time `json:"clearedAt"         orm:"cleared_at"          description:""`
	Version           int64      `json:"version"           orm:"version"             description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"          description:""`
}
