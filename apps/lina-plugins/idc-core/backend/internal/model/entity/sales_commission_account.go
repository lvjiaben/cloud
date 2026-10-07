// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesCommissionAccount is the golang structure for table sales_commission_account.
type SalesCommissionAccount struct {
	Id                   int64      `json:"id"                   orm:"id"                     description:""`
	TenantId             int        `json:"tenantId"             orm:"tenant_id"              description:""`
	AccountNo            string     `json:"accountNo"            orm:"account_no"             description:""`
	BeneficiaryTypeCode  string     `json:"beneficiaryTypeCode"  orm:"beneficiary_type_code"  description:""`
	BeneficiaryReference string     `json:"beneficiaryReference" orm:"beneficiary_reference"  description:""`
	CurrencyCode         string     `json:"currencyCode"         orm:"currency_code"          description:""`
	CreditedAmountMinor  int64      `json:"creditedAmountMinor"  orm:"credited_amount_minor"  description:""`
	PendingAmountMinor   int64      `json:"pendingAmountMinor"   orm:"pending_amount_minor"   description:""`
	AvailableAmountMinor int64      `json:"availableAmountMinor" orm:"available_amount_minor" description:""`
	FrozenAmountMinor    int64      `json:"frozenAmountMinor"    orm:"frozen_amount_minor"    description:""`
	PaidAmountMinor      int64      `json:"paidAmountMinor"      orm:"paid_amount_minor"      description:""`
	ReversedAmountMinor  int64      `json:"reversedAmountMinor"  orm:"reversed_amount_minor"  description:""`
	StatusCode           string     `json:"statusCode"           orm:"status_code"            description:""`
	Version              int64      `json:"version"              orm:"version"                description:""`
	CreatedAt            *time.Time `json:"createdAt"            orm:"created_at"             description:""`
	UpdatedAt            *time.Time `json:"updatedAt"            orm:"updated_at"             description:""`
}
