// This file defines the bounded catalog product detail response graph.

package v1

type CatalogProductDetailProduct struct {
	DirectoryNo        string `json:"directoryNo" dc:"Single member product-navigation reference; empty means unclassified" eg:"pdr_example"`
	DescriptionVersion int64  `json:"descriptionVersion" dc:"Optimistic revision for bilingual sales copy updates" eg:"1"`
	ProductNo          string `json:"productNo" dc:"Stable product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	CategoryNo         string `json:"categoryNo" dc:"Stable category business number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	CategoryNameZhCn   string `json:"categoryNameZhCn" dc:"Simplified Chinese category name" eg:"Cloud servers"`
	CategoryNameEnUs   string `json:"categoryNameEnUs" dc:"English category name" eg:"Cloud servers"`
	Status             string `json:"status" dc:"Product status: draft, active, suspended, or retired" eg:"active"`
	PublicSlug         string `json:"publicSlug" dc:"Public URL slug" eg:"hong-kong-cloud"`
	NameZhCn           string `json:"nameZhCn" dc:"Simplified Chinese product name" eg:"Hong Kong Elastic Cloud"`
	NameEnUs           string `json:"nameEnUs" dc:"English product name" eg:"Hong Kong Elastic Cloud"`
	SummaryZhCn        string `json:"summaryZhCn" dc:"Simplified Chinese sales summary" eg:"Low-latency cloud server"`
	SummaryEnUs        string `json:"summaryEnUs" dc:"English sales summary" eg:"Low-latency cloud server"`
	SortOrder          int    `json:"sortOrder" dc:"Merchant-defined ascending display order" eg:"10"`
	CreatedAt          int64  `json:"createdAt" dc:"Creation time as a Unix timestamp in milliseconds" eg:"1786161600000"`
	UpdatedAt          int64  `json:"updatedAt" dc:"Update time as a Unix timestamp in milliseconds" eg:"1786161660000"`
}

