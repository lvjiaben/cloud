// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecyclePolicy is the golang structure of table plugin_idc_core_service_lifecycle_policy for DAO operations like Where/Data.
type ServiceLifecyclePolicy struct {
	g.Meta                `orm:"table:plugin_idc_core_service_lifecycle_policy, do:true"`
	Id                    any        //
	TenantId              any        //
	PolicyNo              any        //
	ServiceNo             any        //
	PolicyVersion         any        //
	ReminderCount         any        //
	GraceSeconds          any        //
	SuspendEnabled        any        //
	SuspendAfterSeconds   *int64     //
	TerminateEnabled      any        //
	TerminateAfterSeconds *int64     //
	ConfiguredBy          any        //
	ContentDigest         any        //
	CreatedAt             *time.Time //
}
