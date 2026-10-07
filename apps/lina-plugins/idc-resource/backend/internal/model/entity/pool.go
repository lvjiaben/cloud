// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Pool is the golang structure for table pool.
type Pool struct {
	Id              int64      `json:"id"              orm:"id"               description:""`
	TenantId        int        `json:"tenantId"        orm:"tenant_id"        description:""`
	PoolNo          string     `json:"poolNo"          orm:"pool_no"          description:""`
	ChannelNo       string     `json:"channelNo"       orm:"channel_no"       description:""`
	DisplayName     string     `json:"displayName"     orm:"display_name"     description:""`
	ResourceKind    string     `json:"resourceKind"    orm:"resource_kind"    description:""`
	StatusCode      string     `json:"statusCode"      orm:"status_code"      description:""`
	RoutingPriority int        `json:"routingPriority" orm:"routing_priority" description:""`
	RoutingWeight   int        `json:"routingWeight"   orm:"routing_weight"   description:"Relative selection weight within the same routing priority"`
	Selector        string     `json:"selector"        orm:"selector"         description:"Non-secret normalized placement selector evaluated in set-based routing"`
	Version         int64      `json:"version"         orm:"version"          description:""`
	CreatedBy       int        `json:"createdBy"       orm:"created_by"       description:""`
	UpdatedBy       int        `json:"updatedBy"       orm:"updated_by"       description:""`
	CreatedAt       *time.Time `json:"createdAt"       orm:"created_at"       description:""`
	UpdatedAt       *time.Time `json:"updatedAt"       orm:"updated_at"       description:""`
	DeletedAt       *time.Time `json:"deletedAt"       orm:"deleted_at"       description:""`
}
