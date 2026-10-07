// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// SalesCommissionSettlementItem is the golang structure of table plugin_idc_core_sales_commission_settlement_item for DAO operations like Where/Data.
type SalesCommissionSettlementItem struct {
	g.Meta               `orm:"table:plugin_idc_core_sales_commission_settlement_item, do:true"`
	Id                   any        //
	TenantId             any        //
	ItemNo               any        //
	SettlementNo         any        //
	CommissionNo         any        //
	CurrencyCode         any        //
	BeneficiaryTypeCode  any        //
	BeneficiaryReference any        //
	AmountMinor          any        //
	CreatedAt            *time.Time //
}
