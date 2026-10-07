// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// MarketingValueGrant is the golang structure for table marketing_value_grant.
type MarketingValueGrant struct {
	Id                        int64      `json:"id"                        orm:"id"                          description:""`
	TenantId                  int        `json:"tenantId"                  orm:"tenant_id"                   description:""`
	GrantNo                   string     `json:"grantNo"                   orm:"grant_no"                    description:""`
	GrantTypeCode             string     `json:"grantTypeCode"             orm:"grant_type_code"             description:""`
	CampaignNo                string     `json:"campaignNo"                orm:"campaign_no"                 description:""`
	CampaignTypeCode          string     `json:"campaignTypeCode"          orm:"campaign_type_code"          description:""`
	BenefitNo                 string     `json:"benefitNo"                 orm:"benefit_no"                  description:""`
	BenefitTypeCode           string     `json:"benefitTypeCode"           orm:"benefit_type_code"           description:""`
	BenefitSnapshotDigest     string     `json:"benefitSnapshotDigest"     orm:"benefit_snapshot_digest"     description:""`
	CustomerNo                string     `json:"customerNo"                orm:"customer_no"                 description:""`
	CurrencyCode              string     `json:"currencyCode"              orm:"currency_code"               description:""`
	SourceReference           string     `json:"sourceReference"           orm:"source_reference"            description:""`
	OriginalAmountMinor       int64      `json:"originalAmountMinor"       orm:"original_amount_minor"       description:""`
	PendingAmountMinor        int64      `json:"pendingAmountMinor"        orm:"pending_amount_minor"        description:""`
	AvailableAmountMinor      int64      `json:"availableAmountMinor"      orm:"available_amount_minor"      description:""`
	FrozenAmountMinor         int64      `json:"frozenAmountMinor"         orm:"frozen_amount_minor"         description:""`
	ConsumedAmountMinor       int64      `json:"consumedAmountMinor"       orm:"consumed_amount_minor"       description:""`
	ExpiredAmountMinor        int64      `json:"expiredAmountMinor"        orm:"expired_amount_minor"        description:""`
	ReversedAmountMinor       int64      `json:"reversedAmountMinor"       orm:"reversed_amount_minor"       description:""`
	Withdrawable              bool       `json:"withdrawable"              orm:"withdrawable"                description:""`
	GlobalQuotaReserved       bool       `json:"globalQuotaReserved"       orm:"global_quota_reserved"       description:""`
	CustomerQuotaReserved     bool       `json:"customerQuotaReserved"     orm:"customer_quota_reserved"     description:""`
	ExpiresAt                 *time.Time `json:"expiresAt"                 orm:"expires_at"                  description:""`
	StatusCode                string     `json:"statusCode"                orm:"status_code"                 description:""`
	FinancialPostingReference *string    `json:"financialPostingReference" orm:"financial_posting_reference" description:""`
	CreatedAt                 *time.Time `json:"createdAt"                 orm:"created_at"                  description:""`
	UpdatedAt                 *time.Time `json:"updatedAt"                 orm:"updated_at"                  description:""`
}
