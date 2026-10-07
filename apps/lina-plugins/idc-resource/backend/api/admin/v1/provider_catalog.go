// This file exposes bounded staff-only supplier sales reads. Neither endpoint
// imports, publishes, prices or mutates a local or remote product.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

type ProviderCatalogListReq struct {
	g.Meta      `path:"/admin/resource/provider-configurations/interfaces/{interfaceNo}/catalog-products" method:"get" tags:"IDC Resource Channels" summary:"Read upstream products" dc:"Read a bounded supplier-visible page through the selected interface; does not fetch one detail per row or import products" permission:"idc:admin:resource:configuration:list"`
	InterfaceNo string `json:"interfaceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Target local supplier interface" eg:"pfi_0123456789abcdef0123456789abcdef"`
	Page        int    `json:"page" d:"1" v:"min:1|max:1000000" dc:"One-based page" eg:"1"`
	PageSize    int    `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Maximum rows per page" eg:"20"`
	Keyword     string `json:"keyword" v:"max-length:128" dc:"Product-name search" eg:"Cloud"`
}

type ProviderCatalogListRes struct {
	Items []resourceinterfaceprojectioncap.CatalogProduct `json:"items" dc:"Current page of supplier-authored names and descriptions" eg:"[]"`
	Total int                                             `json:"total" dc:"Exact visible count matching the current filter" eg:"21"`
}

type ProviderCatalogProductReq struct {
	g.Meta      `path:"/admin/resource/provider-configurations/interfaces/{interfaceNo}/catalog-product" method:"get" tags:"IDC Resource Channels" summary:"Read selected upstream product" dc:"Read one explicitly selected supplier product; base prices and configurable-option surcharges remain separate and never imply a complete machine quote" permission:"idc:admin:resource:configuration:list"`
	InterfaceNo string `json:"interfaceNo" in:"path" v:"required|regex:^pfi_[0-9a-f]{32}$" dc:"Target local supplier interface" eg:"pfi_0123456789abcdef0123456789abcdef"`
	Reference   string `json:"reference" v:"required|max-length:256" dc:"Opaque supplier product selection returned by the list" eg:"101"`
}

type ProviderCatalogProductRes struct {
	Item *resourceinterfaceprojectioncap.CatalogDetail `json:"item" dc:"Field-whitelisted selected product and explicit price components" eg:"{}"`
}
