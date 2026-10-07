// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// BillingLedgerAccount is the golang structure for table billing_ledger_account.
type BillingLedgerAccount struct {
	Id                int64      `json:"id"                orm:"id"                  description:""`
	TenantId          int        `json:"tenantId"          orm:"tenant_id"           description:""`
	AccountNo         string     `json:"accountNo"         orm:"account_no"          description:""`
	OwnerTypeCode     string     `json:"ownerTypeCode"     orm:"owner_type_code"     description:""`
	OwnerReference    string     `json:"ownerReference"    orm:"owner_reference"     description:""`
	CustomerNo        *string    `json:"customerNo"        orm:"customer_no"         description:""`
	AccountTypeCode   string     `json:"accountTypeCode"   orm:"account_type_code"   description:""`
	NormalBalanceCode string     `json:"normalBalanceCode" orm:"normal_balance_code" description:""`
	CurrencyCode      string     `json:"currencyCode"      orm:"currency_code"       description:""`
	StatusCode        string     `json:"statusCode"        orm:"status_code"         description:""`
	Withdrawable      bool       `json:"withdrawable"      orm:"withdrawable"        description:""`
	ClosedAt          *time.Time `json:"closedAt"          orm:"closed_at"           description:""`
	Version           int64      `json:"version"           orm:"version"             description:""`
	CreatedAt         *time.Time `json:"createdAt"         orm:"created_at"          description:""`
	UpdatedAt         *time.Time `json:"updatedAt"         orm:"updated_at"          description:""`
}
