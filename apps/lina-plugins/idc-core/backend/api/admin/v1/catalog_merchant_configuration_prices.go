// These types are restricted to the authorized merchant configuration editor.
// Public price candidates keep their opaque field and option identities.
package v1

// CatalogMerchantPriceTarget joins already visible merchant choices to local
// price dimensions without exposing credentials, costs or supplier responses.
type CatalogMerchantPriceTarget struct {
	FieldKey       string `json:"fieldKey" dc:"Field already visible in the authorized merchant configuration editor" eg:"local_validation_method"`
	CandidateValue string `json:"candidateValue" dc:"Editable choice value; empty only for a quantity target" eg:"dns"`
	Quantity       bool   `json:"quantity" dc:"Whether this target prices a numeric quantity rather than a fixed choice" eg:"false"`
	DimensionNo    string `json:"dimensionNo" dc:"Existing local price dimension, or empty before its first price is saved" eg:"cpd_0123456789abcdef0123456789abcdef"`
	PricingMode    string `json:"pricingMode" dc:"Existing algorithm: fixed, per_unit, band_total, graduated; empty before first save" eg:"fixed"`
}

// CatalogMerchantConfigurationPriceRow saves one complete currency and billing
// schedule together with the schema containing its target choice.
type CatalogMerchantConfigurationPriceRow struct {
	FieldKey        string                  `json:"fieldKey" v:"required|max-length:128" dc:"Field in the customer configuration submitted in the same request" eg:"local_validation_method"`
	CandidateValue  string                  `json:"candidateValue" v:"max-length:512" dc:"Exact sellable choice value; must be empty for quantity pricing" eg:"dns"`
	Quantity        bool                    `json:"quantity" dc:"True for a numeric quantity price, false for an individual sellable choice" eg:"false"`
	PricingMode     string                  `json:"pricingMode" v:"in:fixed,per_unit,band_total,graduated" dc:"graduated sums each band unit price times its own quantity and permits setup only in the first band; fixed choice charge or per_unit quantity or band_total whole-band charge; omitted defaults to fixed for choices and per_unit for quantity" eg:"fixed"`
	Active          *bool                   `json:"active" dc:"Whether the schedule is active; omitted defaults to true" eg:"true"`
	PreviousPriceNo string                  `json:"previousPriceNo" v:"max-length:40" dc:"Current local price-row identity to fence concurrent updates; empty creates a schedule" eg:"prc_0123456789abcdef0123456789abcdef"`
	BillingRuleNo   string                  `json:"billingRuleNo" v:"required|max-length:40" dc:"Active billing rule belonging to the edited product version" eg:"bil_0123456789abcdef0123456789abcdef"`
	CurrencyCode    string                  `json:"currencyCode" v:"required|length:3,3" dc:"Explicit uppercase currency code without currency fallback" eg:"CNY"`
	Tiers           []CatalogPriceTierInput `json:"tiers" v:"required" dc:"Complete schedule; at most 256 tiers across all submitted price rows" eg:"[]"`
}
