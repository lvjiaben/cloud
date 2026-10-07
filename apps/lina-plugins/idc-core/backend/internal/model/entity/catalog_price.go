// =================================================================================
// Code generated and maintained by GoFrame CLI tool. DO NOT EDIT.
// =================================================================================

package entity

import (
	"time"
)

// CatalogPrice is the golang structure for table catalog_price.
type CatalogPrice struct {
	Id                            int64      `json:"id"                            orm:"id"                               description:""`
	TenantId                      int        `json:"tenantId"                      orm:"tenant_id"                        description:""`
	PriceNo                       string     `json:"priceNo"                       orm:"price_no"                         description:""`
	VersionNo                     string     `json:"versionNo"                     orm:"version_no"                       description:""`
	BillingRuleNo                 string     `json:"billingRuleNo"                 orm:"billing_rule_no"                  description:""`
	OptionNo                      *string    `json:"optionNo"                      orm:"option_no"                        description:"Null for the product base price, otherwise a version-owned option business number"`
	CurrencyCode                  string     `json:"currencyCode"                  orm:"currency_code"                    description:""`
	AmountMinor                   int64      `json:"amountMinor"                   orm:"amount_minor"                     description:""`
	SetupFeeMinor                 int64      `json:"setupFeeMinor"                 orm:"setup_fee_minor"                  description:""`
	RenewalAmountMinor            *int64     `json:"renewalAmountMinor"            orm:"renewal_amount_minor"             description:""`
	MinimumQuantity               int64      `json:"minimumQuantity"               orm:"minimum_quantity"                 description:""`
	MaximumQuantity               *int64     `json:"maximumQuantity"               orm:"maximum_quantity"                 description:""`
	StatusCode                    string     `json:"statusCode"                    orm:"status_code"                      description:""`
	CreatedAt                     *time.Time `json:"createdAt"                     orm:"created_at"                       description:""`
	ConfigurationPriceDimensionNo string     `json:"configurationPriceDimensionNo" orm:"configuration_price_dimension_no" description:""`
}
