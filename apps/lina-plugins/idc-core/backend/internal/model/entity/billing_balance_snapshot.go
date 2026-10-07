// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingBalanceSnapshot is the golang structure for table billing_balance_snapshot.
type BillingBalanceSnapshot struct {
	Id                  int64      `json:"id"                  orm:"id"                    description:""`
	TenantId            int        `json:"tenantId"            orm:"tenant_id"             description:""`
	SnapshotNo          string     `json:"snapshotNo"          orm:"snapshot_no"           description:""`
	AccountNo           string     `json:"accountNo"           orm:"account_no"            description:""`
	CurrencyCode        string     `json:"currencyCode"        orm:"currency_code"         description:""`
	NormalBalanceCode   string     `json:"normalBalanceCode"   orm:"normal_balance_code"   description:""`
	TransactionNo       string     `json:"transactionNo"       orm:"transaction_no"        description:""`
	SequenceNo          int64      `json:"sequenceNo"          orm:"sequence_no"           description:""`
	PreviousSnapshotNo  *string    `json:"previousSnapshotNo"  orm:"previous_snapshot_no"  description:""`
	PreviousSequenceNo  *int64     `json:"previousSequenceNo"  orm:"previous_sequence_no"  description:""`
	OpeningBalanceMinor int64      `json:"openingBalanceMinor" orm:"opening_balance_minor" description:""`
	DebitAmountMinor    int64      `json:"debitAmountMinor"    orm:"debit_amount_minor"    description:""`
	CreditAmountMinor   int64      `json:"creditAmountMinor"   orm:"credit_amount_minor"   description:""`
	ClosingBalanceMinor int64      `json:"closingBalanceMinor" orm:"closing_balance_minor" description:""`
	ContentDigest       string     `json:"contentDigest"       orm:"content_digest"        description:""`
	CreatedAt           *time.Time `json:"createdAt"           orm:"created_at"            description:""`
}
