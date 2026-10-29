// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// ChannelServiceQuote is the golang structure for table channel_service_quote.
type ChannelServiceQuote struct {
	Id                             int64      `json:"id"                             orm:"id"                                description:""`
	TenantId                       int        `json:"tenantId"                       orm:"tenant_id"                         description:""`
	QuoteNo                        string     `json:"quoteNo"                        orm:"quote_no"                          description:""`
	ChannelId                      string     `json:"channelId"                      orm:"channel_id"                        description:""`
	PayerCustomerNo                string     `json:"payerCustomerNo"                orm:"payer_customer_no"                 description:""`
	ServiceOwnerCustomerNo         string     `json:"serviceOwnerCustomerNo"         orm:"service_owner_customer_no"         description:""`
	ServiceNo                      string     `json:"serviceNo"                      orm:"service_no"                        description:""`
	ActionCode                     string     `json:"actionCode"                     orm:"action_code"                       description:""`
	CurrencyCode                   string     `json:"currencyCode"                   orm:"currency_code"                     description:""`
	DirectionCode                  string     `json:"directionCode"                  orm:"direction_code"                    description:""`
	CurrentConfigurationSnapshotNo string     `json:"currentConfigurationSnapshotNo" orm:"current_configuration_snapshot_no" description:""`
	TargetConfigurationSnapshotNo  string     `json:"targetConfigurationSnapshotNo"  orm:"target_configuration_snapshot_no"  description:""`
	TargetVersionNo                string     `json:"targetVersionNo"                orm:"target_version_no"                 description:""`
	TargetBillingRuleNo            string     `json:"targetBillingRuleNo"            orm:"target_billing_rule_no"            description:""`
	CurrentRecurringAmountMinor    int64      `json:"currentRecurringAmountMinor"    orm:"current_recurring_amount_minor"    description:""`
	TargetRecurringAmountMinor     int64      `json:"targetRecurringAmountMinor"     orm:"target_recurring_amount_minor"     description:""`
	TargetSubtotalMinor            int64      `json:"targetSubtotalMinor"            orm:"target_subtotal_minor"             description:""`
	TargetNetAmountMinor           int64      `json:"targetNetAmountMinor"           orm:"target_net_amount_minor"           description:""`
	TargetTaxAmountMinor           int64      `json:"targetTaxAmountMinor"           orm:"target_tax_amount_minor"           description:""`
	SubtotalMinor                  int64      `json:"subtotalMinor"                  orm:"subtotal_minor"                    description:""`
	NetAmountMinor                 int64      `json:"netAmountMinor"                 orm:"net_amount_minor"                  description:""`
	TaxAmountMinor                 int64      `json:"taxAmountMinor"                 orm:"tax_amount_minor"                  description:""`
	AmountMinor                    int64      `json:"amountMinor"                    orm:"amount_minor"                      description:""`
	ProviderParameters             string     `json:"providerParameters"             orm:"provider_parameters"               description:""`
	ResourceChannelNo              string     `json:"resourceChannelNo"              orm:"resource_channel_no"               description:""`
	ResourcePoolNo                 string     `json:"resourcePoolNo"                 orm:"resource_pool_no"                  description:""`
	ResourceReservationNo          string     `json:"resourceReservationNo"          orm:"resource_reservation_no"           description:""`
	ResourceSnapshotNo             string     `json:"resourceSnapshotNo"             orm:"resource_snapshot_no"              description:""`
	IpQuantity                     int        `json:"ipQuantity"                     orm:"ip_quantity"                       description:""`
	MaximumSnapshotAgeSeconds      int        `json:"maximumSnapshotAgeSeconds"      orm:"maximum_snapshot_age_seconds"      description:""`
	PolicyNo                       string     `json:"policyNo"                       orm:"policy_no"                         description:""`
	PolicyVersion                  int64      `json:"policyVersion"                  orm:"policy_version"                    description:""`
	OldIpSetDigest                 string     `json:"oldIpSetDigest"                 orm:"old_ip_set_digest"                 description:""`
	ExpectedServiceVersion         int64      `json:"expectedServiceVersion"         orm:"expected_service_version"          description:""`
	CustomerSubjectVersion         int64      `json:"customerSubjectVersion"         orm:"customer_subject_version"          description:""`
	PayerCustomerSubjectVersion    int64      `json:"payerCustomerSubjectVersion"    orm:"payer_customer_subject_version"    description:""`
	IdempotencyDigest              string     `json:"idempotencyDigest"              orm:"idempotency_digest"                description:""`
	RequestDigest                  string     `json:"requestDigest"                  orm:"request_digest"                    description:""`
	PricingRuleDigest              string     `json:"pricingRuleDigest"              orm:"pricing_rule_digest"               description:""`
	ContentDigest                  string     `json:"contentDigest"                  orm:"content_digest"                    description:""`
	EffectiveAt                    *time.Time `json:"effectiveAt"                    orm:"effective_at"                      description:""`
	IssuedAt                       *time.Time `json:"issuedAt"                       orm:"issued_at"                         description:""`
	ExpiresAt                      *time.Time `json:"expiresAt"                      orm:"expires_at"                        description:""`
	CreatedAt                      *time.Time `json:"createdAt"                      orm:"created_at"                        description:""`
	SourceCode                     string     `json:"sourceCode"                     orm:"source_code"                       description:"Actual admission owner: channel retains real attribution; native has no channel identity"`
	NativeTargetSnapshot           string     `json:"nativeTargetSnapshot"           orm:"native_target_snapshot"            description:""`
	PreparedPrice                  string     `json:"preparedPrice"                  orm:"prepared_price"                    description:"Immutable core price, configuration, merchant policy and local IP reservation facts for native admission"`
	NativeTargetQuoteNo            string     `json:"nativeTargetQuoteNo"            orm:"native_target_quote_no"            description:""`
}
