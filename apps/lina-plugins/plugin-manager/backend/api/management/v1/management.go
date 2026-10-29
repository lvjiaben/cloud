// Package v1 defines the permission-governed plugin workspace read APIs.
package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	pluginv1 "lina-core/api/plugin/v1"
)

// Category is one ordered, configurable business tab.
type Category struct {
	ID   string `json:"id" dc:"Stable category key, extensible by configuration" eg:"server"`
	Name string `json:"name" dc:"Category display name" eg:"服务器"`
}

// CategoriesReq reads the categories available to plugin administrators.
type CategoriesReq struct {
	g.Meta `path:"/categories" method:"get" tags:"插件管理" summary:"读取插件分类" dc:"返回按配置排序的业务分类，不含插件内部信息。" permission:"plugin:list"`
}

// CategoriesRes returns ordered categories without client-side classification rules.
type CategoriesRes struct {
	List []Category `json:"list" dc:"Ordered plugin categories" eg:"[]"`
}

// ListReq searches one category; pagination is performed after server-side classification.
type ListReq struct {
	g.Meta   `path:"/plugins" method:"get" tags:"插件管理" summary:"查询分类插件" dc:"在当前员工公开插件管理权限边界内，按业务分类查询动态托管插件摘要；最多聚合1000个插件，不逐行读取详情。" permission:"plugin:list"`
	Category string `json:"category" v:"required|length:1,64" dc:"Category key returned by categories" eg:"server"`
	ID       string `json:"id" v:"length:0,64" dc:"Optional identifier fragment" eg:"zjmf"`
	Name     string `json:"name" v:"length:0,100" dc:"Optional display name fragment" eg:"魔方"`
	PageNum  int    `json:"pageNum" d:"1" v:"min:1" dc:"Page number, default 1" eg:"1"`
	PageSize int    `json:"pageSize" d:"20" v:"between:1,200" dc:"Page size, default 20, maximum 200" eg:"20"`
}

// ListRes retains only the host's public list DTO and an exact filtered total.
type ListRes struct {
	List  []*pluginv1.PluginListItem `json:"list" dc:"Current page of managed dynamic plugins" eg:"[]"`
	Total int                        `json:"total" dc:"Total matching category and search filters" eg:"5"`
}
