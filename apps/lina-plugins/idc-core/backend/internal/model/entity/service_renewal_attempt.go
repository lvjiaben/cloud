// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ServiceRenewalAttempt is the golang structure for table service_renewal_attempt.
type ServiceRenewalAttempt struct {
	Id                        int64      `json:"id"                        orm:"id"                           description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                    description:""`
	AttemptNo                 string     `json:"attemptNo"                 orm:"attempt_no"                   description:""`
	OperationNo               string     `json:"operationNo"               orm:"operation_no"                 description:""`
	EntitlementNo             string     `json:"entitlementNo"             orm:"entitlement_no"               description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                  description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"                description:""`
	InvoiceNo                 string     `json:"invoiceNo"                 orm:"invoice_no"                   description:""`
	PolicyNo                  string     `json:"policyNo"                  orm:"policy_no"                    description:""`
	PolicyVersion             int64      `json:"policyVersion"             orm:"policy_version"               description:""`
	AttemptSequence           int        `json:"attemptSequence"           orm:"attempt_sequence"             description:""`
	OutcomeCode               string     `json:"outcomeCode"               orm:"outcome_code"                 description:""`
	LedgerTransactionNo       *string    `json:"ledgerTransactionNo"       orm:"ledger_transaction_no"        description:""`
	LedgerTransactionTypeCode *string    `json:"ledgerTransactionTypeCode" orm:"ledger_transaction_type_code" description:""`
	FailureCode               string     `json:"failureCode"               orm:"failure_code"                 description:""`
	NextAttemptAt             *time.Time `json:"nextAttemptAt"             orm:"next_attempt_at"              description:""`
	EvidenceDigest            string     `json:"evidenceDigest"            orm:"evidence_digest"              description:""`
	ContentDigest             string     `json:"contentDigest"             orm:"content_digest"               description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                   description:""`
}
