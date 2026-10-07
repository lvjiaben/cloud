// This file defines staff-only catalog category updates.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogCategoryUpdateReq struct {
	g.Meta           `path:"/admin/catalog/categories/{categoryNo}" method:"put" tags:"IDC Catalog Management" summary:"Update catalog category" dc:"Update one active category with optimistic concurrency and transactional audit." permission:"idc:admin:catalog:category:update"`
	CategoryNo       string `json:"categoryNo" in:"path" v:"required|length:36,36" dc:"Stable category business number"`
	ParentCategoryNo string `json:"parentCategoryNo" v:"max-length:36" dc:"Optional active parent category business number"`
	NameZhCn         string `json:"nameZhCn" v:"required|max-length:128" dc:"Simplified Chinese category name"`
	NameEnUs         string `json:"nameEnUs" v:"required|max-length:128" dc:"English category name"`
	SortOrder        int    `json:"sortOrder" v:"between:-1000000,1000000" dc:"Stable category display order"`
	Version          int64  `json:"version" v:"required|min:1" dc:"Current optimistic concurrency version"`
}

type CatalogCategoryUpdateRes struct {
	Category *CatalogCategoryItem `json:"category" dc:"Committed category projection"`
}
