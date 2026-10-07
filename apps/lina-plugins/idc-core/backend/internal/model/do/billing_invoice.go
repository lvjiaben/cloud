// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// BillingInvoice is the golang structure of table plugin_idc_core_billing_invoice for DAO operations like Where/Data.
type BillingInvoice struct {
	g.Meta                `orm:"table:plugin_idc_core_billing_invoice, do:true"`
	Id                    any        //
	TenantId              any        //
	InvoiceNo             any        //
	CustomerNo            any        //
	OrderNo               *string    //
	SourceCode            any        //
	CurrencyCode          any        //
	StatusCode            any        //
	ItemCount             any        //
	SubtotalMinor         any        //
	DiscountMinor         any        //
	NetAmountMinor        any        //
	TaxAmountMinor        any        //
	TotalAmountMinor      any        //
	PaidAmountMinor       any        //
	RefundableAmountMinor any        //
	RefundedAmountMinor   any        //
	SourceSnapshotDigest  any        //
	ContentDigest         any        //
	PaymentDueAt          *time.Time //
	IssuedAt              *time.Time //
	SettledAt             *time.Time //
	CancelledAt           *time.Time //
	Version               any        //
	CreatedAt             *time.Time //
	UpdatedAt             *time.Time //
	WaivedAmountMinor     any        // Effective reviewed waiver total; never counted as customer payment
	CreditAmountMinor     any        //
}
