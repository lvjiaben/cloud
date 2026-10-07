// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// MarketingValueGrant is the golang structure of table plugin_idc_core_marketing_value_grant for DAO operations like Where/Data.
type MarketingValueGrant struct {
	g.Meta                    `orm:"table:plugin_idc_core_marketing_value_grant, do:true"`
	Id                        any        //
	TenantId                  any        //
	GrantNo                   any        //
	GrantTypeCode             any        //
	CampaignNo                any        //
	CampaignTypeCode          any        //
	BenefitNo                 any        //
	BenefitTypeCode           any        //
	BenefitSnapshotDigest     any        //
	CustomerNo                any        //
	CurrencyCode              any        //
	SourceReference           any        //
	OriginalAmountMinor       any        //
	PendingAmountMinor        any        //
	AvailableAmountMinor      any        //
	FrozenAmountMinor         any        //
	ConsumedAmountMinor       any        //
	ExpiredAmountMinor        any        //
	ReversedAmountMinor       any        //
	Withdrawable              any        //
	GlobalQuotaReserved       any        //
	CustomerQuotaReserved     any        //
	ExpiresAt                 *time.Time //
	StatusCode                any        //
	FinancialPostingReference *string    //
	CreatedAt                 *time.Time //
	UpdatedAt                 *time.Time //
}
