package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogConfigurationPriceBatchRow struct {
	Active          *bool                   `json:"active" dc:"Whether the saved schedule is enabled; omitted retains the legacy default true. Explicit false saves an inactive schedule without enabling purchases." eg:"false"`
	TargetKind      string                  `json:"targetKind" d:"configuration" v:"in:configuration,base" dc:"Explicit target: configuration (default) requires a candidate token; base requires an empty token and changes the base schedule only" eg:"base"`
	CandidateToken  string                  `json:"candidateToken" v:"max-length:80" dc:"Opaque current fixed-price candidate token, required for configuration targets and forbidden for explicit base targets" eg:"candidate_0123456789abcdef"`
	PreviousPriceNo string                  `json:"previousPriceNo" v:"max-length:40" dc:"Current price-row identity for updates; empty creates a new schedule" eg:"prc_0123456789abcdef0123456789abcdef"`
	BillingRuleNo   string                  `json:"billingRuleNo" v:"required|max-length:40" dc:"Active billing rule owned by the same draft" eg:"bil_0123456789abcdef0123456789abcdef"`
	CurrencyCode    string                  `json:"currencyCode" v:"required|length:3,3" dc:"Explicit uppercase currency code" eg:"CNY"`
	Tiers           []CatalogPriceTierInput `json:"tiers" v:"required" dc:"Complete fixed specification price schedule; total batch tier count is at most 256" eg:"[]"`
}

type CatalogConfigurationPriceBatchReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/configuration-price-batches" method:"post" tags:"IDC Catalog Management" summary:"Save product price table" dc:"Atomically validate and save up to 256 explicit base or fixed specification price schedules. Configuration targets require configuration authorization; all targets require price permission, draft locking and stale-edit fences. A failed batch changes no price or dimension." permission:"idc:admin:catalog:price:write"`
	VersionNo string                              `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	Rows      []CatalogConfigurationPriceBatchRow `json:"rows" v:"required" dc:"Changed fixed-price schedules; at most 256 schedules and total tiers" eg:"[]"`
}
type CatalogConfigurationPriceBatchRes struct {
	Saved int `json:"saved" dc:"Number of atomically committed schedules" eg:"6"`
}
