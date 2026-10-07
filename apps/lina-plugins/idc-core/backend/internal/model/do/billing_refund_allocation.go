// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingRefundAllocation is the golang structure of table plugin_idc_core_billing_refund_allocation for DAO operations like Where/Data.
type BillingRefundAllocation struct {
	g.Meta                     `orm:"table:plugin_idc_core_billing_refund_allocation, do:true"`
	Id                         any        //
	TenantId                   any        //
	RefundAllocationNo         any        //
	InvoiceNo                  any        //
	InvoiceItemNo              any        //
	CustomerNo                 any        //
	CurrencyCode               any        //
	PaymentAllocationNo        any        //
	RefundSourceTypeCode       any        //
	RefundSourceReference      any        //
	LedgerTransactionNo        any        //
	LedgerTransactionTypeCode  any        //
	DirectionCode              any        //
	OriginalRefundAllocationNo *string    //
	AmountMinor                any        //
	ContentDigest              any        //
	CreatedAt                  *time.Time //
}
