// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingAdjustmentRequest is the golang structure for table billing_adjustment_request.
type BillingAdjustmentRequest struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	AdjustmentNo              string     `json:"adjustmentNo"              orm:"adjustment_no"                description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                  description:""`
	InvoiceNo                 *string    `json:"invoiceNo"                 orm:"invoice_no"                   description:""`
	AccountNo                 *string    `json:"accountNo"                 orm:"account_no"                   description:""`
	AccountTypeCode           *string    `json:"accountTypeCode"           orm:"account_type_code"            description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"                description:""`
	AdjustmentTypeCode        string     `json:"adjustmentTypeCode"        orm:"adjustment_type_code"         description:""`
	AmountMinor               int64      `json:"amountMinor"               orm:"amount_minor"                 description:""`
	ReasonCode                string     `json:"reasonCode"                orm:"reason_code"                  description:""`
	EvidenceDigest            string     `json:"evidenceDigest"            orm:"evidence_digest"              description:""`
	IdempotencyDigest         string     `json:"idempotencyDigest"         orm:"idempotency_digest"           description:""`
	RequestDigest             string     `json:"requestDigest"             orm:"request_digest"               description:""`
	ApprovalRequired          bool       `json:"approvalRequired"          orm:"approval_required"            description:""`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                  description:""`
	RequestedBy               int        `json:"requestedBy"               orm:"requested_by"                 description:""`
	LedgerTransactionNo       *string    `json:"ledgerTransactionNo"       orm:"ledger_transaction_no"        description:""`
	LedgerTransactionTypeCode *string    `json:"ledgerTransactionTypeCode" orm:"ledger_transaction_type_code" description:""`
	PostedAt                  *time.Time `json:"postedAt"                  orm:"posted_at"                    description:""`
	Version                   int64      `json:"version"                   orm:"version"                      description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                   description:""`
	ProofReference            string     `json:"proofReference"            orm:"proof_reference"              description:"Verified receipt evidence or the exact marketing value-grant business number"`
	ReceivingAccountReference string     `json:"receivingAccountReference" orm:"receiving_account_reference"  description:""`
	BusinessPurposeCode       string     `json:"businessPurposeCode"       orm:"business_purpose_code"        description:""`
	PolicyNo                  string     `json:"policyNo"                  orm:"policy_no"                    description:""`
	PolicyVersion             int64      `json:"policyVersion"             orm:"policy_version"               description:""`
	CreditStatementNo         string     `json:"creditStatementNo"         orm:"credit_statement_no"          description:""`
}
