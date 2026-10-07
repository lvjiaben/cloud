// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditRepayment is the golang structure for table credit_repayment.
type CreditRepayment struct {
	Id                         int64      `json:"id"                         orm:"id"                           description:""`
	TenantId                   int        `json:"tenantId"                   orm:"tenant_id"                    description:""`
	RepaymentNo                string     `json:"repaymentNo"                orm:"repayment_no"                 description:""`
	CreditAccountNo            string     `json:"creditAccountNo"            orm:"credit_account_no"            description:""`
	CustomerNo                 string     `json:"customerNo"                 orm:"customer_no"                  description:""`
	CurrencyCode               string     `json:"currencyCode"               orm:"currency_code"                description:""`
	RepaymentTypeCode          string     `json:"repaymentTypeCode"          orm:"repayment_type_code"          description:""`
	FundingSourceTypeCode      string     `json:"fundingSourceTypeCode"      orm:"funding_source_type_code"     description:""`
	FundingSourceReference     string     `json:"fundingSourceReference"     orm:"funding_source_reference"     description:""`
	LedgerTransactionNo        *string    `json:"ledgerTransactionNo"        orm:"ledger_transaction_no"        description:""`
	AmountMinor                int64      `json:"amountMinor"                orm:"amount_minor"                 description:""`
	AllocatedAmountMinor       int64      `json:"allocatedAmountMinor"       orm:"allocated_amount_minor"       description:""`
	UnallocatedAmountMinor     int64      `json:"unallocatedAmountMinor"     orm:"unallocated_amount_minor"     description:""`
	RefundedAmountMinor        int64      `json:"refundedAmountMinor"        orm:"refunded_amount_minor"        description:""`
	StatusCode                 string     `json:"statusCode"                 orm:"status_code"                  description:""`
	SourceSnapshotDigest       string     `json:"sourceSnapshotDigest"       orm:"source_snapshot_digest"       description:""`
	ConfirmationEvidenceDigest *string    `json:"confirmationEvidenceDigest" orm:"confirmation_evidence_digest" description:""`
	ConfirmedAt                *time.Time `json:"confirmedAt"                orm:"confirmed_at"                 description:""`
	FailedAt                   *time.Time `json:"failedAt"                   orm:"failed_at"                    description:""`
	RefundedAt                 *time.Time `json:"refundedAt"                 orm:"refunded_at"                  description:""`
	Version                    int64      `json:"version"                    orm:"version"                      description:""`
	CreatedAt                  *time.Time `json:"createdAt"                  orm:"created_at"                   description:""`
	UpdatedAt                  *time.Time `json:"updatedAt"                  orm:"updated_at"                   description:""`
}
