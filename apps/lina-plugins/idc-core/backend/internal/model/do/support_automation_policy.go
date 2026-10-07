// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SupportAutomationPolicy is the golang structure of table plugin_idc_core_support_automation_policy for DAO operations like Where/Data.
type SupportAutomationPolicy struct {
	g.Meta        `orm:"table:plugin_idc_core_support_automation_policy, do:true"`
	Id            any        //
	TenantId      any        //
	PolicyNo      any        //
	AutoCloseDays any        //
	ReopenDays    any        //
	Version       any        //
	CreatedBy     any        //
	UpdatedBy     any        //
	CreatedAt     *time.Time //
	UpdatedAt     *time.Time //
}
