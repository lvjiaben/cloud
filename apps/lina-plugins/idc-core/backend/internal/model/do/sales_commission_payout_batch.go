// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutBatch is the golang structure of table plugin_idc_core_sales_commission_payout_batch for DAO operations like Where/Data.
type SalesCommissionPayoutBatch struct {
	g.Meta               `orm:"table:plugin_idc_core_sales_commission_payout_batch, do:true"`
	Id                   any        //
	TenantId             any        //
	BatchNo              any        //
	CurrencyCode         any        //
	ItemCount            any        //
	TotalAmountMinor     any        //
	SucceededCount       any        //
	SucceededAmountMinor any        //
	FailedCount          any        //
	FailedAmountMinor    any        //
	StatusCode           any        //
	CreatedBy            any        //
	CreatedAt            *time.Time //
	CompletedAt          *time.Time //
}
