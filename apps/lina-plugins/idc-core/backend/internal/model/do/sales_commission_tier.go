// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionTier is the golang structure of table plugin_idc_core_sales_commission_tier for DAO operations like Where/Data.
type SalesCommissionTier struct {
	g.Meta          `orm:"table:plugin_idc_core_sales_commission_tier, do:true"`
	Id              any        //
	TenantId        any        //
	TierNo          any        //
	RuleNo          any        //
	TierOrder       any        //
	LowerBoundMinor any        //
	UpperBoundMinor *int64     //
	RateBasisPoints any        //
	CreatedAt       *time.Time //
}
