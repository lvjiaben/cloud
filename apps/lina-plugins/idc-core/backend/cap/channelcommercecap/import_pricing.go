// This file describes portable explicit price schedules without converting a
// preview or a customer-specific whole-item adjustment into component prices.

package channelcommercecap

// ImportPriceSchedule contains one currency and billing rule. Bands preserve
// progressive quantity pricing; nil renewal amounts remain explicitly unknown.
type ImportPriceSchedule struct {
	CurrencyCode          string               `json:"currencyCode"`
	TermReference         string               `json:"termReference"`
	ChargeType            string               `json:"chargeType"`
	PeriodUnit            string               `json:"periodUnit"`
	PeriodCount           int                  `json:"periodCount"`
	PricePhase            string               `json:"pricePhase"`
	QuantityMode          string               `json:"quantityMode"`
	TrialQuantity         int                  `json:"trialQuantity"`
	MinimumPurchaseCycles int                  `json:"minimumPurchaseCycles"`
	MaximumRenewalCycles  *int                 `json:"maximumRenewalCycles"`
	Usage                 *ImportUsagePolicy   `json:"usage,omitempty"`
	Base                  []ImportPriceBand    `json:"base"`
	Options               []ImportOptionPrice  `json:"options"`
	Tax                   ImportTaxPolicy      `json:"tax"`
	WholeItemRule         *ImportWholeItemRule `json:"wholeItemRule,omitempty"`
}

// ImportPriceBand preserves exact minor-unit amounts. Quantity configurations
// use (minimum, maximum]; a nil maximum means the final unlimited band.
type ImportPriceBand struct {
	MinimumQuantity    int64  `json:"minimumQuantity"`
	MaximumQuantity    *int64 `json:"maximumQuantity"`
	AmountMinor        int64  `json:"amountMinor"`
	SetupAmountMinor   int64  `json:"setupAmountMinor"`
	RenewalAmountMinor *int64 `json:"renewalAmountMinor"`
}

// ImportOptionPrice addresses only a schema-safe field and option key.
type ImportOptionPrice struct {
	// PricingMode is empty/fixed for an option, per_unit or band_total for an
	// integer field. Quantity fields use empty Value and retain schema limits.
	PricingMode ImportConfigurationPriceMode `json:"pricingMode,omitempty"`
	FieldKey    string                       `json:"fieldKey"`
	Value       string                       `json:"value"`
	Bands       []ImportPriceBand            `json:"bands"`
}

// ImportConfigurationPriceMode freezes the component charging algorithm.
type ImportConfigurationPriceMode string

const (
	// ImportConfigurationPriceFixed charges the selected option once.
	ImportConfigurationPriceFixed ImportConfigurationPriceMode = "fixed"
	// ImportConfigurationPricePerUnit multiplies the matching band by quantity.
	ImportConfigurationPricePerUnit ImportConfigurationPriceMode = "per_unit"
	// ImportConfigurationPriceBandTotal charges the matching band total once.
	ImportConfigurationPriceBandTotal ImportConfigurationPriceMode = "band_total"
)

// ImportUsagePolicy retains metering semantics rather than inventing zero usage.
type ImportUsagePolicy struct {
	MeterUnit        string `json:"meterUnit"`
	TierMode         string `json:"tierMode"`
	CollectionUnit   string `json:"collectionUnit"`
	CollectionCount  int    `json:"collectionCount"`
	RoundingMode     string `json:"roundingMode"`
	RoundingQuantity int64  `json:"roundingQuantity"`
	MinimumQuantity  *int64 `json:"minimumQuantity"`
	MaximumQuantity  *int64 `json:"maximumQuantity"`
	MissingPolicy    string `json:"missingPolicy"`
}

// ImportTaxPolicy explains upstream amounts. Import does not silently apply
// this merchant's tax jurisdiction to the downstream merchant's sale.
type ImportTaxPolicy struct {
	PriceIncludesTax     bool   `json:"priceIncludesTax"`
	Treatment            string `json:"treatment"`
	RateBasisPoints      *int   `json:"rateBasisPoints"`
	CategoryCode         string `json:"categoryCode"`
	RoundingScope        string `json:"roundingScope"`
	RoundingMode         string `json:"roundingMode"`
	RoundingQuantumMinor int64  `json:"roundingQuantumMinor"`
}

// MaximumImportWholeItemRuleDepth bounds resale chains before JSON encoding and
// evaluation. The complete snapshot is additionally limited to one MiB.
const MaximumImportWholeItemRuleDepth = 16

// ImportWholeItemRule describes the customer's effective negotiated period
// amount, independently of setup fees and ordinary component prices.
type ImportWholeItemRule struct {
	// Prior is evaluated first, preserving each upstream rounding step and
	// configuration allocation. Nil means the ordinary component price.
	Prior               *ImportWholeItemRule `json:"prior,omitempty"`
	Kind                string               `json:"kind"`
	FixedAmountMinor    *int64               `json:"fixedAmountMinor"`
	DiscountBasisPoints *int                 `json:"discountBasisPoints"`
}
