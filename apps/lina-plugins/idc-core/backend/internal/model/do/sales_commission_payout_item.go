// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionPayoutItem is the golang structure of table plugin_idc_core_sales_commission_payout_item for DAO operations like Where/Data.
type SalesCommissionPayoutItem struct {
	g.Meta            `orm:"table:plugin_idc_core_sales_commission_payout_item, do:true"`
	Id                any        //
	TenantId          any        //
	ItemNo            any        //
	BatchNo           any        //
	SettlementNo      any        //
	CurrencyCode      any        //
	AmountMinor       any        //
	StatusCode        any        //
	AttemptNo         any        //
	FailureReasonCode *string    //
	ProcessedAt       *time.Time //
}
