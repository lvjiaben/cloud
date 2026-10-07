// This file defines the bounded staff category-selector endpoint.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogCategoryItem struct {
	CategoryNo       string `json:"categoryNo" dc:"Stable category business number" eg:"ctg_0123456789abcdef0123456789abcdef"`
	ParentCategoryNo string `json:"parentCategoryNo" dc:"Optional stable parent category business number" eg:""`
	NameZhCn         string `json:"nameZhCn" dc:"Simplified Chinese category name" eg:"Cloud servers zh-CN"`
	NameEnUs         string `json:"nameEnUs" dc:"English category name" eg:"Cloud servers"`
	SortOrder        int    `json:"sortOrder" dc:"Stable category display order" eg:"100"`
	Status           string `json:"status" dc:"Stable category status" eg:"active"`
	Version          int64  `json:"version" dc:"Optimistic concurrency version" eg:"1"`
	Deleted          bool   `json:"deleted" dc:"Whether the category is recoverably deleted" eg:"false"`
}

type CatalogCategoryListReq struct {
	g.Meta        `path:"/admin/catalog/categories" method:"get" tags:"IDC Catalog Management" summary:"List catalog categories" dc:"Return at most 500 active or recoverably deleted categories for staff management." permission:"idc:admin:catalog:category:list"`
	DeletionState string `json:"deletionState" v:"in:active,deleted,all" dc:"Deletion state filter; empty means active" eg:"active"`
}

type CatalogCategoryListRes struct {
	Categories []*CatalogCategoryItem `json:"categories" dc:"Bounded active categories" eg:"[]"`
}
