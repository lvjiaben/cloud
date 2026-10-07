// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CommerceQuoteChargeSnapshot is the golang structure for table commerce_quote_charge_snapshot.
type CommerceQuoteChargeSnapshot struct {
	Id                            int64      `json:"id"                            orm:"id"                               description:""`
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	QuoteChargeNo                 string     `json:"quoteChargeNo"                 orm:"quote_charge_no"                  description:""`
	QuoteItemNo                   string     `json:"quoteItemNo"                   orm:"quote_item_no"                    description:""`
	CurrencyCode                  string     `json:"currencyCode"                  orm:"currency_code"                    description:""`
	LineNumber                    int        `json:"lineNumber"                    orm:"line_number"                      description:""`
	ChargeKindCode                string     `json:"chargeKindCode"                orm:"charge_kind_code"                 description:""`
	ChargePhaseCode               string     `json:"chargePhaseCode"               orm:"charge_phase_code"                description:""`
	ConfigurationSnapshotNo       *string    `json:"configurationSnapshotNo"       orm:"configuration_snapshot_no"        description:""`
	OptionNo                      *string    `json:"optionNo"                      orm:"option_no"                        description:""`
	Quantity                      int        `json:"quantity"                      orm:"quantity"                         description:""`
	ChargeAmountMinor             int64      `json:"chargeAmountMinor"             orm:"charge_amount_minor"              description:""`
	SetupFeeMinor                 int64      `json:"setupFeeMinor"                 orm:"setup_fee_minor"                  description:""`
	PricedAmountMinor             int64      `json:"pricedAmountMinor"             orm:"priced_amount_minor"              description:""`
	PriceSourceDigest             string     `json:"priceSourceDigest"             orm:"price_source_digest"              description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	ConfigurationPriceDimensionNo string     `json:"configurationPriceDimensionNo" orm:"configuration_price_dimension_no" description:""`
}
