// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package channelcommercecap

// ProductSnapshot is a public import projection. Configuration is an initial
// evaluated view, not a complete static schema: callers must remotely evaluate
// selections to preserve conditional visibility and dynamic candidate rules.
type ProductSnapshot struct {
	Product           CatalogProduct         `json:"product"`
	Configuration     Configuration          `json:"configuration"`
	ConfigurationMode string                 `json:"configurationMode"`
	Currencies        []CatalogCurrency      `json:"currencies"`
	BillingTerms      []CurrencyBillingTerms `json:"billingTerms"`
	PricingReady      bool                   `json:"pricingReady"`
}

type CatalogCurrency struct {
	CurrencyCode string `json:"currencyCode"`
	Precision    int    `json:"precision"`
}

// CurrencyBillingTerms are explanatory published terms; customer-specific
// prices require an explicit frozen quote after a valid configuration is chosen.
type CurrencyBillingTerms struct {
	CurrencyCode string        `json:"currencyCode"`
	Precision    int           `json:"precision"`
	Terms        []BillingTerm `json:"terms"`
}
