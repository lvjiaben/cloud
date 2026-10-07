// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlement is the golang structure of table plugin_idc_core_sales_commission_settlement for DAO operations like Where/Data.
type SalesCommissionSettlement struct {
	g.Meta               `orm:"table:plugin_idc_core_sales_commission_settlement, do:true"`
	Id                   any        //
	TenantId             any        //
	SettlementNo         any        //
	AccountNo            any        //
	CurrencyCode         any        //
	PayeeProfileNo       any        //
	BeneficiaryTypeCode  any        //
	BeneficiaryReference any        //
	RequestedAmountMinor any        //
	FrozenAmountMinor    any        //
	PaidAmountMinor      *int64     //
	ReturnedAmountMinor  *int64     //
	MinimumAmountMinor   any        //
	StatusCode           any        //
	RequestDigest        any        //
	RequestedAt          *time.Time //
	CompletedAt          *time.Time //
	Version              any        //
}
