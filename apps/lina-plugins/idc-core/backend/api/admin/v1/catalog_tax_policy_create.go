// This file defines explicit per-currency tax-policy creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogTaxPolicyCreateReq struct {
	g.Meta               `path:"/admin/catalog/product-versions/{versionNo}/tax-policies" method:"post" tags:"IDC Catalog Management" summary:"Create catalog tax policy" dc:"Create the only explicit tax and monetary-rounding policy for one priced currency on an editable version." permission:"idc:admin:catalog:tax:write"`
	VersionNo            string `json:"versionNo" in:"path" v:"required|max-length:40" dc:"Stable editable draft version business number" eg:"ver_0123456789abcdef0123456789abcdef"`
	CurrencyCode         string `json:"currencyCode" v:"required|length:3,3" dc:"Explicit uppercase priced currency code" eg:"CNY"`
	PriceIncludesTax     bool   `json:"priceIncludesTax" dc:"Whether catalog prices already include tax" eg:"true"`
	TaxTreatment         string `json:"taxTreatment" v:"required|in:taxable,zero_rated,exempt" dc:"Tax treatment" eg:"taxable"`
	TaxRateBasisPoints   *int   `json:"taxRateBasisPoints" dc:"Tax rate in basis points; null only for exempt treatment" eg:"600"`
	TaxCategoryCode      string `json:"taxCategoryCode" v:"required|max-length:64" dc:"Merchant tax classification code" eg:"cloud_service"`
	RoundingScope        string `json:"roundingScope" v:"required|in:line,total" dc:"Apply monetary rounding per line or after total aggregation" eg:"line"`
	RoundingMode         string `json:"roundingMode" v:"required|in:half_up,down,up" dc:"Stable monetary rounding mode" eg:"half_up"`
	RoundingQuantumMinor string `json:"roundingQuantumMinor" v:"required|regex:^[1-9][0-9]{0,6}$" dc:"Exact rounding quantum in decimal currency minor units" eg:"1"`
}

type CatalogTaxPolicyCreateRes struct {
	TaxPolicyNo          string `json:"taxPolicyNo" dc:"Stable tax-policy business number" eg:"tax_0123456789abcdef0123456789abcdef"`
	VersionNo            string `json:"versionNo" dc:"Owning catalog version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	CurrencyCode         string `json:"currencyCode" dc:"Explicit priced currency code" eg:"CNY"`
	PriceIncludesTax     bool   `json:"priceIncludesTax" dc:"Whether prices already include tax" eg:"true"`
	TaxTreatment         string `json:"taxTreatment" dc:"Tax treatment" eg:"taxable"`
	TaxRateBasisPoints   *int   `json:"taxRateBasisPoints" dc:"Tax rate in basis points, or null for exempt treatment" eg:"600"`
	TaxCategoryCode      string `json:"taxCategoryCode" dc:"Merchant tax classification code" eg:"cloud_service"`
	RoundingScope        string `json:"roundingScope" dc:"Monetary rounding scope" eg:"line"`
	RoundingMode         string `json:"roundingMode" dc:"Monetary rounding mode" eg:"half_up"`
	RoundingQuantumMinor string `json:"roundingQuantumMinor" dc:"Exact rounding quantum in decimal currency minor units" eg:"1"`
}
