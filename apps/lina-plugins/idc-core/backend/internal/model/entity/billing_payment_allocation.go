// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingPaymentAllocation is the golang structure for table billing_payment_allocation.
type BillingPaymentAllocation struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	AllocationNo              string     `json:"allocationNo"              orm:"allocation_no"                description:""`
	InvoiceNo                 string     `json:"invoiceNo"                 orm:"invoice_no"                   description:""`
	InvoiceItemNo             *string    `json:"invoiceItemNo"             orm:"invoice_item_no"              description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                  description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"                description:""`
	PaymentSourceTypeCode     string     `json:"paymentSourceTypeCode"     orm:"payment_source_type_code"     description:""`
	PaymentSourceReference    string     `json:"paymentSourceReference"    orm:"payment_source_reference"     description:""`
	LedgerTransactionNo       string     `json:"ledgerTransactionNo"       orm:"ledger_transaction_no"        description:""`
	LedgerTransactionTypeCode string     `json:"ledgerTransactionTypeCode" orm:"ledger_transaction_type_code" description:""`
	DirectionCode             string     `json:"directionCode"             orm:"direction_code"               description:""`
	OriginalAllocationNo      *string    `json:"originalAllocationNo"      orm:"original_allocation_no"       description:""`
	AmountMinor               int64      `json:"amountMinor"               orm:"amount_minor"                 description:""`
	ContentDigest             string     `json:"contentDigest"             orm:"content_digest"               description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
}
