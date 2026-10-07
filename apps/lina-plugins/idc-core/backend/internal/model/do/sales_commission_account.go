// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionAccount is the golang structure of table plugin_idc_core_sales_commission_account for DAO operations like Where/Data.
type SalesCommissionAccount struct {
	g.Meta               `orm:"table:plugin_idc_core_sales_commission_account, do:true"`
	Id                   any        //
	TenantId             any        //
	AccountNo            any        //
	BeneficiaryTypeCode  any        //
	BeneficiaryReference any        //
	CurrencyCode         any        //
	CreditedAmountMinor  any        //
	PendingAmountMinor   any        //
	AvailableAmountMinor any        //
	FrozenAmountMinor    any        //
	PaidAmountMinor      any        //
	ReversedAmountMinor  any        //
	StatusCode           any        //
	Version              any        //
	CreatedAt            *time.Time //
	UpdatedAt            *time.Time //
}
