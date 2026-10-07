// This file defines the staff-only catalog product workbench list endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductListReq struct {
	g.Meta     `path:"/admin/catalog/products" method:"get" tags:"IDC Catalog Management" summary:"List catalog products" dc:"Return one bounded keyset page of merchant-defined product summaries with latest version, pricing, and tax projections." permission:"idc:admin:catalog:product:list"`
	NamePrefix string `json:"namePrefix" dc:"Optional literal Simplified Chinese name, English name, or public slug prefix; percent and underscore are not wildcards" eg:"cloud"`
	CategoryNo string `json:"categoryNo" v:"regex:^$|^ctg_[0-9a-f]{32}$" dc:"Optional merchant-defined category number; empty includes all categories" eg:"ctg_0123456789abcdef0123456789abcdef"`
	Status     string `json:"status" dc:"Optional product status filter: draft, active, suspended, or retired; empty includes all" eg:"active"`
	Source     string `json:"source" dc:"Optional latest-version source filter: manual, migration, or copy; empty includes all" eg:"manual"`
	Cursor     string `json:"cursor" v:"max-length:128" dc:"Opaque cursor from the previous response; empty requests the first page" eg:""`
	Page       int    `json:"page" v:"min:0" dc:"Optional one-based page number; cannot be combined with cursor" eg:"1"`
	Limit      int    `json:"limit" d:"25" v:"min:1|max:100" dc:"Maximum products returned in this page" eg:"25"`
}

type CatalogProductListRes struct {
	Products   []*CatalogProductSummaryItem `json:"products" dc:"Bounded product workbench summaries" eg:"[]"`
	NextCursor string                       `json:"nextCursor" dc:"Opaque cursor for the next page, or empty when complete" eg:""`
	Total      int                          `json:"total" dc:"Total records matching the current filters and authorized data scope" eg:"0"`
}
