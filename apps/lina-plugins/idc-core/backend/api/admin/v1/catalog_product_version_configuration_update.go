// This file defines draft-only product-version configuration writes.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductVersionConfigurationUpdateReq struct {
	CustomerFieldSelections []*CatalogCustomerFieldSelection `json:"customerFieldSelections" dc:"Explicitly enabled optional customer fields; omitted disables all opt-in fields"`
	g.Meta                  `path:"/admin/catalog/product-versions/{versionNo}/configuration" method:"put" tags:"IDC Catalog Management" summary:"Update product version configuration" dc:"Validate current exact provider declarations and freeze merchant and customer schemas on a draft product version." permission:"idc:admin:catalog:configuration:write"`
	VersionNo               string                                 `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
	MerchantValues          []*CatalogStructuredConfigurationValue `json:"merchantValues" dc:"Required complete typed merchant configuration array; empty is valid when no merchant fields are declared"`
	ExpectedVersion         int64                                  `json:"expectedVersion" v:"min:0" dc:"Expected configuration version; zero creates the first snapshot" eg:"1"`
}

type CatalogProductVersionConfigurationUpdateRes struct {
	Configuration *CatalogProductVersionConfiguration `json:"configuration" dc:"Updated frozen product-version configuration"`
}
