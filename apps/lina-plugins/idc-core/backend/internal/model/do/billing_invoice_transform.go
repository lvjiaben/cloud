// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoiceTransform is the golang structure of table plugin_idc_core_billing_invoice_transform for DAO operations like Where/Data.
type BillingInvoiceTransform struct {
	g.Meta            `orm:"table:plugin_idc_core_billing_invoice_transform, do:true"`
	Id                any        //
	TenantId          any        //
	TransformNo       any        //
	TransformTypeCode any        //
	CustomerNo        any        //
	CurrencyCode      any        //
	SourceCount       any        //
	DestinationCount  any        //
	ItemCount         any        //
	IdempotencyDigest any        //
	RequestDigest     any        //
	ContentDigest     any        //
	RequestedBy       any        //
	CompletedAt       *time.Time //
	CreatedAt         *time.Time //
}
