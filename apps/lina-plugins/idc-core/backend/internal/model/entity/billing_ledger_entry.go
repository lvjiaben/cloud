// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingLedgerEntry is the golang structure for table billing_ledger_entry.
type BillingLedgerEntry struct {
	Id            int64      `json:"id"            orm:"id"              description:""`
	TenantId      int        `json:"tenantId"      orm:"tenant_id"       description:""`
	EntryNo       string     `json:"entryNo"       orm:"entry_no"        description:""`
	TransactionNo string     `json:"transactionNo" orm:"transaction_no"  description:""`
	CurrencyCode  string     `json:"currencyCode"  orm:"currency_code"   description:""`
	LineNumber    int        `json:"lineNumber"    orm:"line_number"     description:""`
	AccountNo     string     `json:"accountNo"     orm:"account_no"      description:""`
	DirectionCode string     `json:"directionCode" orm:"direction_code"  description:""`
	EntryRoleCode string     `json:"entryRoleCode" orm:"entry_role_code" description:""`
	AmountMinor   int64      `json:"amountMinor"   orm:"amount_minor"    description:""`
	InvoiceNo     *string    `json:"invoiceNo"     orm:"invoice_no"      description:""`
	InvoiceItemNo *string    `json:"invoiceItemNo" orm:"invoice_item_no" description:""`
	ReasonCode    string     `json:"reasonCode"    orm:"reason_code"     description:""`
	ContentDigest string     `json:"contentDigest" orm:"content_digest"  description:""`
	CreatedAt     *time.Time `json:"createdAt"     orm:"created_at"      description:""`
}
