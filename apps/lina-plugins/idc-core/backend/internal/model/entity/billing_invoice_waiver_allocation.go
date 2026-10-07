// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingInvoiceWaiverAllocation is the golang structure for table billing_invoice_waiver_allocation.
type BillingInvoiceWaiverAllocation struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	WaiverAllocationNo        string     `json:"waiverAllocationNo"        orm:"waiver_allocation_no"         description:""`
	AdjustmentNo              string     `json:"adjustmentNo"              orm:"adjustment_no"                description:""`
	InvoiceNo                 string     `json:"invoiceNo"                 orm:"invoice_no"                   description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                  description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"                description:""`
	LedgerTransactionNo       string     `json:"ledgerTransactionNo"       orm:"ledger_transaction_no"        description:""`
	LedgerTransactionTypeCode string     `json:"ledgerTransactionTypeCode" orm:"ledger_transaction_type_code" description:""`
	AdjustmentTypeCode        string     `json:"adjustmentTypeCode"        orm:"adjustment_type_code"         description:""`
	AmountMinor               int64      `json:"amountMinor"               orm:"amount_minor"                 description:""`
	ContentDigest             string     `json:"contentDigest"             orm:"content_digest"               description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
}
