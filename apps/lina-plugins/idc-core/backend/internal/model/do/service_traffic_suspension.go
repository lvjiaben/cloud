// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceTrafficSuspension is the golang structure of table plugin_idc_core_service_traffic_suspension for DAO operations like Where/Data.
type ServiceTrafficSuspension struct {
	g.Meta            `orm:"table:plugin_idc_core_service_traffic_suspension, do:true"`
	Id                any        //
	TenantId          any        //
	ServiceNo         any        //
	SourceOperationNo any        //
	Active            any        //
	SuspendedAt       *time.Time //
	ClearedAt         *time.Time //
	Version           any        //
	CreatedAt         *time.Time //
	UpdatedAt         *time.Time //
}
