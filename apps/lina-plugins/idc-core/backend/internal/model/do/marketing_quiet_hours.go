// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingQuietHours is the golang structure of table plugin_idc_core_marketing_quiet_hours for DAO operations like Where/Data.
type MarketingQuietHours struct {
	g.Meta       `orm:"table:plugin_idc_core_marketing_quiet_hours, do:true"`
	Id           any        //
	TenantId     any        //
	CustomerNo   any        //
	TimezoneName any        //
	StartMinute  any        //
	EndMinute    any        //
	Enabled      any        //
	Version      any        //
	CreatedAt    *time.Time //
	UpdatedAt    *time.Time //
}
