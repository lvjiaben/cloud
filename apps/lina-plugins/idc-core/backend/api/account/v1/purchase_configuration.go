// This file defines authenticated evaluation and quote endpoints used when a
// governed purchase configuration contains exact-account file tokens.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"

	publicv1 "lina-plugin-idc-core/backend/api/public/v1"
)

type PurchaseConfigurationEvaluateReq struct {
	g.Meta                `path:"/account/catalog/products/{publicSlug}/purchase-configuration/evaluate" method:"post" tags:"IDC Customer Catalog" summary:"Evaluate my governed purchase configuration" dc:"Revalidate typed values and exact-account governed file tokens against the current frozen declaration."`
	PublicSlug            string                                 `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"ssl-certificate"`
	ConfigurationRevision string                                 `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque published configuration revision" eg:"Q1JfYWJj"`
	Values                []*publicv1.PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows"`
}

type PurchaseConfigurationEvaluateRes struct {
	publicv1.PurchaseConfigurationEvaluation
}

type PurchaseConfigurationQuoteReq struct {
	g.Meta                `path:"/account/catalog/products/{publicSlug}/purchase-configuration/quote" method:"post" tags:"IDC Customer Catalog" summary:"Quote my governed purchase configuration" dc:"Revalidate typed values and exact-account governed file tokens, then return only core-owned published integer price snapshots."`
	PublicSlug            string                                 `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public product slug" eg:"ssl-certificate"`
	ConfigurationRevision string                                 `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque published configuration revision" eg:"Q1JfYWJj"`
	CurrencyCode          string                                 `json:"currencyCode" v:"required|length:3,3" dc:"Exact uppercase ISO 4217 currency code" eg:"CNY"`
	Values                []*publicv1.PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows"`
}

type PurchaseConfigurationQuoteRes struct {
	ConfigurationRevision string                                     `json:"configurationRevision"`
	CurrencyCode          string                                     `json:"currencyCode"`
	Terms                 []*publicv1.PurchaseConfigurationQuoteTerm `json:"terms"`
}

type PurchaseConfigurationFreezeReq struct {
	ServerLoginToken      string `json:"serverLoginToken,omitempty" v:"max-length:128" dc:"Owned product-version server login token for one independent server; omitted for automatic generation" eg:"SLI_0123456789ABCDEF0123456789ABCDEF"`
	g.Meta                `path:"/account/commerce/quote-items/{quoteItemNo}/purchase-configuration" method:"post" tags:"IDC Customer Catalog" summary:"Freeze my governed purchase configuration" dc:"Freeze the compiler-validated typed values for this exact quote item. File tokens remain ready and are consumed only by checkout."`
	QuoteItemNo           string                                 `json:"quoteItemNo" in:"path" v:"required|max-length:40" dc:"Exact immutable quote item number" eg:"QIT_0123456789ABCDEF0123456789ABCDEF"`
	PublicSlug            string                                 `json:"publicSlug" v:"required|length:1,128" dc:"Public product slug" eg:"ssl-certificate"`
	ConfigurationRevision string                                 `json:"configurationRevision" v:"required|max-length:128" dc:"Opaque published configuration revision" eg:"Q1JfYWJj"`
	Values                []*publicv1.PurchaseConfigurationValue `json:"values" dc:"Bounded explicit typed value rows"`
}

type PurchaseConfigurationFreezeRes struct {
	SnapshotNo            string `json:"snapshotNo" dc:"Opaque immutable purchase configuration snapshot number"`
	ConfigurationFrozen   bool   `json:"configurationFrozen" dc:"Server-authoritative frozen state for this exact quote item" eg:"true"`
	ConfigurationRevision string `json:"configurationRevision" dc:"Opaque governed configuration revision frozen for this exact quote item" eg:"Q1JfYWJj"`
}
