// This file defines the bounded catalog product workbench summary projection.

package v1

// CatalogProductBasePricePreview is an exact public base purchase price for one
// explicitly named period, excluding configuration, customer pricing and costs.
type CatalogProductBasePricePreview struct {
	CurrencyCode  string `json:"currencyCode" dc:"Configured ISO currency code; currencies are never converted" eg:"CNY"`
	AmountMinor   string `json:"amountMinor" dc:"Exact base purchase amount in minor units, without configuration supplements" eg:"1100"`
	SetupFeeMinor string `json:"setupFeeMinor" dc:"Exact additional new-purchase setup fee in minor units" eg:"0"`
	PeriodUnit    string `json:"periodUnit" dc:"Billing period unit: once, hour, day, month, or year" eg:"month"`
	PeriodCount   int    `json:"periodCount" dc:"Number of units per billing period" eg:"1"`
	ChargeType    string `json:"chargeType" dc:"Charge type: one_time, recurring, or trial" eg:"recurring"`
}

type CatalogProductSummaryItem struct {
	AvailableQuantity   *int64                          `json:"availableQuantity" dc:"Remaining sellable quantity after held and committed reservations across product versions; null means unlimited" eg:"20"`
	ProvisionedQuantity int64                           `json:"provisionedQuantity" dc:"Successfully delivered units excluding terminated, cancelled, and pending-activation services" eg:"8"`
	TotalQuantity       int64                           `json:"totalQuantity" dc:"Cumulative successfully delivered units including subsequently terminated services; excludes failed attempts" eg:"12"`
	SortOrder           int64                           `json:"sortOrder" dc:"Display order; smaller values appear first" eg:"100"`
	BasePrice           *CatalogProductBasePricePreview `json:"basePrice" dc:"One configured public base purchase price with its period, or null when none exists; not a complete configuration quote or converted minimum" eg:"{}"`
	ProductNo           string                          `json:"productNo" dc:"Stable catalog product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	CategoryNo          string                          `json:"categoryNo" dc:"Stable owning category business number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	CategoryNameZhCn    string                          `json:"categoryNameZhCn" dc:"Simplified Chinese category name" eg:"Cloud Servers zh-CN"`
	CategoryNameEnUs    string                          `json:"categoryNameEnUs" dc:"English category name" eg:"Cloud servers"`
	VersionNo           string                          `json:"versionNo" dc:"Stable latest management version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	VersionNumber       int                             `json:"versionNumber" dc:"Monotonically increasing product version number" eg:"3"`
	Status              string                          `json:"status" dc:"Product status: draft, active, suspended, or retired" eg:"active"`
	VersionStatus       string                          `json:"versionStatus" dc:"Latest version status: draft, published, superseded, or retired" eg:"draft"`
	Source              string                          `json:"source" dc:"Latest version source: manual, migration, or copy" eg:"manual"`
	PublicSlug          string                          `json:"publicSlug" dc:"Public product URL slug" eg:"hong-kong-cloud"`
	NameZhCn            string                          `json:"nameZhCn" dc:"Simplified Chinese sales name" eg:"Hong Kong Cloud zh-CN"`
	NameEnUs            string                          `json:"nameEnUs" dc:"English sales name" eg:"Hong Kong Cloud"`
	SummaryZhCn         string                          `json:"summaryZhCn" dc:"Simplified Chinese sales summary" eg:"Low-latency cloud server zh-CN"`
	SummaryEnUs         string                          `json:"summaryEnUs" dc:"English sales summary" eg:"Low-latency Hong Kong cloud server"`
	BillingRuleCount    int64                           `json:"billingRuleCount" dc:"Number of active billing rules in the latest version" eg:"3"`
	CurrencyCount       int64                           `json:"currencyCount" dc:"Number of explicitly configured price currencies in the latest version" eg:"2"`
	PublishedAt         *int64                          `json:"publishedAt" dc:"Latest-version publication time as a Unix timestamp in milliseconds, or null for a draft" eg:"1786161600000"`
	CreatedAt           int64                           `json:"createdAt" dc:"Product creation time as a Unix timestamp in milliseconds" eg:"1786161600000"`
	UpdatedAt           int64                           `json:"updatedAt" dc:"Product update time as a Unix timestamp in milliseconds" eg:"1786161660000"`
}
