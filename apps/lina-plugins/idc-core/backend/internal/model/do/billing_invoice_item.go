// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceItem is the golang structure of table plugin_idc_core_billing_invoice_item for DAO operations like Where/Data.
type BillingInvoiceItem struct {
	g.Meta             `orm:"table:plugin_idc_core_billing_invoice_item, do:true"`
	Id                 any        //
	TenantId           any        //
	InvoiceItemNo      any        //
	InvoiceNo          any        //
	CustomerNo         any        //
	CurrencyCode       any        //
	LineNumber         any        //
	ItemTypeCode       any        //
	OrderNo            *string    //
	OrderItemNo        *string    //
	SourceReference    any        //
	DescriptionZhCn    any        //
	DescriptionEnUs    any        //
	SubtotalMinor      any        //
	DiscountMinor      any        //
	NetAmountMinor     any        //
	TaxAmountMinor     any        //
	TotalAmountMinor   any        //
	RefundableCapMinor any        //
	ContentDigest      any        //
	CreatedAt          *time.Time //
}
