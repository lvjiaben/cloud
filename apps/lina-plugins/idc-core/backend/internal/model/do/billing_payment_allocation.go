// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingPaymentAllocation is the golang structure of table plugin_idc_core_billing_payment_allocation for DAO operations like Where/Data.
type BillingPaymentAllocation struct {
	g.Meta                    `orm:"table:plugin_idc_core_billing_payment_allocation, do:true"`
	Id                        any        //
	TenantId                  any        //
	AllocationNo              any        //
	InvoiceNo                 any        //
	InvoiceItemNo             *string    //
	CustomerNo                any        //
	CurrencyCode              any        //
	PaymentSourceTypeCode     any        //
	PaymentSourceReference    any        //
	LedgerTransactionNo       any        //
	LedgerTransactionTypeCode any        //
	DirectionCode             any        //
	OriginalAllocationNo      *string    //
	AmountMinor               any        //
	ContentDigest             any        //
	CreatedAt                 *time.Time //
}
