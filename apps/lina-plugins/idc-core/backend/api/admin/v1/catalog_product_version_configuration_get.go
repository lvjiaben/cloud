// This file defines the frozen product-version configuration read endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductVersionConfigurationGetReq struct {
	g.Meta    `path:"/admin/catalog/product-versions/{versionNo}/configuration" method:"get" tags:"IDC Catalog Management" summary:"Get product version configuration" dc:"Return frozen product and customer declarations with safe merchant values." permission:"idc:admin:catalog:configuration:write"`
	VersionNo string `json:"versionNo" in:"path" v:"required|regex:^ver_[0-9a-f]{32}$" dc:"Product version number" eg:"ver_0123456789abcdef0123456789abcdef"`
}

type CatalogProductVersionConfigurationGetRes struct {
	Configuration *CatalogProductVersionConfiguration `json:"configuration" dc:"Frozen product-version configuration, or null before first save"`
}
