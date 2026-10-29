// This file defines bounded merchant directory configuration endpoints.
package v1

import "github.com/gogf/gf/v2/frame/g"

// Directory does not grant access to a customer's purchased services.
type Directory struct {
	PagePluginIDs []string `json:"pagePluginIds" dc:"Ordered plugin IDs for product content blocks, including empty lists; maximum 16" eg:"[\"idc-resource\"]"`

	ParentDirectoryNo string `json:"parentDirectoryNo" dc:"Parent menu reference; empty means top level; at most two levels" eg:"pdr_services"`
	NavigationType    string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL         string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon              string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	DirectoryNo       string `json:"directoryNo" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn          string `json:"nameZhCn" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs          string `json:"nameEnUs" dc:"English directory name" eg:"My cloud services"`
	SortOrder         int    `json:"sortOrder" dc:"Navigation order from 0 to 1000000" eg:"10"`
	StatusCode        string `json:"statusCode" dc:"Navigation availability: enabled or disabled" eg:"enabled"`
	Version           int64  `json:"version" dc:"Current optimistic configuration version" eg:"1"`
	ProductCount      int    `json:"productCount" dc:"Number of bound products" eg:"1"`
}
type ListReq struct {
	Tree     bool `json:"tree" dc:"When true, use tree row ordering and return missing parents as context; totals still count actual menu rows" eg:"true"`
	g.Meta   `path:"/admin/product-directories" method:"get" tags:"IDC Product Directories" summary:"List merchant product directories" dc:"Read bounded shared merchant navigation configuration" permission:"idc:admin:product-directory:list"`
	Page     int `json:"page" d:"1" v:"min:1|max:1000000" dc:"Page number; defaults to 1" eg:"1"`
	PageSize int `json:"pageSize" d:"20" v:"min:1|max:100" dc:"Page size from 1 to 100; defaults to 20" eg:"20"`
}
type ListRes struct {
	Parents  []Directory `json:"parents" dc:"Bounded parent context for current tree page; not additional matching rows" eg:"[]"`
	Items    []Directory `json:"items" dc:"Merchant directories on the current page" eg:"[]"`
	Total    int         `json:"total" dc:"Total matching directories" eg:"1"`
	Page     int         `json:"page" dc:"Current page number" eg:"1"`
	PageSize int         `json:"pageSize" dc:"Current page size" eg:"20"`
}
type CreateReq struct {
	PagePluginIDs *[]string `json:"pagePluginIds" dc:"Ordered plugin IDs for product content blocks; maximum 16; omitted preserves existing selection; empty clears" eg:"[\"idc-resource\"]"`

	ParentDirectoryNo string `json:"parentDirectoryNo" v:"max-length:40" dc:"Parent menu reference; empty means top level; at most two levels" eg:"pdr_services"`
	NavigationType    string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL         string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon              string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	g.Meta            `path:"/admin/product-directories" method:"post" tags:"IDC Product Directories" summary:"Create a merchant product directory" dc:"Create enabled navigation without assigning products; assignment is managed by product editing" permission:"idc:admin:product-directory:create"`
	NameZhCn          string `json:"nameZhCn" v:"required|max-length:128" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs          string `json:"nameEnUs" v:"required|max-length:128" dc:"English directory name" eg:"My cloud services"`
	SortOrder         int    `json:"sortOrder" v:"min:0|max:1000000" dc:"Navigation order from 0 to 1000000" eg:"10"`
}
type CreateRes struct {
	Directory *Directory `json:"directory" dc:"Created merchant directory" eg:"{}"`
}
type UpdateReq struct {
	PagePluginIDs *[]string `json:"pagePluginIds" dc:"Ordered plugin IDs for product content blocks; maximum 16; omitted preserves existing selection; empty clears" eg:"[\"idc-resource\"]"`

	ParentDirectoryNo string `json:"parentDirectoryNo" v:"max-length:40" dc:"Parent menu reference; empty means top level; at most two levels" eg:"pdr_services"`
	NavigationType    string `json:"navigationType" dc:"Member destination: product, system, custom or plugin; default product" eg:"product"`
	TargetURL         string `json:"targetUrl" dc:"Internal page path for system or plugin; HTTP or HTTPS URL for custom; empty for product" eg:"/account/overview"`
	Icon              string `json:"icon" dc:"Iconify icon identifier used in member navigation" eg:"lucide:folder"`
	g.Meta            `path:"/admin/product-directories/{directoryNo}" method:"put" tags:"IDC Product Directories" summary:"Update a merchant product directory" dc:"Update navigation metadata at the expected version, preserving all product assignments" permission:"idc:admin:product-directory:update"`
	DirectoryNo       string `json:"directoryNo" in:"path" v:"required|max-length:40" dc:"Opaque merchant directory reference" eg:"pdr_example"`
	NameZhCn          string `json:"nameZhCn" v:"required|max-length:128" dc:"Chinese directory name" eg:"My cloud services"`
	NameEnUs          string `json:"nameEnUs" v:"required|max-length:128" dc:"English directory name" eg:"My cloud services"`
	SortOrder         int    `json:"sortOrder" v:"min:0|max:1000000" dc:"Navigation order from 0 to 1000000" eg:"10"`
	StatusCode        string `json:"statusCode" v:"required|in:enabled,disabled" dc:"Navigation availability: enabled or disabled" eg:"disabled"`
	ExpectedVersion   int64  `json:"expectedVersion" v:"required|min:1" dc:"Expected optimistic configuration version" eg:"1"`
}
type UpdateRes struct {
	Directory *Directory `json:"directory" dc:"Updated merchant directory" eg:"{}"`
}
