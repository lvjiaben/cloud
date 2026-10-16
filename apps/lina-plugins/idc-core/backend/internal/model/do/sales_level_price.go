// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesLevelPrice is the golang structure of table plugin_idc_core_sales_level_price for DAO operations like Where/Data.
type SalesLevelPrice struct {
	g.Meta                `orm:"table:plugin_idc_core_sales_level_price, do:true"`
	Id                    any        //
	TenantId              any        //
	PriceNo               any        //
	LevelNo               any        //
	PercentageBasisPoints any        //
	StatusCode            any        //
	Version               any        //
	ContentDigest         any        //
	CreatedAt             *time.Time //
}
