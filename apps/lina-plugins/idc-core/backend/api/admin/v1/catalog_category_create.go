// This file defines staff-only catalog category creation.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogCategoryCreateReq struct {
	g.Meta           `path:"/admin/catalog/categories" method:"post" tags:"IDC Catalog Management" summary:"Create catalog category" dc:"Create one active merchant-defined catalog category with transactional audit." permission:"idc:admin:catalog:category:create"`
	ParentCategoryNo string `json:"parentCategoryNo" v:"max-length:36" dc:"Optional stable parent category business number" eg:""`
	NameZhCn         string `json:"nameZhCn" v:"required|max-length:128" dc:"Simplified Chinese category name" eg:"Cloud servers zh-CN"`
	NameEnUs         string `json:"nameEnUs" v:"required|max-length:128" dc:"English category name" eg:"Cloud servers"`
	SortOrder        int    `json:"sortOrder" v:"between:-1000000,1000000" dc:"Stable category display order" eg:"100"`
}

type CatalogCategoryCreateRes struct {
	Category *CatalogCategoryItem `json:"category" dc:"Committed category projection"`
}
