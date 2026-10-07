// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingConditionValue is the golang structure of table plugin_idc_core_marketing_condition_value for DAO operations like Where/Data.
type MarketingConditionValue struct {
	g.Meta       `orm:"table:plugin_idc_core_marketing_condition_value, do:true"`
	Id           any        //
	TenantId     any        //
	ConditionNo  any        //
	ValueText    *string    //
	ValueInteger *int64     //
	SortOrder    any        //
	CreatedAt    *time.Time //
}
