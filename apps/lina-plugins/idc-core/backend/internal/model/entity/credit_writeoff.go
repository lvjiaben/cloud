// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditWriteoff is the golang structure for table credit_writeoff.
type CreditWriteoff struct {
	Id                     int64      `json:"id"                     orm:"id"                       description:""`
	TenantId               int        `json:"tenantId"               orm:"tenant_id"                description:""`
	WriteoffNo             string     `json:"writeoffNo"             orm:"writeoff_no"              description:""`
	StatementNo            string     `json:"statementNo"            orm:"statement_no"             description:""`
	CreditAccountNo        string     `json:"creditAccountNo"        orm:"credit_account_no"        description:""`
	CustomerNo             string     `json:"customerNo"             orm:"customer_no"              description:""`
	CurrencyCode           string     `json:"currencyCode"           orm:"currency_code"            description:""`
	WriteoffTypeCode       string     `json:"writeoffTypeCode"       orm:"writeoff_type_code"       description:""`
	AmountMinor            int64      `json:"amountMinor"            orm:"amount_minor"             description:""`
	OriginalWriteoffNo     *string    `json:"originalWriteoffNo"     orm:"original_writeoff_no"     description:""`
	LedgerTransactionNo    string     `json:"ledgerTransactionNo"    orm:"ledger_transaction_no"    description:""`
	ReasonCode             string     `json:"reasonCode"             orm:"reason_code"              description:""`
	RequestedBy            int        `json:"requestedBy"            orm:"requested_by"             description:""`
	ReviewedBy             int        `json:"reviewedBy"             orm:"reviewed_by"              description:""`
	ApprovalEvidenceDigest string     `json:"approvalEvidenceDigest" orm:"approval_evidence_digest" description:""`
	ContentDigest          string     `json:"contentDigest"          orm:"content_digest"           description:""`
	PostedAt               *time.Time `json:"postedAt"               orm:"posted_at"                description:""`
	CreatedAt              *time.Time `json:"createdAt"              orm:"created_at"               description:""`
}
