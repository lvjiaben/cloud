// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionMaturity is the golang structure for table sales_commission_maturity.
type SalesCommissionMaturity struct {
	Id                   int64      `json:"id"                   orm:"id"                    description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"             description:""`
	MaturityNo           string     `json:"maturityNo"           orm:"maturity_no"           description:""`
	CommissionNo         string     `json:"commissionNo"         orm:"commission_no"         description:""`
	AttemptNo            int        `json:"attemptNo"            orm:"attempt_no"            description:""`
	AccountNo            string     `json:"accountNo"            orm:"account_no"            description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"         description:""`
	BeneficiaryTypeCode  string     `json:"beneficiaryTypeCode"  orm:"beneficiary_type_code" description:""`
	BeneficiaryReference string     `json:"beneficiaryReference" orm:"beneficiary_reference" description:""`
	AmountMinor          int64      `json:"amountMinor"          orm:"amount_minor"          description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"           description:""`
	ReasonCode           string     `json:"reasonCode"           orm:"reason_code"           description:""`
	EvidenceDigest       string     `json:"evidenceDigest"       orm:"evidence_digest"       description:""`
	ProcessedAt          *time.Time `json:"processedAt"          orm:"processed_at"          description:""`
}
