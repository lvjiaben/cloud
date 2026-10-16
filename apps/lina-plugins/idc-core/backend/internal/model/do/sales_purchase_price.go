// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesPurchasePrice is the golang structure of table plugin_idc_core_sales_purchase_price for DAO operations like Where/Data.
type SalesPurchasePrice struct {
	g.Meta              `orm:"table:plugin_idc_core_sales_purchase_price, do:true"`
	Id                  any        //
	TenantId            any        //
	PriceNo             any        //
	CustomerNo          any        //
	LevelNo             any        //
	ProductNo           any        //
	CurrencyCode        any        //
	ChargeTypeCode      any        //
	PeriodUnitCode      any        //
	PeriodCount         any        //
	AdjustmentTypeCode  any        //
	FixedPriceMinor     any        //
	DiscountBasisPoints any        //
	StatusCode          any        //
	Version             any        //
	ContentDigest       any        //
	CreatedAt           *time.Time //
}
