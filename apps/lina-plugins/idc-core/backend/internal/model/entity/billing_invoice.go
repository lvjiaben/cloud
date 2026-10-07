// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoice is the golang structure for table billing_invoice.
type BillingInvoice struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	InvoiceNo             string     `json:"invoiceNo"             orm:"invoice_no"              description:""`
	CustomerNo            string     `json:"customerNo"            orm:"customer_no"             description:""`
	OrderNo               *string    `json:"orderNo"               orm:"order_no"                description:""`
	SourceCode            string     `json:"sourceCode"            orm:"source_code"             description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	StatusCode            string     `json:"statusCode"            orm:"status_code"             description:""`
	ItemCount             int        `json:"itemCount"             orm:"item_count"              description:""`
	SubtotalMinor         int64      `json:"subtotalMinor"         orm:"subtotal_minor"          description:""`
	DiscountMinor         int64      `json:"discountMinor"         orm:"discount_minor"          description:""`
	NetAmountMinor        int64      `json:"netAmountMinor"        orm:"net_amount_minor"        description:""`
	TaxAmountMinor        int64      `json:"taxAmountMinor"        orm:"tax_amount_minor"        description:""`
	TotalAmountMinor      int64      `json:"totalAmountMinor"      orm:"total_amount_minor"      description:""`
	PaidAmountMinor       int64      `json:"paidAmountMinor"       orm:"paid_amount_minor"       description:""`
	RefundableAmountMinor int64      `json:"refundableAmountMinor" orm:"refundable_amount_minor" description:""`
	RefundedAmountMinor   int64      `json:"refundedAmountMinor"   orm:"refunded_amount_minor"   description:""`
	SourceSnapshotDigest  string     `json:"sourceSnapshotDigest"  orm:"source_snapshot_digest"  description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"          description:""`
	PaymentDueAt          *time.Time `json:"paymentDueAt"          orm:"payment_due_at"          description:""`
	IssuedAt              *time.Time `json:"issuedAt"              orm:"issued_at"               description:""`
	SettledAt             *time.Time `json:"settledAt"             orm:"settled_at"              description:""`
	CancelledAt           *time.Time `json:"cancelledAt"           orm:"cancelled_at"            description:""`
	Version               int64      `json:"version"               orm:"version"                 description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
	UpdatedAt             *time.Time `json:"updatedAt"             orm:"updated_at"              description:""`
	WaivedAmountMinor     int64      `json:"waivedAmountMinor"     orm:"waived_amount_minor"     description:"Effective reviewed waiver total; never counted as customer payment"`
	CreditAmountMinor     int64      `json:"creditAmountMinor"     orm:"credit_amount_minor"     description:""`
}
