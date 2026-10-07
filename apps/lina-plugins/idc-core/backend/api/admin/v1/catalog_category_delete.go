// This file defines recoverable catalog category deletion.

package v1

import "github.com/gogf/gf/v2/frame/g"

type CatalogCategoryDeleteReq struct {
	g.Meta     `path:"/admin/catalog/categories/{categoryNo}" method:"delete" tags:"IDC Catalog Management" summary:"Delete catalog category" dc:"Recoverably delete one active category without erasing product history." permission:"idc:admin:catalog:category:delete"`
	CategoryNo string `json:"categoryNo" in:"path" v:"required|length:36,36" dc:"Stable category business number"`
	Version    int64  `json:"version" v:"required|min:1" dc:"Current optimistic concurrency version"`
}

type CatalogCategoryDeleteRes struct{}
