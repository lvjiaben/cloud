// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingRefundAllocation is the golang structure for table billing_refund_allocation.
type BillingRefundAllocation struct {
	Id                         int64      `json:"id"                         orm:"id"                            description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                     description:""`
	RefundAllocationNo         string     `json:"refundAllocationNo"         orm:"refund_allocation_no"          description:""`
	InvoiceNo                  string     `json:"invoiceNo"                  orm:"invoice_no"                    description:""`
	InvoiceItemNo              string     `json:"invoiceItemNo"              orm:"invoice_item_no"               description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                   description:""`
	CurrencyCode               string     `json:"currencyCode"               orm:"currency_code"                 description:""`
	PaymentAllocationNo        string     `json:"paymentAllocationNo"        orm:"payment_allocation_no"         description:""`
	RefundSourceTypeCode       string     `json:"refundSourceTypeCode"       orm:"refund_source_type_code"       description:""`
	RefundSourceReference      string     `json:"refundSourceReference"      orm:"refund_source_reference"       description:""`
	LedgerTransactionNo        string     `json:"ledgerTransactionNo"        orm:"ledger_transaction_no"         description:""`
	LedgerTransactionTypeCode  string     `json:"ledgerTransactionTypeCode"  orm:"ledger_transaction_type_code"  description:""`
	DirectionCode              string     `json:"directionCode"              orm:"direction_code"                description:""`
	OriginalRefundAllocationNo *string    `json:"originalRefundAllocationNo" orm:"original_refund_allocation_no" description:""`
	AmountMinor                int64      `json:"amountMinor"                orm:"amount_minor"                  description:""`
	ContentDigest              string     `json:"contentDigest"              orm:"content_digest"                description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                    description:""`
}
