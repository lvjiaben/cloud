// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Mapping is the golang structure of table plugin_idc_resource_mapping for DAO operations like Where/Data.
type Mapping struct {
	g.Meta              `orm:"table:plugin_idc_resource_mapping, do:true"`
	Id                  any        //
	TenantId            any        //
	MappingNo           any        //
	LocalResourceNo     any        // Opaque core-owned resource number; no cross-plugin database foreign key is used
	ChannelNo           any        //
	PoolNo              any        //
	ExternalResourceRef any        // Opaque Provider resource reference, never interpreted as a local identity
	StatusCode          any        //
	BoundAt             *time.Time //
	ReleasedAt          *time.Time //
	Version             any        //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
