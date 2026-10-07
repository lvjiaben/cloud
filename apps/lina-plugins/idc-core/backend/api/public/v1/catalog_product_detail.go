// This file defines the anonymous published general product detail endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogProductDetailReq struct {
	g.Meta     `path:"/public/catalog/products/{publicSlug}" method:"get" tags:"IDC Public Catalog" summary:"Get one published product" dc:"Return the same safe general-commercial projection used by the public list."`
	PublicSlug string `json:"publicSlug" in:"path" v:"required|length:1,128" dc:"Public browser slug"`
}

type CatalogProductDetailRes struct {
	Product *CatalogProductItem `json:"product"`
}
