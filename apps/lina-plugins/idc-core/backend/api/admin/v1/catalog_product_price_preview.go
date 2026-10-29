// This file defines a bounded staff-only read of saved product selection prices.
// The preview uses the checkout calculator and never reserves a quote or order.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductPricePreviewReq struct {
	g.Meta       `path:"/admin/catalog/products/{productNo}/price-preview" method:"get" tags:"IDC Catalog Management" summary:"Preview saved product selection prices" dc:"Calculate saved product prices for typed preview selections without orders, locked quotes, or supplier writes. Values are bounded JSON query content." permission:"idc:admin:catalog:product:query"`
	ProductNo    string `json:"productNo" in:"path" v:"required|max-length:128" dc:"Stable catalog product business number" eg:"prd_0123456789abcdef0123456789abcdef"`
	VersionNo    string `json:"versionNo" in:"query" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Saved product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	CurrencyCode string `json:"currencyCode" in:"query" v:"required|regex:^[A-Z]{3}$" dc:"Explicit merchant currency code" eg:"CNY"`
	Values       string `json:"values" in:"query" v:"required|max-length:32768" dc:"JSON array of up to 128 typed customer configuration values and at most 32768 UTF-8 bytes; secret and file values are not accepted" eg:"[]"`
}

type CatalogProductPricePreviewRes struct {
	Preview *CatalogProductPricePreview `json:"preview" dc:"Read-only prices from the saved product version" eg:"{}"`
}

type CatalogProductPricePreview struct {
	VersionNo        string                            `json:"versionNo" dc:"Saved product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	CurrencyCode     string                            `json:"currencyCode" dc:"Explicit merchant currency code" eg:"CNY"`
	PriceIncludesTax bool                              `json:"priceIncludesTax" dc:"Whether displayed charge components already include tax" eg:"false"`
	Terms            []*CatalogProductPricePreviewTerm `json:"terms" dc:"Bounded billing period previews using the checkout pricing calculator" eg:"[]"`
}

type CatalogProductPricePreviewTerm struct {
	BillingRuleNo            string                             `json:"billingRuleNo" dc:"Saved billing rule number" eg:"billing_monthly"`
	Status                   string                             `json:"status" dc:"Pricing status: final (known payable amount), unavailable (saved price schedules missing or invalid), pending_usage (actual usage required), or requires_customer_context (customer eligibility or tax context required)" eg:"final"`
	ChargeType               string                             `json:"chargeType" dc:"Saved charge type: one_time (single payment), recurring (periodic), tiered (tiered quantity), usage (actual metered usage), or trial (customer-qualified trial)" eg:"recurring"`
	PeriodUnit               string                             `json:"periodUnit" dc:"Saved billing period unit: once (single payment), hour, day, month, or year" eg:"month"`
	PeriodCount              int                                `json:"periodCount" dc:"Number of units per billing period" eg:"1"`
	Cycles                   int                                `json:"cycles" dc:"Minimum purchase cycles included in this preview" eg:"1"`
	BaseAmountMinor          *string                            `json:"baseAmountMinor" dc:"Base charge in integer minor units, excluding setup fees, or null when unavailable" eg:"1000"`
	ConfigurationAmountMinor *string                            `json:"configurationAmountMinor" dc:"Configuration charge in integer minor units, excluding setup fees, or null when unavailable" eg:"400"`
	SetupAmountMinor         *string                            `json:"setupAmountMinor" dc:"Total setup fees in integer minor units, or null when unavailable" eg:"110"`
	Issues                   []*CatalogProductPricePreviewIssue `json:"issues" dc:"Explicit saved price schedule issues for this period; not upstream, tax or authorization failures" eg:"[]"`
	TaxAmountMinor           *string                            `json:"taxAmountMinor" dc:"Tax in integer minor units, or null when customer context is required" eg:"151"`
	TotalAmountMinor         *string                            `json:"totalAmountMinor" dc:"Total payable amount in integer minor units, or null when not yet final" eg:"1661"`
}

// CatalogProductPricePreviewIssue is a bounded staff-facing saved-price diagnosis.
type CatalogProductPricePreviewIssue struct {
	Kind     string `json:"kind" dc:"Price dimension: base (product base price) or option (selected configuration price)" eg:"option"`
	Reason   string `json:"reason" dc:"Saved price schedule issue: missing (not configured) or invalid (schedule validation failed)" eg:"missing"`
	FieldKey string `json:"fieldKey" dc:"Safe customer configuration field key, empty for base price" eg:"cpu"`
	NameZhCN string `json:"nameZhCN" dc:"Customer configuration label and selected option in Simplified Chinese" eg:"CPU"`
	NameEnUS string `json:"nameEnUS" dc:"Customer configuration label and selected option in English" eg:"CPU"`
}
