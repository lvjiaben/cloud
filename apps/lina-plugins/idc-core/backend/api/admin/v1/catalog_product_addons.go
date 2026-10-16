// This file defines staff-managed allowed add-on offers for one main product.
package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductAddonsGetReq struct {
	g.Meta    `path:"/admin/catalog/products/{productNo}/addons" method:"get" tags:"IDC Catalog Management" summary:"Read allowed product add-ons" dc:"Return the bounded allowed add-on offer set for one visible main product." permission:"idc:admin:catalog:product:query"`
	ProductNo string `json:"productNo" in:"path" v:"required|max-length:128" dc:"Main product business number" eg:"prd_main"`
}
type CatalogProductAddon struct {
	ProductNo string `json:"productNo" dc:"Allowed add-on product number" eg:"prd_backup"`
	NameZhCN  string `json:"nameZhCN" dc:"Simplified Chinese product name" eg:"Backup"`
	NameEnUS  string `json:"nameEnUS" dc:"English product name" eg:"Backup"`
}
type CatalogProductAddonsGetRes struct {
	Addons []*CatalogProductAddon `json:"addons" dc:"At most 100 retained offer associations" eg:"[]"`
}
type CatalogProductAddonsSetReq struct {
	g.Meta     `path:"/admin/catalog/products/{productNo}/addons" method:"put" tags:"IDC Catalog Management" summary:"Set allowed product add-ons" dc:"Replace at most 100 allowed add-on offers with product revision concurrency and audit; paid orders remain unchanged." permission:"idc:admin:catalog:product:update"`
	ProductNo  string   `json:"productNo" in:"path" v:"required|max-length:128" dc:"Main product business number" eg:"prd_main"`
	ProductNos []string `json:"productNos" dc:"Complete allowed offer set; empty removes all future offers" eg:"[]"`
	Version    int64    `json:"version" v:"required|min:1" dc:"Last read optimistic product revision" eg:"1"`
}
type CatalogProductAddonsSetRes struct {
	Version int64 `json:"version" dc:"New optimistic product revision" eg:"2"`
}
