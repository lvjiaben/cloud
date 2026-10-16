// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionOfflinePayout is the golang structure of table plugin_idc_core_sales_commission_offline_payout for DAO operations like Where/Data.
type SalesCommissionOfflinePayout struct {
	g.Meta            `orm:"table:plugin_idc_core_sales_commission_offline_payout, do:true"`
	Id                any        //
	TenantId          any        //
	PayoutNo          any        //
	AccountNo         any        //
	CurrencyCode      any        //
	AmountMinor       any        //
	EvidenceReference any        //
	PaidAt            *time.Time //
	RecordedBy        any        //
	RequestDigest     any        //
	CreatedAt         *time.Time //
}
