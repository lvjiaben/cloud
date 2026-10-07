// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// Mapping is the golang structure for table mapping.
type Mapping struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	MappingNo           string     `json:"mappingNo"           orm:"mapping_no"            description:""`
	LocalResourceNo     string     `json:"localResourceNo"     orm:"local_resource_no"     description:"Opaque core-owned resource number; no cross-plugin database foreign key is used"`
	ChannelNo           string     `json:"channelNo"           orm:"channel_no"            description:""`
	PoolNo              string     `json:"poolNo"              orm:"pool_no"               description:""`
	ExternalResourceRef string     `json:"externalResourceRef" orm:"external_resource_ref" description:"Opaque Provider resource reference, never interpreted as a local identity"`
	StatusCode          string     `json:"statusCode"          orm:"status_code"           description:""`
	BoundAt             *time.Time `json:"boundAt"             orm:"bound_at"              description:""`
	ReleasedAt          *time.Time `json:"releasedAt"          orm:"released_at"           description:""`
	Version             int64      `json:"version"             orm:"version"               description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
	UpdatedAt           *time.Time `json:"updatedAt"           orm:"updated_at"            description:""`
}
