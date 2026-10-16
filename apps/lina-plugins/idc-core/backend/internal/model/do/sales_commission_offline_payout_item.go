// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionOfflinePayoutItem is the golang structure of table plugin_idc_core_sales_commission_offline_payout_item for DAO operations like Where/Data.
type SalesCommissionOfflinePayoutItem struct {
	g.Meta       `orm:"table:plugin_idc_core_sales_commission_offline_payout_item, do:true"`
	Id           any        //
	TenantId     any        //
	PayoutNo     any        //
	CommissionNo any        //
	AmountMinor  any        //
	CreatedAt    *time.Time //
}
