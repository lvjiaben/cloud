// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingLedgerTransaction is the golang structure for table billing_ledger_transaction.
type BillingLedgerTransaction struct {
	Id                    int64      `json:"id"                    orm:"id"                      description:""`
	TenantId              int        `json:"tenantId"              orm:"tenant_id"               description:""`
	TransactionNo         string     `json:"transactionNo"         orm:"transaction_no"          description:""`
	TransactionTypeCode   string     `json:"transactionTypeCode"   orm:"transaction_type_code"   description:""`
	CurrencyCode          string     `json:"currencyCode"          orm:"currency_code"           description:""`
	SourceTypeCode        string     `json:"sourceTypeCode"        orm:"source_type_code"        description:""`
	SourceReference       string     `json:"sourceReference"       orm:"source_reference"        description:""`
	SourceSnapshotDigest  string     `json:"sourceSnapshotDigest"  orm:"source_snapshot_digest"  description:""`
	OriginalTransactionNo *string    `json:"originalTransactionNo" orm:"original_transaction_no" description:""`
	EntryCount            int        `json:"entryCount"            orm:"entry_count"             description:""`
	DebitTotalMinor       int64      `json:"debitTotalMinor"       orm:"debit_total_minor"       description:""`
	CreditTotalMinor      int64      `json:"creditTotalMinor"      orm:"credit_total_minor"      description:""`
	ContentDigest         string     `json:"contentDigest"         orm:"content_digest"          description:""`
	PostedBy              int        `json:"postedBy"              orm:"posted_by"               description:""`
	PostedAt              *time.Time `json:"postedAt"              orm:"posted_at"               description:""`
	CreatedAt             *time.Time `json:"createdAt"             orm:"created_at"              description:""`
}
