package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

type CatalogConfigurationReq struct {
	g.Meta     `path:"/channel/catalog/products/{publicSlug}/configuration" method:"get" tags:"Open API" summary:"Read published product configuration" dc:"Read safe opaque field and option tokens for one currently published product; requires catalog.read."`
	PublicSlug string `json:"publicSlug" v:"required|max-length:160" dc:"Product slug returned by the catalog" eg:"cloud-server"`
}
type CatalogConfigurationRes struct {
	channelcommercecap.Configuration
}

type CatalogConfigurationEvaluateReq struct {
	g.Meta                `path:"/channel/catalog/products/{publicSlug}/configuration/evaluate" method:"get" tags:"Open API" summary:"Evaluate selected product configuration" dc:"Read dependent choices and validation results for the current configuration revision without changing resources."`
	PublicSlug            string `json:"publicSlug" v:"required|max-length:160" dc:"Product slug returned by the catalog" eg:"cloud-server"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|max-length:4096" dc:"Revision returned by the configuration read" eg:"opaque-revision"`
	Values                string `json:"values" v:"max-length:32768" dc:"URL-encoded JSON array of public typed selections; at most 128 items and 32768 UTF-8 bytes; omitted means no selections" eg:"[]"`
}
type CatalogConfigurationEvaluateRes struct {
	channelcommercecap.ConfigurationEvaluation
}

type CatalogBillingTermsReq struct {
	g.Meta                `path:"/channel/catalog/products/{publicSlug}/billing-terms" method:"get" tags:"Open API" summary:"Read available billing terms for selected configuration" dc:"Return billing tokens and ordinary explanatory amounts in minor currency units. To obtain the authenticated customer price, create a frozen quote; these previews do not include agent pricing."`
	PublicSlug            string `json:"publicSlug" v:"required|max-length:160" dc:"Product slug returned by the catalog" eg:"cloud-server"`
	ConfigurationRevision string `json:"configurationRevision" v:"required|max-length:4096" dc:"Revision returned by the configuration read" eg:"opaque-revision"`
	CurrencyCode          string `json:"currencyCode" v:"required|length:3,3" dc:"Explicit supported ISO currency" eg:"CNY"`
	Values                string `json:"values" v:"max-length:32768" dc:"URL-encoded JSON array of public typed selections; at most 128 items and 32768 UTF-8 bytes; omitted means no selections" eg:"[]"`
}
type CatalogBillingTermsRes struct {
	channelcommercecap.BillingTerms
}
