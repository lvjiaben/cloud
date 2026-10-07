// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ServiceLifecycleReminderRule is the golang structure of table plugin_idc_core_service_lifecycle_reminder_rule for DAO operations like Where/Data.
type ServiceLifecycleReminderRule struct {
	g.Meta           `orm:"table:plugin_idc_core_service_lifecycle_reminder_rule, do:true"`
	Id               any        //
	TenantId         any        //
	ReminderRuleNo   any        //
	PolicyNo         any        //
	PolicyVersion    any        //
	ServiceNo        any        //
	ReminderSequence any        //
	RelativeSeconds  any        //
	ContentDigest    any        //
	CreatedAt        *time.Time //
}
