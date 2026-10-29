// This file defines the anonymous general product list endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductListReq struct {
	g.Meta      `path:"/public/catalog/products" method:"get" tags:"IDC Public Catalog" summary:"List published products" dc:"Return one bounded local projection of merchant-defined products without plugin-private fields or internal identifiers."`
	PublicSlugs []string `json:"publicSlugs" in:"query" dc:"Optional exact public product slugs encoded as publicSlugs[]; at most 24 entries, each 1 to 128 characters; invalid values are rejected and omission leaves the slug filter unset" eg:"[\"cloud-basic\",\"cloud-pro\"]"`
	NamePrefix  string   `json:"namePrefix" in:"query" v:"length:0,128" dc:"Literal bilingual name prefix"`
	CategoryNo  string   `json:"categoryNo" in:"query" v:"length:0,40" dc:"Optional enabled category number"`
	Currency    string   `json:"currency" in:"query" v:"regex:^[A-Z]{3}$" dc:"Optional explicit uppercase currency"`
	Cursor      string   `json:"cursor" in:"query" v:"length:0,128" dc:"Opaque filter-bound next cursor"`
	Limit       int      `json:"limit" in:"query" v:"min:0|max:24" dc:"Page size; zero uses the server default"`
}

type CatalogProductListRes struct {
	Products            []*CatalogProductItem  `json:"products"`
	Categories          []*CatalogCategoryItem `json:"categories"`
	AvailableCurrencies []string               `json:"availableCurrencies"`
	NextCursor          string                 `json:"nextCursor"`
}