type CatalogProductDetailVersion struct {
	VersionNo             string `json:"versionNo" dc:"Stable version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	VersionNumber         int    `json:"versionNumber" dc:"Monotonic product version number" eg:"3"`
	Status                string `json:"status" dc:"Version status: draft, published, superseded, or retired" eg:"draft"`
	Source                string `json:"source" dc:"Version source: manual, migration, or copy" eg:"manual"`
	SourceVersion         string `json:"sourceVersion" dc:"Display-safe external source version, or empty" eg:"2026-08"`
	ChangeSummary         string `json:"changeSummary" dc:"Merchant change summary" eg:"Add yearly billing"`
	RequiresContract      bool   `json:"requiresContract" dc:"Whether this version requires a contract for applicable orders" eg:"false"`
	ContractPolicyCode    string `json:"contractPolicyCode" dc:"Product contract choice: inherit, none, or required" eg:"inherit"`
	ContractTemplateNo    string `json:"contractTemplateNo" dc:"Selected all-customer template number, or empty"`
	ContractPolicyVersion int64  `json:"contractPolicyVersion" dc:"Contract rule optimistic concurrency version" eg:"1"`
	PublishedAt           *int64 `json:"publishedAt" dc:"Publication time as Unix milliseconds, or null" eg:"1786161600000"`
	RetiredAt             *int64 `json:"retiredAt" dc:"Retirement time as Unix milliseconds, or null" eg:"1786161600000"`
	CreatedAt             int64  `json:"createdAt" dc:"Version creation time as Unix milliseconds" eg:"1786161600000"`
}

// CatalogFulfillmentPolicySummary deliberately excludes plugin methods,
// technical granularity codes, digests, and retry settings.
type CatalogFulfillmentPolicySummary struct {
	TrialSupported   bool   `json:"trialSupported" dc:"Whether the exact automatic fulfillment plugin supports real trial suspension and recovery" eg:"false"`
	StrategyCode     string `json:"strategyCode" dc:"Merchant-facing fulfillment choice: manual, approval_then_plugin, or payment_confirmed_plugin" eg:"manual"`
	CapabilityZhCn   string `json:"capabilityZhCn" dc:"Simplified Chinese plugin capability description, or empty for manual fulfillment" eg:"\u7b7e\u53d1\u5e76\u7ba1\u7406\u8bc1\u4e66"`
	CapabilityEnUs   string `json:"capabilityEnUs" dc:"English plugin capability description, or empty for manual fulfillment" eg:"Issue and manage certificates"`
	QuantityHandling string `json:"quantityHandling" dc:"Safe presentation choice: together or separately" eg:"together"`
}

type CatalogProductDetailBillingRule struct {
	BillingRuleNo         string  `json:"billingRuleNo" dc:"Stable billing-rule business number" eg:"bill_monthly"`
	RuleCode              string  `json:"ruleCode" dc:"Version-local billing code" eg:"monthly"`
	ChargeType            string  `json:"chargeType" dc:"Charge type: one_time, recurring, tiered, usage, or trial" eg:"recurring"`
	PeriodUnit            string  `json:"periodUnit" dc:"Period unit: once, hour, day, month, or year" eg:"month"`
	PeriodCount           int     `json:"periodCount" dc:"Positive period count" eg:"1"`
	PricePhase            string  `json:"pricePhase" dc:"Price phase: new_purchase, renewal, or both" eg:"both"`
	QuantityMode          string  `json:"quantityMode" dc:"Quantity mode: fixed, per_unit, tiered, or usage" eg:"fixed"`
	TrialQuantity         int     `json:"trialQuantity" dc:"Trial entitlement count" eg:"0"`
	Status                string  `json:"status" dc:"Billing status: active or inactive" eg:"active"`
	MinimumPurchaseCycles int     `json:"minimumPurchaseCycles" dc:"Minimum new-purchase cycles" eg:"1"`
	MaximumRenewalCycles  *int    `json:"maximumRenewalCycles" dc:"Maximum renewal cycles, or null" eg:"12"`
	MeterUnit             *string `json:"meterUnit" dc:"Authoritative usage meter unit, or null" eg:"gb"`
	TierMode              *string `json:"tierMode" dc:"Tier mode, or null" eg:"graduated"`
	UsageCollectionUnit   *string `json:"usageCollectionUnit" dc:"Usage collection unit, or null" eg:"hour"`
	UsageCollectionCount  *int    `json:"usageCollectionCount" dc:"Usage collection count, or null" eg:"1"`
	UsageRoundingMode     *string `json:"usageRoundingMode" dc:"Usage rounding mode, or null" eg:"up"`
	UsageRoundingQuantity *int64  `json:"usageRoundingQuantity" dc:"Usage rounding quantity, or null" eg:"1"`
	UsageMinimumQuantity  *int64  `json:"usageMinimumQuantity" dc:"Usage minimum quantity, or null" eg:"0"`
	UsageMaximumQuantity  *int64  `json:"usageMaximumQuantity" dc:"Usage maximum quantity, or null" eg:"1024"`
	UsageMissingPolicy    *string `json:"usageMissingPolicy" dc:"Missing usage policy, or null" eg:"pending_recalculation"`
}

type CatalogProductDetailPrice struct {
	PriceNo                         string  `json:"priceNo" dc:"Stable price business number" eg:"price_cny_month"`
	BillingRuleNo                   string  `json:"billingRuleNo" dc:"Owning billing-rule business number" eg:"bill_monthly"`
	CurrencyCode                    string  `json:"currencyCode" dc:"Uppercase ISO-style three-letter currency code" eg:"CNY"`
	AmountMinor                     string  `json:"amountMinor" dc:"Exact base or tier amount in decimal minor units" eg:"9900"`
	SetupFeeMinor                   string  `json:"setupFeeMinor" dc:"Exact one-time setup fee in decimal minor units" eg:"0"`
	RenewalAmountMinor              *string `json:"renewalAmountMinor" dc:"Exact renewal amount in decimal minor units, or null" eg:"10900"`
	MinimumQuantity                 int64   `json:"minimumQuantity" dc:"Inclusive tier lower bound" eg:"0"`
	MaximumQuantity                 *int64  `json:"maximumQuantity" dc:"Exclusive tier upper bound, or null" eg:"10"`
	Status                          string  `json:"status" dc:"Price status: active or inactive" eg:"active"`
	ConfigurationPriceDimensionNo   string  `json:"configurationPriceDimensionNo" dc:"Configuration price dimension number, or empty for base price"`
	ConfigurationFieldLabelZhCn     string  `json:"configurationFieldLabelZhCn" dc:"Frozen Chinese configuration field label, or empty for base price"`
	ConfigurationFieldLabelEnUs     string  `json:"configurationFieldLabelEnUs" dc:"Frozen English configuration field label, or empty for base price"`
	ConfigurationCandidateLabelZhCn string  `json:"configurationCandidateLabelZhCn" dc:"Frozen Chinese candidate label, or empty for base price"`
	ConfigurationCandidateLabelEnUs string  `json:"configurationCandidateLabelEnUs" dc:"Frozen English candidate label, or empty for base price"`
}

type CatalogProductDetailTaxPolicy struct {
	TaxPolicyNo          string `json:"taxPolicyNo" dc:"Stable tax-policy business number" eg:"tax_0123456789abcdef0123456789abcdef"`
	CurrencyCode         string `json:"currencyCode" dc:"Explicit priced currency code" eg:"CNY"`
	PriceIncludesTax     bool   `json:"priceIncludesTax" dc:"Whether prices already include tax" eg:"true"`
	TaxTreatment         string `json:"taxTreatment" dc:"Tax treatment" eg:"taxable"`
	TaxRateBasisPoints   *int   `json:"taxRateBasisPoints" dc:"Tax rate in basis points, or null for exempt treatment" eg:"600"`
	TaxCategoryCode      string `json:"taxCategoryCode" dc:"Merchant tax classification code" eg:"cloud_service"`
	RoundingScope        string `json:"roundingScope" dc:"Monetary rounding scope" eg:"line"`
	RoundingMode         string `json:"roundingMode" dc:"Monetary rounding mode" eg:"half_up"`
	RoundingQuantumMinor string `json:"roundingQuantumMinor" dc:"Exact rounding quantum in decimal currency minor units" eg:"1"`
}
