// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// SalesAgentCreditLimit is the golang structure for table sales_agent_credit_limit.
type SalesAgentCreditLimit struct {
	Id               int64      `json:"id"               orm:"id"                 description:""`
	TenantId         int        `json:"tenantId"         orm:"tenant_id"          description:""`
	LimitNo          string     `json:"limitNo"          orm:"limit_no"           description:""`
	QualificationNo  string     `json:"qualificationNo"  orm:"qualification_no"   description:""`
	CurrencyCode     string     `json:"currencyCode"     orm:"currency_code"      description:""`
	LimitAmountMinor int64      `json:"limitAmountMinor" orm:"limit_amount_minor" description:""`
	StatusCode       string     `json:"statusCode"       orm:"status_code"        description:""`
	EffectiveAt      *time.Time `json:"effectiveAt"      orm:"effective_at"       description:""`
	ExpiresAt        *time.Time `json:"expiresAt"        orm:"expires_at"         description:""`
	CreatedAt        *time.Time `json:"createdAt"        orm:"created_at"         description:""`
}
