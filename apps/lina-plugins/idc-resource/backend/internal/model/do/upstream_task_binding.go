// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// UpstreamTaskBinding is the golang structure of table plugin_idc_resource_upstream_task_binding for DAO operations like Where/Data.
type UpstreamTaskBinding struct {
	g.Meta              `orm:"table:plugin_idc_resource_upstream_task_binding, do:true"`
	Id                  any        //
	TenantId            any        //
	BindingNo           any        //
	TaskNo              any        //
	StepNo              any        //
	ChannelNo           any        //
	ExternalTaskRef     any        //
	ExternalResourceRef any        //
	StatusCode          any        //
	LastProgress        any        //
	LastObservedAt      *time.Time //
	CreatedAt           *time.Time //
	UpdatedAt           *time.Time //
}
