// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceWaiverAllocation is the golang structure of table plugin_idc_core_billing_invoice_waiver_allocation for DAO operations like Where/Data.
type BillingInvoiceWaiverAllocation struct {
	g.Meta                    `orm:"table:plugin_idc_core_billing_invoice_waiver_allocation, do:true"`
	Id                        any        //
	TenantId                  any        //
	WaiverAllocationNo        any        //
	AdjustmentNo              any        //
	InvoiceNo                 any        //
	CustomerNo                any        //
	CurrencyCode              any        //
	LedgerTransactionNo       any        //
	LedgerTransactionTypeCode any        //
	AdjustmentTypeCode        any        //
	AmountMinor               any        //
	ContentDigest             any        //
	CreatedAt                 *time.Time //
}
