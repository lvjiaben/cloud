// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionSettlement is the golang structure for table sales_commission_settlement.
type SalesCommissionSettlement struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	SettlementNo         string     `json:"settlementNo"         orm:"settlement_no"          description:""`
	AccountNo            string     `json:"accountNo"            orm:"account_no"             description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	PayeeProfileNo       string     `json:"payeeProfileNo"       orm:"payee_profile_no"       description:""`
	BeneficiaryTypeCode  string     `json:"beneficiaryTypeCode"  orm:"beneficiary_type_code"  description:""`
	BeneficiaryReference string     `json:"beneficiaryReference" orm:"beneficiary_reference"  description:""`
	RequestedAmountMinor int64      `json:"requestedAmountMinor" orm:"requested_amount_minor" description:""`
	FrozenAmountMinor    int64      `json:"frozenAmountMinor"    orm:"frozen_amount_minor"    description:""`
	PaidAmountMinor      *int64     `json:"paidAmountMinor"      orm:"paid_amount_minor"      description:""`
	ReturnedAmountMinor  *int64     `json:"returnedAmountMinor"  orm:"returned_amount_minor"  description:""`
	MinimumAmountMinor   int64      `json:"minimumAmountMinor"   orm:"minimum_amount_minor"   description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	RequestDigest        string     `json:"requestDigest"        orm:"request_digest"         description:""`
	RequestedAt          *time.Time `json:"requestedAt"          orm:"requested_at"           description:""`
	CompletedAt          *time.Time `json:"completedAt"          orm:"completed_at"           description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
}
