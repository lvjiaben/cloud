// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceItemDerivation is the golang structure of table plugin_idc_core_billing_invoice_item_derivation for DAO operations like Where/Data.
type BillingInvoiceItemDerivation struct {
	g.Meta                   `orm:"table:plugin_idc_core_billing_invoice_item_derivation, do:true"`
	Id                       any        //
	TenantId                 any        //
	TransformNo              any        //
	SourceInvoiceNo          any        //
	SourceInvoiceItemNo      any        //
	DestinationInvoiceNo     any        //
	DestinationInvoiceItemNo any        //
	CustomerNo               any        //
	CurrencyCode             any        //
	SourceContentDigest      any        //
	DestinationContentDigest any        //
	ContentDigest            any        //
	CreatedAt                *time.Time //
}
