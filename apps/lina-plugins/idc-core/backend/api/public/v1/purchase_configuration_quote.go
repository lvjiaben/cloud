// This file defines explanatory current published pricing for a valid configuration.

package v1

import "github.com/gogf/gf/v2/frame/g"

type PurchaseConfigurationQuoteReq struct {
	g.Meta                `path:"/public/catalog/products/{publicSlug}/purchase-configuration/quote" method:"post" tags:"IDC Public Catalog" summary:"Quote governed purchase configuration" dc:"Revalidate configuration and return only core-owned explicit integer price snapshots; this is not a locked quote."`
	PublicSlug            string                        `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public browser slug"`
	ConfigurationRevision string                        `json:"configurationRevision" v:"required" dc:"Opaque current published configuration revision"`
	CurrencyCode          string                        `json:"currencyCode" v:"required|regex:^[A-Z]{3}$" dc:"Explicit ISO-style currency code"`
	Values                []*PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows"`
}

type PurchaseConfigurationQuoteRes struct {
	ConfigurationRevision string                            `json:"configurationRevision"`
	CurrencyCode          string                            `json:"currencyCode"`
	Terms                 []*PurchaseConfigurationQuoteTerm `json:"terms"`
}
