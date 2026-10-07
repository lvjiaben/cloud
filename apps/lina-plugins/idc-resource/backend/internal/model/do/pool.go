// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// Pool is the golang structure of table plugin_idc_resource_pool for DAO operations like Where/Data.
type Pool struct {
	g.Meta          `orm:"table:plugin_idc_resource_pool, do:true"`
	Id              any        //
	TenantId        any        //
	PoolNo          any        //
	ChannelNo       any        //
	DisplayName     any        //
	ResourceKind    any        //
	StatusCode      any        //
	RoutingPriority any        //
	RoutingWeight   any        // Relative selection weight within the same routing priority
	Selector        any        // Non-secret normalized placement selector evaluated in set-based routing
	Version         any        //
	CreatedBy       any        //
	UpdatedBy       any        //
	CreatedAt       *time.Time //
	UpdatedAt       *time.Time //
	DeletedAt       *time.Time //
}
