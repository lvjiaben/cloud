// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CreditAccount is the golang structure for table credit_account.
type CreditAccount struct {
	Id                       int64      `json:"id"                       orm:"id"                          description:""`
	TenantId                 int        `json:"tenantId"                 orm:"tenant_id"                   description:""`
	CreditAccountNo          string     `json:"creditAccountNo"          orm:"credit_account_no"           description:""`
	CustomerNo               string     `json:"customerNo"               orm:"customer_no"                 description:""`
	CurrencyCode             string     `json:"currencyCode"             orm:"currency_code"               description:""`
	LedgerAccountNo          string     `json:"ledgerAccountNo"          orm:"ledger_account_no"           description:""`
	LedgerAccountTypeCode    string     `json:"ledgerAccountTypeCode"    orm:"ledger_account_type_code"    description:""`
	OriginApplicationNo      string     `json:"originApplicationNo"      orm:"origin_application_no"       description:""`
	CurrentTermsNo           string     `json:"currentTermsNo"           orm:"current_terms_no"            description:""`
	CurrentTermsVersion      int64      `json:"currentTermsVersion"      orm:"current_terms_version"       description:""`
	StatusCode               string     `json:"statusCode"               orm:"status_code"                 description:""`
	TotalLimitMinor          int64      `json:"totalLimitMinor"          orm:"total_limit_minor"           description:""`
	AvailableLimitMinor      int64      `json:"availableLimitMinor"      orm:"available_limit_minor"       description:""`
	UnbilledPrincipalMinor   int64      `json:"unbilledPrincipalMinor"   orm:"unbilled_principal_minor"    description:""`
	BilledPrincipalMinor     int64      `json:"billedPrincipalMinor"     orm:"billed_principal_minor"      description:""`
	DelinquentPrincipalMinor int64      `json:"delinquentPrincipalMinor" orm:"delinquent_principal_minor"  description:""`
	WrittenOffPrincipalMinor int64      `json:"writtenOffPrincipalMinor" orm:"written_off_principal_minor" description:""`
	ValidFrom                *time.Time `json:"validFrom"                orm:"valid_from"                  description:""`
	ValidUntil               *time.Time `json:"validUntil"               orm:"valid_until"                 description:""`
	FreezeReasonCode         *string    `json:"freezeReasonCode"         orm:"freeze_reason_code"          description:""`
	FrozenAt                 *time.Time `json:"frozenAt"                 orm:"frozen_at"                   description:""`
	ClosedAt                 *time.Time `json:"closedAt"                 orm:"closed_at"                   description:""`
	Version                  int64      `json:"version"                  orm:"version"                     description:""`
	CreatedAt                *time.Time `json:"createdAt"                orm:"created_at"                  description:""`
	UpdatedAt                *time.Time `json:"updatedAt"                orm:"updated_at"                  description:""`
}
