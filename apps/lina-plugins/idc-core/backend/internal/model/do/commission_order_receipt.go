// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// CommissionOrderReceipt is the golang structure of table plugin_idc_core_commission_order_receipt for DAO operations like Where/Data.
type CommissionOrderReceipt struct {
	g.Meta       `orm:"table:plugin_idc_core_commission_order_receipt, do:true"`
	Id           any        //
	TenantId     any        //
	OrderNo      any        //
	SettlementNo any        //
	SourceDigest any        //
	CreatedAt    *time.Time //
	InvoiceNo    any        //
}
