// This file defines recoverable catalog category restoration.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogCategoryRestoreReq struct {
	g.Meta     `path:"/admin/catalog/categories/{categoryNo}/restore" method:"put" tags:"IDC Catalog Management" summary:"Restore catalog category" dc:"Restore one recoverably deleted category with optimistic concurrency and transactional audit." permission:"idc:admin:catalog:category:restore"`
	CategoryNo string `json:"categoryNo" in:"path" v:"required|length:36,36" dc:"Stable category business number"`
	Version    int64  `json:"version" v:"required|min:1" dc:"Current optimistic concurrency version"`
}

type CatalogCategoryRestoreRes struct {
	Category *CatalogCategoryItem `json:"category" dc:"Restored category projection"`
}
