// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package do

import (
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ChannelServiceQuote is the golang structure of table plugin_idc_core_channel_service_quote for DAO operations like Where/Data.
type ChannelServiceQuote struct {
	g.Meta                         `orm:"table:plugin_idc_core_channel_service_quote, do:true"`
	Id                             any        //
	TenantId                       any        //
	QuoteNo                        any        //
	ChannelId                      any        //
	PayerCustomerNo                any        //
	ServiceOwnerCustomerNo         any        //
	ServiceNo                      any        //
	ActionCode                     any        //
	CurrencyCode                   any        //
	DirectionCode                  any        //
	CurrentConfigurationSnapshotNo any        //
	TargetConfigurationSnapshotNo  any        //
	TargetVersionNo                any        //
	TargetBillingRuleNo            any        //
	CurrentRecurringAmountMinor    any        //
	TargetRecurringAmountMinor     any        //
	TargetSubtotalMinor            any        //
	TargetNetAmountMinor           any        //
	TargetTaxAmountMinor           any        //
	SubtotalMinor                  any        //
	NetAmountMinor                 any        //
	TaxAmountMinor                 any        //
	AmountMinor                    any        //
	ProviderParameters             any        //
	ResourceChannelNo              any        //
	ResourcePoolNo                 any        //
	ResourceReservationNo          any        //
	ResourceSnapshotNo             any        //
	IpQuantity                     any        //
	MaximumSnapshotAgeSeconds      any        //
	PolicyNo                       any        //
	PolicyVersion                  any        //
	OldIpSetDigest                 any        //
	ExpectedServiceVersion         any        //
	CustomerSubjectVersion         any        //
	PayerCustomerSubjectVersion    any        //
	IdempotencyDigest              any        //
	RequestDigest                  any        //
	PricingRuleDigest              any        //
	ContentDigest                  any        //
	EffectiveAt                    *time.Time //
	IssuedAt                       *time.Time //
	ExpiresAt                      *time.Time //
	CreatedAt                      *time.Time //
	SourceCode                     any        // Actual admission owner: channel retains real attribution; native has no channel identity
	NativeTargetSnapshot           any        //
	PreparedPrice                  any        // Immutable core price, configuration, merchant policy and local IP reservation facts for native admission
	NativeTargetQuoteNo            any        //
}
