// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceTransformSource is the golang structure of table plugin_idc_core_billing_invoice_transform_source for DAO operations like Where/Data.
type BillingInvoiceTransformSource struct {
	g.Meta              `orm:"table:plugin_idc_core_billing_invoice_transform_source, do:true"`
	Id                  any        //
	TenantId            any        //
	TransformNo         any        //
	SourceInvoiceNo     any        //
	CustomerNo          any        //
	CurrencyCode        any        //
	Position            any        //
	SourceContentDigest any        //
	ContentDigest       any        //
	CreatedAt           *time.Time //
}
