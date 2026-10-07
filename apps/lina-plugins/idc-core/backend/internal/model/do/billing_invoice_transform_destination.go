// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceTransformDestination is the golang structure of table plugin_idc_core_billing_invoice_transform_destination for DAO operations like Where/Data.
type BillingInvoiceTransformDestination struct {
	g.Meta                   `orm:"table:plugin_idc_core_billing_invoice_transform_destination, do:true"`
	Id                       any        //
	TenantId                 any        //
	TransformNo              any        //
	DestinationInvoiceNo     any        //
	CustomerNo               any        //
	CurrencyCode             any        //
	Position                 any        //
	DestinationContentDigest any        //
	ContentDigest            any        //
	CreatedAt                *time.Time //
}
